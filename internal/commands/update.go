package commands

import (
	"bufio"
	"fmt"
	"os"
	"sort"
	"strings"
	"sync"

	"github.com/DeprecatedLuar/sat/internal/common"
	"github.com/DeprecatedLuar/sat/internal/drift"
	"github.com/DeprecatedLuar/sat/internal/manifest"
	"github.com/DeprecatedLuar/sat/internal/sources"
	"github.com/DeprecatedLuar/sat/internal/ui"
)

const updateUsage = "usage: sat update [<tool> ...] [--cargo|--brew|--nix|--apt|--gh|--appimage|--flatpak|--npm|--uv|--go|--sat] [-y|--yes]"

// selfToolName is how sat lists itself among the update candidates. sat has
// no manifest entry of its own, so its outdated row is synthesized during the
// scan rather than read from manifest.All().
const selfToolName = "sat"

// Source-type aliases recognized alongside their canonical common.Source*
// constants (older manifests / scan output may still record these).
const (
	sourceAliasRust   = "rust"
	sourceAliasNixOS  = "nixos"
	sourceAliasGitHub = "github"
)

// HandleUpdate routes between self-update and the outdated-scan flow, which
// serves bare, named and source-filtered updates alike.
func HandleUpdate(args []string, version, repo string) error {
	if len(args) == 1 && args[0] == "sat" {
		return HandleSelfUpdate(version, repo)
	}

	// Force a reconcile regardless of the TTL, so a tool updated outside
	// sat since the last reconcile doesn't get reported as outdated (or
	// pointlessly re-updated) against a stale recorded version. Silent -
	// drift isn't user-actionable and this command already owns the
	// terminal with spinners.
	if _, err := drift.Reconcile(); err != nil && os.Getenv(common.EnvSATDebug) != "" {
		fmt.Fprintf(os.Stderr, "%s drift reconcile: %v\n", common.DebugPrefix, err)
	}

	var tools []string
	var sourceFilter string
	var skipConfirm bool
	for _, arg := range args {
		if arg == "-y" || arg == "--yes" {
			skipConfirm = true
			continue
		}
		if sel, ok := common.LookupSourceFlag(arg); ok {
			sourceFilter = sel.Type
			continue
		}
		if strings.HasPrefix(arg, "--") || strings.HasPrefix(arg, "-") {
			return fmt.Errorf("unknown flag: %s\n%s", arg, updateUsage)
		}
		tools = append(tools, arg)
	}

	return updateOutdated(tools, sourceFilter, skipConfirm, version, repo)
}

// updateEntry updates one tracked entry via its recorded source, mirroring
// uninstall.go's removeViaSource dispatch shape. On success, re-records the
// entry's new version in the manifest so the next outdated scan compares
// against the post-update version instead of the stale pre-update one.
func updateEntry(tool, sourceStr string) {
	var newVersion string
	err := ui.RunWithSpinner(tool, sourceStr, func() error {
		v, err := updateViaSource(tool, sourceStr)
		newVersion = v
		return err
	})
	if err != nil {
		ui.StatusError(tool, sourceStr, err.Error())
		return
	}

	sourceType := manifest.GetSourceType(sourceStr)
	identity := manifest.GetSourceIdentity(sourceStr)
	newSourceStr := manifest.BuildSourceString(sourceType, identity, newVersion)
	if err := manifest.Add(tool, newSourceStr); err != nil {
		fmt.Fprintf(os.Stderr, "sat: warning: %s updated but failed to record new version in manifest: %v\n", tool, err)
	}

	ui.StatusOK(tool, newSourceStr)
}

