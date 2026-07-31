package ui

import (
	"strings"
	"unicode/utf8"
)

// resultIndent is the leading indent the caller (displaySourceResults)
// prepends to every rendered result via fmt.Printf("  %s\n", ...).
const resultIndent = 2

// MinInlineDescCol is the minimum space left for a description on the name
// line for RenderMultiline to render it inline instead of on a hanging
// "└ " line. Keeps a long name from squeezing even a short description into
// an orphaned fragment jammed against the right edge.
const MinInlineDescCol = 24

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

// ParseResult splits a search result in "name version - description" format
// into its name, version, and description parts. desc is "" when no
// description is present.
func ParseResult(result string) (name, version, desc string) {
	parts := strings.SplitN(result, " - ", 2)
	pre := parts[0]
	if len(parts) == 2 {
		desc = parts[1]
	}

	preParts := strings.Fields(pre)
	if len(preParts) == 0 {
		return "", "", desc
	}

	name = preParts[0]
	version = strings.Join(preParts[1:], " ")
	return name, version, desc
}

// ColorizeResult colorizes a search result's package name and dims its
// description. Expects "name version - description" format; falls back to
// coloring the whole string if no description is present.
func ColorizeResult(result, nameColor string) string {
	parts := strings.SplitN(result, " - ", 2)
	if len(parts) != 2 {
		return nameColor + result + Reset
	}

	name, version, desc := ParseResult(result)
	if name == "" {
		return result
	}

	versionPart := ""
	if version != "" {
		versionPart = " " + version
	}

	return nameColor + name + Reset + versionPart + Dim + " - " + desc + Reset
}

// wrapWords greedily word-wraps text to the given width, never breaking a
// word mid-way. width is guarded to at least 1.
func wrapWords(text string, width int) []string {
	if width < 1 {
		width = 1
	}

	words := strings.Fields(text)
	if len(words) == 0 {
		return []string{}
	}

	lines := []string{}
	current := words[0]
	for _, word := range words[1:] {
		if len(current)+1+len(word) <= width {
			current += " " + word
		} else {
			lines = append(lines, current)
			current = word
		}
	}
	lines = append(lines, current)
	return lines
}

// RenderMultiline renders a search result as a multi-line block: the name
// (colored nameColor) and version on the first line, and the full
// description word-wrapped below it under a hanging "└ " indent. width is
// the full terminal width; the wrap width is width-8 for the indent.
// Results with no description render as just the name line.
func RenderMultiline(result, nameColor string, width int) string {
	name, version, desc := ParseResult(result)

	versionPart := ""
	if version != "" {
		versionPart = " " + version
	}
	nameLine := nameColor + name + Reset + versionPart

	if desc == "" {
		return nameLine
	}

	available := width - resultIndent - utf8.RuneCountInString(name) - utf8.RuneCountInString(versionPart) - len(" - ")
	if available >= MinInlineDescCol && utf8.RuneCountInString(desc) <= available {
		return ColorizeResult(result, nameColor)
	}

	wrapWidth := width - 8
	lines := wrapWords(desc, wrapWidth)

	var b strings.Builder
	b.WriteString(nameLine)
	b.WriteString("\n      " + Dim + "└ " + lines[0])
	for _, line := range lines[1:] {
		b.WriteString("\n        " + line)
	}
	b.WriteString(Reset)

	return b.String()
}
