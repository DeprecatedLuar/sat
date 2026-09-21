package common

import (
	"fmt"
	"os"
	"path/filepath"
)

// ResolveBinName returns the name to use for a symlink in LocalBin(): name
// if nothing occupies it, or if it's already a symlink into ownerDir (a
// reinstall/update of the same install). Otherwise something else owns the
// name - another source's install or a user's own file - so it falls back
// to name-suffix and warns, rather than silently overwriting it. Errors if
// the suffixed name is taken by a foreign entry too.
func ResolveBinName(name, suffix, ownerDir string) (string, error) {
	if BinNameAvailable(name, ownerDir) {
		return name, nil
	}

	suffixed := name + "-" + suffix
	if !BinNameAvailable(suffixed, ownerDir) {
		return "", fmt.Errorf("%s and %s in %s are both taken by other files", name, suffixed, LocalBin())
	}

	fmt.Fprintf(os.Stderr, "sat: warning: %s already exists in %s, installing as %s\n", name, LocalBin(), suffixed)
	return suffixed, nil
}

// LinkBin points LocalBin()/name at target, replacing a previous link left
// by the same install (a no-op if it already points there). Callers resolve
// name through ResolveBinName first so this never clobbers something foreign.
func LinkBin(name, target string) error {
	if err := os.MkdirAll(LocalBin(), 0755); err != nil {
		return fmt.Errorf("failed to create %s: %w", LocalBin(), err)
	}

	linkPath := filepath.Join(LocalBin(), name)
	if current, err := os.Readlink(linkPath); err == nil && current == target {
		return nil
	}
	if err := os.Remove(linkPath); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("failed to replace %s: %w", linkPath, err)
	}
	if err := os.Symlink(target, linkPath); err != nil {
		return fmt.Errorf("failed to create symlink %s: %w", linkPath, err)
	}
	return nil
}

// UnlinkBin removes LocalBin()/name only if it's a symlink into ownerDir,
// so uninstalling one source's tool never deletes another source's link
// that took over the same name. A missing link is not an error.
func UnlinkBin(name, ownerDir string) error {
	linkPath := filepath.Join(LocalBin(), name)
	if !linksInto(linkPath, ownerDir) {
		return nil
	}
	if err := os.Remove(linkPath); err != nil && !os.IsNotExist(err) {
		return err
	}
	return nil
}

// BinNameAvailable reports whether LocalBin()/name is free to use: absent,
// or a symlink into ownerDir. Silent, for callers that only need to probe.
func BinNameAvailable(name, ownerDir string) bool {
	linkPath := filepath.Join(LocalBin(), name)
	if _, err := os.Lstat(linkPath); os.IsNotExist(err) {
		return true
	}
	return linksInto(linkPath, ownerDir)
}

// linksInto reports whether path is a symlink whose target lives directly
// inside dir.
func linksInto(path, dir string) bool {
	target, err := os.Readlink(path)
	if err != nil {
		return false
	}
	return filepath.Dir(filepath.Clean(target)) == filepath.Clean(dir)
}
