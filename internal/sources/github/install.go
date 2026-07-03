package github

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"slices"
	"strings"

	"github.com/DeprecatedLuar/sat/internal/common"
)

// huberBinDirRel is huber's own binary directory, relative to $HOME.
const huberBinDirRel = ".huber/bin"

// goCmdMainRe matches "cmd/<name>/main.go" tree paths, mirroring bash's
// `grep -oP '^cmd/\K[^/]+(?=/main\.go$)'` (lib/sources/github.sh:284).
var goCmdMainRe = regexp.MustCompile(`^cmd/([^/]+)/main\.go$`)

// huberBinDir returns huber's own binary directory (~/.huber/bin).
func huberBinDir() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, huberBinDirRel)
}

// repoBaseName extracts the repo part of an "owner/repo" string.
func repoBaseName(repoPath string) string {
	if idx := strings.LastIndex(repoPath, "/"); idx != -1 {
		return repoPath[idx+1:]
	}
	return repoPath
}

// firstSegment returns the portion of name before its first hyphen,
// mirroring bash's "${repo_name%%-*}" used for the aggressive huber
// binary search fallback (lib/sources/github.sh:262).
func firstSegment(name string) string {
	if before, _, found := strings.Cut(name, "-"); found {
		return before
	}
	return name
}

func isExecutableFile(path string) bool {
	info, err := os.Stat(path)
	return err == nil && !info.IsDir() && info.Mode()&0111 != 0
}

func containsPath(tree []string, path string) bool {
	return slices.Contains(tree, path)
}

// InstallHuber installs repoPath via huber (binary release install) and
// symlinks the resolved binary into common.LocalBin(). Binary resolution
// mirrors bash's _install_huber (lib/sources/github.sh:229): try release
// asset names first, then the exact repo name, then an aggressive glob
// search over huber's symlink directory.
func InstallHuber(repoPath string) (binName, srcString string, err error) {
	if _, err := exec.LookPath("huber"); err != nil {
		return "", "", fmt.Errorf("huber not installed")
	}

	if err := common.RunQuiet("huber", "install", repoPath); err != nil {
		return "", "", fmt.Errorf("huber install failed: %w", err)
	}

	repoName := repoBaseName(repoPath)
	dir := huberBinDir()

	var huberBin string
	if bins, err := getReleaseBinaries(repoPath); err == nil {
		for _, bin := range bins {
			candidate := filepath.Join(dir, bin)
			if isExecutableFile(candidate) {
				huberBin = candidate
				break
			}
		}
	}

	if huberBin == "" {
		candidate := filepath.Join(dir, repoName)
		if isExecutableFile(candidate) {
			huberBin = candidate
		}
	}

	if huberBin == "" {
		matches, _ := filepath.Glob(filepath.Join(dir, "*"+firstSegment(repoName)+"*"))
		for _, m := range matches {
			if info, err := os.Lstat(m); err == nil && info.Mode()&os.ModeSymlink != 0 {
				huberBin = m
				break
			}
		}
	}

	if huberBin == "" {
		return "", "", fmt.Errorf("could not resolve huber-installed binary for %s", repoPath)
	}

	binName = filepath.Base(huberBin)
	localBin := common.LocalBin()
	if err := os.MkdirAll(localBin, 0755); err != nil {
		return "", "", err
	}
	symlink := filepath.Join(localBin, binName)
	os.Remove(symlink)
	if err := os.Symlink(huberBin, symlink); err != nil {
		return "", "", err
	}

	return binName, "gh:" + repoPath, nil
}

// InstallGo builds repoPath from source via "go install", mirroring bash's
// _install_go (lib/sources/github.sh:274). Requires a go.mod in the repo
// tree; prefers a cmd/<name>/main.go entrypoint, falling back to
// <repo>/main.go or a root main.go.
func InstallGo(repoPath string, tree []string) (binName, srcString string, err error) {
	if _, err := exec.LookPath("go"); err != nil {
		return "", "", fmt.Errorf("go not installed")
	}
	if !containsPath(tree, "go.mod") {
		return "", "", fmt.Errorf("no go.mod found in %s", repoPath)
	}

	repoName := repoBaseName(repoPath)
	var goBin, goSubdir string

	for _, p := range tree {
		if m := goCmdMainRe.FindStringSubmatch(p); m != nil {
			goBin = m[1]
			goSubdir = "cmd"
			break
		}
	}
	if goBin == "" && containsPath(tree, repoName+"/main.go") {
		goBin = repoName
	}

	var goPath string
	switch {
	case goBin != "" && goSubdir != "":
		goPath = fmt.Sprintf("github.com/%s/%s/%s@latest", repoPath, goSubdir, goBin)
	case goBin != "":
		goPath = fmt.Sprintf("github.com/%s/%s@latest", repoPath, goBin)
	case containsPath(tree, "main.go"):
		goPath = fmt.Sprintf("github.com/%s@latest", repoPath)
	default:
		return "", "", fmt.Errorf("no installable go entrypoint found in %s", repoPath)
	}

	if err := common.RunQuiet("go", "install", goPath); err != nil {
		return "", "", fmt.Errorf("go install failed: %w", err)
	}

	if goBin == "" {
		goBin = repoName
	}
	return goBin, "go:github.com/" + repoPath, nil
}

// InstallPython installs repoPath from source via "uv tool install",
// mirroring bash's _install_python (lib/sources/github.sh:302). Requires
// one of pyproject.toml/setup.py/setup.cfg in the repo tree.
func InstallPython(repoPath string, tree []string) (binName, srcString string, err error) {
	if _, err := exec.LookPath("uv"); err != nil {
		return "", "", fmt.Errorf("uv not installed")
	}
	if !containsPath(tree, "pyproject.toml") && !containsPath(tree, "setup.py") && !containsPath(tree, "setup.cfg") {
		return "", "", fmt.Errorf("no python project files found in %s", repoPath)
	}

	if err := common.RunQuiet("uv", "tool", "install", "git+https://github.com/"+repoPath); err != nil {
		return "", "", fmt.Errorf("uv tool install failed: %w", err)
	}

	return repoBaseName(repoPath), "uv", nil
}
