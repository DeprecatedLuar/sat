package selfheal

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/DeprecatedLuar/sat/internal/common"
	"github.com/DeprecatedLuar/sat/internal/config"
	"github.com/DeprecatedLuar/sat/internal/manifest"
	"github.com/DeprecatedLuar/sat/internal/sources"
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

	// Ensure config.toml exists with defaults
	if err := config.EnsureDefault(); err != nil {
		return fmt.Errorf("failed to ensure config: %w", err)
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

	backfillAppImageDesktopEntries()

	return nil
}

// backfillAppImageDesktopEntries generates desktop entries for AppImages
// installed before desktop-entry generation existed (or installed while
// unsquashfs was unavailable). Idempotent and local-only: skips any tool
// that already has an entry or has been marked permanently skippable via
// sources.HasDesktopEntry, so the common case is one cheap stat per
// installed AppImage.
func backfillAppImageDesktopEntries() {
	appimagesDir := common.AppImagesDir()
	entries, err := os.ReadDir(appimagesDir)
	if err != nil {
		return
	}

	skipDirs := map[string]bool{
		common.AppImageApplicationsDirName: true,
		common.AppImageIconsDirName:        true,
	}

	for _, entry := range entries {
		name := entry.Name()
		if entry.IsDir() || skipDirs[name] {
			continue
		}
		if sources.HasDesktopEntry(name) {
			continue
		}

		appimagePath := filepath.Join(appimagesDir, name)
		symlinkPath := filepath.Join(common.LocalBin(), name)
		repoPath := manifest.GetSourceIdentity(manifest.Get(name))

		sources.InstallDesktopEntry(appimagePath, symlinkPath, name, repoPath)
	}
}
