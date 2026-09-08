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

func TestFilterRelevantCollapsedSeparators(t *testing.T) {
	// Mirrors real `sat search kde-connect` nix output. kdePackages.kdeconnect-kde
	// is the actual KDE Connect package: its name matches only at the "."
	// boundary, so it survives solely because the collapsed-query fallback
	// leaves the name's delimiters intact. Collapsing both sides flattens it to
	// "kdepackageskdeconnectkde", which matches nothing — regression guard.
	results := []string{
		"emacsPackages.kdeconnect 20231029.2250 - no description",
		"kdePackages.kdeconnect-kde 25.08.3 - Multi-platform app that allows your devices to communicate",
		"yaziPlugins.kdeconnect-send 0 - Send selected files to your smartphone",
		"kdeconnect_waybar 1.1.2 - A highly customizable KDE Connect module for Waybar",
		"bat 0.24.0 - a cat clone with syntax highlighting",
		"jq 1.7.1 - lightweight command-line JSON processor",
	}

	want := []string{
		"emacsPackages.kdeconnect 20231029.2250 - no description",
		"kdePackages.kdeconnect-kde 25.08.3 - Multi-platform app that allows your devices to communicate",
		"yaziPlugins.kdeconnect-send 0 - Send selected files to your smartphone",
		"kdeconnect_waybar 1.1.2 - A highly customizable KDE Connect module for Waybar",
	}

	for _, query := range []string{"kde-connect", "kdeconnect"} {
		got := FilterRelevant(results, query)
		if !reflect.DeepEqual(got, want) {
			t.Errorf("FilterRelevant(%q) = %v, want %v", query, got, want)
		}
	}
}

// TestFilterRelevantSeparatorVariantParity locks the core promise of the
// separator-insensitive search: a hyphenated query must never return fewer
// results than its collapsed spelling.
func TestFilterRelevantSeparatorVariantParity(t *testing.T) {
	results := []string{
		"kdePackages.kdeconnect-kde 25.08.3 - Multi-platform app",
		"emacsPackages.kdeconnect 20231029.2250 - no description",
		"kdeconnect-cli 0.2.0 - A Command Line Interface",
		"kde-connect 26.04.2 - Communicate with your handheld devices",
		"unrelated-tool 1.0.0 - nothing to do with it",
	}

	hyphen := FilterRelevant(results, "kde-connect")
	collapsed := FilterRelevant(results, "kdeconnect")

	if len(hyphen) < len(collapsed) {
		t.Errorf("hyphenated query returned fewer results than collapsed:\n  kde-connect: %v\n  kdeconnect:  %v", hyphen, collapsed)
	}
	for _, want := range collapsed {
		found := false
		for _, got := range hyphen {
			if got == want {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("result %q matched by %q but dropped by %q", want, "kdeconnect", "kde-connect")
		}
	}
}

func TestFilterRelevantCollapsedDescription(t *testing.T) {
	results := []string{
		"soduto 1.0.0 - Soduto is a KDEConnect compatible client for macOS",
	}

	got := FilterRelevant(results, "kde-connect")
	want := []string{"soduto 1.0.0 - Soduto is a KDEConnect compatible client for macOS"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("FilterRelevant(%q) = %v, want %v", "kde-connect", got, want)
	}
}

func TestCollapseSeparators(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want string
	}{
		{"plain word unchanged", "kdeconnect", "kdeconnect"},
		{"hyphen collapsed", "kde-connect", "kdeconnect"},
		{"mixed delimiters collapsed", "foo_bar.baz", "foobarbaz"},
		{"all-delimiter string collapses to empty", "---", ""},
		{"empty string", "", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := CollapseSeparators(tt.in)
			if got != tt.want {
				t.Errorf("CollapseSeparators(%q) = %q, want %q", tt.in, got, tt.want)
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

func TestParseResult(t *testing.T) {
	tests := []struct {
		name        string
		result      string
		wantName    string
		wantVersion string
		wantDesc    string
	}{
		{
			name:        "with description",
			result:      "ripgrep 14.1.0 - recursively search directories",
			wantName:    "ripgrep",
			wantVersion: "14.1.0",
			wantDesc:    "recursively search directories",
		},
		{
			name:        "no description",
			result:      "ripgrep 14.1.0",
			wantName:    "ripgrep",
			wantVersion: "14.1.0",
			wantDesc:    "",
		},
		{
			name:        "multi-word version",
			result:      "pkg 1.0 extra - some desc",
			wantName:    "pkg",
			wantVersion: "1.0 extra",
			wantDesc:    "some desc",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			name, version, desc := ParseResult(tt.result)
			if name != tt.wantName || version != tt.wantVersion || desc != tt.wantDesc {
				t.Errorf("ParseResult(%q) = (%q, %q, %q), want (%q, %q, %q)",
					tt.result, name, version, desc, tt.wantName, tt.wantVersion, tt.wantDesc)
			}
		})
	}
}

func TestWrapWords(t *testing.T) {
	tests := []struct {
		name  string
		text  string
		width int
		want  []string
	}{
		{
			name:  "fits on one line",
			text:  "short text",
			width: 40,
			want:  []string{"short text"},
		},
		{
			name:  "wraps on spaces without breaking words",
			text:  "recursively search directories for a pattern using regex",
			width: 20,
			want: []string{
				"recursively search",
				"directories for a",
				"pattern using regex",
			},
		},
		{
			name:  "empty text",
			text:  "",
			width: 20,
			want:  []string{},
		},
		{
			name:  "width guarded to 1 when non-positive",
			text:  "a b",
			width: 0,
			want:  []string{"a", "b"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := wrapWords(tt.text, tt.width)
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("wrapWords(%q, %d) = %v, want %v", tt.text, tt.width, got, tt.want)
			}
		})
	}
}

func TestRenderMultiline(t *testing.T) {
	tests := []struct {
		name   string
		result string
		width  int
		want   string
	}{
		{
			name:   "no description prints name line only",
			result: "ripgrep 14.1.0",
			width:  40,
			want:   Rust + "ripgrep" + Reset + " 14.1.0",
		},
		{
			name:   "with description prints hanging indent block",
			result: "ripgrep 14.1.0 - recursively search directories for patterns",
			width:  28, // wrap width = 28 - 8 = 20
			want: Rust + "ripgrep" + Reset + " 14.1.0\n" +
				"      " + Dim + "└ recursively search\n" +
				"        directories for\n" +
				"        patterns" + Reset,
		},
		{
			name:   "short description fits inline, rendered same as ColorizeResult",
			result: "bat 0.24.0 - fast cat clone",
			width:  50, // available = 50-2-3-7-3 = 35 >= 24, desc len 14 <= 35
			want:   ColorizeResult("bat 0.24.0 - fast cat clone", Rust),
		},
		{
			name:   "long name with short description still uses hanging form (guardrail)",
			result: "@comunica/actor-rdf-metadata-extract-hydra-count 5.3.0 - ok",
			width:  70, // available = 70-2-48-6-3 = 11 < MinInlineDescCol(24), forces hanging
			want: Rust + "@comunica/actor-rdf-metadata-extract-hydra-count" + Reset + " 5.3.0\n" +
				"      " + Dim + "└ ok" + Reset,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := RenderMultiline(tt.result, Rust, tt.width)
			if got != tt.want {
				t.Errorf("RenderMultiline(%q, Rust, %d) = %q, want %q", tt.result, tt.width, got, tt.want)
			}
		})
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

