package sources

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"

	"github.com/DeprecatedLuar/sat/internal/common"
	"github.com/DeprecatedLuar/sat/internal/manifest"
)

const reasonMissing = "missing"

// appImageZsyncPattern matches the embedded zsync self-update string that
// linuxdeploy/appimagetool write into many AppImages (owner|repo, capped to
// avoid runaway matches on non-matching binary noise).
var appImageZsyncPattern = regexp.MustCompile(`gh-releases-zsync\|([^|\x00]{1,100})\|([^|\x00]{1,200})`)

// ExtractAppImageRepo recovers an "owner/repo" identity from an AppImage
// binary by searching for the embedded zsync update string. Returns "" if
// the AppImage doesn't embed one - not every AppImage carries it.
func ExtractAppImageRepo(path string) string {
	data, err := os.ReadFile(path)
	if err != nil {
		return ""
	}

	m := appImageZsyncPattern.FindSubmatch(data)
	if m == nil {
		return ""
	}

	return string(m[1]) + "/" + string(m[2])
}

// AppImageManifestIssues compares the manifest's appimage entries against
// the files in the AppImage storage directory: a tracked entry whose file is
// gone is pruned, an executable file with no entry is adopted. Exact (the
// directory is sat-owned) and independent of $PATH.
func AppImageManifestIssues() ManifestIssues {
	var issues ManifestIssues

	entries, err := manifest.All()
	if err != nil {
		fmt.Fprintf(os.Stderr, "sat: warning: appimage manifest check: %v\n", err)
		return issues
	}

	dir := common.AppImagesDir()
	tracked := make(map[string]bool)
	for _, e := range entries {
		if manifest.CanonicalSourceType(manifest.GetSourceType(e.Source)) != common.SourceAppImage {
			continue
		}
		tracked[e.Tool] = true
		if !fileExists(filepath.Join(dir, e.Tool)) {
			issues.Prune = append(issues.Prune, PrunedEntry{Tool: e.Tool, SourceType: common.SourceAppImage, Reason: reasonMissing})
		}
	}

	files, err := os.ReadDir(dir)
	if err != nil {
		if !os.IsNotExist(err) {
			fmt.Fprintf(os.Stderr, "sat: warning: appimage manifest check: %v\n", err)
		}
		return issues
	}
	for _, f := range files {
		if f.IsDir() || tracked[f.Name()] {
			continue
		}
		info, err := f.Info()
		if err != nil || info.Mode()&executableMask == 0 {
			continue
		}
		repo := ExtractAppImageRepo(filepath.Join(dir, f.Name()))
		issues.Repair = append(issues.Repair, RepairedEntry{
			Tool:      f.Name(),
			NewSource: manifest.BuildSourceString(common.SourceAppImage, repo, ""),
		})
	}

	return issues
}
