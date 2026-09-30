package commands

import (
	"fmt"
	"os"

	"github.com/DeprecatedLuar/sat/internal/common"
	"github.com/DeprecatedLuar/sat/internal/manifest"
	"github.com/DeprecatedLuar/sat/internal/sources"
	"github.com/DeprecatedLuar/sat/internal/ui"
)

const uninstallUsage = "usage: sat uninstall <tool>[:source] [<tool>[:source] ...] [--<source>]"

// Uninstall removes one or more tools: resolves each spec to exactly one
// manifest entry, delegates removal to that entry's source pipeline, then
// drops the entry from the manifest on success.
func Uninstall(args []string) error {
	specs := scopeSpecs(args)
	if len(specs) == 0 {
		return fmt.Errorf(uninstallUsage)
	}

	for _, s := range specs {
		uninstallOne(s)
	}

	return nil
}

func uninstallOne(s scopedSpec) {
	entries, err := resolveTargets(s.spec, s.source)
	if err != nil {
		ui.StatusFail(err.Error())
		return
	}
	name, _ := common.ParseToolSpec(s.spec)
	entry, err := requireOne(name, entries)
	if err != nil {
		ui.StatusFail(err.Error())
		return
	}

	err = ui.RunWithSpinner(entry.Tool, entry.Source, func() error {
		return removeViaSource(entry.Tool, entry.Source)
	})
	if err != nil {
		ui.StatusError(entry.Tool, entry.Source, err.Error())
		return
	}

	if err := manifest.Remove(entry.Tool, manifest.GetSourceType(entry.Source)); err != nil {
		fmt.Fprintf(os.Stderr, "sat: warning: %s removed but failed to update manifest: %v\n", entry.Tool, err)
	}

	ui.StatusRemoved(entry.Tool, entry.Source)
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
	case common.SourceCargo, sourceAliasRust:
		return sources.CargoUninstall(tool, sourceStr)
	case common.SourceBrew:
		return sources.BrewUninstall(tool, sourceStr)
	case common.SourceNix:
		return sources.NixUninstall(tool, sourceStr)
	case sourceAliasNixOS:
		return fmt.Errorf("declarative NixOS package - remove it from your NixOS configuration instead")
	case common.PkgManagerAPT, common.PkgManagerPacman, common.PkgManagerAPK, common.PkgManagerDNF, common.SourceSystem:
		return sources.Uninstall(tool, sourceStr)
	case common.SourceGH, sourceAliasGitHub:
		return sources.GitHubUninstall(tool, sourceStr)
	case common.SourceAppImage:
		return sources.AppImageUninstall(tool, sourceStr)
	case common.SourceNPM:
		return sources.NpmUninstall(tool, sourceStr)
	case common.SourceFlatpak:
		return sources.FlatpakUninstall(tool, sourceStr)
	case common.SourceUV:
		return sources.UvUninstall(tool, sourceStr)
	case common.SourceGo:
		return sources.GoUninstall(tool)
	case common.SourceSat:
		return fmt.Errorf("%s uninstall not yet implemented in the Go port", ui.SourceDisplay(sourceStr))
	case "unknown":
		return sources.UnknownUninstall(tool, sourceStr)
	default:
		return fmt.Errorf("no automated removal for source %q - remove manually", sourceType)
	}
}
