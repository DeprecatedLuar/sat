package commands

import (
	"fmt"
	"strings"

	"github.com/DeprecatedLuar/sat/internal/common"
	"github.com/DeprecatedLuar/sat/internal/manifest"
	"github.com/DeprecatedLuar/sat/internal/scanner"
	"github.com/DeprecatedLuar/sat/internal/sources"
	"github.com/DeprecatedLuar/sat/internal/ui"
)

const (
	trackUsage   = "usage: sat track <tool>[:source] [<tool>[:source] ...] [--<source>]"
	untrackUsage = "usage: sat untrack <tool>[:source] [<tool>[:source] ...] [--<source>]"

	sourceUnknown = "unknown"
)

// Track adds already-installed tools to the manifest. Each spec resolves to
// one install: the one on $PATH, or the one from the named source when a
// suffix or flag is given, so a tool installed from several sources can be
// tracked once per source.
func Track(args []string) error {
	specs := scopeSpecs(args)
	if len(specs) == 0 {
		return fmt.Errorf(trackUsage)
	}

	for _, s := range specs {
		trackOne(s)
	}
	return nil
}

func trackOne(s scopedSpec) {
	name, route := common.ParseToolSpec(s.spec)
	if route == "" {
		route = s.source
	}
	name = common.NaturalName(name)

	installs := common.FindInstalls(name)
	if len(installs) == 0 {
		ui.StatusFail(fmt.Sprintf("%s: not found", name))
		return
	}

	install, err := pickInstall(name, installs, route)
	if err != nil {
		ui.StatusFail(err.Error())
		return
	}

	if existing := manifest.Get(name, install.Source); existing != "" {
		fmt.Printf("%s%s: already tracked%s [%s]\n", ui.Dim, name, ui.Reset, ui.SourceDisplay(existing))
		return
	}

	identity, version, err := describeInstall(name, install)
	if err != nil {
		ui.StatusFail(fmt.Sprintf("%s: %v", name, err))
		return
	}

	srcString := manifest.BuildSourceString(install.Source, identity, version)
	if err := manifest.Add(name, srcString); err != nil {
		ui.StatusFail(fmt.Sprintf("%s: failed to update manifest: %v", name, err))
		return
	}
	ui.StatusOK(name, srcString)
}

// pickInstall selects the install to track. Without a route it is the active
// one; with a route it is the install owned by that source. Installs of an
// unknown source cannot be tracked.
func pickInstall(name string, installs []common.Install, route string) (common.Install, error) {
	if route != "" {
		want := manifest.CanonicalSourceType(common.SourceTypeForRoute(route))
		for _, in := range installs {
			if manifest.CanonicalSourceType(in.Source) == want {
				return in, nil
			}
		}
		return common.Install{}, fmt.Errorf("%s: not installed from %s (found: %s)", name, ui.SourceDisplay(want), joinInstallSources(installs))
	}

	chosen := installs[0]
	for _, in := range installs {
		if in.Active {
			chosen = in
			break
		}
	}
	if chosen.Source == sourceUnknown {
		return common.Install{}, fmt.Errorf("%s: unknown source for %s (found: %s) - use %s:<source> or --<source>", name, chosen.Path, joinInstallSources(installs), name)
	}
	return chosen, nil
}

// joinInstallSources lists the display names of installs, comma separated.
func joinInstallSources(installs []common.Install) string {
	names := make([]string, len(installs))
	for i, in := range installs {
		names[i] = ui.SourceDisplay(in.Source)
	}
	return strings.Join(names, ", ")
}

// describeInstall resolves the manifest identity and current version of an
// install: flatpak wrappers carry their app ID, appimages their repo.
func describeInstall(name string, in common.Install) (identity, version string, err error) {
	switch in.Source {
	case common.SourceFlatpak:
		identity, err = sources.FlatpakWrapperAppID(in.Real)
		if err != nil {
			return "", "", err
		}
	case common.SourceAppImage:
		identity = sources.ExtractAppImageRepo(in.Real)
	}
	return identity, scanner.GetVersionForSource(name, in.Source, identity), nil
}

// Untrack removes manifest entries without uninstalling anything. A name
// tracked from several sources needs a :source suffix or --<source> flag.
func Untrack(args []string) error {
	specs := scopeSpecs(args)
	if len(specs) == 0 {
		return fmt.Errorf(untrackUsage)
	}

	for _, s := range specs {
		untrackOne(s)
	}
	return nil
}

func untrackOne(s scopedSpec) {
	entries, err := resolveTargets(s.spec, s.source)
	if err != nil {
		ui.StatusFail(err.Error())
		return
	}
	name, _ := common.ParseToolSpec(s.spec)
	entry, err := requireOne(name, entries)
	if err != nil {
		ui.StatusFail(err.Error())
		return
	}

	if err := manifest.Remove(entry.Tool, manifest.GetSourceType(entry.Source)); err != nil {
		ui.StatusFail(fmt.Sprintf("%s: failed to update manifest: %v", entry.Tool, err))
		return
	}
	ui.StatusRemoved(entry.Tool, entry.Source)
}
