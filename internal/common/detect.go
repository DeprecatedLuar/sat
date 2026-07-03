package common

import (
	"os/exec"
	"path/filepath"
	"strings"
)

const (
	// Tool spec parsing
	ToolSpecDelimiter  = ":"
	ToolSpecFieldCount = 2

	// Source type aliases
	AliasRs      = "rs"
	AliasRust    = "rust"
	AliasPy      = "py"
	AliasPython  = "python"
	AliasJs      = "js"
	AliasNode    = "node"
	AliasSys     = "sys"
	AliasSystem  = "system"
	AliasFpk     = "fpk"
	AliasFlatpak = "flatpak"
	AliasGh      = "gh"
	AliasGithub  = "github"
	AliasRelease = "release"
	AliasRel     = "rel"

	// Source types
	SourceCargo     = "cargo"
	SourceUV        = "uv"
	SourceNPM       = "npm"
	SourceGo        = "go"
	SourceBrew      = "brew"
	SourceNix       = "nix"
	SourceFlatpak   = "flatpak"
	SourceGH        = "gh"
	SourceGHRelease = "gh-release"
	SourceSystem    = "system"
	SourceAppImage  = "appimage"
	SourceManual    = "manual"

	// Installation path patterns
	PathCargo1      = "/.cargo/bin/"
	PathCargo2      = "/dev-tools/cargo/bin/"
	PathUV1         = "/.local/share/uv/"
	PathUV2         = "/dev-tools/uv/"
	PathNPM1        = "/node_modules/.bin/"
	PathNPM2        = "/.npm/"
	PathNPM3        = "/dev-tools/npm/"
	PathGo1         = "/go/bin/"
	PathGo2         = "/dev-tools/go/bin/"
	PathBrew1       = "/Cellar/"
	PathBrew2       = "/Homebrew/"
	PathBrew3       = "/linuxbrew/"
	PathNix1        = "/.nix-profile/"
	PathNix2        = "/nix/store/"
	PathFlatpak     = "/flatpak/"
	PathUsrBin      = "/usr/bin/"
	PathUsrLocalBin = "/usr/local/bin/"
	PathBin         = "/bin/"
	PathSbin        = "/sbin/"
)

// ParseToolSpec parses tool:source syntax (e.g., "ranger:py" -> tool=ranger, source=uv)
// Returns tool name and source type (empty string if no source specified)
func ParseToolSpec(spec string) (name, source string) {
	if !strings.Contains(spec, ToolSpecDelimiter) {
		return spec, ""
	}

	parts := strings.SplitN(spec, ToolSpecDelimiter, ToolSpecFieldCount)
	name = parts[0]
	src := parts[1]

	// Map aliases to actual sources
	switch src {
	case AliasPy, AliasPython:
		source = SourceUV
	case AliasRs, AliasRust:
		source = SourceCargo
	case AliasJs, AliasNode:
		source = SourceNPM
	case AliasSys, AliasSystem:
		source = SourceSystem
	case SourceGo:
		source = SourceGo
	case SourceBrew:
		source = SourceBrew
	case SourceNix:
		source = SourceNix
	case AliasFpk, AliasFlatpak:
		source = SourceFlatpak
	case AliasGh, AliasGithub:
		source = SourceGH
	case AliasRelease, AliasRel:
		source = SourceGHRelease
	default:
		source = src
	}

	return name, source
}

// DetectSource detects the source of an installed tool from its binary location
func DetectSource(tool string) string {
	binPath, err := exec.LookPath(tool)
	if err != nil {
		return ""
	}

	// Resolve symlinks to find actual location
	realPath, err := filepath.EvalSymlinks(binPath)
	if err != nil {
		realPath = binPath
	}

	// Match against common installation paths
	switch {
	case strings.Contains(realPath, PathCargo1),
		strings.Contains(realPath, PathCargo2):
		return SourceCargo

	case strings.Contains(realPath, PathUV1),
		strings.Contains(realPath, PathUV2):
		return SourceUV

	case strings.Contains(realPath, PathNPM1),
		strings.Contains(realPath, PathNPM2),
		strings.Contains(realPath, PathNPM3):
		return SourceNPM

	case strings.Contains(realPath, PathGo1),
		strings.Contains(realPath, PathGo2):
		return SourceGo

	case strings.Contains(realPath, PathBrew1),
		strings.Contains(realPath, PathBrew2),
		strings.Contains(realPath, PathBrew3):
		return SourceBrew

	case strings.Contains(realPath, PathNix1),
		strings.Contains(realPath, PathNix2):
		return SourceNix

	case strings.Contains(realPath, PathFlatpak):
		return SourceFlatpak

	case strings.Contains(realPath, AppImagesDir()):
		return SourceAppImage

	case strings.HasPrefix(realPath, PathUsrBin),
		strings.HasPrefix(realPath, PathUsrLocalBin),
		strings.HasPrefix(realPath, PathBin),
		strings.HasPrefix(realPath, PathSbin):
		return SourceSystem

	case strings.Contains(realPath, "/.local/opt/"):
		return SourceManual

	default:
		return "unknown"
	}
}
