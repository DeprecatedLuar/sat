package commands

import (
	"fmt"
	"os"

	"github.com/DeprecatedLuar/sat/internal/common"
	"github.com/DeprecatedLuar/sat/internal/manifest"
	"github.com/DeprecatedLuar/sat/internal/sources"
	"github.com/DeprecatedLuar/sat/internal/ui"
)

const uninstallUsage = "usage: sat uninstall <tool> [<tool> ...]"

// Uninstall removes one or more tools: looks up each tool's recorded
// source in the manifest, delegates removal to that source's own
// uninstall pipeline, then drops it from the manifest on success.
func Uninstall(args []string) error {
	if len(args) == 0 {
		return fmt.Errorf(uninstallUsage)
	}

	for _, tool := range args {
		uninstallOne(tool)
	}

	return nil
}

func uninstallOne(tool string) {
	sourceStr := manifest.Get(tool)
	if sourceStr == "" {
		ui.StatusFail(fmt.Sprintf("%s is not tracked by sat", tool))
		return
	}

	if err := removeViaSource(tool, sourceStr); err != nil {
		ui.StatusFail(fmt.Sprintf("%s: %v", tool, err))
		return
	}

	if err := manifest.Remove(tool); err != nil {
		fmt.Fprintf(os.Stderr, "sat: warning: %s removed but failed to update manifest: %v\n", tool, err)
	}

	ui.Status(fmt.Sprintf("%s removed", tool))
}

// removeViaSource dispatches to the source-specific uninstall pipeline
// recorded in the manifest for tool. Each source module owns its full
// removal pipeline end to end: real package managers (cargo/brew/nix/
// apt/...) get their native uninstall command; sources with no package
// manager backing them (gh releases, appimage, scanned unknown binaries)
// do their own manual filesystem cleanup. This function only routes, it
// never removes anything itself.
func removeViaSource(tool, sourceStr string) error {
	sourceType := manifest.GetSourceType(sourceStr)

	switch sourceType {
	case common.SourceCargo, "rust":
		return sources.CargoUninstall(tool, sourceStr)
	case common.SourceBrew:
		return sources.BrewUninstall(tool, sourceStr)
	case common.SourceNix:
		return sources.NixUninstall(tool, sourceStr)
	case "nixos":
		return fmt.Errorf("declarative NixOS package - remove it from your NixOS configuration instead")
	case "apt", "pacman", "apk", "dnf", common.SourceSystem:
		return sources.Uninstall(tool, sourceStr)
	case common.SourceGH, "github":
		return sources.GitHubUninstall(tool, sourceStr)
	case common.SourceAppImage:
		return sources.AppImageUninstall(tool, sourceStr)
	case common.SourceNPM:
		return sources.NpmUninstall(tool, sourceStr)
	case common.SourceFlatpak:
		return sources.FlatpakUninstall(tool, sourceStr)
	case common.SourceUV:
		return sources.UvUninstall(tool, sourceStr)
	case common.SourceGo, common.SourceSat:
		return fmt.Errorf("%s uninstall not yet implemented in the Go port", ui.SourceDisplay(sourceStr))
	case "unknown":
		return sources.UnknownUninstall(tool, sourceStr)
	default:
		return fmt.Errorf("no automated removal for source %q - remove manually", sourceType)
	}
}
