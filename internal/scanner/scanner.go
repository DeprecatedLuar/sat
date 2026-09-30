package scanner

import (
	"fmt"

	"github.com/DeprecatedLuar/sat/internal/drift"
	"github.com/DeprecatedLuar/sat/internal/manifest"
	"github.com/DeprecatedLuar/sat/internal/sources"
	"github.com/DeprecatedLuar/sat/internal/ui"
)

const (
	// Cleanup reasons
	ReasonExcluded = "excluded"
)

// ScanResult represents the result of a scan operation
type ScanResult struct {
	Added    int
	Pruned   int
	Repaired int
}

// ScanAll scans all ecosystems and returns scan results
func ScanAll() (*ScanResult, error) {
	result := &ScanResult{}

	// Scan source-specific ecosystems via sources API
	result.Added += scanSource(sources.CargoScan)
	result.Added += scanSource(sources.BrewScan)
	result.Added += scanSource(sources.NixScan)
	result.Added += scanSource(sources.SystemScan)
	result.Added += scanSource(sources.NpmScan)
	result.Added += scanSource(sources.UvScan)

	// Legacy directory scans for sources not yet modularized
	result.Added += scanDir("go", GoBinDir())

	// Special ecosystem scans
	result.Added += scanSource(ScanFlatpak)
	result.Added += scanSource(ScanAppImages)
	result.Added += scanSource(ScanLocalBin)

	// Clean up AFTER scanning (removes exclusions, brew deps, stale unknowns,
	// backfills missing npm versions)
	result.Pruned, result.Repaired = CleanupManifest()

	return result, nil
}

// scanSource runs a scan function (source adapter or ecosystem scanner) and adds packages to manifest
func scanSource(scanFunc func() ([]sources.Package, error)) int {
	packages, err := scanFunc()
	if err != nil || packages == nil {
		return 0
	}

	added := 0
	for _, pkg := range packages {
		if tryAddPackage(pkg) {
			added++
		}
	}

	return added
}

// scanDir scans a directory using generic directory scanner
func scanDir(source, dir string) int {
	packages, err := ScanDir(source, dir)
	if err != nil || packages == nil {
		return 0
	}

	added := 0
	for _, pkg := range packages {
		if tryAddPackage(pkg) {
			added++
		}
	}

	return added
}

// perSourceDedupe lists sources whose artifacts are owned by sat, so the
// same name may legitimately be tracked from several of them at once. Every
// other source can report one binary twice (system/nix/nixos/localbin/go), so
// those dedupe by name.
var perSourceDedupe = map[string]bool{
	SourceFlatpak:  true,
	SourceAppImage: true,
}

// alreadyTracked reports whether pkg is already in the manifest under the
// identity its source dedupes by.
func alreadyTracked(pkg sources.Package) bool {
	if perSourceDedupe[pkg.Source] {
		return manifest.Has(pkg.Name, pkg.Source)
	}
	return len(manifest.Lookup(pkg.Name)) > 0
}

// tryAddPackage attempts to add a package to the manifest
func tryAddPackage(pkg sources.Package) bool {
	// Skip if excluded or already tracked
	if IsExcluded(pkg.Name, pkg.Source) {
		return false
	}
	if alreadyTracked(pkg) {
		return false
	}
	// TODO: Check master manifest for shell sessions (Phase 12)

	// Get version if not already set
	version := pkg.Version
	if version == "" {
		version = GetVersionForSource(pkg.Name, pkg.Source, pkg.Identity)
	}

	// Skip auxiliary system tools (no version = not a main package)
	if ShouldSkipAuxiliary(pkg.Source, version) {
		return false
	}

	// Build full source string and add to manifest
	srcString := manifest.BuildSourceString(pkg.Source, pkg.Identity, version)
	if err := manifest.Add(pkg.Name, srcString); err != nil {
		return false
	}

	// Display with version
	color := ui.SourceColor(srcString)
	ui.DisplayToolEntry(pkg.Name, srcString, color+"+"+ui.Reset+" ", "")
	return true
}

