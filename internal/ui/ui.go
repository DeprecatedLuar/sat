package ui

import (
	"fmt"
	"sort"
	"strings"

	"github.com/DeprecatedLuar/sat/internal/common"
	"github.com/DeprecatedLuar/sat/internal/manifest"
)

const (
	// Status symbols
	check = "\033[0;92m✓\033[0m" // Green checkmark
	cross = "\033[0;91m✗\033[0m" // Red X
	bang  = "\033[0;93m!\033[0m" // Yellow bang

	// Display width constants
	StatusWidth   = 40 // Width for status messages
	ToolNameWidth = 20 // Width for tool names in list

	// reasonIndent/reasonContinueIndent are the hanging indents for
	// StatusError's "└ reason" line, matching RenderMultiline's convention
	// in search.go but shifted to sit under the tool name past "[x] ".
	reasonIndent         = "    └ "
	reasonContinueIndent = "      "
)

// sourceInfo bundles the color, pastel light-color, and display name for a
// source type, so the three can't drift out of sync the way three parallel
// switch statements did (SourceLight was missing the "unknown" case that
// SourceColor had).
type sourceInfo struct {
	color   string
	light   string
	display string
}

// sourceTable maps every known source-type alias to its display info.
// Multiple keys intentionally share the same sourceInfo value (e.g.
// "cargo"/"rust" are aliases for the same ecosystem).
var sourceTable = buildSourceTable()

func buildSourceTable() map[string]sourceInfo {
	groups := []struct {
		aliases []string
		info    sourceInfo
	}{
		{[]string{"cargo", "rust"}, sourceInfo{Rust, RustLight, "rust"}},
		{[]string{"npm", "node"}, sourceInfo{Node, NodeLight, "node"}},
		{[]string{"uv", "pip", "python"}, sourceInfo{Python, PythonLight, "python"}},
		{[]string{"apt", "apk", "pacman", "dnf", "pkg", "system"}, sourceInfo{System, SystemLight, "system"}},
		{[]string{"flatpak", "flathub"}, sourceInfo{Flatpak, FlatpakLight, "flatpak"}},
		{[]string{"github", "gh"}, sourceInfo{Repo, RepoLight, "github"}},
		{[]string{"huber"}, sourceInfo{Repo, RepoLight, "huber"}},
		{[]string{"appimage"}, sourceInfo{AppImage, AppImageLight, "appimage"}},
		{[]string{"sat"}, sourceInfo{Sat, Sat, "sat"}}, // no light variant defined, falls back to Sat
		{[]string{"go"}, sourceInfo{Go, GoLight, "go"}},
		{[]string{"brew"}, sourceInfo{Brew, BrewLight, "brew"}},
		{[]string{"nix", "nixos"}, sourceInfo{Nix, NixLight, "nix"}},
		{[]string{"manual"}, sourceInfo{Manual, ManualLight, "manual"}},
		{[]string{"unknown"}, sourceInfo{Dim, Reset, "unknown"}},
	}

	table := make(map[string]sourceInfo)
	for _, g := range groups {
		for _, alias := range g.aliases {
			table[alias] = g.info
		}
	}
	return table
}

// SourceColor returns the ANSI color code for a source string
func SourceColor(sourceStr string) string {
	if info, ok := sourceTable[manifest.GetSourceType(sourceStr)]; ok {
		return info.color
	}
	return Reset
}

// SourceLight returns the pastel/light color for a source string
func SourceLight(sourceStr string) string {
	if info, ok := sourceTable[manifest.GetSourceType(sourceStr)]; ok {
		return info.light
	}
	return Reset
}

// SourceDisplay returns the human-friendly display name for a source type
func SourceDisplay(sourceStr string) string {
	sourceType := manifest.GetSourceType(sourceStr)
	if info, ok := sourceTable[sourceType]; ok {
		return info.display
	}
	return sourceType
}

// Status prints a simple status message
func Status(msg string) {
	fmt.Printf("\r%-*s\n", StatusWidth, msg)
}

// statusLine is the single layout for every tool status line: marker,
// styled+padded tool name, source tag. The spinner's rotating frame and the
// ✓/✗ completion markers all render through this, so a line never reflows
// when its task finishes. marker and nameStyle are expected to already carry
// their own ANSI codes (e.g. check/cross/a spinner frame color for marker;
// SourceLight(sourceStr) for the common case, or Dim+Strikethrough to mark a
// tool as removed, for nameStyle).
func statusLine(marker, nameStyle, tool, sourceStr string) string {
	return fmt.Sprintf("\r[%s] %s%-*s%s [%s%s%s]",
		marker,
		nameStyle, ToolNameWidth, TruncateName(tool, ToolNameWidth), Reset,
		SourceColor(sourceStr), SourceDisplay(sourceStr), Reset)
}

