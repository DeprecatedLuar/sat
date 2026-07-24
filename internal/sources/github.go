package sources

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/DeprecatedLuar/sat/internal/common"
	"github.com/DeprecatedLuar/sat/internal/manifest"
	"github.com/DeprecatedLuar/sat/internal/sources/github"
)

// AmbiguousMatchError is re-exported so callers outside this package (e.g.
// commands.Install) can detect an ambiguous short-name match via errors.As
// without importing the github subpackage directly.
type AmbiguousMatchError = github.AmbiguousMatchError

// GitHubInstall resolves input (an "owner/repo" path, or a short name to
// search for) and installs it via the given method. method is one of
// "auto" (try huber, then appimage, then language-routed go/python),
// "release" (huber only), or "appimage" (AppImage only). Sequences calls
// between the github subpackage's mechanics and AppImageInstall - it has
// no install logic of its own, mirroring bash's install_github
// orchestrator (lib/sources/github.sh:316).
func GitHubInstall(input, method string) (binName, srcString string, err error) {
	repo := input
	lang := ""
	if !strings.Contains(input, "/") {
		repo, lang, err = github.SearchBestMatch(input)
		if err != nil {
			return "", "", err
		}
	}

	tree, err := github.FetchTree(repo)
	if err != nil {
		return "", "", err
	}
	if len(tree) == 0 {
		return "", "", fmt.Errorf("could not fetch repository tree for %s", repo)
	}

	switch method {
	case "", "auto":
		return githubInstallAuto(repo, lang, tree)
	case "release":
		if _, err := exec.LookPath("huber"); err != nil {
			return "", "", fmt.Errorf("huber required for release install method")
		}
		return github.InstallHuber(repo)
	case "appimage":
		bin, err := AppImageInstall(repo)
		if err != nil {
			return "", "", err
		}
		return bin, common.SourceAppImage + manifest.SourceDelimiter + repo, nil
	default:
		return "", "", fmt.Errorf("unknown github install method: %s", method)
	}
}

// githubInstallAuto tries every install method in order and, if all fail,
// returns one concise reason instead of whichever method happened to run
// last (which could be a confusing non-sequitur, e.g. "not a python
// project" for a plain C repo).
func githubInstallAuto(repo, lang string, tree []string) (binName, srcString string, err error) {
	if bin, src, err := github.InstallHuber(repo); err == nil {
		return bin, src, nil
	}

	if bin, err := AppImageInstall(repo); err == nil {
		return bin, common.SourceAppImage + manifest.SourceDelimiter + repo, nil
	}

	switch lang {
	case "Go":
		if bin, src, err := github.InstallGo(repo, tree); err == nil {
			return bin, src, nil
		}
	case "Python":
		if bin, src, err := github.InstallPython(repo, tree); err == nil {
			return bin, src, nil
		}
	default:
		if bin, src, err := github.InstallGo(repo, tree); err == nil {
			return bin, src, nil
		}
		if bin, src, err := github.InstallPython(repo, tree); err == nil {
			return bin, src, nil
		}
	}

	return "", "", fmt.Errorf("no binary, AppImage, Go, or Python entrypoint for %s", repo)
}

// GitHubSearch searches GitHub repositories, formatted for display.
func GitHubSearch(query string) ([]string, error) {
	return github.Search(query)
}

// GitHubUninstall removes a GitHub-sourced install. If source carries
// owner/repo metadata (gh:owner/repo) and huber is available, prefers
// "huber uninstall"; otherwise best-effort removes the symlinks sat/huber
// may have created (missing files are not an error, matching bash's rm -f
// semantics).
func GitHubUninstall(pkg, source string) error {
	if repo := strings.TrimPrefix(source, common.SourceGH+manifest.SourceDelimiter); repo != source && strings.Contains(repo, "/") {
		if _, err := exec.LookPath("huber"); err == nil {
			if common.RunQuiet("huber", "uninstall", repo) == nil {
				return nil
			}
		}
	}

	home, _ := os.UserHomeDir()
	var firstErr error
	if err := os.Remove(filepath.Join(common.LocalBin(), pkg)); err != nil && !os.IsNotExist(err) {
		firstErr = err
	}
	if err := os.Remove(filepath.Join(home, github.HuberBinDirRel, pkg)); err != nil && !os.IsNotExist(err) && firstErr == nil {
		firstErr = err
	}
	return firstErr
}

// GitHubUpdate re-installs the latest release via huber. Non-huber GitHub
// installs (go/uv/appimage) have no update path, matching bash (appimage
// updates go through AppImageUpdate instead).
func GitHubUpdate(tool, repo string) error {
	if _, err := exec.LookPath("huber"); err != nil {
		return fmt.Errorf("huber not installed")
	}
	return common.RunQuiet("huber", "update", repo)
}

// GitHubQueryLatestVersion returns the latest GitHub release tag for repo.
func GitHubQueryLatestVersion(repo string) string {
	tag, err := github.LatestReleaseTag(repo)
	if err != nil {
		return ""
	}
	return tag
}

// GitHubGetVersion returns the installed version: huber's own record if
// the tool was huber-installed, else the latest release tag as a proxy
// (non-huber GitHub installs carry no independent version tracking).
func GitHubGetVersion(repo string) string {
	if _, err := exec.LookPath("huber"); err == nil {
		var out bytes.Buffer
		cmd := exec.Command("huber", "info", repo)
		cmd.Stdout = &out
		if cmd.Run() == nil {
			if m := regexp.MustCompile(`(?i)Version:\s*(\S+)`).FindStringSubmatch(out.String()); len(m) > 1 {
				return m[1]
			}
		}
	}
	return GitHubQueryLatestVersion(repo)
}

// GitHubCheckOutdated checks whether a newer release exists for repo.
// huber-managed only, matching bash's check_outdated_github.
func GitHubCheckOutdated(tool, repo string) (current, latest string, err error) {
	if _, err := exec.LookPath("huber"); err != nil {
		return "", "", fmt.Errorf("huber not installed")
	}

	current = manifest.GetSourceVersion(manifest.Get(tool))

	if current == "" {
		var out bytes.Buffer
		cmd := exec.Command("huber", "update", "--dryrun")
		cmd.Stdout = &out
		_ = cmd.Run() // dryrun output matters even on non-zero exit

		re := regexp.MustCompile(`(?i)Updating package ` + regexp.QuoteMeta(repo) + ` from (\S+)`)
		if m := re.FindStringSubmatch(out.String()); len(m) > 1 {
			current = m[1]
		}
	}
	if current == "" {
		return "", "", fmt.Errorf("could not determine installed version for %s", tool)
	}

	latest = GitHubQueryLatestVersion(repo)
	if latest == "" || latest == current {
		return "", "", fmt.Errorf("already up to date")
	}

	return current, latest, nil
}
