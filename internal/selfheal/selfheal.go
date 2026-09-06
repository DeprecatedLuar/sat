package selfheal

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/DeprecatedLuar/sat/internal/common"
	"github.com/DeprecatedLuar/sat/internal/config"
	"github.com/DeprecatedLuar/sat/internal/drift"
	"github.com/DeprecatedLuar/sat/internal/manifest"
	"github.com/DeprecatedLuar/sat/internal/sources"
)

// skipDriftCommands are commands that read nothing from the manifest, so
// paying the drift reconcile cost for them - even once per interval - is
// pure regression rather than the invisible-by-design cost it is for
// everything else.
var skipDriftCommands = map[string]bool{
	"version": true,
	"help":    true,
	"--help":  true,
	"-h":      true,
}

// Run performs idempotent initialization on every invocation. command is
// the CLI command being dispatched (empty for the no-args case), used only
// to decide whether this run should skip the drift reconcile.
func Run(command string) error {
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
		manifest.StateDir(),
	}

	for _, dir := range dirs {
		if err := os.MkdirAll(dir, manifest.DirPermissions); err != nil {
			return fmt.Errorf("failed to create directory %s: %w", dir, err)
		}
	}

	// Each source package owns its own reconciliation logic end to end;
	// this is just the sequencing point.
	sources.BackfillDesktopEntries()
	sources.ReconcileWrappers()

	if !skipDriftCommands[command] {
		if _, err := drift.Ensure(); err != nil && os.Getenv(common.EnvSATDebug) != "" {
			fmt.Fprintf(os.Stderr, "%s drift reconcile: %v\n", common.DebugPrefix, err)
		}
	}

	return nil
}
