package manifest

import (
	"os"
	"path/filepath"
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
	if !Has("ripgrep", "cargo") {
		t.Error("Has(ripgrep) = false, want true")
	}

	// Test Get
	got := Get("ripgrep", "cargo")
	want := "cargo::14.1.0"
	if got != want {
		t.Errorf("Get(ripgrep) = %q, want %q", got, want)
	}

	// Test Add another tool
	err = Add("fd", "gh:sharkdp/fd:v10.0.0")
	if err != nil {
		t.Fatalf("Add(fd) error = %v", err)
	}

	if !Has("fd", "gh") {
		t.Error("Has(fd) = false, want true")
	}

	// Test Remove
	err = Remove("ripgrep", "cargo")
	if err != nil {
		t.Fatalf("Remove() error = %v", err)
	}

	if Has("ripgrep", "cargo") {
		t.Error("Has(ripgrep) after Remove = true, want false")
	}

	// fd should still exist
	if !Has("fd", "gh") {
		t.Error("Has(fd) after removing ripgrep = false, want true")
	}

	// Test removing non-existent tool (should not error)
	err = Remove("nonexistent", "cargo")
	if err != nil {
		t.Errorf("Remove(nonexistent) error = %v, want nil", err)
	}
}

func TestGetNonExistent(t *testing.T) {
	tmpDir := t.TempDir()
	origSatData := os.Getenv("SAT_DATA")
	os.Setenv("SAT_DATA", tmpDir)
	defer os.Setenv("SAT_DATA", origSatData)

	got := Get("nonexistent", "cargo")
	if got != "" {
		t.Errorf("Get(nonexistent) = %q, want empty string", got)
	}
}

func TestAddMany(t *testing.T) {
	tmpDir := t.TempDir()
	origSatData := os.Getenv("SAT_DATA")
	os.Setenv("SAT_DATA", tmpDir)
	defer os.Setenv("SAT_DATA", origSatData)

	if err := Add("ripgrep", "cargo::14.1.0"); err != nil {
		t.Fatalf("Add(ripgrep) error = %v", err)
	}
	if err := Add("fd", "cargo::10.0.0"); err != nil {
		t.Fatalf("Add(fd) error = %v", err)
	}
	if err := Add("bat", "cargo::0.24.0"); err != nil {
		t.Fatalf("Add(bat) error = %v", err)
	}

	// Update fd in place, add a brand new tool, and leave ripgrep untouched
	// by omitting it - AddMany must only touch what it's given.
	changed, err := AddMany([]Entry{
		{Tool: "fd", Source: "cargo::10.3.0"},
		{Tool: "zoxide", Source: "cargo::0.9.8"},
	})
	if err != nil {
		t.Fatalf("AddMany() error = %v", err)
	}
	if changed != 2 {
		t.Errorf("AddMany() changed = %d, want 2", changed)
	}

	entries, err := All()
	if err != nil {
		t.Fatalf("All() error = %v", err)
	}
	if len(entries) != 4 {
		t.Fatalf("All() len = %d, want 4 (ripgrep, fd, bat, zoxide)", len(entries))
	}

	// fd must have kept its original position (index 1), only its source changed.
	if entries[1].Tool != "fd" || entries[1].Source != "cargo::10.3.0" {
		t.Errorf("entries[1] = %+v, want {fd cargo::10.3.0} at same position", entries[1])
	}
	// ripgrep and bat must be untouched.
	if entries[0].Tool != "ripgrep" || entries[0].Source != "cargo::14.1.0" {
		t.Errorf("entries[0] = %+v, want ripgrep unchanged", entries[0])
	}
	if entries[2].Tool != "bat" || entries[2].Source != "cargo::0.24.0" {
		t.Errorf("entries[2] = %+v, want bat unchanged", entries[2])
	}
	// zoxide is new, appended at the end.
	if entries[3].Tool != "zoxide" || entries[3].Source != "cargo::0.9.8" {
		t.Errorf("entries[3] = %+v, want {zoxide cargo::0.9.8} appended", entries[3])
	}
}

func TestAddManyNoOpDoesNotWrite(t *testing.T) {
	tmpDir := t.TempDir()
	origSatData := os.Getenv("SAT_DATA")
	os.Setenv("SAT_DATA", tmpDir)
	defer os.Setenv("SAT_DATA", origSatData)

	if err := Add("ripgrep", "cargo::14.1.0"); err != nil {
		t.Fatalf("Add() error = %v", err)
	}

	info, err := os.Stat(ManifestPath())
	if err != nil {
		t.Fatalf("Stat() error = %v", err)
	}
	before := info.ModTime()

	// Re-applying the exact same source should be a true no-op: no file write.
	changed, err := AddMany([]Entry{{Tool: "ripgrep", Source: "cargo::14.1.0"}})
	if err != nil {
		t.Fatalf("AddMany() error = %v", err)
	}
	if changed != 0 {
		t.Errorf("AddMany() changed = %d, want 0 for identical source", changed)
	}

	info, err = os.Stat(ManifestPath())
	if err != nil {
		t.Fatalf("Stat() error = %v", err)
	}
	if !info.ModTime().Equal(before) {
		t.Error("AddMany() with no real changes modified the manifest file")
	}
}

func TestAddManyEmpty(t *testing.T) {
	tmpDir := t.TempDir()
	origSatData := os.Getenv("SAT_DATA")
	os.Setenv("SAT_DATA", tmpDir)
	defer os.Setenv("SAT_DATA", origSatData)

	changed, err := AddMany(nil)
	if err != nil {
		t.Fatalf("AddMany(nil) error = %v", err)
	}
	if changed != 0 {
		t.Errorf("AddMany(nil) changed = %d, want 0", changed)
	}
}

