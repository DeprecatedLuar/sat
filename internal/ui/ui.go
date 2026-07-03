package ui

import (
	"fmt"

	"github.com/DeprecatedLuar/sat/internal/manifest"
)

const (
	// Status symbols
	Check = "\033[0;92m✓\033[0m" // Green checkmark
	Cross = "\033[0;91m✗\033[0m" // Red X

	// Display width constants
	StatusWidth     = 40  // Width for status messages
	MessageWidth    = 25  // Width for tool messages
	ToolNameWidth   = 20  // Width for tool names in list
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

// StatusOK prints a success message with checkmark and source tag
func StatusOK(msg, sourceStr string) {
	display := SourceDisplay(sourceStr)
	color := SourceColor(sourceStr)
	fmt.Printf("\r[%s] %-*s [%s%s%s]\n", Check, MessageWidth, msg, color, display, Reset)
}

// StatusFail prints a failure message with cross
func StatusFail(msg string) {
	fmt.Printf("\r[%s] %s\n", Cross, msg)
}

// DisplayToolEntry prints a formatted tool entry with optional prefix/suffix
// Format:   prefix + tool_name (ToolNameWidth chars) + [color source] + version + suffix
func DisplayToolEntry(prog, sourceStr, prefix, suffix string) {
	display := SourceDisplay(sourceStr)
	color := SourceColor(sourceStr)
	version := manifest.GetSourceVersion(sourceStr)

	if version != "" {
		fmt.Printf("  %s%-*s [%s%s%s] %sv%s%s%s\n",
			prefix, ToolNameWidth, prog, color, display, Reset, Dim, version, Reset, suffix)
	} else {
		fmt.Printf("  %s%-*s [%s%s%s]%s\n",
			prefix, ToolNameWidth, prog, color, display, Reset, suffix)
	}
}
