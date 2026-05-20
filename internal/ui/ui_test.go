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
	}

	for _, tt := range tests {
		got := SourceColor(tt.sourceStr)
		if got != tt.want {
			t.Errorf("SourceColor(%q) = %q, want %q", tt.sourceStr, got, tt.want)
		}
	}
}
