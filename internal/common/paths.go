package common

import (
	"os"
	"path/filepath"

	"github.com/DeprecatedLuar/sat/internal/manifest"
)

const (
	// LocalBinSubpath is ~/.local/bin, relative to the user's home directory.
	LocalBinSubpath = ".local/bin"
)

// AppImagesDir returns the directory where sat stores managed AppImage
// binaries: $SAT_DATA/bin/appimages (respects the same SAT_DATA/XDG
// overrides as manifest.DataDir).
func AppImagesDir() string {
	return filepath.Join(manifest.DataDir(), manifest.BinDirName, manifest.AppImagesDirName)
}

// AppImageApplicationsDirName and AppImageIconsDirName are the subdirectories
// of AppImagesDir() holding canonical .desktop files and icons extracted
// from installed AppImages. Nested here rather than as top-level siblings
// since AppImage installs are currently the only producer of desktop
// entries/icons within sat.
const (
	AppImageApplicationsDirName = "applications"
	AppImageIconsDirName        = "icons"
)

// AppImageApplicationsDir returns the directory holding canonical .desktop
// files generated for installed AppImages: $SAT_DATA/bin/appimages/applications.
func AppImageApplicationsDir() string {
	return filepath.Join(AppImagesDir(), AppImageApplicationsDirName)
}

// AppImageIconsDir returns the directory holding icons extracted for
// installed AppImages: $SAT_DATA/bin/appimages/icons.
func AppImageIconsDir() string {
	return filepath.Join(AppImagesDir(), AppImageIconsDirName)
}

// DesktopApplicationsDir returns the standard XDG applications directory
// that desktop environments scan for .desktop files: ~/.local/share/applications.
func DesktopApplicationsDir() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".local", "share", "applications")
}

// LocalBin returns ~/.local/bin, the directory sat symlinks installed
// binaries (e.g. AppImages) into so they land on the user's PATH.
func LocalBin() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, LocalBinSubpath)
}

// FlatpakWrapperDir returns the directory where sat stores generated
// flatpak launcher wrapper scripts: $SAT_DATA/bin/flatpak. Flatpak apps
// aren't directly on $PATH, so each tracked app gets a small wrapper here
// (symlinked into LocalBin()) that execs `flatpak run <app_id>`.
func FlatpakWrapperDir() string {
	return filepath.Join(manifest.DataDir(), manifest.BinDirName, manifest.FlatpakDirName)
}
