package commands

import (
	"bufio"
	"fmt"
	"os"
	"sort"
	"strings"
	"sync"

	"github.com/DeprecatedLuar/sat/internal/common"
	"github.com/DeprecatedLuar/sat/internal/manifest"
	"github.com/DeprecatedLuar/sat/internal/sources"
	"github.com/DeprecatedLuar/sat/internal/ui"
)

const updateUsage = "usage: sat update [<tool> ...] [--cargo|--brew|--nix|--apt|--gh|--appimage|--flatpak|--npm|--uv|--go]"

// Source-type aliases recognized alongside their canonical common.Source*
// constants (older manifests / scan output may still record these).
const (
	sourceAliasRust   = "rust"
	sourceAliasNixOS  = "nixos"
	sourceAliasGitHub = "github"
)

// updateFlagSource maps a CLI flag to the source type it scopes
// sat update to (list.go's filterAliases is the read-only List equivalent).
var updateFlagSource = map[string]string{
	"--cargo":    common.SourceCargo,
	"--rust":     common.SourceCargo,
	"--brew":     common.SourceBrew,
	"--nix":      common.SourceNix,
	"--apt":      common.SourceSystem,
	"--system":   common.SourceSystem,
	"--sys":      common.SourceSystem,
	"--gh":       common.SourceGH,
	"--github":   common.SourceGH,
	"--appimage": common.SourceAppImage,
	"--flatpak":  common.SourceFlatpak,
	"--npm":      common.SourceNPM,
	"--uv":       common.SourceUV,
	"--go":       common.SourceGo,
}

// HandleUpdate routes between self-update, explicit tool updates, and the
// interactive outdated-scan flow.
func HandleUpdate(args []string, version, repo string) error {
	if len(args) == 1 && args[0] == "sat" {
		return HandleSelfUpdate(version, repo)
	}

	var tools []string
	var sourceFilter string
	for _, arg := range args {
		if src, ok := updateFlagSource[arg]; ok {
			sourceFilter = src
			continue
		}
		if strings.HasPrefix(arg, "--") {
			return fmt.Errorf("unknown flag: %s\n%s", arg, updateUsage)
		}
		tools = append(tools, arg)
	}

	if len(tools) > 0 {
		for _, tool := range tools {
			updateOne(tool)
		}
		return nil
	}

	return updateOutdated(sourceFilter)
}

// updateOne updates a single tool via its recorded source, mirroring
// uninstall.go's removeViaSource dispatch shape. On success, re-records the
// tool's new version in the manifest so the next outdated scan compares
// against the post-update version instead of the stale pre-update one.
func updateOne(tool string) {
	sourceStr := manifest.Get(tool)
	if sourceStr == "" {
		ui.StatusFail(fmt.Sprintf("%s is not tracked by sat", tool))
		return
	}

	var newVersion string
	err := ui.RunWithSpinner(tool, sourceStr, func() error {
		v, err := updateViaSource(tool, sourceStr)
		newVersion = v
		return err
	})
	if err != nil {
		ui.StatusFail(fmt.Sprintf("%s: %v", tool, err))
		return
	}

	sourceType := manifest.GetSourceType(sourceStr)
	identity := manifest.GetSourceIdentity(sourceStr)
	newSourceStr := manifest.BuildSourceString(sourceType, identity, newVersion)
	if err := manifest.Add(tool, newSourceStr); err != nil && os.Getenv(common.EnvSATDebug) != "" {
		fmt.Fprintf(os.Stderr, "%s failed to record %s's new version in manifest: %v\n", common.DebugPrefix, tool, err)
	}

	ui.StatusOK(fmt.Sprintf("%s updated", tool), newSourceStr)
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
		return "", fmt.Errorf("%s update not yet implemented in the Go port", ui.SourceDisplay(sourceStr))
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

// updateOutdated scans the manifest for outdated tools, batched per source
// type in parallel (mirrors search.go's searchAllSources concurrency
// shape), prints what's outdated, and offers a single bulk confirmation
// before updating everything shown. Each source-type group is checked
// sequentially inside its own goroutine so a source with many tracked
// tools (e.g. cargo hitting crates.io per package) doesn't burst a remote
// registry with concurrent requests; only the source types run in parallel.
func updateOutdated(sourceFilter string) error {
	entries, err := manifest.All()
	if err != nil {
		return err
	}

	grouped := make(map[string][]manifest.Entry)
	for _, e := range entries {
		sourceType := manifest.GetSourceType(e.Source)
		if sourceFilter != "" && sourceType != sourceFilter {
			continue
		}
		grouped[sourceType] = append(grouped[sourceType], e)
	}

	// flatpak is checked separately in bulk (see collectFlatpakOutdated) so
	// it doesn't go through the generic per-entry checkOutdated dispatch.
	flatpakEntries := grouped[common.SourceFlatpak]
	delete(grouped, common.SourceFlatpak)
	checkFlatpak := sourceFilter == "" || sourceFilter == common.SourceFlatpak

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
			outdated = append(outdated, fpOutdated...)
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

	fmt.Printf("\nUpdate all %d? [y/N] ", len(outdated))
	reader := bufio.NewReader(os.Stdin)
	answer, _ := reader.ReadString('\n')
	answer = strings.ToLower(strings.TrimSpace(answer))
	if answer != "y" && answer != "yes" {
		return nil
	}

	var depRefs []string
	for _, o := range outdated {
		if o.dep {
			depRefs = append(depRefs, o.identity)
			continue
		}
		updateOne(o.tool)
	}
	if len(depRefs) > 0 {
		if err := sources.FlatpakUpdateRefs(depRefs); err != nil {
			ui.StatusFail(fmt.Sprintf("flatpak runtime update: %v", err))
		} else {
			ui.StatusOK(fmt.Sprintf("%d flatpak runtime(s) updated", len(depRefs)), common.SourceFlatpak)
		}
	}
	return nil
}

// flatpakDisplayVersions resolves current/latest for display, since some
// flatpak refs report no version string at all, and others report the
// same string on both sides (flatpak's own version tag doesn't always
// change between updates, even though remote-ls --updates confirms a
// newer commit is pending).
func flatpakDisplayVersions(current, latest string) (string, string) {
	if latest == "" || current == latest {
		latest = "update available"
	}
	if current == "" {
		current = "unknown"
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
