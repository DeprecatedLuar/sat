package sources

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/DeprecatedLuar/sat/internal/common"
	"github.com/DeprecatedLuar/sat/internal/manifest"
)

// flatpakWrapperScript is the template for a flatpak launcher wrapper,
// mirroring bash's create_flatpak_wrapper (lib/sources/flatpak.sh).
const flatpakWrapperScript = "#!/usr/bin/env bash\nexec flatpak run %s \"$@\"\n"

// flatpakWrapperAppIDRe extracts the app_id a wrapper script execs, so
// callers can find "the wrapper for this app_id" without recomputing its
// name (which may have been suffixed by resolveWrapperName to avoid a
// collision).
var flatpakWrapperAppIDRe = regexp.MustCompile(`exec flatpak run (\S+)`)

// flatpakShortName extracts a short launcher name from a flatpak app ID,
// e.g. "org.gimp.GIMP" -> "gimp". Mirrors bash's get_flatpak_shortname.
func flatpakShortName(appID string) string {
	parts := strings.Split(appID, ".")
	return strings.ToLower(parts[len(parts)-1])
}

// resolveWrapperName returns a wrapper name guaranteed not to collide with
// an existing $PATH binary or another app's wrapper: candidate if free,
// otherwise candidate-2, candidate-3, ... Collisions are a rare edge case,
// so a numeric suffix is used rather than a smarter rename.
func resolveWrapperName(candidate string) string {
	name := candidate
	for i := 2; ; i++ {
		if _, err := exec.LookPath(name); err != nil {
			if _, err := os.Stat(filepath.Join(common.FlatpakWrapperDir(), name)); os.IsNotExist(err) {
				return name
			}
		}
		name = fmt.Sprintf("%s-%d", candidate, i)
	}
}

// createFlatpakWrapper writes a launcher wrapper script for appID at
// common.FlatpakWrapperDir()/name and symlinks it into common.LocalBin().
func createFlatpakWrapper(name, appID string) error {
	dir := common.FlatpakWrapperDir()
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create flatpak wrapper directory: %w", err)
	}

	scriptPath := filepath.Join(dir, name)
	script := fmt.Sprintf(flatpakWrapperScript, appID)
	if err := os.WriteFile(scriptPath, []byte(script), 0755); err != nil {
		return fmt.Errorf("failed to write flatpak wrapper: %w", err)
	}

	symlinkPath := filepath.Join(common.LocalBin(), name)
	os.Remove(symlinkPath)
	if err := os.Symlink(scriptPath, symlinkPath); err != nil {
		return fmt.Errorf("failed to symlink flatpak wrapper: %w", err)
	}

	return nil
}

// removeFlatpakWrapper deletes a launcher wrapper's script and symlink.
// Matches bash's rm -f semantics: missing files are not an error.
func removeFlatpakWrapper(name string) error {
	os.Remove(filepath.Join(common.FlatpakWrapperDir(), name))
	os.Remove(filepath.Join(common.LocalBin(), name))
	return nil
}

// EnsureFlatpakWrapper makes sure a launcher wrapper exists for appID,
// creating one (collision-safe) if it doesn't. Idempotent: safe to call
// from install, scan, and selfheal reconciliation alike. Returns the
// wrapper's name, which may differ from flatpakShortName(appID) if a
// collision required a suffix.
func EnsureFlatpakWrapper(appID string) (name string, err error) {
	if name, ok := findWrapperForAppID(appID); ok {
		return name, nil
	}

	name = resolveWrapperName(flatpakShortName(appID))
	if err := createFlatpakWrapper(name, appID); err != nil {
		return "", err
	}
	return name, nil
}

// findWrapperForAppID scans common.FlatpakWrapperDir() for the wrapper
// script that execs appID, returning its name. Matching by script content
// (rather than recomputing the name from appID) is necessary because
// resolveWrapperName may have suffixed the wrapper away from its natural
// short name to avoid a collision.
func findWrapperForAppID(appID string) (name string, ok bool) {
	entries, err := os.ReadDir(common.FlatpakWrapperDir())
	if err != nil {
		return "", false
	}

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		data, err := os.ReadFile(filepath.Join(common.FlatpakWrapperDir(), entry.Name()))
		if err != nil {
			continue
		}
		if m := flatpakWrapperAppIDRe.FindStringSubmatch(string(data)); len(m) > 1 && m[1] == appID {
			return entry.Name(), true
		}
	}

	return "", false
}

// listInstalledFlatpakAppIDs returns every currently installed flatpak app
// ID (flatpak list --app --columns=application), or nil if flatpak isn't
// installed or the command fails.
func listInstalledFlatpakAppIDs() []string {
	if _, err := exec.LookPath("flatpak"); err != nil {
		return nil
	}

	var output bytes.Buffer
	cmd := exec.Command("flatpak", "list", "--app", "--columns=application")
	cmd.Stdout = &output
	if cmd.Run() != nil {
		return nil
	}

	var ids []string
	for _, line := range strings.Split(output.String(), "\n") {
		line = strings.TrimSpace(line)
		if line != "" {
			ids = append(ids, line)
		}
	}
	return ids
}