// CleanupManifest removes stale manifest entries after scanning. Exclusion
// patterns are cross-cutting user policy, so they're applied here directly;
// every other kind of staleness is source-specific and reported by that
// source's own ManifestIssues function (e.g. sources.NpmManifestIssues,
// sources.BrewManifestIssues, sources.UnknownManifestIssues) - this
// function only applies what's reported (manifest.Remove/manifest.Add +
// display), the same data-in/data-out contract scanSource follows for
// newly discovered packages. Returns (pruned, repaired) counts.
func CleanupManifest() (int, int) {
	pruned := 0
	repaired := 0

	entries, err := manifest.All()
	if err == nil {
		for _, e := range entries {
			if IsExcluded(e.Tool, manifest.GetSourceType(e.Source)) {
				if err := manifest.Remove(e.Tool, manifest.GetSourceType(e.Source)); err != nil {
					ui.Warn(fmt.Sprintf("failed to prune %s: %v", e.Tool, err))
					continue
				}
				fmt.Printf("  %s- %-20s (%s)%s\n", ui.Dim, e.Tool, ReasonExcluded, ui.Reset)
				pruned++
			}
		}
	}

	p, r := ApplyExactIssues(sources.SnapshotFlatpak())
	pruned += p
	repaired += r

	for _, issues := range []sources.ManifestIssues{
		sources.BrewManifestIssues(),
		sources.UnknownManifestIssues(),
		sources.NpmManifestIssues(),
		sources.StaleManifestIssues(),
	} {
		p, r := ApplyManifestIssues(issues)
		pruned += p
		repaired += r
	}

	// sat scan is the explicit "make it right" command, so this calls
	// Check/Apply directly rather than the TTL-gated drift.Ensure - a scan
	// should never skip reconciling just because selfheal ran recently.
	repaired += applyDrifts(drift.Check())

	return pruned, repaired
}

// applyDrifts rewrites the manifest for every drift found in a single
// batched write (drift.Apply), then prints each correction with the same
// "~" marker ApplyManifestIssues uses for a repair.
func applyDrifts(drifts []drift.Drift) (repaired int) {
	changed, err := drift.Apply(drifts)
	if err != nil || changed == 0 {
		return 0
	}
	for _, d := range drifts {
		newSource := d.NewSource()
		color := ui.SourceColor(newSource)
		ui.DisplayToolEntry(d.Tool, newSource, color+"~"+ui.Reset+" ", "")
	}
	return changed
}

// ApplyExactIssues applies the checks that need no $PATH and are cheap
// enough for every invocation (appimage files, flatpak installs). The one
// list shared by selfheal and scan.
func ApplyExactIssues(flatpak sources.FlatpakSnapshot) (pruned, repaired int) {
	for _, issues := range []sources.ManifestIssues{
		sources.AppImageManifestIssues(),
		sources.FlatpakManifestIssues(flatpak),
	} {
		p, r := ApplyManifestIssues(issues)
		pruned += p
		repaired += r
	}
	return
}

// ApplyManifestIssues mutates the manifest and prints per the staleness a
// source module reported, without knowing anything about which source it
// came from or why. Entries excluded by policy are never added.
func ApplyManifestIssues(issues sources.ManifestIssues) (pruned, repaired int) {
	for _, p := range issues.Prune {
		if err := manifest.Remove(p.Tool, p.SourceType); err != nil {
			ui.Warn(fmt.Sprintf("failed to prune %s: %v", p.Tool, err))
			continue
		}
		fmt.Printf("  %s- %-20s (%s)%s\n", ui.Dim, p.Tool, p.Reason, ui.Reset)
		pruned++
	}
	for _, r := range issues.Repair {
		if IsExcluded(r.Tool, manifest.GetSourceType(r.NewSource)) {
			continue
		}
		if err := manifest.Add(r.Tool, r.NewSource); err != nil {
			ui.Warn(fmt.Sprintf("failed to update %s: %v", r.Tool, err))
			continue
		}
		repaired++
		color := ui.SourceColor(r.NewSource)
		ui.DisplayToolEntry(r.Tool, r.NewSource, color+"~"+ui.Reset+" ", "")
	}
	return
}