// updateViaSource dispatches to the source-specific update function
// recorded in the manifest for tool, returning the tool's version after
// the update so the caller can refresh the manifest.
func updateViaSource(tool, sourceStr string) (newVersion string, err error) {
	sourceType := manifest.GetSourceType(sourceStr)
	identity := manifest.GetSourceIdentity(sourceStr)

	switch sourceType {
	case common.SourceCargo, sourceAliasRust:
		if err := sources.CargoUpdate(tool); err != nil {
			return "", err
		}
		return sources.CargoGetVersion(tool), nil
	case common.SourceBrew:
		if err := sources.BrewUpdate(tool); err != nil {
			return "", err
		}
		return sources.BrewGetVersion(tool), nil
	case common.SourceNix:
		if err := sources.NixUpdate(tool); err != nil {
			return "", err
		}
		return sources.NixGetVersion(tool), nil
	case sourceAliasNixOS:
		return "", fmt.Errorf("declarative NixOS package - update it via your NixOS configuration instead")
	case common.PkgManagerAPT, common.PkgManagerPacman, common.PkgManagerAPK, common.PkgManagerDNF, common.SourceSystem:
		if err := sources.Update(tool); err != nil {
			return "", err
		}
		return sources.GetVersion(tool), nil
	case common.SourceGH, sourceAliasGitHub:
		if err := sources.GitHubUpdate(tool, identity); err != nil {
			return "", err
		}
		return sources.GitHubGetVersion(identity), nil
	case common.SourceAppImage:
		if err := sources.AppImageUpdate(tool, identity); err != nil {
			return "", err
		}
		return sources.AppImageGetVersion(identity), nil
	case common.SourceSat:
		return "", fmt.Errorf("use 'sat update sat' to update sat itself")
	case common.SourceNPM:
		if err := sources.NpmUpdate(tool, identity); err != nil {
			return "", err
		}
		return sources.NpmGetVersion(tool), nil
	case common.SourceFlatpak:
		if err := sources.FlatpakUpdate(tool, identity); err != nil {
			return "", err
		}
		return sources.FlatpakGetVersion(identity), nil
	case common.SourceUV:
		if err := sources.UvUpdate(tool); err != nil {
			return "", err
		}
		return sources.UvGetVersion(tool), nil
	case common.SourceGo:
		if err := sources.GoUpdate(tool, identity); err != nil {
			return "", err
		}
		return sources.GoGetVersion(tool), nil
	default:
		return "", fmt.Errorf("no automated update for source %q", sourceType)
	}
}

// checkOutdated dispatches to the source-specific outdated check for tool.
// ok is false when the source has no CheckOutdated implementation
// (go, or a non-apt system package manager) or the check itself failed -
// callers should silently skip these from a bulk scan rather than treat
// them as errors. flatpak is not handled here - see collectFlatpakOutdated,
// which checks all pending flatpak updates (tracked apps and untracked
// runtimes alike) in one bulk call instead of one checkOutdated per entry.
func checkOutdated(tool, sourceStr string) (current, latest string, ok bool) {
	sourceType := manifest.GetSourceType(sourceStr)
	identity := manifest.GetSourceIdentity(sourceStr)

	var err error
	switch sourceType {
	case common.SourceCargo, sourceAliasRust:
		current, latest, err = sources.CargoCheckOutdated(tool)
	case common.SourceBrew:
		current, latest, err = sources.BrewCheckOutdated(tool)
	case common.SourceNix, sourceAliasNixOS:
		current, latest, err = sources.NixCheckOutdated(tool, sourceType)
	case common.PkgManagerAPT, common.PkgManagerPacman, common.PkgManagerAPK, common.PkgManagerDNF, common.SourceSystem:
		current, latest, err = sources.CheckOutdated(tool)
	case common.SourceGH, sourceAliasGitHub:
		current, latest, err = sources.GitHubCheckOutdated(tool, identity)
	case common.SourceAppImage:
		current, latest, err = sources.AppImageCheckOutdated(tool, identity)
	case common.SourceNPM:
		current, latest, err = sources.NpmCheckOutdated(tool, identity)
	case common.SourceUV:
		current, latest, err = sources.UvCheckOutdated(tool)
	default:
		return "", "", false
	}

	if err != nil || current == "" || latest == "" {
		return "", "", false
	}
	if !common.VersionIsNewer(latest, current) {
		return "", "", false
	}
	return current, latest, true
}

// outdatedGroup is the grouping key for o - tracked flatpak apps and
// untracked flatpak deps group together under one source (e.g. "flatpak"),
// even though outdatedTag distinguishes them in the printed row.
func outdatedGroup(o outdatedEntry) string {
	return ui.SourceDisplay(o.source)
}

// outdatedTag is the bracketed source tag shown for o.
func outdatedTag(o outdatedEntry) string {
	tag := ui.SourceDisplay(o.source)
	if o.dep {
		tag += ":dep"
	}
	return tag
}

// outdatedEntry is one tool found to have a newer version available.
// dep marks a flatpak runtime with no manifest entry of its own (identity
// carries its app ID, since there is no tool name to look one up from).
type outdatedEntry struct {
	tool, source, current, latest string
	dep                           bool
	identity                      string
}

