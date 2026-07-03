package ui

import "testing"

func TestSourceDisplay(t *testing.T) {
	tests := []struct {
		sourceStr string
		want      string
	}{
		{"cargo::14.1.0", "rust"},
		{"npm::1.0.0", "node"},
		{"uv::0.5.0", "python"},
		{"pacman::", "system"},
		{"gh:owner/repo:v1.0", "github"},
		{"brew::5.0", "brew"},
	}

	for _, tt := range tests {
		got := SourceDisplay(tt.sourceStr)
		if got != tt.want {
			t.Errorf("SourceDisplay(%q) = %q, want %q", tt.sourceStr, got, tt.want)
		}
	}
}

func TestSourceColor(t *testing.T) {
	tests := []struct {
		sourceStr string
		want      string
	}{
		{"cargo::1.0", Rust},
		{"npm::1.0", Node},
		{"uv::1.0", Python},
		{"system::", System},
		{"unknown::", Dim},
	}

	for _, tt := range tests {
		got := SourceColor(tt.sourceStr)
		if got != tt.want {
			t.Errorf("SourceColor(%q) = %q, want %q", tt.sourceStr, got, tt.want)
		}
	}
}

// TestSourceLight locks in the fix for a prior drift bug: SourceLight was
// missing the "unknown" case that SourceColor/SourceDisplay had, and would
// silently fall back to Reset instead of a defined light color.
func TestSourceLight(t *testing.T) {
	tests := []struct {
		sourceStr string
		want      string
	}{
		{"cargo::1.0", RustLight},
		{"npm::1.0", NodeLight},
		{"system::", SystemLight},
		{"unknown::", Reset},
	}

	for _, tt := range tests {
		got := SourceLight(tt.sourceStr)
		if got != tt.want {
			t.Errorf("SourceLight(%q) = %q, want %q", tt.sourceStr, got, tt.want)
		}
	}
}
