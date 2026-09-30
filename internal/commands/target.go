package commands

import (
	"fmt"
	"strings"

	"github.com/DeprecatedLuar/sat/internal/common"
	"github.com/DeprecatedLuar/sat/internal/manifest"
	"github.com/DeprecatedLuar/sat/internal/ui"
)

// scopedSpec pairs a tool spec with the install-route source of the flag in
// effect for it; source is "" when no flag applied.
type scopedSpec struct {
	spec   string
	source string
}

// scopeSpecs splits args into tool specs. A source flag (e.g. --flatpak)
// applies to every spec that follows it until the next source flag; args
// that are not source flags are specs.
func scopeSpecs(args []string) []scopedSpec {
	currentSource := ""
	var specs []scopedSpec
	for _, arg := range args {
		if sel, ok := common.LookupSourceFlag(arg); ok {
			currentSource = sel.Route
			continue
		}
		specs = append(specs, scopedSpec{spec: arg, source: currentSource})
	}
	return specs
}

// resolveTargets maps a "name[:source]" spec to the manifest entries it
// addresses. An explicit source (the suffix wins over flagSource) resolves
// to exactly that entry; without one, every entry tracked under the name is
// returned. Errors when nothing matches.
func resolveTargets(spec, flagSource string) ([]manifest.Entry, error) {
	name, source := common.ParseToolSpec(spec)
	if source == "" {
		source = flagSource
	}

	if source == "" {
		entries := manifest.Lookup(name)
		if len(entries) == 0 {
			return nil, fmt.Errorf("%s is not tracked by sat", name)
		}
		return entries, nil
	}

	sourceType := common.SourceTypeForRoute(source)
	if src := manifest.Get(name, sourceType); src != "" {
		return []manifest.Entry{{Tool: name, Source: src}}, nil
	}

	if tracked := manifest.Lookup(name); len(tracked) > 0 {
		return nil, fmt.Errorf("%s is not tracked from %s (tracked from %s)", name, sourceType, joinSources(tracked))
	}
	return nil, fmt.Errorf("%s is not tracked from %s", name, sourceType)
}

// requireOne returns the single entry in entries, or an error naming every
// source when the name is ambiguous.
func requireOne(name string, entries []manifest.Entry) (manifest.Entry, error) {
	if len(entries) != 1 {
		return manifest.Entry{}, fmt.Errorf("%s is tracked from %s - use %s:<source> or --<source>", name, joinSources(entries), name)
	}
	return entries[0], nil
}

// joinSources lists the display names of entries as "a, b and c".
func joinSources(entries []manifest.Entry) string {
	names := make([]string, len(entries))
	for i, e := range entries {
		names[i] = ui.SourceDisplay(e.Source)
	}
	if len(names) == 1 {
		return names[0]
	}
	return strings.Join(names[:len(names)-1], ", ") + " and " + names[len(names)-1]
}
