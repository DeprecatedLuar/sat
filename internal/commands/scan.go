package commands

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/DeprecatedLuar/sat/internal/common"
	"github.com/DeprecatedLuar/sat/internal/manifest"
	"github.com/DeprecatedLuar/sat/internal/scan"
	"github.com/DeprecatedLuar/sat/internal/sources"
	"github.com/DeprecatedLuar/sat/internal/ui"
)

const (
	// Environment variables
	EnvHome          = "HOME"
	EnvCargoHome     = "CARGO_HOME"
	EnvNPMPrefix     = "NPM_CONFIG_PREFIX"

	// Default paths
	DefaultCargoHome  = ".cargo"
	DefaultNPMGlobal  = ".npm-global"
	LocalShareDir     = ".local/share"
	LocalBinDir       = ".local/bin"
	NixProfileBin     = ".nix-profile/bin"

	// Subdirectories
	BinSubdir         = "bin"
	UVToolsSubdir     = "uv/tools"
	SATSubdir         = "sat"
	AppImagesSubdir   = "appimages"

	// Source types
	SourceCargo    = "cargo"
	SourceNPM      = "npm"
	SourceUV       = "uv"
	SourceGo       = "go"
	SourceBrew     = "brew"
	SourceNix      = "nix"
	SourceFlatpak  = "flatpak"
	SourceAppImage = "appimage"
	SourceUnknown  = "unknown"
	SourceSystem   = "system"
	SourceAPT      = "apt"
	SourcePacman   = "pacman"
	SourceAPK      = "apk"
	SourceDNF      = "dnf"

	// Brew list pattern
	BrewBinPattern = "/bin/"

	// Flatpak app ID parsing
	FlatpakIDSeparator = "."

	// Manifest format
	CommentPrefix  = "#"
	ManifestDelim  = "="
	FieldCount     = 2

	// Executable permission mask
	ExecutableMask = 0111

	// Cleanup reasons
	ReasonExcluded = "excluded"
	ReasonBrewDep  = "brew dep"
	ReasonSymlink  = "symlink"
)

// Scan scans ecosystems and populates the manifest
func Scan() error {
	fmt.Println("Scanning ecosystems...")

	added := 0

	// Clean up manifest before scanning
	pruned := cleanupManifest()

	// Scan directory-based sources
	added += scanDir(SourceCargo, cargoBinDir())
	added += scanDir(SourceNPM, npmBinDir())
	added += scanDir(SourceUV, filepath.Join(os.Getenv(EnvHome), LocalShareDir, UVToolsSubdir))
	added += scanDir(SourceGo, goBinDir())

	// Scan Homebrew
	added += scanBrew()

	// Scan Nix profile
	added += scanNix()

	// Scan system packages
	added += scanSystem()

	// Scan Flatpak (simplified - no wrapper creation yet)
	added += scanFlatpak()

	// Scan AppImages (simplified - no repo extraction yet)
	added += scanAppImages()

	// Scan local bin
	added += scanLocalBin()

	fmt.Println()
	if pruned > 0 {
		fmt.Printf("Pruned %d entries\n", pruned)
	}
	fmt.Printf("Added %d packages to manifest\n", added)

	return nil
}

// tryAddTool attempts to add a tool to the manifest
// Returns true if added, false if skipped
func tryAddTool(prog, srcInput string) bool {
	// Parse source string (might already include identity)
	sourceType := manifest.GetSourceType(srcInput)
	identity := manifest.GetSourceIdentity(srcInput)

	// Skip if excluded or already tracked
	if scan.IsExcluded(prog, sourceType) {
		return false
	}
	if manifest.Has(prog) {
		return false
	}
	// TODO: Check master manifest for shell sessions (Phase 12)

	// Detect identity (for sources that need it)
	// For now, we'll skip complex identity detection
	// This will be enhanced in later phases

	// Detect version based on source type
	version := manifest.GetSourceVersion(srcInput)
	if version == "" {
		version = getVersionForSource(prog, sourceType, identity)
	}

	// For system package managers: skip if no version (auxiliary tools)
	switch sourceType {
	case SourceAPT, SourcePacman, SourceAPK, SourceDNF:
		if version == "" {
			return false
		}
	}

	// Build full source string and add to manifest
	srcString := manifest.BuildSourceString(sourceType, identity, version)
	if err := manifest.Add(prog, srcString); err != nil {
		return false
	}

	// Display with version
	color := ui.SourceColor(srcString)
	ui.DisplayToolEntry(prog, srcString, color+"+"+ui.Reset+" ", "")
	return true
}

