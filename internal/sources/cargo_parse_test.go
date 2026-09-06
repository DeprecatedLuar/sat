package sources

import (
	"reflect"
	"testing"
)

func TestParseCargoInstallListEntries(t *testing.T) {
	tests := []struct {
		name   string
		output string
		want   []cargoCrateEntry
	}{
		{
			name:   "single crate one binary",
			output: "pipes-rs v0.1.1:\n    pipes-rs\n",
			want: []cargoCrateEntry{
				{Crate: "pipes-rs", Version: "0.1.1", Bins: []string{"pipes-rs"}},
			},
		},
		{
			name:   "crate with binary name differing from crate name",
			output: "ripgrep v14.1.1:\n    rg\n",
			want: []cargoCrateEntry{
				{Crate: "ripgrep", Version: "14.1.1", Bins: []string{"rg"}},
			},
		},
		{
			name:   "multiple binaries under one crate",
			output: "cargo-edit v0.12.2:\n    cargo-add\n    cargo-rm\n    cargo-upgrade\n",
			want: []cargoCrateEntry{
				{Crate: "cargo-edit", Version: "0.12.2", Bins: []string{"cargo-add", "cargo-rm", "cargo-upgrade"}},
			},
		},
		{
			name:   "multiple crates",
			output: "pipes-rs v0.1.1:\n    pipes-rs\nripgrep v14.1.1:\n    rg\n",
			want: []cargoCrateEntry{
				{Crate: "pipes-rs", Version: "0.1.1", Bins: []string{"pipes-rs"}},
				{Crate: "ripgrep", Version: "14.1.1", Bins: []string{"rg"}},
			},
		},
		{
			name:   "git-sourced crate with trailing URL segment",
			output: "my-tool v0.3.0 (https://github.com/example/my-tool#abcdef):\n    my-tool\n",
			want: []cargoCrateEntry{
				{Crate: "my-tool", Version: "0.3.0", Bins: []string{"my-tool"}},
			},
		},
		{
			name:   "empty output",
			output: "",
			want:   nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := parseCargoInstallListEntries(tt.output)
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("parseCargoInstallListEntries(%q) = %+v, want %+v", tt.output, got, tt.want)
			}
		})
	}
}

func TestParseCargoInstallList(t *testing.T) {
	output := "ripgrep v14.1.1:\n    rg\ncargo-edit v0.12.2:\n    cargo-add\n    cargo-rm\n"
	got := parseCargoInstallList(output)
	want := map[string]string{
		"ripgrep":    "14.1.1",
		"rg":         "14.1.1",
		"cargo-edit": "0.12.2",
		"cargo-add":  "0.12.2",
		"cargo-rm":   "0.12.2",
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("parseCargoInstallList(%q) = %v, want %v", output, got, want)
	}
}
