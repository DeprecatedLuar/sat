package scanner

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/DeprecatedLuar/sat/internal/common"
	"github.com/DeprecatedLuar/sat/internal/sources"
)

const (
	// Source types
	SourceFlatpak  = "flatpak"
	SourceAppImage = "appimage"
	SourceUnknown  = "unknown"

	// Flatpak parsing
	FlatpakIDSeparator = "."

	// Executable permission mask
	ExecutableMask = 0111
)

// ScanDir scans a directory for binaries from a specific source
// Returns a list of found packages
func ScanDir(source, dir string) ([]sources.Package, error) {
	if dir == "" || !DirExists(dir) {
		return nil, nil
	}

	var packages []sources.Package
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		info, err := entry.Info()
		if err != nil || !IsExecutable(info) {
			continue
		}

		prog := entry.Name()
		packages = append(packages, sources.Package{
			Name:     prog,
			Source:   source,
			Identity: "",
			Version:  "",
		})
	}

	return packages, nil
}

// ScanFlatpak scans Flatpak user apps
func ScanFlatpak() ([]sources.Package, error) {
	if _, err := exec.LookPath("flatpak"); err != nil {
		return nil, nil
	}

	var packages []sources.Package
	var output strings.Builder
	cmd := exec.Command("flatpak", "list", "--app", "--columns=application")
	cmd.Stdout = &output

	if cmd.Run() != nil {
		return nil, nil
	}

	for _, appID := range strings.Split(output.String(), "\n") {
		appID = strings.TrimSpace(appID)
		if appID == "" {
			continue
		}

		// Use last component as prog name (org.gimp.GIMP → gimp)
		parts := strings.Split(appID, FlatpakIDSeparator)
		prog := strings.ToLower(parts[len(parts)-1])

		packages = append(packages, sources.Package{
			Name:     prog,
			Source:   SourceFlatpak,
			Identity: appID,
			Version:  "",
		})
		// TODO: Create flatpak wrapper (Phase 5)
	}

	return packages, nil
}

// ScanAppImages scans AppImage directory
func ScanAppImages() ([]sources.Package, error) {
	appimageDir := AppImagesDir()
	if !DirExists(appimageDir) {
		return nil, nil
	}

	var packages []sources.Package
	entries, err := os.ReadDir(appimageDir)
	if err != nil {
		return nil, err
	}

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		info, err := entry.Info()
		if err != nil || !IsExecutable(info) {
			continue
		}

		prog := entry.Name()
		// TODO: Extract GitHub repo from AppImage (Phase 6)
		packages = append(packages, sources.Package{
			Name:     prog,
			Source:   SourceAppImage,
			Identity: "",
			Version:  "",
		})
	}

	return packages, nil
}

// ScanLocalBin scans ~/.local/bin for unknown sources
func ScanLocalBin() ([]sources.Package, error) {
	localBin := LocalBin()
	if !DirExists(localBin) {
		return nil, nil
	}

	var packages []sources.Package
	entries, err := os.ReadDir(localBin)
	if err != nil {
		return nil, err
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
		if err != nil || !IsExecutable(info) {
			continue
		}

		prog := entry.Name()

		// Detect source from binary location
		source := common.DetectSource(prog)
		identity := ""

		// If unknown, store the resolved path as identity
		if source == SourceUnknown {
			realPath, err := filepath.EvalSymlinks(binPath)
			if err != nil {
				realPath = binPath
			}
			identity = realPath
		}

		packages = append(packages, sources.Package{
			Name:     prog,
			Source:   source,
			Identity: identity,
			Version:  "",
		})
	}

	return packages, nil
}

// GetVersionForSource gets version for a given tool and source
func GetVersionForSource(prog, sourceType, identity string) string {
	switch sourceType {
	case "cargo":
		return sources.CargoGetVersion(prog)
	case "brew":
		return sources.BrewGetVersion(prog)
	case "nix":
		return sources.NixGetVersion(prog)
	case "nixos":
		return sources.NixOSGetVersion(prog)
	case "apt", "pacman", "apk", "dnf", "system":
		return sources.GetVersion(prog)
	// TODO: Add more sources as they're implemented in later phases
	// case "npm": return sources.NPMGetVersion(prog)
	// case "uv": return sources.UVGetVersion(prog)
	// case "go": return sources.GoGetVersion(prog)
	// case "flatpak": return sources.FlatpakGetVersion(identity)
	}
	return ""
}

// ShouldSkipAuxiliary checks if a package without version should be skipped
// (system packages with no version are auxiliary tools, not main packages)
func ShouldSkipAuxiliary(sourceType, version string) bool {
	switch sourceType {
	case "apt", "pacman", "apk", "dnf":
		return version == ""
	}
	return false
}
