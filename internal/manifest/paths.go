package manifest

import (
	"os"
	"path/filepath"
)

const (
	// Environment variables
	EnvSATData     = "SAT_DATA"
	EnvXDGDataHome = "XDG_DATA_HOME"

	// Directory and file names
	AppName           = "sat"
	DefaultDataPath   = ".local/share/sat"
	ManifestFileName  = "manifest"
	ShellDirName      = "shell"
	BinDirName        = "bin"
	AppImagesDirName  = "appimages"
	FlatpakDirName    = "flatpak"
)

// DataDir returns the base sat data directory
// Priority: SAT_DATA env var > XDG_DATA_HOME/sat > ~/.local/share/sat
func DataDir() string {
	if dir := os.Getenv(EnvSATData); dir != "" {
		return dir
	}
	if xdgData := os.Getenv(EnvXDGDataHome); xdgData != "" {
		return filepath.Join(xdgData, AppName)
	}
	home, _ := os.UserHomeDir()
	return filepath.Join(home, DefaultDataPath)
}

// ManifestPath returns the system manifest file path
func ManifestPath() string {
	return filepath.Join(DataDir(), ManifestFileName)
}

// ShellMasterPath returns the master shell manifest path
func ShellMasterPath() string {
	return filepath.Join(DataDir(), ShellDirName, ManifestFileName)
}

// ShellDirPath returns the session directory for a given PID
func ShellDirPath(pid string) string {
	return filepath.Join(DataDir(), ShellDirName, pid)
}

// ShellSessionManifest returns the session manifest path for a PID
func ShellSessionManifest(pid string) string {
	return filepath.Join(ShellDirPath(pid), ManifestFileName)
}
