package sources

import (
	"os/exec"

	"github.com/DeprecatedLuar/sat/internal/common"
	"github.com/DeprecatedLuar/sat/internal/manifest"
)

const reasonNotOnPath = "not on PATH"

// exactCheckTypes are source types whose entries have an exact, $PATH-free
// check of their own (see AppImageManifestIssues, FlatpakManifestIssues and
// UnknownManifestIssues) and so are excluded from the $PATH fallback.
var exactCheckTypes = map[string]bool{
	common.SourceAppImage: true,
	common.SourceFlatpak:  true,
	unknownSourceType:     true,
}

// StaleManifestIssues is the generic fallback for sources with no exact
// check: an entry whose binary is not found on $PATH is pruned. $PATH
// dependent, so it belongs to explicit scans only.
func StaleManifestIssues() ManifestIssues {
	var issues ManifestIssues

	entries, err := manifest.All()
	if err != nil {
		return issues
	}

	for _, e := range entries {
		sourceType := manifest.CanonicalSourceType(manifest.GetSourceType(e.Source))
		if exactCheckTypes[sourceType] {
			continue
		}
		if _, err := exec.LookPath(e.Tool); err != nil {
			issues.Prune = append(issues.Prune, PrunedEntry{Tool: e.Tool, SourceType: sourceType, Reason: reasonNotOnPath})
		}
	}

	return issues
}
