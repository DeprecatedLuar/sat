package common

import "testing"

func TestParseToolSpec(t *testing.T) {
	tests := []struct {
		spec       string
		wantName   string
		wantSource string
	}{
		{"ripgrep", "ripgrep", ""},
		{"ripgrep:rs", "ripgrep", "cargo"},
		{"ripgrep:rust", "ripgrep", "cargo"},
		{"ranger:py", "ranger", "uv"},
		{"ranger:python", "ranger", "uv"},
		{"fd:sys", "fd", "system"},
		{"bat:gh", "bat", "gh"},
		{"hyperfine:brew", "hyperfine", "brew"},
	}

	for _, tt := range tests {
		gotName, gotSource := ParseToolSpec(tt.spec)
		if gotName != tt.wantName {
			t.Errorf("ParseToolSpec(%q) name = %q, want %q", tt.spec, gotName, tt.wantName)
		}
		if gotSource != tt.wantSource {
			t.Errorf("ParseToolSpec(%q) source = %q, want %q", tt.spec, gotSource, tt.wantSource)
		}
	}
}
