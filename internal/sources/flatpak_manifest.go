package sources

import (
	"github.com/DeprecatedLuar/sat/internal/common"
	"github.com/DeprecatedLuar/sat/internal/manifest"
)

const reasonNotInstalled = "not installed"

// FlatpakManifestIssues reports flatpak entries whose app ID is absent from
// a successful `flatpak list`. An unavailable list reports nothing: absence
// is only evidence when the list was actually obtained.
func FlatpakManifestIssues(snap FlatpakSnapshot) ManifestIssues {
	var issues ManifestIssues
	if !snap.OK {
		return issues
	}

	entries, err := manifest.All()
	if err != nil {
		return issues
	}

	for _, e := range entries {
		if manifest.CanonicalSourceType(manifest.GetSourceType(e.Source)) != common.SourceFlatpak {
			continue
		}
		appID := manifest.GetSourceIdentity(e.Source)
		if appID == "" || snap.Installed[appID] {
			continue
		}
		issues.Prune = append(issues.Prune, PrunedEntry{Tool: e.Tool, SourceType: common.SourceFlatpak, Reason: reasonNotInstalled})
	}

	return issues
}
