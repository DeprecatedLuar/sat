package drift

import (
	"github.com/DeprecatedLuar/sat/internal/common"
	"github.com/DeprecatedLuar/sat/internal/manifest"
	"github.com/DeprecatedLuar/sat/internal/sources"
)

// provider resolves live installed versions for one source type or a group
// of aliases for the same ecosystem (e.g. "cargo"/"rust"). key extracts
// whatever identifier that ecosystem's bulk command reports by (usually the
// tool name; flatpak reports by appID instead).
type provider struct {
	types []string
	key   func(e manifest.Entry) string
	load  func(tools []string) map[string]string
}

func defaultKey(e manifest.Entry) string { return e.Tool }

func identityKey(e manifest.Entry) string { return manifest.GetSourceIdentity(e.Source) }

// providers lists every source drift knows how to reconcile against a real,
// live-queried installed version. A source type absent from this list is
// skipped entirely - not "assumed uninstalled" - because it cannot report
// a trustworthy installed version at all:
//
//   - appimage: no local version metadata whatsoever (see AppImageGetVersion's
//     own doc - it returns the latest upstream release tag, not what's
//     actually installed).
//   - gh/github: only meaningful via huber; otherwise GitHubGetVersion
//     returns the latest UPSTREAM tag as a proxy, and treating that as
//     "installed" would make every gh-tracked tool look permanently
//     up to date.
//   - go: no source module implements version lookup at all.
//   - sat, manual, unknown: not backed by any package manager to query.
var providers = []provider{
	{[]string{common.SourceNPM}, defaultKey, sources.NpmInstalledVersions},
	{[]string{common.SourceUV}, defaultKey, sources.UvInstalledVersions},
	{[]string{common.SourceCargo, "rust"}, defaultKey, sources.CargoInstalledVersions},
	{[]string{common.SourceNix}, defaultKey, sources.NixInstalledVersions},
	{[]string{"nixos"}, defaultKey, sources.NixOSInstalledVersions},
	{[]string{common.SourceFlatpak}, identityKey, sources.FlatpakAppVersions},
	{[]string{common.SourceBrew}, defaultKey, sources.BrewInstalledVersions},
	{
		[]string{
			common.SourceSystem,
			common.PkgManagerAPT,
			common.PkgManagerPacman,
			common.PkgManagerDNF,
			common.PkgManagerAPK,
		},
		defaultKey,
		sources.InstalledVersions,
	},
}
