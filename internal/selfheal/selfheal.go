package selfheal

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/DeprecatedLuar/sat/internal/common"
	"github.com/DeprecatedLuar/sat/internal/manifest"
)

// Run performs idempotent initialization on every invocation
func Run() error {
	// Ensure base data directory exists
	dataDir := manifest.DataDir()
	if err := os.MkdirAll(dataDir, manifest.DirPermissions); err != nil {
		return fmt.Errorf("failed to create data directory: %w", err)
	}

	// Ensure manifest file exists
	if err := manifest.EnsureManifest(manifest.ManifestPath()); err != nil {
		return fmt.Errorf("failed to ensure manifest: %w", err)
	}

	// Ensure OS info cache exists
	if err := common.EnsureOSInfo(); err != nil {
		return fmt.Errorf("failed to ensure OS info: %w", err)
	}

	// Ensure directory structure
	dirs := []string{
		filepath.Join(dataDir, manifest.ShellDirName),
		filepath.Join(dataDir, manifest.BinDirName, manifest.AppImagesDirName),
		filepath.Join(dataDir, manifest.BinDirName, manifest.FlatpakDirName),
	}

	for _, dir := range dirs {
		if err := os.MkdirAll(dir, manifest.DirPermissions); err != nil {
			return fmt.Errorf("failed to create directory %s: %w", dir, err)
		}
	}

	return nil
}
