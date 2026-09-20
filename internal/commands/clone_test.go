package commands

import "testing"

func TestResolveCloneURL(t *testing.T) {
	tests := []struct {
		name    string
		host    string
		input   string
		want    string
		wantErr bool
	}{
		{"default github", "", "owner/repo", "https://github.com/owner/repo.git", false},
		{"gitlab", "gitlab.com", "owner/repo", "https://gitlab.com/owner/repo.git", false},
		{"codeberg", "codeberg.org", "owner/repo", "https://codeberg.org/owner/repo.git", false},
		{"gitlab subgroup", "gitlab.com", "grp/sub/repo", "https://gitlab.com/grp/sub/repo.git", false},
		{"strips .git", "", "owner/repo.git", "https://github.com/owner/repo.git", false},
		{"full https url", "", "https://example.com/a/b.git", "https://example.com/a/b.git", false},
		{"ssh url", "", "git@github.com:a/b.git", "git@github.com:a/b.git", false},
		{"flag with url", "gitlab.com", "https://example.com/a/b.git", "", true},
		{"bare name", "", "repo", "", true},
		{"empty segment", "", "owner/", "", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := resolveCloneURL(tt.host, tt.input)
			if (err != nil) != tt.wantErr {
				t.Fatalf("err = %v, wantErr %v", err, tt.wantErr)
			}
			if got != tt.want {
				t.Errorf("got %q, want %q", got, tt.want)
			}
		})
	}
}

func TestParseCloneArgs(t *testing.T) {
	host, pos, err := parseCloneArgs([]string{"--gl", "a/b", "dest"})
	if err != nil || host != "gitlab.com" || len(pos) != 2 {
		t.Fatalf("got host=%q pos=%v err=%v", host, pos, err)
	}

	for _, bad := range [][]string{
		{},
		{"--gh", "--gl", "a/b"},
		{"--nope", "a/b"},
		{"a/b", "c", "d"},
	} {
		if _, _, err := parseCloneArgs(bad); err == nil {
			t.Errorf("expected error for %v", bad)
		}
	}
}
