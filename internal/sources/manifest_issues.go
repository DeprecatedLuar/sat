package sources

// PrunedEntry is a manifest entry a source module has determined should be
// removed, with the reason shown to the user.
type PrunedEntry struct {
	Tool       string
	SourceType string
	Reason     string
}

// RepairedEntry is a manifest entry a source module has determined should
// be written or rewritten with corrected metadata (e.g. a version fetched
// live, or an untracked artifact adopted).
type RepairedEntry struct {
	Tool      string
	NewSource string
}

// ManifestIssues is what a source module reports after reviewing its own
// already-tracked manifest entries for staleness. The scanner applies these
// generically (manifest.Remove/manifest.Add + display) without needing to
// know why - each source module owns the "why", the scanner only routes.
type ManifestIssues struct {
	Prune  []PrunedEntry
	Repair []RepairedEntry
}
