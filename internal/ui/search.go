package ui

import "strings"

// nameDelimiters are the characters treated as word boundaries when matching
// a search query against a package name (e.g. "rg" should match "ripgrep-rg"
// but not "programming").
var nameDelimiters = []string{"-", "_", "@", "/", "."}

// FilterRelevant filters search results down to those whose name matches the
// query at a word boundary, or whose description contains the query.
// Each result is expected in "name version - description" format.
func FilterRelevant(results []string, query string) []string {
	query = strings.ToLower(query)
	filtered := []string{}

	for _, result := range results {
		parts := strings.SplitN(result, " - ", 2)
		if len(parts) == 0 {
			continue
		}

		name := strings.Fields(parts[0])
		if len(name) == 0 {
			continue
		}

		nameLower := strings.ToLower(name[0])

		if matchesWithDelimiters(nameLower, query) {
			filtered = append(filtered, result)
			continue
		}

		if len(parts) > 1 && strings.Contains(strings.ToLower(parts[1]), query) {
			filtered = append(filtered, result)
		}
	}

	return filtered
}

// matchesWithDelimiters checks if query appears in name with word boundaries
func matchesWithDelimiters(name, query string) bool {
	if name == query {
		return true
	}

	if strings.HasPrefix(name, query) {
		if len(name) > len(query) {
			nextChar := string(name[len(query)])
			for _, d := range nameDelimiters {
				if nextChar == d {
					return true
				}
			}
		}
		return true
	}

	if strings.HasSuffix(name, query) {
		if len(name) > len(query) {
			prevIdx := len(name) - len(query) - 1
			if prevIdx >= 0 {
				prevChar := string(name[prevIdx])
				for _, d := range nameDelimiters {
					if prevChar == d {
						return true
					}
				}
			}
		}
		return true
	}

	for _, d := range nameDelimiters {
		pattern := d + query + d
		if strings.Contains(name, pattern) {
			return true
		}
		if strings.Contains(name, d+query) {
			return true
		}
		if strings.Contains(name, query+d) {
			return true
		}
	}

	return false
}

// ColorizeResult colorizes a search result's package name and dims its
// description. Expects "name version - description" format; falls back to
// coloring the whole string if no description is present.
func ColorizeResult(result, nameColor string) string {
	parts := strings.SplitN(result, " - ", 2)
	if len(parts) != 2 {
		return nameColor + result + Reset
	}

	pre := parts[0]
	desc := parts[1]

	preParts := strings.Fields(pre)
	if len(preParts) == 0 {
		return result
	}

	name := preParts[0]
	version := ""
	if len(preParts) > 1 {
		version = " " + strings.Join(preParts[1:], " ")
	}

	return nameColor + name + Reset + version + Dim + " - " + desc + Reset
}
