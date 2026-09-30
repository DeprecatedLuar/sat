package scanner

import (
	"os"

	"github.com/DeprecatedLuar/sat/internal/common"
)

// appImagesDir returns the AppImage storage directory
func appImagesDir() string {
	return common.AppImagesDir()
}

// localBin returns the ~/.local/bin directory
func localBin() string {
	return common.LocalBin()
}

// DirExists checks if a directory exists
func DirExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.IsDir()
}

// IsExecutable checks if a file has executable permissions
func IsExecutable(info os.FileInfo) bool {
	return info.Mode()&ExecutableMask != 0
}
