package common

import "testing"

func TestSourceFlagAndAliasShareTable(t *testing.T) {
	tests := []struct {
		flag      string
		alias     string
		wantRoute string
		wantType  string
	}{
		{"--img", "img", SourceGHAppImage, SourceAppImage},
		{"--appimage", "appimage", SourceGHAppImage, SourceAppImage},
		{"--fpk", "fpk", SourceFlatpak, SourceFlatpak},
		{"--flatpak", "flatpak", SourceFlatpak, SourceFlatpak},
		{"--rel", "rel", SourceGHRelease, SourceGH},
		{"--py", "py", SourceUV, SourceUV},
		{"--rust", "rust", SourceCargo, SourceCargo},
		{"--apt", "apt", SourceSystem, SourceSystem},
	}
	for _, tt := range tests {
		fromFlag, ok := LookupSourceFlag(tt.flag)
		if !ok {
			t.Errorf("flag %s not recognized", tt.flag)
			continue
		}
		fromAlias, ok := LookupSourceAlias(tt.alias)
		if !ok || fromFlag.Route != fromAlias.Route {
			t.Errorf("%s and %s resolve differently", tt.flag, tt.alias)
		}
		if fromFlag.Route != tt.wantRoute || fromFlag.Type != tt.wantType {
			t.Errorf("%s = route %q type %q, want %q %q", tt.flag, fromFlag.Route, fromFlag.Type, tt.wantRoute, tt.wantType)
		}
	}
}

func TestSourceTypeForRoute(t *testing.T) {
	if got := SourceTypeForRoute(SourceGHAppImage); got != SourceAppImage {
		t.Errorf("gh-appimage -> %q, want appimage", got)
	}
	if got := SourceTypeForRoute(SourceGHRelease); got != SourceGH {
		t.Errorf("gh-release -> %q, want gh", got)
	}
	if got := SourceTypeForRoute("whatever"); got != "whatever" {
		t.Errorf("unknown route changed to %q", got)
	}
}

func TestLookupSourceFlagRejectsNonFlags(t *testing.T) {
	for _, arg := range []string{"npm", "-y", "--yes", "--bogus"} {
		if _, ok := LookupSourceFlag(arg); ok {
			t.Errorf("%q should not be a source flag", arg)
		}
	}
}

func TestNixFilterCoversNixOS(t *testing.T) {
	sel, _ := LookupSourceFlag("--nix")
	if len(sel.ListTokens) != 2 {
		t.Errorf("--nix list tokens = %v, want nix and nixos", sel.ListTokens)
	}
}
