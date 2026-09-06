package manifest

import (
	"os"
	"path/filepath"
)

const (
	// Environment variables
	EnvSATData      = "SAT_DATA"
	EnvXDGDataHome  = "XDG_DATA_HOME"
	EnvXDGStateHome = "XDG_STATE_HOME"

	// Directory and file names
	AppName          = "sat"
	DefaultDataPath  = ".local/share/sat"
	DefaultStatePath = ".local/state/sat"
	ManifestFileName = "manifest"
	ShellDirName     = "shell"
	BinDirName       = "bin"
	AppImagesDirName = "appimages"
	FlatpakDirName   = "flatpak"
	SourcesDirName   = "sources"
	StateDirName     = "state"
	DriftStampName   = "drift"
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

// StateDir returns the directory for machine-local, regenerable-if-lost
// state (e.g. the drift reconciliation stamp) - distinct from DataDir,
// which holds the manifest and other data a user would not want to lose.
// Priority: SAT_DATA/state (so tests using SAT_DATA stay isolated and
// never touch the real XDG state dir) > XDG_STATE_HOME/sat > ~/.local/state/sat.
func StateDir() string {
	if dir := os.Getenv(EnvSATData); dir != "" {
		return filepath.Join(dir, StateDirName)
	}
	if xdgState := os.Getenv(EnvXDGStateHome); xdgState != "" {
		return filepath.Join(xdgState, AppName)
	}
	home, _ := os.UserHomeDir()
	return filepath.Join(home, DefaultStatePath)
}

// DriftStampPath returns the path to the version-drift reconciliation stamp.
func DriftStampPath() string {
	return filepath.Join(StateDir(), DriftStampName)
}
