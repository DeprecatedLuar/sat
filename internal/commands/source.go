package commands

import (
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/DeprecatedLuar/sat/internal/bootstrap"
	"github.com/DeprecatedLuar/sat/internal/ui"
)

const sourceUsage = "usage: sat source <packagemanager>\n       sat source rm <packagemanager>\n       sat source ls"

// sourceRemoveArg is the "sat source rm <pm>" subcommand keyword.
const sourceRemoveArg = "rm"

// sourceListArg and sourceListArgAlt are the "sat source ls"/"sat source
// list" subcommand keywords.
const (
	sourceListArg    = "ls"
	sourceListArgAlt = "list"
)

// sourceListOrder is the fixed display order for `sat source ls`.
var sourceListOrder = []string{"uv", "cargo", "brew", "nix", "huber"}

// nixosMarkerFile indicates a declarative NixOS system, where package
// managers like Nix are already present and managed via /etc/nixos
// configuration rather than bootstrapped by sat.
const nixosMarkerFile = "/etc/NIXOS"

// sourceAliases normalizes package-manager names/aliases to their
// canonical bootstrap target. "gh"/"github" map to "huber" because
// sources.GitHubInstall's release-install path depends on huber being
// present (internal/sources/github.go), not on the `gh` CLI itself.
var sourceAliases = map[string]string{
	"gh":       "huber",
	"github":   "huber",
	"homebrew": "brew",
	"python":   "uv",
}

// sourceCompanions lists extra binaries installed alongside a package
// manager's primary binary that should be relocated together with it.
var sourceCompanions = map[string][]string{
	"uv": {"uvx", "uvw"},
}

// Source bootstraps a package manager binary (e.g. uv, cargo, brew, nix,
// huber) so that `sat install` has something to fall back to. This is
// distinct from `sat info`, which reports where an already-installed
// tool came from.
func Source(args []string) error {
	if len(args) == 0 {
		return fmt.Errorf(sourceUsage)
	}

	if args[0] == sourceRemoveArg {
		if len(args) < 2 {
			return fmt.Errorf(sourceUsage)
		}
		return removeSource(normalizeSourcePM(args[1]))
	}

	if args[0] == sourceListArg || args[0] == sourceListArgAlt {
		listSources()
		return nil
	}

	pm := normalizeSourcePM(args[0])

	if pm == "nix" {
		if _, err := os.Stat(nixosMarkerFile); err == nil {
			ui.Status("bruh")
			return nil
		}
	}

	if existing, err := exec.LookPath(pm); err == nil {
		ui.Status(fmt.Sprintf("%s is already installed at %s", pm, existing))
		return nil
	}

	installedPath, err := dispatchBootstrap(pm)
	if err != nil {
		return err
	}

	// nix's binary lives inside the immutable /nix/store, already exposed
	// via ~/.nix-profile/bin/nix - relocating it would add a fragile extra
	// symlink hop with no benefit, and nix-installer's own uninstaller tears
	// the whole tree down regardless.
	if pm == "nix" {
		ui.Status(fmt.Sprintf("%s → %s", pm, installedPath))
		return nil
	}

	moved, err := bootstrap.Relocate(installedPath, sourceCompanions[pm]...)
	if err != nil {
		return err
	}

	if len(moved) > 0 {
		ui.Status(fmt.Sprintf("%s (+ %s) → %s", pm, strings.Join(moved, " "), installedPath))
	} else {
		ui.Status(fmt.Sprintf("%s → %s", pm, installedPath))
	}

	return nil
}

// normalizeSourcePM maps a package-manager name/alias to its canonical
// bootstrap target via sourceAliases.
func normalizeSourcePM(pm string) string {
	if canonical, ok := sourceAliases[pm]; ok {
		return canonical
	}
	return pm
}

// removeSource dispatches to the matching bootstrap.<PM>Remove() and
// reports the result.
func removeSource(pm string) error {
	if err := dispatchRemove(pm); err != nil {
		return err
	}
	ui.Status(fmt.Sprintf("%s removed", pm))
	return nil
}

// dispatchRemove routes to the bootstrap package's remover for pm,
// mirroring dispatchBootstrap.
func dispatchRemove(pm string) error {
	switch pm {
	case "uv":
		return bootstrap.UvRemove()
	case "cargo":
		return bootstrap.CargoRemove()
	case "brew":
		return bootstrap.BrewRemove()
	case "nix":
		return bootstrap.NixRemove()
	case "huber":
		return bootstrap.HuberRemove()
	default:
		return fmt.Errorf("unknown source %q", pm)
	}
}

// listSources prints every bootstrap-able package manager, in its native
// source color with an "available" tag when found on $PATH, or dimmed
// with "not available" otherwise.
func listSources() {
	for _, pm := range sourceListOrder {
		printSourceStatus(pm)
	}
}

func printSourceStatus(pm string) {
	if path, err := exec.LookPath(pm); err == nil {
		color := ui.SourceColor(pm)
		fmt.Printf("  %s%-*s available%s  %s%s%s\n", color, ui.ToolNameWidth, pm, ui.Reset, ui.Dim, path, ui.Reset)
		return
	}

	fmt.Printf("  %s%-*s not available%s\n", ui.Dim, ui.ToolNameWidth, pm, ui.Reset)
}

// dispatchBootstrap routes to the bootstrap package's installer for pm.
// Each installer is self-contained: it runs its package manager's own
// official install method and reports where the resulting binary landed.
func dispatchBootstrap(pm string) (installedPath string, err error) {
	switch pm {
	case "uv":
		return bootstrap.UvInstall()
	case "cargo":
		return bootstrap.CargoInstall()
	case "brew":
		return bootstrap.BrewInstall()
	case "nix":
		return bootstrap.NixInstall()
	case "huber":
		return bootstrap.HuberInstall()
	default:
		return "", fmt.Errorf("unknown source %q", pm)
	}
}
