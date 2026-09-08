package commands

import (
	"reflect"
	"testing"
)

func TestQueryVariants(t *testing.T) {
	tests := []struct {
		name  string
		query string
		want  []string
	}{
		{"no separators, single variant", "kdeconnect", []string{"kdeconnect"}},
		{"hyphen adds collapsed variant, literal first", "kde-connect", []string{"kde-connect", "kdeconnect"}},
		{"all-separator query stays single variant, no empty string", "---", []string{"---"}},
		{"empty query stays single variant", "", []string{""}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := queryVariants(tt.query)
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("queryVariants(%q) = %v, want %v", tt.query, got, tt.want)
			}
		})
	}
}

func TestMergeVariantResults(t *testing.T) {
	tests := []struct {
		name       string
		resultSets [][]string
		want       []string
	}{
		{
			name: "duplicate name across variants, literal-variant form kept",
			resultSets: [][]string{
				{"kdeconnect-kde 25.08.3 - Multi-platform app"}, // literal "kde-connect" variant
				{"kdeconnect-kde 25.08.3 - Multi-platform app", "kdeconnect_waybar 1.1.2 - Waybar module"}, // collapsed variant
			},
			want: []string{
				"kdeconnect-kde 25.08.3 - Multi-platform app",
				"kdeconnect_waybar 1.1.2 - Waybar module",
			},
		},
		{
			name: "distinct names from both variants, literal variant ordered first",
			resultSets: [][]string{
				{"ripgrep 14.1.0 - search tool"},
				{"rg-utils 1.0.0 - unrelated helper"},
			},
			want: []string{
				"ripgrep 14.1.0 - search tool",
				"rg-utils 1.0.0 - unrelated helper",
			},
		},
		{
			name:       "single empty variant set yields empty merge, not nil-panic",
			resultSets: [][]string{{}},
			want:       []string{},
		},
		{
			name: "case-insensitive dedupe",
			resultSets: [][]string{
				{"Foo 1.0.0 - first"},
				{"foo 2.0.0 - second, should be dropped"},
			},
			want: []string{"Foo 1.0.0 - first"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := mergeVariantResults(tt.resultSets)
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("mergeVariantResults(%v) = %v, want %v", tt.resultSets, got, tt.want)
			}
		})
	}
}

func TestSearchVariantsUnknownSourceErrors(t *testing.T) {
	// executeSearch's default case rejects unknown sources without touching
	// the network, so this exercises searchVariants' "every variant errored"
	// short-circuit end to end without requiring network access.
	_, err := searchVariants("not-a-real-source", "kde-connect")
	if err == nil {
		t.Fatal("searchVariants with an unknown source: expected error, got nil")
	}
}
