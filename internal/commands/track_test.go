package commands

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/DeprecatedLuar/sat/internal/common"
	"github.com/DeprecatedLuar/sat/internal/manifest"
)

// isolatedTool installs an executable under an isolated HOME's .local/opt
// (classified as a manual install) and puts it on an isolated PATH.
func isolatedTool(t *testing.T, name string) {
	t.Helper()
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("SAT_DATA", t.TempDir())
	dir := filepath.Join(home, ".local", "opt", "bin")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, name), []byte("#!/bin/sh\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", dir)
}

func TestTrackAndUntrack(t *testing.T) {
	isolatedTool(t, "mytool")

	if err := Track([]string{"mytool"}); err != nil {
		t.Fatal(err)
	}
	if !manifest.Has("mytool", common.SourceManual) {
		t.Fatal("mytool not tracked as manual")
	}

	if err := Track([]string{"mytool"}); err != nil {
		t.Fatal(err)
	}
	if got := len(manifest.Lookup("mytool")); got != 1 {
		t.Fatalf("re-track duplicated entry: %d entries", got)
	}

	if err := Untrack([]string{"mytool"}); err != nil {
		t.Fatal(err)
	}
	if manifest.Has("mytool", common.SourceManual) {
		t.Fatal("mytool still tracked after untrack")
	}
}

func TestTrackMissingOrWrongSourceAddsNothing(t *testing.T) {
	isolatedTool(t, "mytool")

	if err := Track([]string{"absent", "mytool:rs"}); err != nil {
		t.Fatal(err)
	}
	entries, err := manifest.All()
	if err != nil || len(entries) != 0 {
		t.Fatalf("manifest = %+v, err %v; want empty", entries, err)
	}
}

func TestTrackUsageAndUntrackUntracked(t *testing.T) {
	isolatedTool(t, "mytool")

	if Track(nil) == nil || Untrack(nil) == nil || Info(nil) == nil {
		t.Fatal("no args must return a usage error")
	}
	if err := Untrack([]string{"mytool"}); err != nil {
		t.Fatalf("untracking an untracked tool reports, not errors: %v", err)
	}
}

func TestPickInstall(t *testing.T) {
	installs := []common.Install{
		{Source: common.SourceCargo, Path: "/a/cargo/x", Active: true},
		{Source: common.SourceAppImage, Path: "/a/appimage/x"},
	}

	got, err := pickInstall("x", installs, "")
	if err != nil || got.Source != common.SourceCargo {
		t.Fatalf("default = %+v, %v; want the active cargo install", got, err)
	}

	got, err = pickInstall("x", installs, common.SourceGHAppImage)
	if err != nil || got.Source != common.SourceAppImage {
		t.Fatalf("img = %+v, %v; want the appimage install", got, err)
	}

	if _, err = pickInstall("x", installs, common.SourceNPM); err == nil {
		t.Fatal("a source with no install must error")
	}

	unknown := []common.Install{{Source: sourceUnknown, Path: "/opt/x", Active: true}}
	if _, err = pickInstall("x", unknown, ""); err == nil {
		t.Fatal("an active install of unknown source must error")
	}
}