func TestWriteEntriesLeavesNoTempResidue(t *testing.T) {
	tmpDir := t.TempDir()
	origSatData := os.Getenv("SAT_DATA")
	os.Setenv("SAT_DATA", tmpDir)
	defer os.Setenv("SAT_DATA", origSatData)

	if err := Add("ripgrep", "cargo::14.1.0"); err != nil {
		t.Fatalf("Add() error = %v", err)
	}
	if _, err := AddMany([]Entry{{Tool: "fd", Source: "cargo::10.3.0"}}); err != nil {
		t.Fatalf("AddMany() error = %v", err)
	}
	if err := Remove("ripgrep", "cargo"); err != nil {
		t.Fatalf("Remove() error = %v", err)
	}

	dirEntries, err := os.ReadDir(tmpDir)
	if err != nil {
		t.Fatalf("ReadDir() error = %v", err)
	}
	for _, e := range dirEntries {
		if e.Name() != ManifestFileName {
			t.Errorf("unexpected residue in data dir: %s (want only %q)", e.Name(), ManifestFileName)
		}
	}

	entries, err := All()
	if err != nil {
		t.Fatalf("All() error = %v", err)
	}
	if len(entries) != 1 || entries[0].Tool != "fd" {
		t.Errorf("All() = %+v, want only fd", entries)
	}
}

func TestStateDirRespectsSATData(t *testing.T) {
	tmpDir := t.TempDir()
	origSatData := os.Getenv("SAT_DATA")
	os.Setenv("SAT_DATA", tmpDir)
	defer os.Setenv("SAT_DATA", origSatData)

	want := filepath.Join(tmpDir, StateDirName)
	if got := StateDir(); got != want {
		t.Errorf("StateDir() = %q, want %q", got, want)
	}

	wantStamp := filepath.Join(want, DriftStampName)
	if got := DriftStampPath(); got != wantStamp {
		t.Errorf("DriftStampPath() = %q, want %q", got, wantStamp)
	}
}

func withSatData(t *testing.T) {
	t.Helper()
	t.Setenv("SAT_DATA", t.TempDir())
}

func TestMultiSourceCoexist(t *testing.T) {
	withSatData(t)

	for _, src := range []string{"flatpak:com.x.Zap:1.0", "appimage:owner/zap:v1"} {
		if err := Add("zapzap", src); err != nil {
			t.Fatalf("Add(%s) error = %v", src, err)
		}
	}

	got := Lookup("zapzap")
	if len(got) != 2 {
		t.Fatalf("Lookup len = %d, want 2: %+v", len(got), got)
	}
	if got[0].Source != "flatpak:com.x.Zap:1.0" || got[1].Source != "appimage:owner/zap:v1" {
		t.Errorf("Lookup order/content wrong: %+v", got)
	}
}

func TestAddUpdatesOnlyOwnSource(t *testing.T) {
	withSatData(t)

	Add("zapzap", "flatpak:com.x.Zap:1.0")
	Add("zapzap", "appimage:owner/zap:v1")
	if err := Add("zapzap", "appimage:owner/zap:v2"); err != nil {
		t.Fatalf("Add() error = %v", err)
	}

	if got := Get("zapzap", "appimage"); got != "appimage:owner/zap:v2" {
		t.Errorf("appimage = %q, want v2", got)
	}
	if got := Get("zapzap", "flatpak"); got != "flatpak:com.x.Zap:1.0" {
		t.Errorf("flatpak = %q, want unchanged", got)
	}
	if n := len(Lookup("zapzap")); n != 2 {
		t.Errorf("Lookup len = %d, want 2", n)
	}
}

func TestRemoveLeavesOtherSource(t *testing.T) {
	withSatData(t)

	Add("cowsay", "npm::1.0")
	Add("cowsay", "uv::2.0")
	if err := Remove("cowsay", "uv"); err != nil {
		t.Fatalf("Remove() error = %v", err)
	}

	if Has("cowsay", "uv") {
		t.Error("uv entry still present")
	}
	if !Has("cowsay", "npm") {
		t.Error("npm entry was removed")
	}
}

func TestAliasesMatchCanonical(t *testing.T) {
	withSatData(t)

	Add("ripgrep", "rust::14.0.0")
	if !Has("ripgrep", "cargo") {
		t.Error("rust-recorded entry should match cargo")
	}
	if err := Add("ripgrep", "cargo::14.1.0"); err != nil {
		t.Fatalf("Add() error = %v", err)
	}
	if n := len(Lookup("ripgrep")); n != 1 {
		t.Errorf("Lookup len = %d, want 1 (cargo replaces rust line)", n)
	}

	Add("curl", "apt::8.0")
	if !Has("curl", "system") {
		t.Error("apt-recorded entry should match system")
	}

	Add("hello", "nixos::")
	if Has("hello", "nix") {
		t.Error("nixos must stay distinct from nix")
	}
}

func TestAddManyIsSourceScoped(t *testing.T) {
	withSatData(t)

	Add("cowsay", "npm::1.0")
	Add("cowsay", "uv::1.0")

	changed, err := AddMany([]Entry{
		{Tool: "cowsay", Source: "uv::2.0"},
		{Tool: "cowsay", Source: "cargo::3.0"},
	})
	if err != nil {
		t.Fatalf("AddMany() error = %v", err)
	}
	if changed != 2 {
		t.Errorf("changed = %d, want 2", changed)
	}
	if got := Get("cowsay", "npm"); got != "npm::1.0" {
		t.Errorf("npm = %q, want unchanged", got)
	}
	if got := Get("cowsay", "uv"); got != "uv::2.0" {
		t.Errorf("uv = %q, want uv::2.0", got)
	}
	if got := Get("cowsay", "cargo"); got != "cargo::3.0" {
		t.Errorf("cargo = %q, want appended", got)
	}
}
