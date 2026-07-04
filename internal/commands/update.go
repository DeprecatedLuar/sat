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
	case common.SourceCargo, "rust":
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
	case "nixos":
		return "", fmt.Errorf("declarative NixOS package - update it via your NixOS configuration instead")
	case "apt", "pacman", "apk", "dnf", common.SourceSystem:
		if err := sources.Update(tool); err != nil {
			return "", err
		}
		return sources.GetVersion(tool), nil
	case common.SourceGH, "github":
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
	case common.SourceUV, common.SourceGo:
		return "", fmt.Errorf("%s update not yet implemented in the Go port", ui.SourceDisplay(sourceStr))
	default:
		return "", fmt.Errorf("no automated update for source %q", sourceType)
	}
}

// checkOutdated dispatches to the source-specific outdated check for tool.
// ok is false when the source has no CheckOutdated implementation
// (flatpak/uv/go, or a non-apt system package manager) or the check
// itself failed - callers should silently skip these from a bulk scan
// rather than treat them as errors.
func checkOutdated(tool, sourceStr string) (current, latest string, ok bool) {
	sourceType := manifest.GetSourceType(sourceStr)
	identity := manifest.GetSourceIdentity(sourceStr)

	var err error
	switch sourceType {
	case common.SourceCargo, "rust":
		current, latest, err = sources.CargoCheckOutdated(tool)
	case common.SourceBrew:
		current, latest, err = sources.BrewCheckOutdated(tool)
	case common.SourceNix, "nixos":
		current, latest, err = sources.NixCheckOutdated(tool, sourceType)
	case "apt", "pacman", "apk", "dnf", common.SourceSystem:
		current, latest, err = sources.CheckOutdated(tool)
	case common.SourceGH, "github":
		current, latest, err = sources.GitHubCheckOutdated(tool, identity)
	case common.SourceAppImage:
		current, latest, err = sources.AppImageCheckOutdated(tool, identity)
	case common.SourceNPM:
		current, latest, err = sources.NpmCheckOutdated(tool, identity)
	default:
		return "", "", false
	}

	if err != nil || current == "" || latest == "" {
		return "", "", false
	}
	return current, latest, true
}

// outdatedEntry is one tool found to have a newer version available.
type outdatedEntry struct {
	tool, source, current, latest string
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
	wg.Wait()

	if len(outdated) == 0 {
		fmt.Println("Everything up to date")
		return nil
	}

	sort.Slice(outdated, func(i, j int) bool { return outdated[i].tool < outdated[j].tool })

	for _, o := range outdated {
		display := ui.SourceDisplay(o.source)
		color := ui.SourceColor(o.source)
		fmt.Printf("  %-*s [%s%s%s] %s -> %s\n", ui.ToolNameWidth, o.tool, color, display, ui.Reset, o.current, o.latest)
	}

	fmt.Printf("\nUpdate all %d? [y/N] ", len(outdated))
	reader := bufio.NewReader(os.Stdin)
	answer, _ := reader.ReadString('\n')
	answer = strings.ToLower(strings.TrimSpace(answer))
	if answer != "y" && answer != "yes" {
		return nil
	}

	for _, o := range outdated {
		updateOne(o.tool)
	}
	return nil
}
