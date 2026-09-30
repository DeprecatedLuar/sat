package commands

import (
	"fmt"

	"github.com/DeprecatedLuar/sat/internal/common"
	"github.com/DeprecatedLuar/sat/internal/manifest"
	"github.com/DeprecatedLuar/sat/internal/ui"
)

const (
	infoUsage = "usage: sat info <tool> [<tool> ...]"

	versionUnknown = "unknown"
	indent         = "  "
	nestedIndent   = "    "
)

// Info prints, for each tool, the active install (path, target, repo,
// version), every manifest entry tracked under the name, and the installs
// shadowed by the active one.
func Info(args []string) error {
	if len(args) == 0 {
		return fmt.Errorf(infoUsage)
	}

	for i, arg := range args {
		if i > 0 {
			fmt.Println()
		}
		infoOne(arg)
	}
	return nil
}

func infoOne(arg string) {
	name := common.NaturalName(arg)
	installs := common.FindInstalls(name)
	tracked := manifest.Lookup(name)

	if len(installs) == 0 && len(tracked) == 0 {
		ui.StatusFail(fmt.Sprintf("%s: not found", name))
		return
	}

	shadowed := installs
	if len(installs) > 0 {
		primary := primaryInstall(installs)
		printInstall(name, primary, tracked)
		shadowed = without(installs, primary)
	} else {
		fmt.Println(name)
	}

	printTracked(tracked)
	printShadowed(shadowed, len(installs) > 0)
}

// primaryInstall is the install a shell resolves the name to, or the first
// one found when none of them is on $PATH.
func primaryInstall(installs []common.Install) common.Install {
	for _, in := range installs {
		if in.Active {
			return in
		}
	}
	return installs[0]
}

func without(installs []common.Install, drop common.Install) []common.Install {
	var rest []common.Install
	for _, in := range installs {
		if in.Path != drop.Path {
			rest = append(rest, in)
		}
	}
	return rest
}

func printInstall(name string, in common.Install, tracked []manifest.Entry) {
	version := installVersion(name, in, tracked)
	fmt.Printf("%s%s%s [%s%s%s] %s%s%s\n",
		ui.SourceLight(in.Source), name, ui.Reset,
		ui.SourceColor(in.Source), ui.SourceDisplay(in.Source), ui.Reset,
		ui.Dim, version, ui.Reset)
	fmt.Printf("%spath:   %s\n", indent, in.Path)
	if in.Real != in.Path {
		fmt.Printf("%starget: %s\n", indent, in.Real)
	}
	if repo := trackedRepo(in.Source, tracked); repo != "" {
		fmt.Printf("%srepo:   %s\n", indent, repo)
	}
}

// trackedEntry returns the entry tracked for the source type, if any.
func trackedEntry(sourceType string, tracked []manifest.Entry) (manifest.Entry, bool) {
	want := manifest.CanonicalSourceType(sourceType)
	for _, e := range tracked {
		if manifest.CanonicalSourceType(manifest.GetSourceType(e.Source)) == want {
			return e, true
		}
	}
	return manifest.Entry{}, false
}

// trackedRepo is the owner/repo identity of a tracked gh entry.
func trackedRepo(sourceType string, tracked []manifest.Entry) string {
	if manifest.CanonicalSourceType(sourceType) != common.SourceGH {
		return ""
	}
	e, ok := trackedEntry(sourceType, tracked)
	if !ok {
		return ""
	}
	return manifest.GetSourceIdentity(e.Source)
}

// installVersion prefers the manifest's reconciled version, falling back to
// asking the source.
func installVersion(name string, in common.Install, tracked []manifest.Entry) string {
	if e, ok := trackedEntry(in.Source, tracked); ok {
		if v := manifest.GetSourceVersion(e.Source); v != "" {
			return v
		}
	}
	if _, v, err := describeInstall(name, in); err == nil && v != "" {
		return v
	}
	return versionUnknown
}

func printTracked(tracked []manifest.Entry) {
	if len(tracked) == 0 {
		fmt.Printf("%snot tracked\n", indent)
		return
	}
	fmt.Printf("%stracked:\n", indent)
	for _, e := range tracked {
		fmt.Printf("%s[%s%s%s] %s%s%s\n", nestedIndent,
			ui.SourceColor(e.Source), ui.SourceDisplay(e.Source), ui.Reset,
			ui.Dim, manifest.GetSourceVersion(e.Source), ui.Reset)
	}
}

// printShadowed lists installs that are not the primary one. With no
// primary, every install is reported as off $PATH rather than shadowed.
func printShadowed(installs []common.Install, hasPrimary bool) {
	if len(installs) == 0 {
		return
	}
	label := "shadowed:"
	if !hasPrimary {
		label = "installed:"
	}
	fmt.Printf("%s%s\n", indent, label)
	for _, in := range installs {
		fmt.Printf("%s%s[%s%s%s%s] %s%s\n", nestedIndent,
			ui.Dim, ui.SourceColor(in.Source), ui.SourceDisplay(in.Source), ui.Reset, ui.Dim, in.Path, ui.Reset)
	}
}
