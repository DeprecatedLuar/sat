package commands

import "testing"

func TestFlatpakDisplayVersions(t *testing.T) {
	tests := []struct {
		name            string
		current, latest string
		wantCurrent     string
		wantLatest      string
	}{
		{"normal bump", "3.2.2", "3.3.0", "3.2.2", "3.3.0"},
		{"same version is a rebuild", "1.7.1", "1.7.1", "1.7.1", "rebuild"},
		{"blank latest", "154.0.8025.0-1", "", "154.0.8025.0-1", "?"},
		{"blank current", "", "50", "?", "50"},
		{"both blank", "", "", "?", "?²"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotCurrent, gotLatest := flatpakDisplayVersions(tt.current, tt.latest)
			if gotCurrent != tt.wantCurrent || gotLatest != tt.wantLatest {
				t.Errorf("flatpakDisplayVersions(%q, %q) = (%q, %q), want (%q, %q)",
					tt.current, tt.latest, gotCurrent, gotLatest, tt.wantCurrent, tt.wantLatest)
			}
		})
	}
}
