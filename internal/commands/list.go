package commands

import (
	"fmt"
	"strings"

	"github.com/DeprecatedLuar/sat/internal/common"
	"github.com/DeprecatedLuar/sat/internal/manifest"
	"github.com/DeprecatedLuar/sat/internal/ui"
)

// List displays tracked packages from the system manifest, optionally
// filtered by source, grouped by source with the largest group first.
// Display only: manifest maintenance belongs to selfheal and scan.
func List(args []string) error {
	var filters []string
	for _, arg := range args {
		sel, ok := common.LookupSourceFlag(arg)
		if !ok {
			return fmt.Errorf("unknown source filter: %s", arg)
		}
		filters = append(filters, sel.ListTokens...)
	}

	entries, err := manifest.All()
	if err != nil {
		return err
	}

	var shown []manifest.Entry
	for _, e := range entries {
		if matchesFilter(e.Source, filters) {
			shown = append(shown, e)
		}
	}

	if len(shown) > 0 {
		displayGrouped(shown)
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

	counts := make(map[string]int, len(groups))
	for name, entries := range groups {
		counts[name] = len(entries)
	}

	for _, name := range ui.GroupedOrder(order, counts) {
		for _, e := range groups[name] {
			ui.DisplayToolEntry(e.Tool, e.Source, "", "")
		}
	}
}
