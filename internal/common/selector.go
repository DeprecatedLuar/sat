package common

import "strings"

const (
	// SourceFlagPrefix marks a selector written as a CLI flag (e.g. --npm).
	SourceFlagPrefix = "--"

	// SourceNixOS is the declarative NixOS source type.
	SourceNixOS = "nixos"
)

// SourceSelector is what a flag or ":suffix" alias resolves to.
type SourceSelector struct {
	// Route is the install source the selector forces (may be a method
	// such as gh-appimage that records a different source type).
	Route string
	// Type is the source type recorded in the manifest for that install.
	Type string
	// ListTokens are the source-type/display tokens a list filter matches.
	ListTokens []string
}

var (
	selUV       = SourceSelector{SourceUV, SourceUV, []string{"python", SourceUV}}
	selCargo    = SourceSelector{SourceCargo, SourceCargo, []string{SourceCargo, "rust"}}
	selNPM      = SourceSelector{SourceNPM, SourceNPM, []string{SourceNPM, "node"}}
	selSystem   = SourceSelector{SourceSystem, SourceSystem, []string{SourceSystem}}
	selNix      = SourceSelector{SourceNix, SourceNix, []string{SourceNix, SourceNixOS}}
	selNixOS    = SourceSelector{SourceNixOS, SourceNixOS, []string{SourceNix, SourceNixOS}}
	selFlatpak  = SourceSelector{SourceFlatpak, SourceFlatpak, []string{SourceFlatpak}}
	selGH       = SourceSelector{SourceGH, SourceGH, []string{"github", "repo", SourceGH}}
	selRelease  = SourceSelector{SourceGHRelease, SourceGH, []string{"github", "repo", SourceGH}}
	selAppImage = SourceSelector{SourceGHAppImage, SourceAppImage, []string{SourceAppImage}}
	selGo       = SourceSelector{SourceGo, SourceGo, []string{SourceGo}}
	selBrew     = SourceSelector{SourceBrew, SourceBrew, []string{SourceBrew}}
	selSat      = SourceSelector{SourceSat, SourceSat, []string{SourceSat}}
	selManual   = SourceSelector{SourceManual, SourceManual, []string{SourceManual}}
)

// sourceSelectors is the single flag/alias table: the key is the bare name
// accepted both as "--name" and as "tool:name".
var sourceSelectors = map[string]SourceSelector{
	"py": selUV, "python": selUV, "uv": selUV,
	"rs": selCargo, "rust": selCargo, "cargo": selCargo,
	"js": selNPM, "node": selNPM, "npm": selNPM,
	"sys": selSystem, "system": selSystem, "apt": selSystem,
	"nix": selNix, "nixos": selNixOS,
	"fpk": selFlatpak, "flatpak": selFlatpak,
	"gh": selGH, "github": selGH,
	"rel": selRelease, "release": selRelease,
	"img": selAppImage, "appimage": selAppImage,
	"go": selGo, "brew": selBrew, "sat": selSat, "manual": selManual,
}

// routeTypes maps each install route to the manifest source type it records.
var routeTypes = buildRouteTypes()

func buildRouteTypes() map[string]string {
	types := make(map[string]string, len(sourceSelectors))
	for _, sel := range sourceSelectors {
		types[sel.Route] = sel.Type
	}
	return types
}

// LookupSourceAlias resolves a bare alias (the part after "tool:").
func LookupSourceAlias(alias string) (SourceSelector, bool) {
	sel, ok := sourceSelectors[alias]
	return sel, ok
}

// LookupSourceFlag resolves a CLI flag such as "--npm".
func LookupSourceFlag(arg string) (SourceSelector, bool) {
	name, ok := strings.CutPrefix(arg, SourceFlagPrefix)
	if !ok {
		return SourceSelector{}, false
	}
	return LookupSourceAlias(name)
}

// SourceTypeForRoute returns the manifest source type an install route
// records; a route with no table entry is returned unchanged.
func SourceTypeForRoute(route string) string {
	if t, ok := routeTypes[route]; ok {
		return t
	}
	return route
}
