// Package bootstrap installs the package-manager binaries themselves
// (uv, cargo, brew, nix, huber) so that internal/sources has something to
// install packages through. This is a distinct concern from
// internal/sources, which assumes its package manager is already present
// and only manages packages via it.
package bootstrap

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/DeprecatedLuar/sat/internal/common"
)

// dirPermissions is used for directories and executables this package
// creates while bootstrapping package-manager binaries.
const dirPermissions = 0755

// Relocate moves a freshly-bootstrapped binary (plus any companion
// binaries installed alongside it, e.g. uv's "uvx") from its installer's
// default location into common.SourcesBinDir(), then symlinks each one
// back to its original path so it stays reachable on $PATH while
// remaining under sat's management.
//
// installedPath is the primary binary's current location (e.g.
// ~/.local/bin/uv). companionNames are other binary names expected in the
// same directory (e.g. "uvx"); any that don't exist are silently skipped.
// Returns the companions that were actually found and relocated.
func Relocate(installedPath string, companionNames ...string) (moved []string, err error) {
	sourcesDir := common.SourcesBinDir()
	if err := os.MkdirAll(sourcesDir, dirPermissions); err != nil {
		return nil, fmt.Errorf("failed to create sources bin directory: %w", err)
	}

	if err := relocateOne(installedPath, sourcesDir); err != nil {
		return nil, err
	}

	installDir := filepath.Dir(installedPath)
	for _, name := range companionNames {
		compPath := filepath.Join(installDir, name)
		if _, err := os.Stat(compPath); err != nil {
			continue
		}
		if err := relocateOne(compPath, sourcesDir); err != nil {
			return moved, err
		}
		moved = append(moved, name)
	}

	return moved, nil
}

// relocateOne moves a single binary into sourcesDir and symlinks its
// original path back to the new location.
func relocateOne(path, sourcesDir string) error {
	dest := filepath.Join(sourcesDir, filepath.Base(path))

	if err := os.Rename(path, dest); err != nil {
		return fmt.Errorf("failed to move %s: %w", path, err)
	}

	os.Remove(path)
	if err := os.Symlink(dest, path); err != nil {
		return fmt.Errorf("failed to symlink %s: %w", path, err)
	}

	return nil
}

// isSatManaged reports whether path is a symlink whose target lives inside
// common.SourcesBinDir() — i.e. a binary sat itself relocated there via
// Relocate. Every Remove function checks this before touching anything, so
// sat never attempts to uninstall a package manager it didn't bootstrap
// (e.g. a system-provided binary from outside sat's management).
func isSatManaged(path string) bool {
	target, err := os.Readlink(path)
	if err != nil {
		return false
	}
	return filepath.Dir(target) == common.SourcesBinDir()
}

// Unrelocate reverses Relocate: for originalPath and any sat-managed
// companion found in the same directory, removes the symlink and deletes
// the corresponding file from common.SourcesBinDir().
func Unrelocate(originalPath string, companionNames ...string) error {
	if err := unrelocateOne(originalPath); err != nil {
		return err
	}

	installDir := filepath.Dir(originalPath)
	for _, name := range companionNames {
		compPath := filepath.Join(installDir, name)
		if isSatManaged(compPath) {
			if err := unrelocateOne(compPath); err != nil {
				return err
			}
		}
	}

	return nil
}

// unrelocateOne removes a single sat-managed symlink and the relocated
// file it points to.
func unrelocateOne(path string) error {
	target, err := os.Readlink(path)
	if err != nil {
		return fmt.Errorf("%s is not a symlink managed by sat: %w", path, err)
	}

	if err := os.Remove(path); err != nil {
		return fmt.Errorf("failed to remove %s: %w", path, err)
	}

	if err := os.Remove(target); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("failed to remove %s: %w", target, err)
	}

	return nil
}
