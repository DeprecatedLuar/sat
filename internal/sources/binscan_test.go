package sources

import (
	"os"
	"path/filepath"
	"testing"
)

func TestScanBinDirsOwnedBy(t *testing.T) {
	binDir := t.TempDir()

	// An executable "owned" by a manually-installed package
	ripgrepPath := filepath.Join(binDir, "rg")
	if err := os.WriteFile(ripgrepPath, []byte("#!/bin/sh\n"), 0755); err != nil {
		t.Fatalf("failed to write executable fixture: %v", err)
	}

	// An executable NOT owned by a manually-installed package (dependency-only)
	depPath := filepath.Join(binDir, "some-lib-helper")
	if err := os.WriteFile(depPath, []byte("#!/bin/sh\n"), 0755); err != nil {
		t.Fatalf("failed to write executable fixture: %v", err)
	}

	// A non-executable file, should be skipped regardless of ownership
	nonExecPath := filepath.Join(binDir, "readme.txt")
	if err := os.WriteFile(nonExecPath, []byte("not a binary"), 0644); err != nil {
		t.Fatalf("failed to write non-executable fixture: %v", err)
	}

	isManual := map[string]bool{"ripgrep": true}

	ownerFn := func(binPath string) string {
		switch filepath.Base(binPath) {
		case "rg":
			return "ripgrep"
		case "some-lib-helper":
			return "some-dependency"
		default:
			return ""
		}
	}

	versionFn := func(binName string) string {
		if binName == "rg" {
			return "14.1.0"
		}
		return ""
	}

	packages := scanBinDirsOwnedBy([]string{binDir}, "apt", isManual, ownerFn, versionFn)

	if len(packages) != 1 {
		t.Fatalf("scanBinDirsOwnedBy() returned %d packages, want 1: %+v", len(packages), packages)
	}

	got := packages[0]
	want := Package{Name: "rg", Source: "apt", Identity: "", Version: "14.1.0"}
	if got != want {
		t.Errorf("scanBinDirsOwnedBy() = %+v, want %+v", got, want)
	}
}

func TestScanBinDirsOwnedByMissingDir(t *testing.T) {
	packages := scanBinDirsOwnedBy(
		[]string{filepath.Join(t.TempDir(), "does-not-exist")},
		"apt",
		map[string]bool{"ripgrep": true},
		func(string) string { return "ripgrep" },
		func(string) string { return "1.0" },
	)

	if packages != nil {
		t.Errorf("scanBinDirsOwnedBy() with missing dir = %+v, want nil", packages)
	}
}
