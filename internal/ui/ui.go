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

// SourceColor returns the ANSI color code for a source string
func SourceColor(sourceStr string) string {
	sourceType := manifest.GetSourceType(sourceStr)

	switch sourceType {
	case "cargo", "rust":
		return Rust
	case "npm", "node":
		return Node
	case "uv", "pip", "python":
		return Python
	case "apt", "apk", "pacman", "dnf", "pkg", "system":
		return System
	case "flatpak", "flathub":
		return Flatpak
	case "github", "gh":
		return Repo
	case "appimage":
		return AppImage
	case "sat":
		return Sat
	case "go":
		return Go
	case "brew":
		return Brew
	case "nix", "nixos":
		return Nix
	case "manual":
		return Manual
	case "unknown":
		return Dim
	default:
		return Reset
	}
}

// SourceLight returns the pastel/light color for a source string
func SourceLight(sourceStr string) string {
	sourceType := manifest.GetSourceType(sourceStr)

	switch sourceType {
	case "cargo", "rust":
		return RustLight
	case "npm", "node":
		return NodeLight
	case "uv", "pip", "python":
		return PythonLight
	case "apt", "apk", "pacman", "dnf", "pkg", "system":
		return SystemLight
	case "flatpak", "flathub":
		return FlatpakLight
	case "github", "gh":
		return RepoLight
	case "appimage":
		return AppImageLight
	case "sat":
		return Sat // No light variant defined
	case "go":
		return GoLight
	case "brew":
		return BrewLight
	case "nix", "nixos":
		return NixLight
	case "manual":
		return ManualLight
	default:
		return Reset
	}
}

// SourceDisplay returns the human-friendly display name for a source type
func SourceDisplay(sourceStr string) string {
	sourceType := manifest.GetSourceType(sourceStr)

	switch sourceType {
	case "cargo", "rust":
		return "rust"
	case "npm", "node":
		return "node"
	case "uv", "pip", "python":
		return "python"
	case "apt", "apk", "pacman", "dnf", "pkg":
		return "system"
	case "flatpak", "flathub":
		return "flatpak"
	case "github", "gh":
		return "github"
	case "appimage":
		return "appimage"
	case "sat":
		return "sat"
	case "go":
		return "go"
	case "brew":
		return "brew"
	case "nix", "nixos":
		return "nix"
	case "manual":
		return "manual"
	default:
		return sourceType
	}
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
