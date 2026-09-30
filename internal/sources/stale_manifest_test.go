package sources

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/DeprecatedLuar/sat/internal/manifest"
)

func TestStaleManifestIssuesPathFallback(t *testing.T) {
	t.Setenv("SAT_DATA", t.TempDir())
	bin := t.TempDir()
	os.WriteFile(filepath.Join(bin, "present"), []byte("#!/bin/sh\n"), 0755)
	t.Setenv("PATH", bin)

	manifest.Add("present", "cargo::1")
	manifest.Add("absent", "npm::1")
	manifest.Add("zapzap", "appimage:o/z:")
	manifest.Add("fp", "flatpak:com.x.Fp:1")

	issues := StaleManifestIssues()
	if len(issues.Prune) != 1 || issues.Prune[0].Tool != "absent" || issues.Prune[0].SourceType != "npm" {
		t.Errorf("prune = %+v, want only absent/npm", issues.Prune)
	}
}