// targetEntries returns the manifest entries an update covers: every entry
// when names is empty, otherwise every entry tracked under each name (all
// sources of a name, never an ambiguity error). Unknown names are reported
// and skipped. sourceFilter, when set, keeps only that source type.
func targetEntries(names []string, sourceFilter string) ([]manifest.Entry, error) {
	var entries []manifest.Entry
	if len(names) == 0 {
		all, err := manifest.All()
		if err != nil {
			return nil, err
		}
		entries = all
	}
	seen := make(map[string]bool)
	for _, name := range names {
		found, err := resolveTargets(name, "")
		if err != nil {
			ui.StatusFail(err.Error())
			continue
		}
		for _, e := range found {
			if !seen[e.Tool+"="+e.Source] {
				seen[e.Tool+"="+e.Source] = true
				entries = append(entries, e)
			}
		}
	}

	if sourceFilter == "" {
		return entries, nil
	}
	var filtered []manifest.Entry
	for _, e := range entries {
		if manifest.CanonicalSourceType(manifest.GetSourceType(e.Source)) == sourceFilter {
			filtered = append(filtered, e)
		}
	}
	return filtered, nil
}

// updateOutdated scans the targeted manifest entries for outdated tools, batched per source
// type in parallel (mirrors search.go's searchAllSources concurrency
// shape), prints what's outdated, and offers a single bulk confirmation
// before updating everything shown. Each source-type group is checked
// sequentially inside its own goroutine so a source with many tracked
// tools (e.g. cargo hitting crates.io per package) doesn't burst a remote
// registry with concurrent requests; only the source types run in parallel.
func updateOutdated(names []string, sourceFilter string, skipConfirm bool, version, repo string) error {
	entries, err := targetEntries(names, sourceFilter)
	if err != nil {
		return err
	}
	named := len(names) > 0
	if named && len(entries) == 0 {
		return nil
	}

	grouped := make(map[string][]manifest.Entry)
	for _, e := range entries {
		sourceType := manifest.GetSourceType(e.Source)
		grouped[sourceType] = append(grouped[sourceType], e)
	}

	// flatpak is checked separately in bulk (see collectFlatpakOutdated) so
	// it doesn't go through the generic per-entry checkOutdated dispatch.
	flatpakEntries := grouped[common.SourceFlatpak]
	delete(grouped, common.SourceFlatpak)
	checkFlatpak := sourceFilter == "" || sourceFilter == common.SourceFlatpak
	if named {
		checkFlatpak = len(flatpakEntries) > 0
	}

	var mu sync.Mutex
	var outdated []outdatedEntry
	var wg sync.WaitGroup

	for _, group := range grouped {
		wg.Add(1)
		go func(group []manifest.Entry) {
			defer wg.Done()
			for _, e := range group {
				current, latest, ok := checkOutdated(e.Tool, e.Source)
				if !ok || current == latest {
					continue
				}
				mu.Lock()
				outdated = append(outdated, outdatedEntry{tool: e.Tool, source: e.Source, current: current, latest: latest})
				mu.Unlock()
			}
		}(group)
	}
	if checkFlatpak {
		wg.Add(1)
		go func() {
			defer wg.Done()
			fpOutdated := collectFlatpakOutdated(flatpakEntries)
			mu.Lock()
			for _, o := range fpOutdated {
				if !named || !o.dep {
					outdated = append(outdated, o)
				}
			}
			mu.Unlock()
		}()
	}
	if !named && (sourceFilter == "" || sourceFilter == common.SourceSat) {
		wg.Add(1)
		go func() {
			defer wg.Done()
			latest, ok := SelfUpdateCheck(version, repo)
			if !ok {
				return
			}
			mu.Lock()
			outdated = append(outdated, outdatedEntry{
				tool: selfToolName, source: common.SourceSat, current: version, latest: latest,
			})
			mu.Unlock()
		}()
	}
	wg.Wait()

	if len(outdated) == 0 {
		fmt.Println("Everything up to date")
		return nil
	}

	sort.Slice(outdated, func(i, j int) bool {
		if outdated[i].dep != outdated[j].dep {
			return !outdated[i].dep // real apps before deps within a group
		}
		return outdated[i].tool < outdated[j].tool
	})

	// Group by source (largest group first, mirroring list.go's
	// displayGrouped), rebuilding outdated in that order so the later
	// apply loop walks the same grouped sequence it was confirmed in.
	groups := make(map[string][]outdatedEntry)
	var groupOrder []string
	for _, o := range outdated {
		group := outdatedGroup(o)
		if _, ok := groups[group]; !ok {
			groupOrder = append(groupOrder, group)
		}
		groups[group] = append(groups[group], o)
	}
	counts := make(map[string]int, len(groups))
	for group, entries := range groups {
		counts[group] = len(entries)
	}
	outdated = outdated[:0]
	for _, group := range ui.GroupedOrder(groupOrder, counts) {
		outdated = append(outdated, groups[group]...)
	}

	for _, o := range outdated {
		color := ui.SourceColor(o.source)
		fmt.Printf("  %-*s [%s%s%s] %s -> %s\n",
			ui.ToolNameWidth, ui.TruncateName(o.tool, ui.ToolNameWidth), color, outdatedTag(o), ui.Reset, o.current, o.latest)
	}

	if skipConfirm {
		fmt.Printf("\nUpdating all %d\n", len(outdated))
	} else {
		fmt.Printf("\nUpdate all %d? [y/N] ", len(outdated))
		reader := bufio.NewReader(os.Stdin)
		answer, _ := reader.ReadString('\n')
		answer = strings.ToLower(strings.TrimSpace(answer))
		if answer != "y" && answer != "yes" {
			return nil
		}
	}

	var depRefs []string
	var selfLatest string
	for _, o := range outdated {
		if o.source == common.SourceSat {
			selfLatest = o.latest
			continue
		}
		if o.dep {
			depRefs = append(depRefs, o.identity)
			continue
		}
		updateEntry(o.tool, o.source)
	}
	if len(depRefs) > 0 {
		label := fmt.Sprintf("%d flatpak runtimes", len(depRefs))
		err := ui.RunWithSpinner(label, common.SourceFlatpak, func() error {
			return sources.FlatpakUpdateRefs(depRefs)
		})
		if err != nil {
			ui.StatusFail(fmt.Sprintf("flatpak runtime update: %v", err))
		} else {
			ui.Status(fmt.Sprintf("%d flatpak runtime(s) updated [%s]", len(depRefs), ui.SourceDisplay(common.SourceFlatpak)))
		}
	}

	// Deliberately last: the installer swaps out the binary this process is
	// running from, so every other tool must already be done by the time it
	// runs. Nothing may be sequenced after this.
	if selfLatest != "" {
		return selfUpdateInstall(selfLatest, repo)
	}
	return nil
}

