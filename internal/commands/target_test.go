package commands

import (
	"strings"
	"testing"

	"github.com/DeprecatedLuar/sat/internal/manifest"
)

func seedCowsay(t *testing.T) {
	t.Helper()
	t.Setenv("SAT_DATA", t.TempDir())
	for _, src := range []string{"npm::1.0", "uv::1.0", "cargo::1.0", "nix::1.0"} {
		if err := manifest.Add("cowsay", src); err != nil {
			t.Fatal(err)
		}
	}
}

func TestResolveTargetsAllSources(t *testing.T) {
	seedCowsay(t)

	entries, err := resolveTargets("cowsay", "")
	if err != nil || len(entries) != 4 {
		t.Fatalf("resolveTargets = %d entries, err %v; want 4", len(entries), err)
	}

	_, err = requireOne("cowsay", entries)
	if err == nil {
		t.Fatal("requireOne should error on ambiguity")
	}
	for _, want := range []string{"node", "python", "rust", "nix", "cowsay:<source>"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("ambiguity error %q missing %q", err, want)
		}
	}
}

func TestResolveTargetsExplicitSource(t *testing.T) {
	seedCowsay(t)

	entries, err := resolveTargets("cowsay:py", "")
	if err != nil || len(entries) != 1 || entries[0].Source != "uv::1.0" {
		t.Fatalf("cowsay:py = %+v, err %v", entries, err)
	}

	entries, err = resolveTargets("cowsay", "npm")
	if err != nil || len(entries) != 1 || entries[0].Source != "npm::1.0" {
		t.Fatalf("flag npm = %+v, err %v", entries, err)
	}

	entries, err = resolveTargets("cowsay:uv", "npm")
	if err != nil || entries[0].Source != "uv::1.0" {
		t.Fatalf("suffix must win over flag: %+v, err %v", entries, err)
	}
}

func TestResolveTargetsErrors(t *testing.T) {
	seedCowsay(t)

	if _, err := resolveTargets("cowsay:brew", ""); err == nil || !strings.Contains(err.Error(), "not tracked from brew") {
		t.Errorf("untracked pair error = %v", err)
	}
	if _, err := resolveTargets("nothing", ""); err == nil || !strings.Contains(err.Error(), "not tracked by sat") {
		t.Errorf("unknown name error = %v", err)
	}
}

func TestTargetEntriesUpdateNamesAndFilter(t *testing.T) {
	seedCowsay(t)

	all, _ := targetEntries([]string{"cowsay"}, "")
	if len(all) != 4 {
		t.Errorf("named update = %d entries, want all 4", len(all))
	}
	filtered, _ := targetEntries([]string{"cowsay"}, "cargo")
	if len(filtered) != 1 {
		t.Errorf("filtered update = %d entries, want 1", len(filtered))
	}
}

func TestScopeSpecs(t *testing.T) {
	got := scopeSpecs([]string{"a", "--flatpak", "b", "c", "--nix", "d"})
	want := []scopedSpec{{"a", ""}, {"b", "flatpak"}, {"c", "flatpak"}, {"d", "nix"}}
	if len(got) != len(want) {
		t.Fatalf("got %+v", got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("spec %d = %+v, want %+v", i, got[i], want[i])
		}
	}
}