// getVersionForSource gets version for a given tool and source
func getVersionForSource(prog, sourceType, identity string) string {
	switch sourceType {
	case SourceCargo:
		return sources.CargoGetVersion(prog)
	case SourceBrew:
		return sources.BrewGetVersion(prog)
	case SourceNix:
		return sources.NixGetVersion(prog)
	case "nixos":
		return sources.NixOSGetVersion(prog)
	case SourceAPT, SourcePacman, SourceAPK, SourceDNF, SourceSystem:
		return sources.GetVersion(prog)
	// TODO: Add more sources as they're implemented in later phases
	// case SourceNPM: return sources.NPMGetVersion(prog)
	// case SourceUV: return sources.UVGetVersion(prog)
	// case SourceGo: return sources.GoGetVersion(prog)
	// case SourceFlatpak: return sources.FlatpakGetVersion(identity)
	}
	return ""
}

// cleanupManifest removes stale manifest entries
func cleanupManifest() int {
	// Get brew leaves for dependency detection
	brewLeaves := make(map[string]bool)
	if _, err := exec.LookPath("brew"); err == nil {
		var output strings.Builder
		cmd := exec.Command("brew", "leaves")
		cmd.Stdout = &output
		if cmd.Run() == nil {
			for _, leaf := range strings.Split(output.String(), "\n") {
				leaf = strings.TrimSpace(leaf)
				if leaf != "" {
					brewLeaves[leaf] = true
				}
			}
		}
	}

	pruned := 0
	manifestPath := manifest.ManifestPath()
	entries, err := readManifestEntries(manifestPath)
	if err != nil {
		return 0
	}

	for prog, srcString := range entries {
		shouldPrune := false
		reason := ""

		sourceType := manifest.GetSourceType(srcString)

		// Check exclusion patterns
		if scan.IsExcluded(prog, sourceType) {
			shouldPrune = true
			reason = ReasonExcluded
		} else if sourceType == SourceBrew && len(brewLeaves) > 0 {
			// Check brew deps (not in leaves)
			if !brewLeaves[prog] {
				shouldPrune = true
				reason = ReasonBrewDep
			}
		} else {
			// Check if managed as symlink in ~/.local/bin (dotfiles, etc.)
			// But exclude sat-managed symlinks (appimages, flatpak wrappers)
			localBin := filepath.Join(os.Getenv(EnvHome), LocalBinDir, prog)
			if info, err := os.Lstat(localBin); err == nil && info.Mode()&os.ModeSymlink != 0 {
				target, _ := filepath.EvalSymlinks(localBin)
				satDir := filepath.Join(os.Getenv(EnvHome), LocalShareDir, SATSubdir) + string(filepath.Separator)
				if !strings.HasPrefix(target, satDir) {
					shouldPrune = true
					reason = ReasonSymlink
				}
			}
		}

		if shouldPrune {
			manifest.Remove(prog)
			fmt.Printf("  %s- %-20s (%s)%s\n", ui.Dim, prog, reason, ui.Reset)
			pruned++
		}
	}

	return pruned
}

// scanDir scans a directory for binaries from a specific source
func scanDir(source, dir string) int {
	if dir == "" || !dirExists(dir) {
		return 0
	}

	added := 0
	entries, err := os.ReadDir(dir)
	if err != nil {
		return 0
	}

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		info, err := entry.Info()
		if err != nil || !isExecutable(info) {
			continue
		}

		prog := entry.Name()
		if tryAddTool(prog, source) {
			added++
		}
	}

	return added
}

// scanBrew scans Homebrew explicit installs
func scanBrew() int {
	if _, err := exec.LookPath("brew"); err != nil {
		return 0
	}

	added := 0

	// Get brew leaves (explicit installs)
	var leavesOutput strings.Builder
	leavesCmd := exec.Command("brew", "leaves")
	leavesCmd.Stdout = &leavesOutput

	if leavesCmd.Run() != nil {
		return 0
	}

	for _, formula := range strings.Split(leavesOutput.String(), "\n") {
		formula = strings.TrimSpace(formula)
		if formula == "" {
			continue
		}

		// Get actual binaries installed by this formula
		var listOutput strings.Builder
		listCmd := exec.Command("brew", "list", formula)
		listCmd.Stdout = &listOutput

		if listCmd.Run() != nil {
			continue
		}

		for _, line := range strings.Split(listOutput.String(), "\n") {
			if !strings.Contains(line, BrewBinPattern) {
				continue
			}
			prog := filepath.Base(strings.TrimSpace(line))
			if tryAddTool(prog, SourceBrew) {
				added++
			}
		}
	}

	return added
}

// scanNix scans Nix user profile
func scanNix() int {
	nixProfile := filepath.Join(os.Getenv(EnvHome), NixProfileBin)
	if !dirExists(nixProfile) {
		return 0
	}

	return scanDir(SourceNix, nixProfile)
}

// scanSystem scans system packages
func scanSystem() int {
	packages, err := scan.ScanSystemPackages()
	if err != nil {
		return 0
	}

	added := 0
	for _, pkg := range packages {
		if tryAddTool(pkg.Name, pkg.Source) {
			added++
		}
	}

	return added
}

