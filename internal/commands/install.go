package commands

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/DeprecatedLuar/sat/internal/common"
	"github.com/DeprecatedLuar/sat/internal/config"
	"github.com/DeprecatedLuar/sat/internal/manifest"
	"github.com/DeprecatedLuar/sat/internal/sources"
	"github.com/DeprecatedLuar/sat/internal/ui"
)

const installUsage = "usage: sat install <program>[:source] ... [--system|--rust|--python|--node|--go|--brew|--nix|--flatpak|--gh] <program> ..."

// installFlagSource maps a CLI flag to the source it forces for every spec
// that doesn't carry its own ":source" suffix (bash's DEFAULT_SOURCE).
var installFlagSource = map[string]string{
	"--system":  common.SourceSystem,
	"--sys":     common.SourceSystem,
	"--rust":    common.SourceCargo,
	"--rs":      common.SourceCargo,
	"--python":  common.SourceUV,
	"--py":      common.SourceUV,
	"--node":    common.SourceNPM,
	"--js":      common.SourceNPM,
	"--go":      common.SourceGo,
	"--brew":    common.SourceBrew,
	"--nix":     common.SourceNix,
	"--flatpak": common.SourceFlatpak,
	"--gh":      common.SourceGH,
	"--github":  common.SourceGH,
}

// installResult is what a successful install attempt (direct-repo, forced
// source, or one step of the fallback chain) needs to record in the
// manifest and print to the user.
type installResult struct {
	binName  string // resolved binary name (may differ from the requested tool)
	source   string // canonical source type recorded in the manifest
	identity string // e.g. "owner/repo" for gh/appimage/go, empty otherwise
	version  string
}

// installSpec pairs a tool spec with the source flag in effect for it (see
// Install's positional scoping below); source is "" when no flag applied.
type installSpec struct {
	spec   string
	source string
}

// Install installs one or more tools. A --flag (e.g. --flatpak) forces the
// source for every spec that follows it, until the next flag switches to a
// different source - so a single invocation can mix sources, e.g.
// "sat install --flatpak a b --nix c d". Specs before the first flag have
// no forced source. For each spec, tries in order: a direct "owner/repo"
// GitHub path, an already-installed check, its forced source (":source"
// suffix or the flag in scope), or the fallback chain configured in
// ~/.config/sat/config.toml.
func Install(args []string) error {
	currentSource := ""
	var specs []installSpec
	for _, arg := range args {
		if source, ok := installFlagSource[arg]; ok {
			currentSource = source
			continue
		}
		specs = append(specs, installSpec{spec: arg, source: currentSource})
	}

	if len(specs) == 0 {
		return fmt.Errorf(installUsage)
	}

	for _, s := range specs {
		installOne(s.spec, s.source)
	}

	return nil
}

func installOne(spec, defaultSource string) {
	tool, explicitSource := common.ParseToolSpec(spec)
	forceSource := explicitSource
	if forceSource == "" {
		forceSource = defaultSource
	}

	if strings.Contains(tool, "/") {
		installDirectRepo(tool)
		return
	}

	if forceSource == "" {
		if _, err := exec.LookPath(tool); err == nil {
			detected := common.DetectSource(tool)
			ui.Status(fmt.Sprintf("%s already installed (%s) - use %s:<source> to force reinstall", tool, ui.SourceDisplay(detected), tool))
			return
		}
		installFallback(tool)
		return
	}

	installForced(tool, forceSource)
}

// installDirectRepo installs an explicit "owner/repo" spec via GitHub,
// bypassing the already-installed check and fallback chain entirely
// (matches bash's install.sh precedence: repo-path specs always attempt
// install regardless of what's already on PATH).
func installDirectRepo(repoPath string) {
	repoName := repoPath
	if idx := strings.LastIndex(repoPath, "/"); idx != -1 {
		repoName = repoPath[idx+1:]
	}

	var result installResult
	err := ui.RunWithSpinner(repoName, common.SourceGH, func() error {
		r, err := installFromGitHub(repoPath, "auto")
		result = r
		return err
	})

	if err != nil {
		ui.StatusFail(fmt.Sprintf("%s not found", repoName))
		return
	}

	finishInstall(result)
}

// installForced installs tool via exactly one source, no fallback.
func installForced(tool, source string) {
	var result installResult
	err := ui.RunWithSpinner(tool, source, func() error {
		r, err := trySource(tool, source)
		result = r
		return err
	})

	if err != nil {
		if !reportIfAmbiguous(err) {
			ui.StatusFail(fmt.Sprintf("%s not found in %s: %s", tool, ui.SourceDisplay(source), err))
		}
		return
	}

	finishInstall(result)
}

