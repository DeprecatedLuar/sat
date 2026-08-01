package common

import "testing"

func TestVersionIsNewer(t *testing.T) {
	tests := []struct {
		name           string
		latest, current string
		want           bool
	}{
		{"newer patch", "1.49.1", "1.49.0", true},
		{"newer minor", "1.50.0", "1.49.0", true},
		{"older major (kimi collision)", "0.0.5.78", "1.49.0", false},
		{"equal", "1.49.0", "1.49.0", false},
		{"shorter is newer", "1.3", "1.2.9", true},
		{"longer is newer", "1.2.1", "1.2", true},
		{"equal differing lengths", "1.2.0", "1.2", false},
		{"v-prefix both", "v2.0.0", "v1.9.0", true},
		{"v-prefix mixed", "v1.9.0", "1.10.0", false},
		{"non-numeric latest falls back true", "abc123", "1.0.0", true},
		{"non-numeric current falls back true", "1.0.0", "abc123", true},
		{"both non-numeric falls back true", "main", "master", true},
		{"dash separated build tags", "1.2.0-3", "1.2.0-1", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := VersionIsNewer(tt.latest, tt.current)
			if got != tt.want {
				t.Errorf("VersionIsNewer(%q, %q) = %v, want %v", tt.latest, tt.current, got, tt.want)
			}
		})
	}
}
