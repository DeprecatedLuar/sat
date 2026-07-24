package scanner

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/DeprecatedLuar/sat/internal/common"
)

const (
	// Subdirectories
	BinSubdir = "bin"

	// Tool names
	GoTool = "go"
)

// GoBinDir returns the go bin directory
func GoBinDir() string {
	if _, err := exec.LookPath(GoTool); err != nil {
		return ""
	}

	var output strings.Builder
	cmd := exec.Command(GoTool, "env", "GOPATH")
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