// reportIfAmbiguous prints and reports whether err is an
// AmbiguousMatchError - a short tool name matching more than one GitHub
// repository. This is not a "try the next source" failure: the ambiguity
// is independent of which source was being tried, so callers should stop
// the whole install attempt for this spec instead of falling back.
func reportIfAmbiguous(err error) bool {
	var ambig *sources.AmbiguousMatchError
	if errors.As(err, &ambig) {
		ui.StatusFail(ambig.Error())
		return true
	}
	return false
}

// installFallback tries each source in the configured install order,
// stopping at the first success.
func installFallback(tool string) {
	cfg, err := config.Load()
	if err != nil && os.Getenv(common.EnvSATDebug) != "" {
		fmt.Fprintf(os.Stderr, "%s failed to load config: %v\n", common.DebugPrefix, err)
	}

	var lastSource string
	var lastErr error

	for _, source := range cfg.InstallOrder {
		if os.Getenv(common.EnvSATDebug) != "" {
			fmt.Fprintf(os.Stderr, "%s trying %s for %s\n", common.DebugPrefix, source, tool)
		}

		var result installResult
		err := ui.RunWithSpinner(tool, source, func() error {
			r, err := trySource(tool, source)
			result = r
			return err
		})
		if err != nil {
			if reportIfAmbiguous(err) {
				return
			}
			lastSource, lastErr = source, err
			continue
		}

		finishInstall(result)
		return
	}

	if lastErr != nil {
		ui.StatusFail(fmt.Sprintf("%s not found (%s: %s)", tool, ui.SourceDisplay(lastSource), lastErr))
		return
	}
	ui.StatusFail(fmt.Sprintf("%s not found", tool))
}

// finishInstall records a successful install in the system manifest and
// prints the success status line.
func finishInstall(result installResult) {
	if result.binName == "" {
		return
	}

	sourceString := manifest.BuildSourceString(result.source, result.identity, result.version)
	if err := manifest.Add(result.binName, sourceString); err != nil && os.Getenv(common.EnvSATDebug) != "" {
		fmt.Fprintf(os.Stderr, "%s failed to record %s in manifest: %v\n", common.DebugPrefix, result.binName, err)
	}

	ui.StatusOK(result.binName, sourceString)
}

// trySource attempts to install tool via exactly one source, returning
// enough metadata to record it in the manifest.
func trySource(tool, source string) (installResult, error) {
	switch source {
	case common.SourceCargo:
		if err := sources.CargoInstall(tool); err != nil {
			return installResult{}, err
		}
		return installResult{binName: tool, source: source, version: sources.CargoGetVersion(tool)}, nil

	case common.SourceBrew:
		if err := sources.BrewInstall(tool); err != nil {
			return installResult{}, err
		}
		return installResult{binName: tool, source: source, version: sources.BrewGetVersion(tool)}, nil

	case common.SourceNix:
		if err := sources.NixInstall(tool); err != nil {
			return installResult{}, err
		}
		return installResult{binName: tool, source: source, version: sources.NixGetVersion(tool)}, nil

	case common.SourceSystem:
		if err := sources.Install(tool); err != nil {
			return installResult{}, err
		}
		return installResult{binName: tool, source: source, version: sources.GetVersion(tool)}, nil

	case common.SourceGH:
		return installFromGitHub(tool, "auto")
	case common.SourceGHRelease:
		return installFromGitHub(tool, "release")
	case common.SourceGHAppImage:
		return installFromGitHub(tool, "appimage")

	case common.SourceNPM:
		identity, err := sources.NpmInstall(tool)
		if err != nil {
			return installResult{}, err
		}
		return installResult{binName: tool, source: source, identity: identity, version: sources.NpmGetVersion(tool)}, nil

	case common.SourceFlatpak:
		binName, appID, err := sources.FlatpakInstall(tool)
		if err != nil {
			return installResult{}, err
		}
		return installResult{binName: binName, source: source, identity: appID, version: sources.FlatpakGetVersion(appID)}, nil

	case common.SourceUV, common.SourceGo, common.SourceSat:
		return installResult{}, fmt.Errorf("%s install not yet implemented in the Go port", ui.SourceDisplay(source))

	default:
		return installResult{}, fmt.Errorf("unknown source: %s", source)
	}
}

// installFromGitHub installs via sources.GitHubInstall and resolves the
// resulting source string (which may be "gh:owner/repo",
// "appimage:owner/repo", "go:github.com/owner/repo", or "uv") into an
// installResult.
func installFromGitHub(tool, method string) (installResult, error) {
	bin, srcStr, err := sources.GitHubInstall(tool, method)
	if err != nil {
		return installResult{}, err
	}

	sourceType := manifest.GetSourceType(srcStr)
	identity := manifest.GetSourceIdentity(srcStr)

	version := ""
	switch sourceType {
	case common.SourceGH:
		version = sources.GitHubGetVersion(identity)
	case common.SourceAppImage:
		version = sources.AppImageGetVersion(identity)
	}

	return installResult{binName: bin, source: sourceType, identity: identity, version: version}, nil
}
