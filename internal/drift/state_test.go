package drift

import (
	"os"
	"testing"
	"time"

	"github.com/DeprecatedLuar/sat/internal/manifest"
)

func readStampRaw(t *testing.T) (string, bool) {
	t.Helper()
	data, err := os.ReadFile(manifest.DriftStampPath())
	if err != nil {
		return "", false
	}
	return string(data), true
}

func TestEnsureWithinTTL(t *testing.T) {
	withSatData(t)

	// No stamp yet - first call must run (and leave a stamp behind).
	if _, err := EnsureWithin(time.Hour); err != nil {
		t.Fatalf("EnsureWithin() first call error = %v", err)
	}
	first, ok := readStampRaw(t)
	if !ok {
		t.Fatal("EnsureWithin() left no stamp file after first run")
	}

	// Immediately within the window - must be a no-op, stamp untouched.
	if _, err := EnsureWithin(time.Hour); err != nil {
		t.Fatalf("EnsureWithin() second call error = %v", err)
	}
	second, ok := readStampRaw(t)
	if !ok || second != first {
		t.Errorf("EnsureWithin() within the window changed the stamp: before=%q after=%q", first, second)
	}

	// Backdate the stamp past the interval - must run again.
	if err := writeStamp(time.Now().Add(-2 * time.Hour)); err != nil {
		t.Fatalf("writeStamp() error = %v", err)
	}
	backdated, _ := readStampRaw(t)

	if _, err := EnsureWithin(time.Hour); err != nil {
		t.Fatalf("EnsureWithin() after backdating error = %v", err)
	}
	third, ok := readStampRaw(t)
	if !ok || third == backdated {
		t.Errorf("EnsureWithin() with a stale stamp did not reconcile: stamp unchanged at %q", third)
	}
}

func TestEnsureWithinDisabled(t *testing.T) {
	withSatData(t)

	// interval <= 0 must never even look at the stamp file.
	changed, err := EnsureWithin(0)
	if err != nil {
		t.Fatalf("EnsureWithin(0) error = %v", err)
	}
	if changed != 0 {
		t.Errorf("EnsureWithin(0) changed = %d, want 0", changed)
	}
	if _, ok := readStampRaw(t); ok {
		t.Error("EnsureWithin(0) created a stamp file despite being disabled")
	}
}

func TestEnsureWithinMissingOrCorruptStamp(t *testing.T) {
	withSatData(t)

	// Corrupt/malformed stamp content must be treated as "never ran", not
	// as an error that blocks reconciliation.
	dir := manifest.StateDir()
	if err := os.MkdirAll(dir, manifest.DirPermissions); err != nil {
		t.Fatalf("MkdirAll() error = %v", err)
	}
	if err := os.WriteFile(manifest.DriftStampPath(), []byte("not a valid stamp"), manifest.FilePermissions); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	if _, err := EnsureWithin(time.Hour); err != nil {
		t.Fatalf("EnsureWithin() with a corrupt stamp error = %v", err)
	}
	content, ok := readStampRaw(t)
	if !ok || content == "not a valid stamp" {
		t.Errorf("EnsureWithin() did not treat a corrupt stamp as stale: content = %q", content)
	}
}

func TestClaimIsExclusive(t *testing.T) {
	withSatData(t)

	release1, ok1 := claim()
	if !ok1 {
		t.Fatal("claim() first call ok = false, want true")
	}

	_, ok2 := claim()
	if ok2 {
		t.Error("claim() second call while held ok = true, want false")
	}

	release1()

	release3, ok3 := claim()
	if !ok3 {
		t.Error("claim() after release ok = false, want true")
	}
	release3()
}

func TestClaimReclaimsStaleLock(t *testing.T) {
	withSatData(t)

	dir := manifest.StateDir()
	if err := os.MkdirAll(dir, manifest.DirPermissions); err != nil {
		t.Fatalf("MkdirAll() error = %v", err)
	}
	path := lockPath()
	if err := os.WriteFile(path, nil, manifest.FilePermissions); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}
	old := time.Now().Add(-staleLockAge - time.Minute)
	if err := os.Chtimes(path, old, old); err != nil {
		t.Fatalf("Chtimes() error = %v", err)
	}

	release, ok := claim()
	if !ok {
		t.Fatal("claim() did not reclaim a stale lock")
	}
	release()
}
