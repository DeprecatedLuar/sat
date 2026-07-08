package commands

import (
	"fmt"
	"os"
	"strings"
	"sync"

	"github.com/DeprecatedLuar/sat/internal/common"
	"github.com/DeprecatedLuar/sat/internal/sources"
	"github.com/DeprecatedLuar/sat/internal/ui"
)

// searchSources is the canonical list of ecosystems searched by
// searchAllSources, and the order results are displayed in (matches bash's
// display order). Single source of truth for both concerns.
var searchSources = []string{"system", "brew", "nix", "rust", "python", "node", "github", "flatpak"}

// Search searches for packages across multiple ecosystems
func Search(args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("usage: sat search <program>[:source] [--wrap] [--all]")
	}

	// Parse flags and query
	var query string
	var sourceFilter string
	noWrap := true
	filter := true

	for _, arg := range args {
		switch arg {
		case "--wrap":
			noWrap = false
		case "--all":
			filter = false
		default:
			// Parse source specifier (query:source)
			if strings.Contains(arg, ":") {
				parts := strings.Split(arg, ":")
				query = parts[0]
				if len(parts) > 1 {
					sourceFilter = parts[len(parts)-1]
				}
			} else {
				if query == "" {
					query = arg
				} else {
					query += "-" + arg // Normalize spaces to hyphens
				}
			}
		}
	}

	if query == "" {
		return fmt.Errorf("usage: sat search <program>[:source] [--wrap] [--all]")
	}

	// Get terminal width
	termWidth := getTerminalWidth()
	contentWidth := termWidth - 4

	// Print header
	printSearchHeader(query, termWidth)

	// Single source search
	if sourceFilter != "" {
		return searchSingleSource(query, sourceFilter, filter, contentWidth)
	}

	// Multi-source parallel search
	return searchAllSources(query, filter, noWrap, contentWidth)
}

// searchSingleSource searches a specific ecosystem
func searchSingleSource(query, source string, filterResults bool, width int) error {
	// Map source aliases to canonical names
	sourceName := mapSourceAlias(source)
	if sourceName == "" {
		return fmt.Errorf("unknown source: %s", source)
	}

	// Execute search
	results, err := executeSearch(sourceName, query)
	if err != nil || len(results) == 0 {
		if sourceName == "pypi" {
			fmt.Printf("No exact match on pypi for %q (pypi only supports exact package-name lookups, not fuzzy search — check the other ecosystems' results for the real name)\n", query)
		} else {
			fmt.Printf("No results found in %s\n", sourceName)
		}
		return nil
	}

	// Filter if requested
	if filterResults {
		results = ui.FilterRelevant(results, query)
	}

	if len(results) == 0 {
		fmt.Printf("No relevant results found in %s\n", sourceName)
		return nil
	}

	// Display results
	displaySourceResults(sourceName, results, width)
	return nil
}

// searchAllSources searches all available ecosystems in parallel
func searchAllSources(query string, filterResults, noWrap bool, width int) error {
	// Results map and mutex
	resultsMap := make(map[string][]string)
	var mu sync.Mutex
	var wg sync.WaitGroup

	// Launch parallel searches
	for _, src := range searchSources {
		wg.Add(1)
		go func(source string) {
			defer wg.Done()

			results, err := executeSearch(source, query)
			if err != nil || len(results) == 0 {
				return
			}

			// Filter if requested
			if filterResults {
				results = ui.FilterRelevant(results, query)
			}

			if len(results) > 0 {
				mu.Lock()
				resultsMap[source] = results
				mu.Unlock()
			}
		}(src)
	}

	// Wait for all searches to complete
	wg.Wait()

	// Display results in bash-compatible order
	hasResults := false

	for _, src := range searchSources {
		if results, ok := resultsMap[src]; ok && len(results) > 0 {
			hasResults = true
			displaySourceResults(src, results, width)
			fmt.Println()
		}
	}

	if !hasResults {
		fmt.Println("No results found")
	}

	return nil
}

// executeSearch calls the appropriate search function for a source
func executeSearch(source, query string) ([]string, error) {
	debugf("dispatching search: source=%s query=%s", source, query)

	var results []string
	var err error

	switch source {
	case "system":
		results, err = sources.Search(query)
	case "cargo", "rust":
		results, err = sources.CargoSearch(query)
	case "brew", "homebrew":
		results, err = sources.BrewSearch(query)
	case "nix":
		results, err = sources.NixSearch(query)
	case "npm", "node", "js":
		results, err = sources.NpmSearch(query)
	case "pypi", "python", "py", "uv":
		results, err = sources.PyPISearch(query)
	case "github", "gh", "repo":
		results, err = sources.GitHubSearch(query)
	case "flatpak", "flathub":
		results, err = sources.FlatpakSearch(query)
	default:
		return nil, fmt.Errorf("search not implemented for %s", source)
	}

	if err != nil {
		debugf("  %s: error: %v", source, err)
	} else {
		debugf("  %s: %d result(s)", source, len(results))
	}
	return results, err
}

// debugf prints a [debug]-prefixed trace line to stderr when SAT_DEBUG is
// set, matching bash's documented "sat --debug search shows fallback chain"
// behavior.
func debugf(format string, args ...any) {
	if os.Getenv(common.EnvSATDebug) != "" {
		fmt.Fprintf(os.Stderr, "%s "+format+"\n", append([]any{common.DebugPrefix}, args...)...)
	}
}

// displaySourceResults formats and displays results for a source
func displaySourceResults(source string, results []string, width int) {
	// Get color for source
	sourceStr := source + "::"
	color := ui.SourceColor(sourceStr)
	light := ui.SourceLight(sourceStr)

	// Print source header
	fmt.Printf("%s%s:%s\n", color, source, ui.Reset)

	// Print each result
	for _, result := range results {
		// Truncate if needed
		if len(result) > width {
			result = result[:width]
		}

		// Colorize and display
		formatted := ui.ColorizeResult(result, light)
		fmt.Printf("  %s\n", formatted)
	}
}

// mapSourceAlias maps source aliases to canonical names
func mapSourceAlias(source string) string {
	switch source {
	case "gh", "github", "repo":
		return "github"
	case "sys", "system", "apt", "pacman", "dnf", "apk":
		return "system"
	case "rs", "rust", "cargo":
		return "cargo"
	case "js", "node", "npm":
		return "npm"
	case "py", "python", "uv", "pypi":
		return "pypi"
	case "brew", "homebrew":
		return "brew"
	case "nix":
		return "nix"
	case "flatpak", "flathub":
		return "flatpak"
	default:
		return ""
	}
}

// printSearchHeader prints the search header
func printSearchHeader(query string, width int) {
	header := fmt.Sprintf("──[%s]", strings.ToUpper(query))
	padding := width - len(header)
	if padding < 0 {
		padding = 0
	}

	fmt.Printf("%s%s\n\n", header, strings.Repeat("─", padding))
}

// getTerminalWidth returns the terminal width or 80 as fallback
func getTerminalWidth() int {
	width, err := common.GetTerminalWidth()
	if err != nil || width == 0 {
		return 80
	}
	return width
}
