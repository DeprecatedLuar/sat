package github

import "testing"

func TestFormatCandidate(t *testing.T) {
	tests := []struct {
		name string
		c    githubCandidate
		want string
	}{
		{
			name: "long description is not truncated",
			c: githubCandidate{
				NameWithOwner: "facebookresearch/hydra",
				LatestTag:     "v1.3.4",
				Description:   "Hydra is a framework for elegantly configuring complex applications",
			},
			want: "facebookresearch/hydra v1.3.4 - Hydra is a framework for elegantly configuring complex applications",
		},
		{
			name: "no releases falls back to placeholder version",
			c: githubCandidate{
				NameWithOwner: "hydra-synth/hydra",
				Description:   "Livecoding networked visuals in the browser",
			},
			want: "hydra-synth/hydra unreleased - Livecoding networked visuals in the browser",
		},
		{
			name: "empty description falls back to placeholder",
			c: githubCandidate{
				NameWithOwner: "odenny/hydra",
			},
			want: "odenny/hydra unreleased - (no description)",
		},
		{
			name: "only first line of a multi-line description is kept",
			c: githubCandidate{
				NameWithOwner: "NixOS/hydra",
				Description:   "Hydra, the Nix-based continuous build system\nmain readme content",
			},
			want: "NixOS/hydra unreleased - Hydra, the Nix-based continuous build system",
		},
		{
			name: "star count is shown as a description prefix",
			c: githubCandidate{
				NameWithOwner: "ojack/hydra",
				LatestTag:     "v2.1.0",
				Description:   "Livecoding networked visuals in the browser",
				Stars:         12500,
			},
			want: "ojack/hydra v2.1.0 - (★12.5k) Livecoding networked visuals in the browser",
		},
		{
			name: "zero stars omits the star token entirely",
			c: githubCandidate{
				NameWithOwner: "odenny/hydra",
				Description:   "no stars yet",
			},
			want: "odenny/hydra unreleased - no stars yet",
		},
		{
			name: "star count under 1000 shown verbatim",
			c: githubCandidate{
				NameWithOwner: "small/repo",
				Description:   "tiny project",
				Stars:         999,
			},
			want: "small/repo unreleased - (★999) tiny project",
		},
		{
			name: "star count in millions",
			c: githubCandidate{
				NameWithOwner: "huge/repo",
				LatestTag:     "v1.0.0",
				Description:   "very popular",
				Stars:         1500000,
			},
			want: "huge/repo v1.0.0 - (★1.5M) very popular",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := formatCandidate(tt.c)
			if got != tt.want {
				t.Errorf("formatCandidate(%+v) = %q, want %q", tt.c, got, tt.want)
			}
		})
	}
}

func TestHumanizeStars(t *testing.T) {
	tests := []struct {
		n    int
		want string
	}{
		{0, "0"},
		{999, "999"},
		{1000, "1k"},
		{1200, "1.2k"},
		{45231, "45.2k"},
		{999999, "1000k"},
		{1000000, "1M"},
		{1500000, "1.5M"},
	}

	for _, tt := range tests {
		got := humanizeStars(tt.n)
		if got != tt.want {
			t.Errorf("humanizeStars(%d) = %q, want %q", tt.n, got, tt.want)
		}
	}
}