// scanFlatpak scans Flatpak user apps (simplified)
func scanFlatpak() int {
	if _, err := exec.LookPath("flatpak"); err != nil {
		return 0
	}

	added := 0

	var output strings.Builder
	cmd := exec.Command("flatpak", "list", "--app", "--columns=application")
	cmd.Stdout = &output

	if cmd.Run() != nil {
		return 0
	}

	for _, appID := range strings.Split(output.String(), "\n") {
		appID = strings.TrimSpace(appID)
		if appID == "" {
			continue
		}

		// Use last component as prog name (org.gimp.GIMP → gimp)
		parts := strings.Split(appID, FlatpakIDSeparator)
		prog := strings.ToLower(parts[len(parts)-1])

		// Build source string with app ID as identity
		srcString := manifest.BuildSourceString(SourceFlatpak, appID, "")
		if tryAddTool(prog, srcString) {
			added++
			// TODO: Create flatpak wrapper (Phase 5)
		}
	}

	return added
}

// scanAppImages scans AppImage directory (simplified)
func scanAppImages() int {
	appimageDir := filepath.Join(os.Getenv(EnvHome), LocalShareDir, SATSubdir, BinSubdir, AppImagesSubdir)
	if !dirExists(appimageDir) {
		return 0
	}

	added := 0
	entries, err := os.ReadDir(appimageDir)
	if err != nil {
		return 0
	}

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		info, err := entry.Info()
		if err != nil || !isExecutable(info) {
			continue
		}

		prog := entry.Name()

		// TODO: Extract GitHub repo from AppImage (Phase 6)
		// For now, track without repo info
		if tryAddTool(prog, SourceAppImage) {
			added++
		}
	}

	return added
}

// scanLocalBin scans ~/.local/bin for unknown sources
func scanLocalBin() int {
	localBin := filepath.Join(os.Getenv(EnvHome), LocalBinDir)
	if !dirExists(localBin) {
		return 0
	}

	added := 0
	entries, err := os.ReadDir(localBin)
	if err != nil {
		return 0
	}

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		// Skip symlinks (managed elsewhere)
		binPath := filepath.Join(localBin, entry.Name())
		if info, err := os.Lstat(binPath); err == nil && info.Mode()&os.ModeSymlink != 0 {
			continue
		}

		info, err := entry.Info()
		if err != nil || !isExecutable(info) {
			continue
		}

		prog := entry.Name()

		// Detect source from binary location
		source := common.DetectSource(prog)

		// If unknown, store the resolved path as identity
		if source == SourceUnknown {
			realPath, err := filepath.EvalSymlinks(binPath)
			if err != nil {
				realPath = binPath
			}
			source = manifest.BuildSourceString(SourceUnknown, realPath, "")
		}

		if tryAddTool(prog, source) {
			added++
		}
	}

	return added
}

// Helper functions

func cargoBinDir() string {
	cargoHome := os.Getenv(EnvCargoHome)
	if cargoHome == "" {
		cargoHome = filepath.Join(os.Getenv(EnvHome), DefaultCargoHome)
	}
	return filepath.Join(cargoHome, BinSubdir)
}

func npmBinDir() string {
	// Try NPM_CONFIG_PREFIX
	npmPrefix := os.Getenv(EnvNPMPrefix)
	if npmPrefix != "" {
		return filepath.Join(npmPrefix, BinSubdir)
	}

	// Try npm config
	if _, err := exec.LookPath(SourceNPM); err == nil {
		var output strings.Builder
		cmd := exec.Command(SourceNPM, "config", "get", "prefix")
		cmd.Stdout = &output
		if cmd.Run() == nil {
			prefix := strings.TrimSpace(output.String())
			if prefix != "" {
				return filepath.Join(prefix, BinSubdir)
			}
		}
	}

	// Fallback
	return filepath.Join(os.Getenv(EnvHome), DefaultNPMGlobal, BinSubdir)
}

func goBinDir() string {
	if _, err := exec.LookPath(SourceGo); err != nil {
		return ""
	}

	var output strings.Builder
	cmd := exec.Command(SourceGo, "env", "GOPATH")
	cmd.Stdout = &output
	if cmd.Run() != nil {
		return ""
	}

	gopath := strings.TrimSpace(output.String())
	if gopath == "" {
		return ""
	}

	return filepath.Join(gopath, BinSubdir)
}

func dirExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.IsDir()
}

func isExecutable(info os.FileInfo) bool {
	return info.Mode()&ExecutableMask != 0
}

func readManifestEntries(path string) (map[string]string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return make(map[string]string), nil
		}
		return nil, err
	}

	entries := make(map[string]string)
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, CommentPrefix) {
			continue
		}

		parts := strings.SplitN(line, ManifestDelim, FieldCount)
		if len(parts) == FieldCount {
			entries[parts[0]] = parts[1]
		}
	}

	return entries, nil
}
