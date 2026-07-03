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

// LocalBin returns ~/.local/bin, the directory sat symlinks installed
// binaries (e.g. AppImages) into so they land on the user's PATH.
func LocalBin() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, LocalBinSubpath)
}
