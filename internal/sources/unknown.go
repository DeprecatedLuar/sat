package sources

import (
	"fmt"
	"os"
	"strings"

	"github.com/DeprecatedLuar/sat/internal/manifest"
)

// UnknownUninstall removes a manually-placed binary that scan couldn't
// attribute to any package manager. source's identity field is the
// resolved absolute path recorded at scan time (see scanner.ScanLocalBin).
// Refuses to touch anything outside the user's home directory, since the
// path comes from a possibly-stale scan rather than a live package
// manager query.
func UnknownUninstall(pkg, source string) error {
	path := manifest.GetSourceIdentity(source)
	if path == "" {
		return fmt.Errorf("no recorded path for %s - remove manually", pkg)
	}

	home, err := os.UserHomeDir()
	if err != nil || !strings.HasPrefix(path, home+string(os.PathSeparator)) {
		return fmt.Errorf("recorded path %s is outside your home directory - remove manually", path)
	}

	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		return err
	}
	return nil
}

// UnknownManifestIssues reports "unknown"-tracked manifest entries whose
// recorded path (resolved at scan time by scanner.ScanLocalBin) no longer
// exists, and should be pruned.
func UnknownManifestIssues() ManifestIssues {
	var issues ManifestIssues

	entries, err := manifest.All()
	if err != nil {
		return issues
	}

	for _, e := range entries {
		if manifest.GetSourceType(e.Source) != "unknown" {
			continue
		}

		path := manifest.GetSourceIdentity(e.Source)
		if path == "" || !fileExists(path) {
			issues.Prune = append(issues.Prune, PrunedEntry{Tool: e.Tool, Reason: "missing"})
		}
	}

	return issues
}
