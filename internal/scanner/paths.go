package scanner

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/DeprecatedLuar/sat/internal/common"
)

const (
	// Environment variables (EnvHome defined in exclusion.go)
	EnvCargoHome = "CARGO_HOME"
	EnvNPMPrefix = "NPM_CONFIG_PREFIX"

	// Default paths
	DefaultCargoHome = ".cargo"
	DefaultNPMGlobal = ".npm-global"
	LocalShareDir    = ".local/share"

	// Subdirectories
	BinSubdir     = "bin"
	UVToolsSubdir = "uv/tools"

	// Tool names
	NPMTool = "npm"
	GoTool  = "go"
)

// CargoBinDir returns the cargo bin directory
func CargoBinDir() string {
	cargoHome := os.Getenv(EnvCargoHome)
	if cargoHome == "" {
		cargoHome = filepath.Join(os.Getenv(EnvHome), DefaultCargoHome)
	}
	return filepath.Join(cargoHome, BinSubdir)
}

// NPMBinDir returns the npm global bin directory
func NPMBinDir() string {
	// Try NPM_CONFIG_PREFIX
	npmPrefix := os.Getenv(EnvNPMPrefix)
	if npmPrefix != "" {
		return filepath.Join(npmPrefix, BinSubdir)
	}

	// Try npm config
	if _, err := exec.LookPath(NPMTool); err == nil {
		var output strings.Builder
		cmd := exec.Command(NPMTool, "config", "get", "prefix")
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

// UVToolsDir returns the uv tools directory
func UVToolsDir() string {
	return filepath.Join(os.Getenv(EnvHome), LocalShareDir, UVToolsSubdir)
}

// AppImagesDir returns the AppImage storage directory
func AppImagesDir() string {
	return common.AppImagesDir()
}

// LocalBin returns the ~/.local/bin directory
func LocalBin() string {
	return common.LocalBin()
}

// DirExists checks if a directory exists
func DirExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.IsDir()
}

// FileExists checks if a file exists
func FileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

// IsExecutable checks if a file has executable permissions
func IsExecutable(info os.FileInfo) bool {
	return info.Mode()&ExecutableMask != 0
}
