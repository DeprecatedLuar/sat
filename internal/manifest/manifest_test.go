package manifest

import (
	"os"
	"testing"
)

func TestSourceStringRoundTrip(t *testing.T) {
	tests := []struct {
		source   string
		identity string
		version  string
	}{
		{"cargo", "", "14.1.0"},
		{"gh", "owner/repo", "v1.0.0"},
		{"flatpak", "com.app.id", "1.2.3"},
		{"nixos", "", ""},
	}

	for _, tt := range tests {
		// Build source string
		sourceStr := BuildSourceString(tt.source, tt.identity, tt.version)

		// Parse it back
		gotSource := GetSourceType(sourceStr)
		gotIdentity := GetSourceIdentity(sourceStr)
		gotVersion := GetSourceVersion(sourceStr)

		// Verify round-trip
		if gotSource != tt.source {
			t.Errorf("GetSourceType(%q) = %q, want %q", sourceStr, gotSource, tt.source)
		}
		if gotIdentity != tt.identity {
			t.Errorf("GetSourceIdentity(%q) = %q, want %q", sourceStr, gotIdentity, tt.identity)
		}
		if gotVersion != tt.version {
			t.Errorf("GetSourceVersion(%q) = %q, want %q", sourceStr, gotVersion, tt.version)
		}
	}
}

func TestManifestOperations(t *testing.T) {
	// Create temp directory for test manifest
	tmpDir := t.TempDir()

	// Override DataDir for testing
	origSatData := os.Getenv("SAT_DATA")
	os.Setenv("SAT_DATA", tmpDir)
	defer os.Setenv("SAT_DATA", origSatData)

	// Test Add
	err := Add("ripgrep", "cargo::14.1.0")
	if err != nil {
		t.Fatalf("Add() error = %v", err)
	}

	// Test Has
	if !Has("ripgrep") {
		t.Error("Has(ripgrep) = false, want true")
	}

	// Test Get
	got := Get("ripgrep")
	want := "cargo::14.1.0"
	if got != want {
		t.Errorf("Get(ripgrep) = %q, want %q", got, want)
	}

	// Test Add another tool
	err = Add("fd", "gh:sharkdp/fd:v10.0.0")
	if err != nil {
		t.Fatalf("Add(fd) error = %v", err)
	}

	if !Has("fd") {
		t.Error("Has(fd) = false, want true")
	}

	// Test Remove
	err = Remove("ripgrep")
	if err != nil {
		t.Fatalf("Remove() error = %v", err)
	}

	if Has("ripgrep") {
		t.Error("Has(ripgrep) after Remove = true, want false")
	}

	// fd should still exist
	if !Has("fd") {
		t.Error("Has(fd) after removing ripgrep = false, want true")
	}

	// Test removing non-existent tool (should not error)
	err = Remove("nonexistent")
	if err != nil {
		t.Errorf("Remove(nonexistent) error = %v, want nil", err)
	}
}

func TestGetNonExistent(t *testing.T) {
	tmpDir := t.TempDir()
	origSatData := os.Getenv("SAT_DATA")
	os.Setenv("SAT_DATA", tmpDir)
	defer os.Setenv("SAT_DATA", origSatData)

	got := Get("nonexistent")
	if got != "" {
		t.Errorf("Get(nonexistent) = %q, want empty string", got)
	}
}
