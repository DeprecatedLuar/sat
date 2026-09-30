package scanner

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/DeprecatedLuar/sat/internal/manifest"
	"github.com/DeprecatedLuar/sat/internal/sources"
)

func isolate(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	t.Setenv("SAT_DATA", dir)
	t.Setenv("HOME", dir)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(dir, "config"))
	if err := os.MkdirAll(filepath.Join(dir, "bin", "appimages"), 0755); err != nil {
		t.Fatal(err)
	}
	return dir
}

func TestAlreadyTrackedDedupeScope(t *testing.T) {
	isolate(t)
	manifest.Add("zapzap", "flatpak:com.x.Zap:1")

	if !alreadyTracked(sources.Package{Name: "zapzap", Source: "flatpak"}) {
		t.Error("same flatpak pair should be tracked")
	}
	if alreadyTracked(sources.Package{Name: "zapzap", Source: "appimage"}) {
		t.Error("appimage must dedupe per source, not by name")
	}
	if !alreadyTracked(sources.Package{Name: "zapzap", Source: "system"}) {
		t.Error("system must keep name-level dedupe")
	}
}

func TestApplyManifestIssuesPrunesOnlyOwnSource(t *testing.T) {
	isolate(t)
	manifest.Add("zapzap", "flatpak:com.x.Zap:1")
	manifest.Add("zapzap", "appimage:o/zap:")

	ApplyManifestIssues(sources.ManifestIssues{Prune: []sources.PrunedEntry{
		{Tool: "zapzap", SourceType: "flatpak", Reason: "not installed"},
	}})

	if manifest.Has("zapzap", "flatpak") || !manifest.Has("zapzap", "appimage") {
		t.Errorf("manifest after prune: %+v", manifest.Lookup("zapzap"))
	}
}

func TestExactIssuesAppImageAndFlatpak(t *testing.T) {
	dir := isolate(t)
	img := filepath.Join(dir, "bin", "appimages")

	os.WriteFile(filepath.Join(img, "present"), []byte("x"), 0755)
	os.WriteFile(filepath.Join(img, "untracked"), []byte("x"), 0755)
	manifest.Add("present", "appimage:o/present:")
	manifest.Add("gone", "appimage:o/gone:")
	manifest.Add("zapzap", "flatpak:com.x.Zap:1")
	manifest.Add("kept", "flatpak:com.x.Kept:1")

	snap := sources.FlatpakSnapshot{Installed: map[string]bool{"com.x.Kept": true}, OK: true}
	pruned, repaired := ApplyExactIssues(snap)

	if pruned != 2 || repaired != 1 {
		t.Fatalf("pruned %d repaired %d, want 2 and 1", pruned, repaired)
	}
	if manifest.Has("gone", "appimage") || manifest.Has("zapzap", "flatpak") {
		t.Error("stale entries survived")
	}
	if !manifest.Has("untracked", "appimage") || !manifest.Has("present", "appimage") || !manifest.Has("kept", "flatpak") {
		t.Errorf("valid entries lost: %+v", mustAll(t))
	}
}

func TestExactIssuesSkipFlatpakWhenListUnavailable(t *testing.T) {
	isolate(t)
	manifest.Add("zapzap", "flatpak:com.x.Zap:1")

	pruned, _ := ApplyExactIssues(sources.FlatpakSnapshot{})
	if pruned != 0 || !manifest.Has("zapzap", "flatpak") {
		t.Error("flatpak entry pruned without a successful list")
	}
}

func TestExactIssuesNoopLeavesManifestUntouched(t *testing.T) {
	dir := isolate(t)
	os.WriteFile(filepath.Join(dir, "bin", "appimages", "present"), []byte("x"), 0755)
	manifest.Add("present", "appimage:o/present:")
	manifest.Add("rg", "cargo::14")

	before, _ := os.ReadFile(manifest.ManifestPath())
	snap := sources.FlatpakSnapshot{Installed: map[string]bool{}, OK: true}
	pruned, repaired := ApplyExactIssues(snap)
	after, _ := os.ReadFile(manifest.ManifestPath())

	if pruned != 0 || repaired != 0 || string(before) != string(after) {
		t.Errorf("no-op run changed state: %d %d\n%s\n%s", pruned, repaired, before, after)
	}
}

func mustAll(t *testing.T) []manifest.Entry {
	t.Helper()
	all, err := manifest.All()
	if err != nil {
		t.Fatal(err)
	}
	return all
}