// TermWidth returns the terminal width, or 80 if it can't be determined.
func TermWidth() int {
	width, err := common.GetTerminalWidth()
	if err != nil || width == 0 {
		return 80
	}
	return width
}

// StatusOK prints a success line for tool via a source tagged sourceStr.
func StatusOK(tool, sourceStr string) {
	fmt.Println(statusLine(check, SourceLight(sourceStr), tool, sourceStr))
}

// StatusRemoved prints a success line for a completed removal: same layout
// as StatusOK, but the tool name renders dim + struck through instead of in
// its source color, to visually distinguish "this is gone" from "here it is".
func StatusRemoved(tool, sourceStr string) {
	fmt.Println(statusLine(check, Dim+Strikethrough, tool, sourceStr))
}

// formatReason word-wraps reason under a dim hanging "└ " indent (matching
// RenderMultiline's convention in search.go), at the given width. Returns ""
// for an empty reason, so the caller can skip printing it entirely.
func formatReason(reason string, width int) string {
	if reason == "" {
		return ""
	}

	lines := wrapWords(reason, width-len(reasonContinueIndent))
	if len(lines) == 0 {
		return ""
	}

	var b strings.Builder
	b.WriteString(Dim + reasonIndent + lines[0])
	for _, line := range lines[1:] {
		b.WriteString("\n" + reasonContinueIndent + line)
	}
	b.WriteString(Reset)
	return b.String()
}

// StatusError prints a failure line for tool via a source tagged sourceStr,
// with reason word-wrapped underneath on a dim hanging "└ " indent. An empty
// reason omits that line.
func StatusError(tool, sourceStr, reason string) {
	fmt.Println(statusLine(cross, SourceLight(sourceStr), tool, sourceStr))
	if line := formatReason(reason, TermWidth()); line != "" {
		fmt.Println(line)
	}
}

// StatusFail prints a free-form failure message with cross, for cases with
// no tool/source pair to render (e.g. an untracked tool, or a multi-line
// error like AmbiguousMatchError that already owns its own layout).
func StatusFail(msg string) {
	fmt.Printf("\r[%s] %s\n", cross, msg)
}

// Warn prints a non-fatal, actionable notice. Unlike [debug]-gated logging,
// this is always visible since it tells the user how to fix something
// (e.g. a missing optional dependency), not just what happened internally.
func Warn(msg string) {
	fmt.Printf("\r[%s] %s\n", bang, msg)
}

// GroupedOrder sorts keys (already deduplicated, in first-seen order so
// ties stay deterministic) by descending count, with the "unknown" key
// always sorted last regardless of its count. counts maps each key to how
// many items share it. keys is sorted in place and also returned.
func GroupedOrder(keys []string, counts map[string]int) []string {
	sort.SliceStable(keys, func(i, j int) bool {
		ci, cj := counts[keys[i]], counts[keys[j]]
		if keys[i] == "unknown" {
			ci = 0
		}
		if keys[j] == "unknown" {
			cj = 0
		}
		return ci > cj
	})
	return keys
}

// TruncateName shortens name to width, replacing the tail with a single
// ellipsis character when it doesn't fit, so long names never break
// column alignment.
func TruncateName(name string, width int) string {
	if len(name) <= width {
		return name
	}
	if width <= 1 {
		return "…"
	}
	return name[:width-1] + "…"
}

// DisplayToolEntry prints a formatted tool entry with optional prefix/suffix
// Format:   prefix + tool_name (ToolNameWidth chars) + [color source] + version + suffix
func DisplayToolEntry(prog, sourceStr, prefix, suffix string) {
	display := SourceDisplay(sourceStr)
	color := SourceColor(sourceStr)
	version := manifest.GetSourceVersion(sourceStr)

	if version != "" {
		fmt.Printf("  %s%-*s [%s%s%s] %s%s%s%s\n",
			prefix, ToolNameWidth, prog, color, display, Reset, Dim, version, Reset, suffix)
	} else {
		fmt.Printf("  %s%-*s [%s%s%s]%s\n",
			prefix, ToolNameWidth, prog, color, display, Reset, suffix)
	}
}
