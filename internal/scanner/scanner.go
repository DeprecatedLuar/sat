package scanner

import (
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/DeprecatedLuar/sat/internal/manifest"
	"github.com/DeprecatedLuar/sat/internal/sources"
	"github.com/DeprecatedLuar/sat/internal/ui"
)

const (
	// Cleanup reasons
	ReasonExcluded = "excluded"
	ReasonBrewDep  = "brew dep"
	ReasonMissing  = "missing"

	// Manifest format (CommentPrefix defined in exclusion.go)
	ManifestDelim = "="
	FieldCount    = 2
)

// ScanResult represents the result of a scan operation
type ScanResult struct {
	Added  int
	Pruned int
}

// ScanAll scans all ecosystems and returns scan results
func ScanAll() (*ScanResult, error) {
	result := &ScanResult{}

	// Scan source-specific ecosystems via sources API
	result.Added += scanSource(sources.CargoScan)
	result.Added += scanSource(sources.BrewScan)
	result.Added += scanSource(sources.NixScan)
	result.Added += scanSource(sources.SystemScan)

	// Legacy directory scans for sources not yet modularized
	result.Added += scanDir("npm", NPMBinDir())
	result.Added += scanDir("uv", UVToolsDir())
	result.Added += scanDir("go", GoBinDir())

	// Special ecosystem scans
	result.Added += scanEcosystem(ScanFlatpak)
	result.Added += scanEcosystem(ScanAppImages)
	result.Added += scanEcosystem(ScanLocalBin)

	// Clean up AFTER scanning (removes exclusions, brew deps, stale unknowns)
	result.Pruned = CleanupManifest()

	return result, nil
}

// scanSource runs a source's Scan() method and adds packages to manifest
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

// scanEcosystem runs an ecosystem scanner and adds packages to manifest
func scanEcosystem(scanFunc func() ([]sources.Package, error)) int {
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

// tryAddPackage attempts to add a package to the manifest
func tryAddPackage(pkg sources.Package) bool {
	// Skip if excluded or already tracked
	if IsExcluded(pkg.Name, pkg.Source) {
		return false
	}
	if manifest.Has(pkg.Name) {
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

// CleanupManifest removes stale manifest entries after scanning
// Handles: exclusion patterns, brew dependencies, dotfile symlinks
func CleanupManifest() int {
	brewLeaves := getBrewLeaves()
	pruned := 0
	manifestPath := manifest.ManifestPath()
	entries, err := readManifestEntries(manifestPath)
	if err != nil {
		return 0
	}

	for prog, srcString := range entries {
		sourceType := manifest.GetSourceType(srcString)
		identity := manifest.GetSourceIdentity(srcString)

		shouldPrune := false
		reason := ""

		// 1. Check exclusion patterns
		if IsExcluded(prog, sourceType) {
			shouldPrune = true
			reason = ReasonExcluded
		} else if sourceType == "brew" && len(brewLeaves) > 0 {
			// 2. Check brew deps (not explicitly installed)
			if !brewLeaves[prog] {
				shouldPrune = true
				reason = ReasonBrewDep
			}
		} else if sourceType == "unknown" {
			// 3. Validate unknown sources (check if file still exists)
			if identity == "" || !FileExists(identity) {
				shouldPrune = true
				reason = ReasonMissing
			}
		}

		if shouldPrune {
			manifest.Remove(prog)
			fmt.Printf("  %s- %-20s (%s)%s\n", ui.Dim, prog, reason, ui.Reset)
			pruned++
		}
	}

	return pruned
}

// getBrewLeaves returns map of explicitly installed brew packages
func getBrewLeaves() map[string]bool {
	leaves := make(map[string]bool)
	if _, err := exec.LookPath("brew"); err != nil {
		return leaves
	}

	var output strings.Builder
	cmd := exec.Command("brew", "leaves")
	cmd.Stdout = &output
	if cmd.Run() == nil {
		for _, leaf := range strings.Split(output.String(), "\n") {
			leaf = strings.TrimSpace(leaf)
			if leaf != "" {
				leaves[leaf] = true
			}
		}
	}
	return leaves
}

// readManifestEntries reads manifest file and returns map of tool -> source string
func readManifestEntries(path string) (map[string]string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return make(map[string]string), nil
		}
		return nil, err
	}

	entries := make(map[string]string)
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, CommentPrefix) {
			continue
		}

		parts := strings.SplitN(line, ManifestDelim, FieldCount)
		if len(parts) == FieldCount {
			entries[parts[0]] = parts[1]
		}
	}

	return entries, nil
}
