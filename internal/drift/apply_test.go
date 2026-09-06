package drift

import (
	"os"
	"testing"

	"github.com/DeprecatedLuar/sat/internal/manifest"
)

func withSatData(t *testing.T) string {
	t.Helper()
	tmpDir := t.TempDir()
	orig := os.Getenv("SAT_DATA")
	os.Setenv("SAT_DATA", tmpDir)
	t.Cleanup(func() { os.Setenv("SAT_DATA", orig) })
	return tmpDir
}

func TestApply(t *testing.T) {
	withSatData(t)

	if err := manifest.Add("codex", "npm:@openai/codex:0.153.0"); err != nil {
		t.Fatalf("Add() error = %v", err)
	}
	if err := manifest.Add("claude", "npm:@anthropic-ai/claude-code:2.1.259"); err != nil {
		t.Fatalf("Add() error = %v", err)
	}

	drifts := []Drift{
		{Tool: "codex", Source: "npm:@openai/codex:0.153.0", Old: "0.153.0", New: "0.153.4"},
	}
	changed, err := Apply(drifts)
	if err != nil {
		t.Fatalf("Apply() error = %v", err)
	}
	if changed != 1 {
		t.Errorf("Apply() changed = %d, want 1", changed)
	}

	if got := manifest.Get("codex"); got != "npm:@openai/codex:0.153.4" {
		t.Errorf("Get(codex) = %q, want %q", got, "npm:@openai/codex:0.153.4")
	}
	// claude was not in the drift list and must be untouched.
	if got := manifest.Get("claude"); got != "npm:@anthropic-ai/claude-code:2.1.259" {
		t.Errorf("Get(claude) = %q, want unchanged", got)
	}
}

func TestApplyEmpty(t *testing.T) {
	withSatData(t)

	changed, err := Apply(nil)
	if err != nil {
		t.Fatalf("Apply(nil) error = %v", err)
	}
	if changed != 0 {
		t.Errorf("Apply(nil) changed = %d, want 0", changed)
	}
}
