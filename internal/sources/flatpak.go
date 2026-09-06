package sources

import (
	"bytes"
	"encoding/json"
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

// flatpakMaxSearchResults caps how many flatpak search results are returned.
const flatpakMaxSearchResults = 5

// flatpakFlathubRepoURL is Flathub's official remote definition file.
const flatpakFlathubRepoURL = "https://flathub.org/repo/flathub.flatpakrepo"

// ensureFlathubRemote makes sure the flathub remote is registered before any
// command that depends on it (search, install). --if-not-exists makes this
// idempotent, so it's safe to call unconditionally rather than checking
// `flatpak remotes` first.
func ensureFlathubRemote() error {
	return common.RunQuiet("flatpak", "remote-add", "--if-not-exists", "flathub", flatpakFlathubRepoURL)
}

// flatpakShortName extracts a short launcher name from a flatpak app ID,
// e.g. "org.gimp.GIMP" -> "gimp". Mirrors bash's get_flatpak_shortname.
// Used as a fallback by FlatpakToolName when no clean display name is
// available (e.g. flatpak isn't installed, or the app isn't in the list).
func flatpakShortName(appID string) string {
	parts := strings.Split(appID, ".")
	return strings.ToLower(parts[len(parts)-1])
}

// flatpakCleanNameRe matches display names made up only of letters and
// digits - the only kind safe to use verbatim as a tool/command name.
var flatpakCleanNameRe = regexp.MustCompile(`^[A-Za-z0-9]+$`)

// FlatpakDisplayNames returns every installed flatpak app's canonical
// display name, keyed by app ID (flatpak list --app --columns=application,name).
// Returns an empty map if flatpak isn't installed or the command fails.
func FlatpakDisplayNames() map[string]string {
	names := make(map[string]string)
	if _, err := exec.LookPath("flatpak"); err != nil {
		return names
	}

	var output bytes.Buffer
	cmd := exec.Command("flatpak", "list", "--app", "--columns=application,name")
	cmd.Stdout = &output
	if cmd.Run() != nil {
		return names
	}

	for _, line := range strings.Split(output.String(), "\n") {
		fields := strings.Split(line, "\t")
		if len(fields) < 2 {
			continue
		}
		appID := strings.TrimSpace(fields[0])
		name := strings.TrimSpace(fields[1])
		if appID != "" && name != "" {
			names[appID] = name
		}
	}
	return names
}

// FlatpakToolName picks the tool/manifest name for appID: its canonical
// display name (from displayNames, as returned by FlatpakDisplayNames),
// lowercased, if that name is plain alphanumeric - otherwise falls back to
// flatpakShortName's ID-derived name. This avoids picking up spaces,
// parentheses, etc. from display names like "Google Chrome (unstable)",
// while fixing IDs whose last segment is a generic word rather than the
// app name (e.g. "org.telegram.desktop" -> "telegram", not "desktop").
func FlatpakToolName(appID string, displayNames map[string]string) string {
	if name, ok := displayNames[appID]; ok && flatpakCleanNameRe.MatchString(name) {
		return strings.ToLower(name)
	}
	return flatpakShortName(appID)
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
	displayNames := FlatpakDisplayNames()

	if entries, err := manifest.All(); err == nil {
		for _, e := range entries {
			if manifest.GetSourceType(e.Source) != common.SourceFlatpak {
				continue
			}
			appID := manifest.GetSourceIdentity(e.Source)
			if appID == "" || !installed[appID] {
				continue
			}

			natural := FlatpakToolName(appID, displayNames)
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
	if err := ensureFlathubRemote(); err != nil {
		return nil, fmt.Errorf("failed to configure flathub remote: %w", err)
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

		// Limit to flatpakMaxSearchResults
		if len(results) >= flatpakMaxSearchResults {
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
	if err := ensureFlathubRemote(); err != nil {
		return "", "", fmt.Errorf("failed to configure flathub remote: %w", err)
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

	return FlatpakToolName(appID, FlatpakDisplayNames()), appID, nil
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

// FlatpakUpdateInfo is one ref (app or runtime) flatpak itself reports as
// having a pending update. Branch disambiguates refs that share an AppID
// but have multiple branches installed side by side (e.g. a GL driver
// runtime installed for two SDK versions at once); Name is flatpak's own
// display name, used for refs with no sat-tracked tool name.
type FlatpakUpdateInfo struct {
	AppID   string
	Version string // latest/target version; blank for refs with no version metadata
	Branch  string
	Name    string
}

// flatpakRemoteUpdates runs the actual flatpak query behind
// FlatpakListUpdates - split out so a stale-appstream retry can call it a
// second time without re-running the LookPath guard.
func flatpakRemoteUpdates() ([]FlatpakUpdateInfo, error) {
	var output bytes.Buffer
	cmd := exec.Command("flatpak", "remote-ls", "--updates", "--columns=application,version,branch,name")
	cmd.Stdout = &output
	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("failed to query flatpak updates")
	}

	var updates []FlatpakUpdateInfo
	for _, line := range strings.Split(strings.TrimRight(output.String(), "\n"), "\n") {
		if line == "" {
			continue
		}
		fields := strings.Split(line, "\t")
		if len(fields) < 4 {
			continue
		}
		updates = append(updates, FlatpakUpdateInfo{
			AppID:   strings.TrimSpace(fields[0]),
			Version: strings.TrimSpace(fields[1]),
			Branch:  strings.TrimSpace(fields[2]),
			Name:    strings.TrimSpace(fields[3]),
		})
	}
	return updates, nil
}

// flatpakUpdatesLookAmbiguous reports whether any update in updates carries
// no usable "latest" version - either no version string at all, or one
// identical to what's already installed. Both are symptoms of a stale
// local appstream cache (see FlatpakListUpdates), which is worth paying to
// refresh; a normal batch of clean version bumps never triggers this, so
// FlatpakInstalledVersions (a second flatpak invocation) is only ever
// called on the suspect path.
func flatpakUpdatesLookAmbiguous(updates []FlatpakUpdateInfo) bool {
	installed := FlatpakInstalledVersions()
	for _, u := range updates {
		if u.Version == "" || u.Version == installed[u.AppID+"//"+u.Branch] {
			return true
		}
	}
	return false
}

// FlatpakListUpdates asks flatpak for every ref (app or runtime) with a
// pending update in one non-destructive call, instead of sat polling each
// tracked app's remote individually. This is what lets sat surface
// runtime updates (which sat never tracks in its own manifest) alongside
// tracked-app updates without reimplementing flatpak's own update
// resolution - a ref's presence in this list is flatpak's own
// authoritative "needs updating" signal, not just a version-string diff.
//
// flatpak's local appstream cache (the metadata remote-ls reads from) can
// go stale, in which case it echoes back the installed version as the
// latest one instead of the real pending version. When that's detected,
// this refreshes the cache (`flatpak update --appstream`, ~10s) and
// re-queries once; a refresh failure just falls back to the original,
// possibly-ambiguous result rather than erroring out, since the refresh is
// an enrichment, not a hard dependency.
//
// Returns nil, nil if flatpak isn't installed.
func FlatpakListUpdates() ([]FlatpakUpdateInfo, error) {
	if _, err := exec.LookPath("flatpak"); err != nil {
		return nil, nil
	}

	updates, err := flatpakRemoteUpdates()
	if err != nil || len(updates) == 0 {
		return updates, err
	}

	if !flatpakUpdatesLookAmbiguous(updates) {
		return updates, nil
	}

	if os.Getenv(common.EnvSATDebug) != "" {
		fmt.Fprintf(os.Stderr, "%s flatpak: stale appstream cache detected, refreshing\n", common.DebugPrefix)
	}

	if err := common.RunQuiet("flatpak", "update", "--appstream"); err != nil {
		if os.Getenv(common.EnvSATDebug) != "" {
			fmt.Fprintf(os.Stderr, "%s flatpak: appstream refresh failed: %v\n", common.DebugPrefix, err)
		}
		return updates, nil
	}

	if refreshed, err := flatpakRemoteUpdates(); err == nil {
		return refreshed, nil
	}
	return updates, nil
}

// FlatpakInstalledVersions returns the installed version of every flatpak
// ref (app or runtime), keyed by "appID//branch". Unlike FlatpakGetVersion
// (apps only, via --app), this covers runtimes too in one call.
func FlatpakInstalledVersions() map[string]string {
	versions := make(map[string]string)

	var output bytes.Buffer
	cmd := exec.Command("flatpak", "list", "--columns=application,branch,version")
	cmd.Stdout = &output
	if cmd.Run() != nil {
		return versions
	}

	for _, line := range strings.Split(output.String(), "\n") {
		fields := strings.Split(line, "\t")
		if len(fields) < 3 {
			continue
		}
		appID := strings.TrimSpace(fields[0])
		branch := strings.TrimSpace(fields[1])
		if appID == "" {
			continue
		}
		versions[appID+"//"+branch] = strings.TrimSpace(fields[2])
	}
	return versions
}

// FlatpakUpdateRefs updates multiple flatpak refs in one batched call.
// Used for refs sat doesn't track in its own manifest (runtimes) - there
// is no per-tool manifest entry to route through FlatpakUpdate for these.
func FlatpakUpdateRefs(refs []string) error {
	if len(refs) == 0 {
		return nil
	}
	args := append([]string{"update", "-y"}, refs...)
	return common.RunQuiet("flatpak", args...)
}

// flatpakSearchResponse is the Flathub search result set.
type flatpakSearchResponse struct {
	Hits []struct {
		AppID   string `json:"app_id"`
		Name    string `json:"name"`
		Summary string `json:"summary"`
		Type    string `json:"type"`
	} `json:"hits"`
}

// flatpakAppstreamResponse carries the lifecycle and link metadata that
// only the per-app endpoint exposes.
type flatpakAppstreamResponse struct {
	IsEOL bool              `json:"is_eol"`
	URLs  map[string]string `json:"urls"`
}

// FlatpakLookup resolves an exact application name against Flathub.
//
// Flathub is keyed by reverse-DNS app id, so an exact match means either
// the display name or the id's final segment - "discord" must find
// com.discordapp.Discord. Flathub apps are GUI applications that always
// install something launchable, so BinsKnown stays false rather than
// claiming a binary list this API does not provide.
func FlatpakLookup(name string) (LookupResult, error) {
	body, err := json.Marshal(map[string]string{"query": name})
	if err != nil {
		return LookupResult{}, err
	}

	data, err := common.PostJSON("https://flathub.org/api/v2/search", body, "flathub lookup")
	if err != nil {
		return LookupResult{}, ErrNoMatch
	}

	var search flatpakSearchResponse
	if err := json.Unmarshal(data, &search); err != nil {
		return LookupResult{}, ErrNoMatch
	}

	for _, hit := range search.Hits {
		leaf := hit.AppID
		if idx := strings.LastIndex(leaf, "."); idx != -1 {
			leaf = leaf[idx+1:]
		}
		if !strings.EqualFold(hit.Name, name) && !strings.EqualFold(leaf, name) {
			continue
		}

		result := LookupResult{
			Name:        hit.AppID,
			Description: hit.Summary,
		}
		if hit.Type != "" {
			result.DesktopAppKnown = true
			result.IsDesktopApp = hit.Type == "desktop-application"
		}

		appData, err := common.FetchJSON("https://flathub.org/api/v2/appstream/"+hit.AppID, "flathub appstream")
		if err != nil {
			return result, nil
		}
		var app flatpakAppstreamResponse
		if err := json.Unmarshal(appData, &app); err != nil {
			return result, nil
		}

		result.Homepage = app.URLs["homepage"]
		if repo := app.URLs["vcs-browser"]; repo != "" {
			result.Repo = repo
		}
		if app.IsEOL {
			result.Dead, result.DeadReason = true, "end of life"
		}
		return result, nil
	}

	return LookupResult{}, ErrNoMatch
}
