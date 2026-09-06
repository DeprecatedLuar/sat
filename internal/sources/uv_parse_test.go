package sources

import (
	"reflect"
	"testing"
)

func TestParseUvToolListEntries(t *testing.T) {
	tests := []struct {
		name   string
		output string
		want   []uvToolEntry
	}{
		{
			name:   "package name differs from every binary it exposes",
			output: "graphifyy v0.9.53\n- graphify\n- graphify-mcp\n",
			want: []uvToolEntry{
				{Package: "graphifyy", Version: "0.9.53", Bins: []string{"graphify", "graphify-mcp"}},
			},
		},
		{
			name:   "package name also one of its own binaries",
			output: "kimi-cli v1.50.0\n- kimi\n- kimi-cli\n",
			want: []uvToolEntry{
				{Package: "kimi-cli", Version: "1.50.0", Bins: []string{"kimi", "kimi-cli"}},
			},
		},
		{
			name:   "package name equals its single binary",
			output: "legit v1.2.1\n- legit\n",
			want: []uvToolEntry{
				{Package: "legit", Version: "1.2.1", Bins: []string{"legit"}},
			},
		},
		{
			name:   "the real multi-package uv tool list shape",
			output: "graphifyy v0.9.53\n- graphify\n- graphify-mcp\nkimi-cli v1.50.0\n- kimi\n- kimi-cli\nlegit v1.2.1\n- legit\n",
			want: []uvToolEntry{
				{Package: "graphifyy", Version: "0.9.53", Bins: []string{"graphify", "graphify-mcp"}},
				{Package: "kimi-cli", Version: "1.50.0", Bins: []string{"kimi", "kimi-cli"}},
				{Package: "legit", Version: "1.2.1", Bins: []string{"legit"}},
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
			got := parseUvToolListEntries(tt.output)
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("parseUvToolListEntries(%q) = %+v, want %+v", tt.output, got, tt.want)
			}
		})
	}
}

func TestParseUvToolList(t *testing.T) {
	output := "graphifyy v0.9.53\n- graphify\n- graphify-mcp\nkimi-cli v1.50.0\n- kimi\n- kimi-cli\n"
	got := parseUvToolList(output)
	want := map[string]string{
		"graphifyy":    "0.9.53",
		"graphify":     "0.9.53",
		"graphify-mcp": "0.9.53",
		"kimi-cli":     "1.50.0",
		"kimi":         "1.50.0",
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("parseUvToolList(%q) = %v, want %v", output, got, want)
	}
}
