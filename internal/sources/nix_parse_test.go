package sources

import "testing"

func TestNixStoreVersion(t *testing.T) {
	tests := []struct {
		name      string
		storePath string
		want      string
	}{
		{
			name:      "simple two-segment version",
			storePath: "/nix/store/fg2fabc123-fd-10.3.0/bin/fd",
			want:      "10.3.0",
		},
		{
			name:      "multi-word derivation name",
			storePath: "/nix/store/hash123-android-tools-35.0.2/bin/adb",
			want:      "35.0.2",
		},
		{
			name:      "no version segment at all",
			storePath: "/nix/store/hash123-steam/bin/steam",
			want:      "",
		},
		{
			name:      "too short a path",
			storePath: "/nix/store",
			want:      "",
		},
		{
			name:      "malformed store entry with no hyphen",
			storePath: "/nix/store/nohyphenentry/bin/x",
			want:      "",
		},
		{
			name:      "single-segment version",
			storePath: "/nix/store/hash123-niri-25.11/bin/niri",
			want:      "25.11",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := nixStoreVersion(tt.storePath)
			if got != tt.want {
				t.Errorf("nixStoreVersion(%q) = %q, want %q", tt.storePath, got, tt.want)
			}
		})
	}
}
