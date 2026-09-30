package scanner

import (
	"fmt"
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

	displayNames := sources.FlatpakDisplayNames()

	for _, appID := range strings.Split(output.String(), "\n") {
		appID = strings.TrimSpace(appID)
		if appID == "" {
			continue
		}

		// Prefer the app's canonical display name (e.g. "Telegram" for
		// org.telegram.desktop, whose last ID segment is a generic word);
		// falls back to the last ID component if the display name isn't
		// plain alphanumeric. This is both the manifest key and the
		// wrapper's filename.
		prog := sources.FlatpakToolName(appID, displayNames)

		if err := sources.EnsureFlatpakWrapper(appID, prog); err != nil {
			fmt.Fprintf(os.Stderr, "sat: warning: flatpak wrapper for %s: %v\n", appID, err)
		}

		packages = append(packages, sources.Package{
			Name:     prog,
			Source:   SourceFlatpak,
			Identity: appID,
			Version:  sources.FlatpakGetVersion(appID),
		})
	}

	return packages, nil
}

// ScanAppImages scans AppImage directory
func ScanAppImages() ([]sources.Package, error) {
	appimageDir := appImagesDir()
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
		identity := sources.ExtractAppImageRepo(filepath.Join(appimageDir, prog))
		packages = append(packages, sources.Package{
			Name:     prog,
			Source:   SourceAppImage,
			Identity: identity,
			Version:  "",
		})
	}

	return packages, nil
}

// ScanLocalBin scans ~/.local/bin for unknown sources
func ScanLocalBin() ([]sources.Package, error) {
	localBinDir := localBin()
	if !DirExists(localBinDir) {
		return nil, nil
	}

	var packages []sources.Package
	entries, err := os.ReadDir(localBinDir)
	if err != nil {
		return nil, err
	}

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		// Skip symlinks (managed elsewhere)
		binPath := filepath.Join(localBinDir, entry.Name())
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
	case "flatpak":
		return sources.FlatpakGetVersion(identity)
	case "uv":
		return sources.UvGetVersion(prog)
		// TODO: Add more sources as they're implemented in later phases
		// case "go": return sources.GoGetVersion(prog)
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
