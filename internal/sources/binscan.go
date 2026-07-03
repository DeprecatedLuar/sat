package sources

import (
	"os"
	"path/filepath"
)

// executableMask matches any executable bit (owner/group/other)
const executableMask = 0111

// scanBinDirsOwnedBy walks binDirs and, for every executable regular file
// whose owning package (per ownerFn) is present in isManual, resolves its
// version (per versionFn) and returns a Package for it.
//
// This is the shared shape behind ApkScan/AptScan/DnfScan: only the
// owning-package lookup command and how the manual-install set is built
// differ per package manager.
func scanBinDirsOwnedBy(binDirs []string, source string, isManual map[string]bool, ownerFn func(binPath string) string, versionFn func(binName string) string) []Package {
	var packages []Package

	for _, binDir := range binDirs {
		entries, err := os.ReadDir(binDir)
		if err != nil {
			continue
		}

		for _, entry := range entries {
			if entry.IsDir() {
				continue
			}

			info, err := entry.Info()
			if err != nil || info.Mode()&executableMask == 0 {
				continue
			}

			binPath := filepath.Join(binDir, entry.Name())
			pkgName := ownerFn(binPath)
			if pkgName == "" || !isManual[pkgName] {
				continue
			}

			packages = append(packages, Package{
				Name:     entry.Name(),
				Source:   source,
				Identity: "",
				Version:  versionFn(entry.Name()),
			})
		}
	}

	return packages
}