// ReconcileWrappers keeps flatpak launcher wrappers in sync on every sat
// invocation: creates a missing wrapper for any manifest-tracked app that
// is also currently installed, backfills any manifest entry still missing
// its recorded version, and removes wrappers for apps no longer installed.
// The "also currently installed" check matters: a manifest entry alone
// (e.g. an app removed via `flatpak uninstall` outside sat, or drifted
// from another machine) isn't enough reason to create a wrapper that would
// just fail at runtime - and without it, a wrapper created purely from the
// manifest would be immediately removed again by the very next step below,
// silently churning every run. No-ops entirely if flatpak isn't installed.
// This is the single entrypoint selfheal calls; all decision logic lives
// here rather than in the caller.
func ReconcileWrappers() (created, pruned int) {
	if _, err := exec.LookPath("flatpak"); err != nil {
		return 0, 0
	}

	installed := make(map[string]bool)
	for _, id := range listInstalledFlatpakAppIDs() {
		installed[id] = true
	}

	if entries, err := manifest.All(); err == nil {
		for _, e := range entries {
			if manifest.GetSourceType(e.Source) != common.SourceFlatpak {
				continue
			}
			appID := manifest.GetSourceIdentity(e.Source)
			if appID == "" || !installed[appID] {
				continue
			}

			natural := flatpakShortName(appID)
			if name, ok := findWrapperForAppID(appID); !ok {
				if createFlatpakWrapper(resolveWrapperName(natural), appID) == nil {
					created++
				}
			} else if name != natural && resolveWrapperName(natural) == natural {
				// The wrapper is stuck on a collision-suffixed name (e.g.
				// "flatseal-2") from when something else occupied the
				// natural name. That collision is gone now - resolving the
				// natural name comes back free - so promote the wrapper to
				// it instead of leaving it suffixed forever.
				if createFlatpakWrapper(natural, appID) == nil {
					removeFlatpakWrapper(name)
				}
			}

			if manifest.GetSourceVersion(e.Source) == "" {
				if version := FlatpakGetVersion(appID); version != "" {
					manifest.Add(e.Tool, manifest.BuildSourceString(common.SourceFlatpak, appID, version))
				}
			}
		}
	}

	wrapperEntries, err := os.ReadDir(common.FlatpakWrapperDir())
	if err != nil {
		return created, pruned
	}
	for _, entry := range wrapperEntries {
		if entry.IsDir() {
			continue
		}
		data, err := os.ReadFile(filepath.Join(common.FlatpakWrapperDir(), entry.Name()))
		if err != nil {
			continue
		}
		m := flatpakWrapperAppIDRe.FindStringSubmatch(string(data))
		if len(m) < 2 || installed[m[1]] {
			continue
		}
		removeFlatpakWrapper(entry.Name())
		pruned++
	}

	return created, pruned
}

// FlatpakSearch searches Flathub
func FlatpakSearch(query string) ([]string, error) {
	// Check if flatpak is available
	if _, err := exec.LookPath("flatpak"); err != nil {
		return nil, nil
	}

	cmd := exec.Command("flatpak", "search", query)
	var output bytes.Buffer
	cmd.Stdout = &output
	cmd.Stderr = nil

	if err := cmd.Run(); err != nil {
		return nil, nil
	}

	// Parse flatpak search output (tab-separated: Name, Description, App ID, Version, Branch, Remotes)
	lines := strings.Split(output.String(), "\n")
	results := []string{}
	seen := make(map[string]bool)

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		// Skip header line
		if strings.HasPrefix(line, "Name") {
			continue
		}

		fields := strings.Split(line, "\t")
		if len(fields) < 4 {
			continue
		}

		// Extract fields
		// name := fields[0]
		desc := fields[1]
		appID := fields[2]
		version := fields[3]

		// Deduplicate by app ID
		if seen[appID] {
			continue
		}
		seen[appID] = true

		// Filter out plugins/extensions unless exact match
		queryLower := strings.ToLower(query)
		appIDLower := strings.ToLower(appID)
		if (strings.Contains(appIDLower, ".plugin.") ||
			strings.Contains(appIDLower, ".addon.") ||
			strings.Contains(appIDLower, ".extension.")) &&
			!strings.Contains(appIDLower, queryLower) {
			continue
		}

		result := fmt.Sprintf("%s %s - %s", appID, version, desc)
		results = append(results, result)

		// Limit to 5 results
		if len(results) >= 5 {
			break
		}
	}

	return results, nil
}

