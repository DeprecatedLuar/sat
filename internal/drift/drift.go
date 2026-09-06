// Package drift keeps the manifest's recorded versions honest against what
// is actually installed. A tool updated outside sat - manually, or by a
// self-updating tool - otherwise keeps its stale recorded version forever,
// since nothing else in sat ever revisits a version once written.
//
// The guiding principle: make the manifest true before anything reads it,
// rather than teaching every *CheckOutdated function to distrust it.
package drift

import (
	"sync"

	"github.com/DeprecatedLuar/sat/internal/manifest"
)

// Drift is one manifest entry whose recorded version disagrees with the
// version actually installed right now.
type Drift struct {
	Tool   string // manifest key
	Source string // the full source string as currently recorded
	Old    string // version currently recorded (may be empty)
	New    string // version observed live (never empty - see diff)
}

// NewSource returns the corrected source string for d: same source type and
// identity, corrected version.
func (d Drift) NewSource() string {
	return manifest.BuildSourceString(
		manifest.GetSourceType(d.Source),
		manifest.GetSourceIdentity(d.Source),
		d.New,
	)
}

// Check queries every live-queryable source and reports every disagreement
// with the manifest. Pure: no manifest mutation, no printing. A source that
// can't be reached (package manager absent, command failed) simply
// contributes nothing rather than failing the whole check.
func Check() []Drift {
	entries, err := manifest.All()
	if err != nil || len(entries) == 0 {
		return nil
	}

	byType := make(map[string][]manifest.Entry, len(entries))
	for _, e := range entries {
		t := manifest.GetSourceType(e.Source)
		byType[t] = append(byType[t], e)
	}

	var mu sync.Mutex
	var drifts []Drift
	var wg sync.WaitGroup

	for _, p := range providers {
		var group []manifest.Entry
		for _, t := range p.types {
			group = append(group, byType[t]...)
		}
		if len(group) == 0 {
			continue
		}

		wg.Add(1)
		go func() {
			defer wg.Done()

			tools := make([]string, 0, len(group))
			for _, e := range group {
				if k := p.key(e); k != "" {
					tools = append(tools, k)
				}
			}
			if len(tools) == 0 {
				return
			}

			live := p.load(tools)
			found := diff(group, p.key, live)
			if len(found) == 0 {
				return
			}

			mu.Lock()
			drifts = append(drifts, found...)
			mu.Unlock()
		}()
	}
	wg.Wait()

	return drifts
}

// diff is Check's pure comparison core: for each entry, look up its live
// version under key(e) and report a Drift when it disagrees with what the
// manifest currently records.
//
// Two rules matter more than the rest of this function combined:
//  1. An entry absent from live, or a live version that is empty, is
//     skipped - never treated as "not installed". Absence from a live
//     query means "unknown", and a whole-provider failure (live == nil or
//     empty) must not be read as "everything under this source vanished".
//  2. Comparison is plain string inequality, not "is New newer than Old" -
//     a downgrade is legitimate drift too, and gating on "newer" would
//     leave sat update offering an update the user just rolled back from.
func diff(entries []manifest.Entry, key func(manifest.Entry) string, live map[string]string) []Drift {
	if len(live) == 0 {
		return nil
	}

	var drifts []Drift
	for _, e := range entries {
		k := key(e)
		if k == "" {
			continue
		}
		newVersion, ok := live[k]
		if !ok || newVersion == "" {
			continue
		}
		oldVersion := manifest.GetSourceVersion(e.Source)
		if oldVersion == newVersion {
			continue
		}
		drifts = append(drifts, Drift{Tool: e.Tool, Source: e.Source, Old: oldVersion, New: newVersion})
	}
	return drifts
}

// Apply rewrites the manifest for every given drift in a single batched
// write. Returns the number of entries actually changed.
func Apply(drifts []Drift) (int, error) {
	if len(drifts) == 0 {
		return 0, nil
	}
	updates := make(map[string]string, len(drifts))
	for _, d := range drifts {
		updates[d.Tool] = d.NewSource()
	}
	return manifest.AddMany(updates)
}

// Reconcile is Check followed by Apply, unconditional - the entrypoint for
// callers that must not skip a run regardless of the TTL (sat update,
// sat scan).
func Reconcile() (int, error) {
	return Apply(Check())
}
