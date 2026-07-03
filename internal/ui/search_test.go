package ui

import (
	"reflect"
	"testing"
)

func TestFilterRelevant(t *testing.T) {
	results := []string{
		"ripgrep 14.1.0 - recursively search directories for a pattern",
		"rg-utils 1.0.0 - unrelated helper tools",
		"bat 0.24.0 - a cat clone with syntax highlighting",
		"fd-find 9.0.0 - a simple, fast alternative to find",
		"jq 1.7.1 - lightweight command-line JSON processor",
	}

	tests := []struct {
		name  string
		query string
		want  []string
	}{
		{
			name:  "exact name match",
			query: "bat",
			want:  []string{"bat 0.24.0 - a cat clone with syntax highlighting"},
		},
		{
			name:  "prefix match at delimiter",
			query: "fd",
			want:  []string{"fd-find 9.0.0 - a simple, fast alternative to find"},
		},
		{
			name:  "description-only match",
			query: "json",
			want:  []string{"jq 1.7.1 - lightweight command-line JSON processor"},
		},
		{
			name:  "no match",
			query: "nonexistent",
			want:  []string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := FilterRelevant(results, tt.query)
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("FilterRelevant(%q) = %v, want %v", tt.query, got, tt.want)
			}
		})
	}
}

func TestMatchesWithDelimiters(t *testing.T) {
	tests := []struct {
		name  string
		query string
		want  bool
	}{
		{"ripgrep", "ripgrep", true},         // exact
		{"fd-find", "fd", true},              // prefix + delimiter
		{"find-fd", "fd", true},              // suffix + delimiter
		{"my-fd-tool", "fd", true},           // middle, delimiter both sides
		{"findfd", "fd", true},               // suffix (no delimiter still matches per HasSuffix branch)
		{"ripgrepx", "rg", false},            // no boundary match, no substring
		{"unrelated", "fd", false},           // no match at all
	}

	for _, tt := range tests {
		got := matchesWithDelimiters(tt.name, tt.query)
		if got != tt.want {
			t.Errorf("matchesWithDelimiters(%q, %q) = %v, want %v", tt.name, tt.query, got, tt.want)
		}
	}
}

func TestColorizeResult(t *testing.T) {
	tests := []struct {
		name   string
		result string
		want   string
	}{
		{
			name:   "with description",
			result: "ripgrep 14.1.0 - recursively search directories",
			want:   Rust + "ripgrep" + Reset + " 14.1.0" + Dim + " - recursively search directories" + Reset,
		},
		{
			name:   "no description",
			result: "ripgrep 14.1.0",
			want:   Rust + "ripgrep 14.1.0" + Reset,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ColorizeResult(tt.result, Rust)
			if got != tt.want {
				t.Errorf("ColorizeResult(%q) = %q, want %q", tt.result, got, tt.want)
			}
		})
	}
}