// FlatpakInstall installs a flatpak app from Flathub and creates a launcher
// wrapper for it. tool may be a full app ID (two-plus dots, e.g.
// "org.gimp.GIMP") or a search term, in which case the top FlatpakSearch
// hit is used - mirroring bash's install_flatpak.
func FlatpakInstall(tool string) (binName, appID string, err error) {
	if _, err := exec.LookPath("flatpak"); err != nil {
		return "", "", fmt.Errorf("flatpak not installed")
	}

	if strings.Count(tool, ".") >= 2 {
		appID = tool
	} else {
		results, err := FlatpakSearch(tool)
		if err != nil || len(results) == 0 {
			return "", "", fmt.Errorf("no flatpak match found for %s", tool)
		}
		appID = strings.Fields(results[0])[0]
	}

	if err := common.RunQuiet("flatpak", "install", "-y", "flathub", appID); err != nil {
		return "", "", fmt.Errorf("flatpak install failed")
	}

	// EnsureFlatpakWrapper's returned name is purely the wrapper file's
	// on-disk name, which may be collision-suffixed (e.g. "flatseal-2" if
	// "flatseal" is already some unrelated PATH binary) - that's a wrapper
	// concern only and must never become the tool's identity in sat. The
	// manifest key (and everything the user types/sees - sat update
	// flatseal, sat list, ...) always stays the plain derived name.
	if _, err := EnsureFlatpakWrapper(appID); err != nil {
		return "", "", err
	}

	return flatpakShortName(appID), appID, nil
}

// FlatpakUninstall removes a flatpak app and its launcher wrapper. Prefers
// the app ID recorded in the manifest, falling back to a case-insensitive
// match against installed apps, then to pkg itself - mirroring bash's
// uninstall_flatpak try-then-fallback order.
func FlatpakUninstall(pkg, sourceStr string) error {
	appID := manifest.GetSourceIdentity(sourceStr)
	if appID == "" {
		for _, id := range listInstalledFlatpakAppIDs() {
			if strings.Contains(strings.ToLower(id), strings.ToLower(pkg)) {
				appID = id
				break
			}
		}
	}
	if appID == "" {
		appID = pkg
	}

	if err := common.RunQuiet("flatpak", "uninstall", "-y", appID); err != nil {
		return fmt.Errorf("flatpak uninstall failed")
	}

	if name, ok := findWrapperForAppID(appID); ok {
		removeFlatpakWrapper(name)
	}

	return nil
}

// FlatpakUpdate updates an installed flatpak app. appID is the identity
// recorded in the manifest (source:appID:version).
func FlatpakUpdate(tool, appID string) error {
	if appID == "" {
		return fmt.Errorf("no app ID recorded for %s", tool)
	}
	return common.RunQuiet("flatpak", "update", "-y", appID)
}

// FlatpakGetVersion returns the installed version of a flatpak app.
func FlatpakGetVersion(appID string) string {
	var output bytes.Buffer
	cmd := exec.Command("flatpak", "list", "--app", "--columns=application,version")
	cmd.Stdout = &output
	if cmd.Run() != nil {
		return ""
	}

	for _, line := range strings.Split(output.String(), "\n") {
		fields := strings.Fields(line)
		if len(fields) >= 2 && fields[0] == appID {
			return fields[1]
		}
	}
	return ""
}

// flatpakRemoteVersionRe extracts the "Version:" line from
// flatpak remote-info output.
var flatpakRemoteVersionRe = regexp.MustCompile(`(?m)^\s*Version:\s*(\S+)`)

// FlatpakCheckOutdated checks if a flatpak app has updates available on
// Flathub. appID is the identity recorded in the manifest.
func FlatpakCheckOutdated(tool, appID string) (current, latest string, err error) {
	if _, err := exec.LookPath("flatpak"); err != nil {
		return "", "", fmt.Errorf("flatpak not installed")
	}
	if appID == "" {
		return "", "", fmt.Errorf("no app ID recorded for %s", tool)
	}

	if sourceStr := manifest.Get(tool); sourceStr != "" {
		current = manifest.GetSourceVersion(sourceStr)
	}
	if current == "" {
		current = FlatpakGetVersion(appID)
	}
	if current == "" {
		return "", "", fmt.Errorf("app not installed")
	}

	var output bytes.Buffer
	cmd := exec.Command("flatpak", "remote-info", "flathub", appID)
	cmd.Stdout = &output
	if cmd.Run() != nil {
		return "", "", fmt.Errorf("failed to query flathub")
	}

	m := flatpakRemoteVersionRe.FindStringSubmatch(output.String())
	if len(m) < 2 {
		return "", "", fmt.Errorf("failed to parse latest version")
	}
	latest = m[1]

	if current == latest {
		return "", "", fmt.Errorf("already up to date")
	}

	return current, latest, nil
}