// flatpakDisplayVersions resolves current/latest for display, since some
// flatpak refs report no version string at all, and others report the
// same string on both sides (flatpak's own version tag doesn't always
// change between updates, even though remote-ls --updates confirms a
// newer commit is pending - sources.FlatpakListUpdates already retries
// against a fresh appstream cache before this is ever called, so a
// same-version pair reaching here means the pending update really is just
// a rebuild). "?" marks a side with no version info at all; when neither
// side has any, that's "?" -> "?²" - one order of magnitude more unknown.
func flatpakDisplayVersions(current, latest string) (string, string) {
	if current == "" && latest == "" {
		return "?", "?²"
	}
	if latest == current {
		return current, "rebuild"
	}
	if current == "" {
		current = "?"
	}
	if latest == "" {
		latest = "?"
	}
	return current, latest
}

// collectFlatpakOutdated checks all pending flatpak updates in one bulk
// call (sources.FlatpakListUpdates), then splits results against
// flatpakEntries (sat-tracked apps): matches become ordinary tracked-tool
// entries, unmatched refs are runtimes with no manifest entry, reported as
// deps using flatpak's own display name. A ref's presence in the update
// list is trusted as-is (flatpak already filtered to "needs updating"),
// since some runtimes report no version string at all in either direction -
// version text is display-only here, with a fallback for the blank case.
func collectFlatpakOutdated(flatpakEntries []manifest.Entry) []outdatedEntry {
	updates, err := sources.FlatpakListUpdates()
	if err != nil || len(updates) == 0 {
		return nil
	}

	tracked := make(map[string]manifest.Entry, len(flatpakEntries))
	for _, e := range flatpakEntries {
		tracked[manifest.GetSourceIdentity(e.Source)] = e
	}
	installed := sources.FlatpakInstalledVersions()

	var outdated []outdatedEntry
	for _, u := range updates {
		if e, ok := tracked[u.AppID]; ok {
			current := manifest.GetSourceVersion(e.Source)
			if current == "" {
				current = installed[u.AppID+"//"+u.Branch]
			}
			current, latest := flatpakDisplayVersions(current, u.Version)
			outdated = append(outdated, outdatedEntry{tool: e.Tool, source: e.Source, current: current, latest: latest})
			continue
		}

		current, latest := flatpakDisplayVersions(installed[u.AppID+"//"+u.Branch], u.Version)
		outdated = append(outdated, outdatedEntry{
			tool:     u.Name,
			source:   common.SourceFlatpak,
			current:  current,
			latest:   latest,
			dep:      true,
			identity: u.AppID + "//" + u.Branch,
		})
	}
	return outdated
}
