package common

import (
	"fmt"
	"os"
	"path/filepath"
)

const (
	appImageCollisionSuffix = "appimage"
	flatpakCollisionSuffix  = "flatpak"
	huberCollisionSuffix    = "gh"

	binFileMode = 0111
)

// binOwner is a sat-owned directory of stored binaries and the suffix its
// links carry in LocalBin() when the natural name is taken.
type binOwner struct {
	dir    string
	suffix string
}

// binOwners returns every sat-owned binary directory. Reconcile treats all
// entries identically.
func binOwners() []binOwner {
	return []binOwner{
		{AppImagesDir(), appImageCollisionSuffix},
		{FlatpakWrapperDir(), flatpakCollisionSuffix},
		{HuberBinDir(), huberCollisionSuffix},
	}
}

// ownerSuffix returns the collision suffix of the owner directory dir.
func ownerSuffix(dir string) (string, bool) {
	dir = filepath.Clean(dir)
	for _, o := range binOwners() {
		if filepath.Clean(o.dir) == dir {
			return o.suffix, true
		}
	}
	return "", false
}

// binLink is a symlink in LocalBin() with its absolute target.
type binLink struct {
	name   string
	target string
}

// localBinLinks lists every symlink in LocalBin().
func localBinLinks() ([]binLink, error) {
	entries, err := os.ReadDir(LocalBin())
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}

	var links []binLink
	for _, e := range entries {
		if e.Type()&os.ModeSymlink == 0 {
			continue
		}
		target, err := os.Readlink(filepath.Join(LocalBin(), e.Name()))
		if err != nil {
			return nil, err
		}
		if !filepath.IsAbs(target) {
			target = filepath.Join(LocalBin(), target)
		}
		links = append(links, binLink{name: e.Name(), target: filepath.Clean(target)})
	}
	return links, nil
}

// onPathElsewhere returns the path of name in a $PATH directory other than
// LocalBin() and the sat-owned directories, or "" if there is none. Every
// directory is checked, not only the first hit.
func onPathElsewhere(name string) string {
	skip := map[string]bool{filepath.Clean(LocalBin()): true}
	for _, o := range binOwners() {
		skip[filepath.Clean(o.dir)] = true
	}

	for _, dir := range filepath.SplitList(os.Getenv("PATH")) {
		if dir == "" {
			continue
		}
		dir = filepath.Clean(dir)
		if skip[dir] {
			continue
		}
		skip[dir] = true
		candidate := filepath.Join(dir, name)
		if _, err := os.Stat(candidate); err == nil {
			return candidate
		}
	}
	return ""
}

// occupant returns what blocks name from being linked to target: a binary
// elsewhere on $PATH, or a LocalBin() entry that is not already a link to
// target. Empty means the name is free.
func occupant(name, target string) string {
	if p := onPathElsewhere(name); p != "" {
		return p
	}
	slot := filepath.Join(LocalBin(), name)
	if _, err := os.Lstat(slot); err != nil {
		return ""
	}
	if current, err := os.Readlink(slot); err == nil && filepath.Clean(current) == target {
		return ""
	}
	return slot
}

// resolveBinName returns natural if free, otherwise natural-suffix if free.
// Errors naming the occupants if both are taken.
func resolveBinName(natural, target, suffix string) (string, error) {
	blocker := occupant(natural, target)
	if blocker == "" {
		return natural, nil
	}
	suffixed := natural + "-" + suffix
	if other := occupant(suffixed, target); other != "" {
		return "", fmt.Errorf("%s is taken by %s and %s by %s", natural, blocker, suffixed, other)
	}
	return suffixed, nil
}

// LinkBin links LocalBin()/<name> to target, the single authority for link
// names. The natural name is basename(target); the owner is dirname(target).
// Every other link to target is removed, so a rename leaves one link. Returns
// the name used.
func LinkBin(target string) (string, error) {
	target = filepath.Clean(target)
	suffix, ok := ownerSuffix(filepath.Dir(target))
	if !ok {
		return "", fmt.Errorf("%s is not in a sat-owned directory", target)
	}
	natural := filepath.Base(target)

	name, err := resolveBinName(natural, target, suffix)
	if err != nil {
		return "", err
	}
	if err := os.MkdirAll(LocalBin(), 0755); err != nil {
		return "", fmt.Errorf("failed to create %s: %w", LocalBin(), err)
	}

	links, err := localBinLinks()
	if err != nil {
		return "", err
	}
	for _, l := range links {
		if l.target == target && l.name != name {
			if err := os.Remove(filepath.Join(LocalBin(), l.name)); err != nil && !os.IsNotExist(err) {
				return "", fmt.Errorf("failed to remove %s: %w", l.name, err)
			}
		}
	}

	linkPath := filepath.Join(LocalBin(), name)
	if current, err := os.Readlink(linkPath); err == nil && filepath.Clean(current) == target {
		return name, nil
	}
	if name != natural {
		fmt.Fprintf(os.Stderr, "sat: warning: %s is already on your PATH, linking as %s\n", natural, name)
	}
	if err := os.Remove(linkPath); err != nil && !os.IsNotExist(err) {
		return "", fmt.Errorf("failed to replace %s: %w", linkPath, err)
	}
	if err := os.Symlink(target, linkPath); err != nil {
		return "", fmt.Errorf("failed to create symlink %s: %w", linkPath, err)
	}
	return name, nil
}

// UnlinkBin removes every LocalBin() link that resolves to target, whatever
// its name. Links to anything else are left alone; none found is not an error.
func UnlinkBin(target string) error {
	target = filepath.Clean(target)
	links, err := localBinLinks()
	if err != nil {
		return err
	}
	for _, l := range links {
		if l.target != target {
			continue
		}
		if err := os.Remove(filepath.Join(LocalBin(), l.name)); err != nil && !os.IsNotExist(err) {
			return err
		}
	}
	return nil
}

// ReconcileBinLinks brings every sat-owned link to its desired state: links
// whose stored binary is gone are removed, and every stored binary is linked
// under its resolved name. Failures warn per link.
func ReconcileBinLinks() {
	removeDanglingLinks()
	linkStoredBinaries()
}

func warnLink(what string, err error) {
	fmt.Fprintf(os.Stderr, "sat: warning: bin link %s: %v\n", what, err)
}

// removeDanglingLinks removes links into a sat-owned directory whose target
// no longer exists.
func removeDanglingLinks() {
	links, err := localBinLinks()
	if err != nil {
		warnLink(LocalBin(), err)
		return
	}
	for _, l := range links {
		if _, ok := ownerSuffix(filepath.Dir(l.target)); !ok {
			continue
		}
		if _, err := os.Stat(l.target); !os.IsNotExist(err) {
			continue
		}
		if err := os.Remove(filepath.Join(LocalBin(), l.name)); err != nil && !os.IsNotExist(err) {
			warnLink(l.name, err)
		}
	}
}

// linkStoredBinaries links every executable file in every sat-owned directory.
func linkStoredBinaries() {
	for _, o := range binOwners() {
		entries, err := os.ReadDir(o.dir)
		if err != nil {
			continue
		}
		for _, e := range entries {
			path := filepath.Join(filepath.Clean(o.dir), e.Name())
			info, err := os.Stat(path)
			if err != nil || info.IsDir() || info.Mode()&binFileMode == 0 {
				continue
			}
			if _, err := LinkBin(path); err != nil {
				warnLink(e.Name(), err)
			}
		}
	}
}
