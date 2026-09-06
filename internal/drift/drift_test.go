package drift

import (
	"reflect"
	"testing"

	"github.com/DeprecatedLuar/sat/internal/manifest"
)

func TestDiff(t *testing.T) {
	tests := []struct {
		name    string
		entries []manifest.Entry
		key     func(manifest.Entry) string
		live    map[string]string
		want    []Drift
	}{
		{
			name:    "version changed",
			entries: []manifest.Entry{{Tool: "codex", Source: "npm:@openai/codex:0.153.0"}},
			key:     defaultKey,
			live:    map[string]string{"codex": "0.153.4"},
			want: []Drift{
				{Tool: "codex", Source: "npm:@openai/codex:0.153.0", Old: "0.153.0", New: "0.153.4"},
			},
		},
		{
			name:    "version identical - no drift",
			entries: []manifest.Entry{{Tool: "codex", Source: "npm:@openai/codex:0.153.4"}},
			key:     defaultKey,
			live:    map[string]string{"codex": "0.153.4"},
			want:    nil,
		},
		{
			name:    "tool absent from live map - never treated as uninstalled",
			entries: []manifest.Entry{{Tool: "codex", Source: "npm:@openai/codex:0.153.0"}},
			key:     defaultKey,
			live:    map[string]string{"other-tool": "1.0.0"},
			want:    nil,
		},
		{
			name:    "live version empty - never written",
			entries: []manifest.Entry{{Tool: "codex", Source: "npm:@openai/codex:0.153.0"}},
			key:     defaultKey,
			live:    map[string]string{"codex": ""},
			want:    nil,
		},
		{
			name:    "recorded version empty, live present - repair case",
			entries: []manifest.Entry{{Tool: "steam", Source: "nixos::"}},
			key:     defaultKey,
			live:    map[string]string{"steam": "1.0.0.81"},
			want: []Drift{
				{Tool: "steam", Source: "nixos::", Old: "", New: "1.0.0.81"},
			},
		},
		{
			name:    "flatpak keyed by identity, not tool name",
			entries: []manifest.Entry{{Tool: "obsidian", Source: "flatpak:md.obsidian.Obsidian:1.5.0"}},
			key:     identityKey,
			live:    map[string]string{"md.obsidian.Obsidian": "1.6.0"},
			want: []Drift{
				{Tool: "obsidian", Source: "flatpak:md.obsidian.Obsidian:1.5.0", Old: "1.5.0", New: "1.6.0"},
			},
		},
		{
			name:    "empty live map - whole provider skipped, not read as everything missing",
			entries: []manifest.Entry{{Tool: "codex", Source: "npm:@openai/codex:0.153.0"}},
			key:     defaultKey,
			live:    map[string]string{},
			want:    nil,
		},
		{
			name:    "nil live map",
			entries: []manifest.Entry{{Tool: "codex", Source: "npm:@openai/codex:0.153.0"}},
			key:     defaultKey,
			live:    nil,
			want:    nil,
		},
		{
			name:    "empty key skipped",
			entries: []manifest.Entry{{Tool: "obsidian", Source: "flatpak::1.5.0"}},
			key:     identityKey,
			live:    map[string]string{"": "1.6.0"},
			want:    nil,
		},
		{
			name: "multiple entries, only one drifted",
			entries: []manifest.Entry{
				{Tool: "codex", Source: "npm:@openai/codex:0.153.0"},
				{Tool: "claude", Source: "npm:@anthropic-ai/claude-code:2.1.259"},
			},
			key: defaultKey,
			live: map[string]string{
				"codex":  "0.153.4",
				"claude": "2.1.259",
			},
			want: []Drift{
				{Tool: "codex", Source: "npm:@openai/codex:0.153.0", Old: "0.153.0", New: "0.153.4"},
			},
		},
		{
			name: "downgrade is still reported as drift",
			entries: []manifest.Entry{
				{Tool: "codex", Source: "npm:@openai/codex:0.153.4"},
			},
			key:  defaultKey,
			live: map[string]string{"codex": "0.150.0"},
			want: []Drift{
				{Tool: "codex", Source: "npm:@openai/codex:0.153.4", Old: "0.153.4", New: "0.150.0"},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := diff(tt.entries, tt.key, tt.live)
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("diff() = %+v, want %+v", got, tt.want)
			}
		})
	}
}

func TestDriftNewSource(t *testing.T) {
	d := Drift{Tool: "codex", Source: "npm:@openai/codex:0.153.0", Old: "0.153.0", New: "0.153.4"}
	want := "npm:@openai/codex:0.153.4"
	if got := d.NewSource(); got != want {
		t.Errorf("NewSource() = %q, want %q", got, want)
	}
}
