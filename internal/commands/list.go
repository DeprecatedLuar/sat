package commands

import (
	"fmt"
	"os/exec"
	"sort"
	"strings"

	"github.com/DeprecatedLuar/sat/internal/manifest"
	"github.com/DeprecatedLuar/sat/internal/ui"
)

// filterAliases maps a --flag source filter argument to the set of
// normalized source-type/display-name tokens it should match against.
// Mirrors installFlagSource's --flag convention so a single invocation can
// combine multiple filters, e.g. "sat ls --flatpak --nix".
var filterAliases = map[string][]string{
	"--fpk": {"flatpak"}, "--flatpak": {"flatpak"},
	"--img": {"appimage"}, "--appimage": {"appimage"},
	"--sys": {"system"}, "--system": {"system"},
	"--nix": {"nix", "nixos"}, "--nixos": {"nix", "nixos"},
	"--gh": {"github", "repo", "gh"}, "--github": {"github", "repo", "gh"},
	"--npm": {"npm", "node"}, "--node": {"npm", "node"},
	"--py": {"python", "uv"}, "--python": {"python", "uv"}, "--uv": {"python", "uv"},
	"--cargo": {"cargo", "rust"}, "--rust": {"cargo", "rust"},
	"--go":     {"go"},
	"--brew":   {"brew"},
	"--sat":    {"sat"},
	"--manual": {"manual"},
}

// List displays tracked packages from the system manifest, optionally
// filtered by source, grouped by source with the largest group first,
// and prunes any entry whose binary is no longer on PATH.
func List(args []string) error {
	var filters []string
	for _, arg := range args {
		aliases, ok := filterAliases[arg]
		if !ok {
			return fmt.Errorf("unknown source filter: %s", arg)
		}
		filters = append(filters, aliases...)
	}

	entries, err := manifest.All()
	if err != nil {
		return err
	}

	var stale []string
	var shown []manifest.Entry
	for _, e := range entries {
		if _, err := exec.LookPath(e.Tool); err != nil {
			stale = append(stale, e.Tool)
			continue
		}
		if matchesFilter(e.Source, filters) {
			shown = append(shown, e)
		}
	}

	if len(shown) > 0 {
		displayGrouped(shown)
	}

	if len(stale) > 0 {
		fmt.Println()
		fmt.Printf("Cleaning %d stale entries...\n", len(stale))
		for _, tool := range stale {
			manifest.Remove(tool)
		}
	}

	if len(shown) == 0 {
		if len(args) > 0 {
			fmt.Printf("No packages found for: %s\n", strings.Join(args, " "))
		} else {
			fmt.Println("No packages tracked by sat")
		}
	}

	return nil
}

func matchesFilter(source string, filters []string) bool {
	if len(filters) == 0 {
		return true
	}
	sourceType := manifest.GetSourceType(source)
	display := ui.SourceDisplay(source)
	for _, f := range filters {
		if f == sourceType || f == display {
			return true
		}
	}
	return false
}

// displayGrouped prints entries grouped by display source name, largest
// group first, with the "unknown" group always sorted last.
func displayGrouped(entries []manifest.Entry) {
	groups := make(map[string][]manifest.Entry)
	var order []string
	for _, e := range entries {
		name := ui.SourceDisplay(e.Source)
		if _, ok := groups[name]; !ok {
			order = append(order, name)
		}
		groups[name] = append(groups[name], e)
	}

	sort.SliceStable(order, func(i, j int) bool {
		ci, cj := len(groups[order[i]]), len(groups[order[j]])
		if order[i] == "unknown" {
			ci = 0
		}
		if order[j] == "unknown" {
			cj = 0
		}
		return ci > cj
	})

	for _, name := range order {
		for _, e := range groups[name] {
			ui.DisplayToolEntry(e.Tool, e.Source, "", "")
		}
	}
}
