package sources

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"time"

	"github.com/DeprecatedLuar/sat/internal/common"
	"github.com/DeprecatedLuar/sat/internal/sources/github"
)

const (
	goRawHost       = "https://raw.githubusercontent.com"
	goRawPathFmt    = "%s/%s/HEAD/%s"
	goModFile       = "go.mod"
	goModuleKeyword = "module"
	goLatest        = "latest"
	goFetchTimeout  = 30 * time.Second
	goTempPrefix    = ".sat-go-"
	goDevelVersion  = "(devel)"
	goModLineKey    = "mod"
	goModVersionAt  = 2 // "mod <path> <version> <sum>"

	goListMainFormat = `{{if eq .Name "main"}}{{.ImportPath}}{{end}}`
	goListAllPkgs    = "./..."
)

var (
	goHelperSegments   = []string{"internal", "script", "scripts", "testdata", "examples", "example"}
	goHelperSubstrings = []string{"gen-docs", "gen_docs", "gendocs"}
	goGenericBinNames  = []string{"cmd", "main", "app", "bin", "cli"}

	goMajorVersionRe = regexp.MustCompile(`^v[0-9]+$`)

	// goRawBase and goRun are seams for tests.
	goRawBase = goRawHost
	goRun     = runGo

	errGoModNotFound = errors.New("go.mod not found")

	// ErrGoNotInstalled reports a missing go toolchain.
	ErrGoNotInstalled = errors.New("go not installed")

	// ErrNotGoProject reports a repository that has no go.mod to install from.
	ErrNotGoProject = errors.New("not a Go project")
)

// AmbiguousGoPackageError reports a module with several unrelated main
// packages, none matching the repository name, so the caller must choose
// rather than sat guessing.
type AmbiguousGoPackageError struct {
	Module     string
	Candidates []string
}

func (e *AmbiguousGoPackageError) Error() string {
	var b strings.Builder
	fmt.Fprintf(&b, "%d main packages matched in %s:\n", len(e.Candidates), e.Module)
	for _, c := range e.Candidates {
		fmt.Fprintf(&b, "  %s\n", c)
	}
	fmt.Fprint(&b, "\nnone matches the repository name; install one with: go install <package>@latest")
	return b.String()
}

// GoInstall installs a Go program from spec, either "owner/repo" on GitHub or
// a module path (first segment has a dot). The entrypoint is resolved from the
// module's own go.mod and package list, never from file names. Returns the
// installed binary name and the package import path recorded as identity.
func GoInstall(spec string) (binName, pkgPath string, err error) {
	if _, err := exec.LookPath(common.GoTool); err != nil {
		return "", "", ErrGoNotInstalled
	}

	module, repoName, err := resolveGoModule(spec)
	if err != nil {
		return "", "", err
	}

	dir, version, err := downloadGoModule(module)
	if err != nil {
		return "", "", err
	}

	mains, err := listGoMainPackages(dir)
	if err != nil {
		return "", "", err
	}

	pkgPath, err = selectGoMainPackage(module, repoName, mains)
	if err != nil {
		return "", "", err
	}

	binName, err = installGoPackage(pkgPath, version, func(produced string) string {
		return goBinaryName(produced, repoName)
	})
	if err != nil {
		return "", "", err
	}
	return binName, pkgPath, nil
}

// GoUpdate reinstalls pkgPath at its latest version under the existing
// binary name.
func GoUpdate(binName, pkgPath string) error {
	if pkgPath == "" {
		return fmt.Errorf("no package path recorded for %s", binName)
	}
	_, err := installGoPackage(pkgPath, goLatest, func(string) string { return binName })
	return err
}

// GoUninstall removes binName from the go bin directory. A missing file is
// not an error.
func GoUninstall(binName string) error {
	if binName == "" || binName != filepath.Base(binName) {
		return fmt.Errorf("invalid binary name %q", binName)
	}
	dir := common.GoBinDir()
	if dir == "" {
		return fmt.Errorf("could not resolve go bin directory")
	}
	if err := os.Remove(filepath.Join(dir, binName)); err != nil && !os.IsNotExist(err) {
		return err
	}
	return nil
}

// GoGetVersion returns the module version embedded in the installed binary,
// or "" when unreadable or not a tagged build.
func GoGetVersion(binName string) string {
	dir := common.GoBinDir()
	if dir == "" {
		return ""
	}
	out, _, err := goRun("", nil, "version", "-m", filepath.Join(dir, binName))
	if err != nil {
		return ""
	}
	return parseGoModVersion(string(out))
}

// parseGoModVersion extracts the main module version from `go version -m`.
func parseGoModVersion(output string) string {
	for _, line := range strings.Split(output, "\n") {
		fields := strings.Fields(line)
		if len(fields) > goModVersionAt && fields[0] == goModLineKey {
			if fields[goModVersionAt] == goDevelVersion {
				return ""
			}
			return fields[goModVersionAt]
		}
	}
	return ""
}

// resolveGoModule returns the module path declared by spec's go.mod and the
// name used to rank its main packages.
func resolveGoModule(spec string) (module, repoName string, err error) {
	if isGoModulePath(spec) {
		return spec, goBaseName(spec), nil
	}

	repoName = path.Base(spec)
	body, err := fetchRawGoMod(spec, goModFile)
	if errors.Is(err, errGoModNotFound) {
		body, err = fetchNestedGoMod(spec, repoName)
	}
	if err != nil {
		return "", "", err
	}

	module, err = parseGoModulePath(body)
	if err != nil {
		return "", "", err
	}
	return module, repoName, nil
}

// isGoModulePath reports whether spec's first segment looks like a host.
func isGoModulePath(spec string) bool {
	first, _, _ := strings.Cut(spec, "/")
	return strings.Contains(first, ".")
}

// fetchRawGoMod reads repo/file at HEAD, returning errGoModNotFound on 404.
func fetchRawGoMod(repo, file string) (string, error) {
	url := fmt.Sprintf(goRawPathFmt, goRawBase, repo, file)
	client := &http.Client{Timeout: goFetchTimeout}
	resp, err := client.Get(url)
	if err != nil {
		return "", fmt.Errorf("fetching %s: %w", file, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return "", errGoModNotFound
	}
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("fetching %s for %s: http %d", file, repo, resp.StatusCode)
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("reading %s: %w", file, err)
	}
	return string(body), nil
}

// fetchNestedGoMod handles repos with no root go.mod by reading the one whose
// directory name equals the repo name.
func fetchNestedGoMod(repo, repoName string) (string, error) {
	tree, err := github.FetchTree(repo)
	if err != nil {
		return "", fmt.Errorf("listing %s: %w", repo, err)
	}

	file := nestedGoModPath(tree, repoName)
	if file == "" {
		return "", fmt.Errorf("%w: no go.mod in %s", ErrNotGoProject, repo)
	}
	return fetchRawGoMod(repo, file)
}

// nestedGoModPath picks the shallowest nested go.mod whose directory base
// equals repoName, or "" when none matches.
func nestedGoModPath(tree []string, repoName string) string {
	best := ""
	for _, p := range tree {
		if path.Base(p) != goModFile || p == goModFile {
			continue
		}
		if path.Base(path.Dir(p)) != repoName {
			continue
		}
		if best == "" || len(p) < len(best) {
			best = p
		}
	}
	return best
}

// parseGoModulePath returns the path on the `module` directive.
func parseGoModulePath(goMod string) (string, error) {
	for _, line := range strings.Split(goMod, "\n") {
		line, _, _ = strings.Cut(line, "//")
		fields := strings.Fields(line)
		if len(fields) == 2 && fields[0] == goModuleKeyword {
			return strings.Trim(fields[1], "\"`"), nil
		}
	}
	return "", fmt.Errorf("no module directive in %s", goModFile)
}

// downloadGoModule fetches module@latest into the module cache and returns
// its source directory and resolved version.
func downloadGoModule(module string) (dir, version string, err error) {
	out, stderr, runErr := goRun("", nil, "mod", "download", "-json", module+"@"+goLatest)

	var res struct {
		Dir     string
		Version string
		Error   string
	}
	if jsonErr := json.Unmarshal(out, &res); jsonErr == nil && res.Error != "" {
		return "", "", fmt.Errorf("go mod download: %s", res.Error)
	}
	if runErr != nil {
		return "", "", goFailure("go mod download", stderr, runErr)
	}
	if res.Dir == "" || res.Version == "" {
		return "", "", fmt.Errorf("go mod download returned no directory for %s", module)
	}
	return res.Dir, res.Version, nil
}

// listGoMainPackages returns the import path of every main package in dir.
func listGoMainPackages(dir string) ([]string, error) {
	out, stderr, err := goRun(dir, nil, "list", "-e", "-f", goListMainFormat, goListAllPkgs)
	if err != nil {
		return nil, goFailure("go list", stderr, err)
	}

	var pkgs []string
	for _, line := range strings.Split(string(out), "\n") {
		if line = strings.TrimSpace(line); line != "" {
			pkgs = append(pkgs, line)
		}
	}
	return pkgs, nil
}

// selectGoMainPackage drops helper packages then ranks the rest: base equal to
// repoName, base equal to the module base, the module root, a sole survivor.
// A tier matching several packages, or no tier matching, is ambiguous.
func selectGoMainPackage(module, repoName string, pkgs []string) (string, error) {
	var cands []string
	for _, p := range pkgs {
		if !isGoHelperPackage(module, p) {
			cands = append(cands, p)
		}
	}
	if len(cands) == 0 {
		return "", fmt.Errorf("no installable main package in %s", module)
	}

	tiers := []func(string) bool{
		func(p string) bool { return goBaseName(p) == repoName },
		func(p string) bool { return goBaseName(p) == goBaseName(module) },
		func(p string) bool { return p == module },
		func(string) bool { return len(cands) == 1 },
	}
	for _, match := range tiers {
		var hits []string
		for _, p := range cands {
			if match(p) {
				hits = append(hits, p)
			}
		}
		switch len(hits) {
		case 0:
		case 1:
			return hits[0], nil
		default:
			return "", &AmbiguousGoPackageError{Module: module, Candidates: hits}
		}
	}
	return "", &AmbiguousGoPackageError{Module: module, Candidates: cands}
}

// isGoHelperPackage reports whether pkg, relative to module, is a helper
// (internal, scripts, examples, doc generators) rather than a product binary.
func isGoHelperPackage(module, pkg string) bool {
	rel := strings.TrimPrefix(strings.TrimPrefix(pkg, module), "/")
	for _, seg := range strings.Split(rel, "/") {
		if slices.Contains(goHelperSegments, seg) {
			return true
		}
		for _, sub := range goHelperSubstrings {
			if strings.Contains(seg, sub) {
				return true
			}
		}
	}
	return false
}

// goBaseName is the last element of an import path, ignoring a /vN suffix.
func goBaseName(importPath string) string {
	segs := strings.Split(importPath, "/")
	if n := len(segs); n > 1 && goMajorVersionRe.MatchString(segs[n-1]) {
		segs = segs[:n-1]
	}
	return segs[len(segs)-1]
}

// goBinaryName replaces a generic produced name (e.g. "cmd") with repoName.
func goBinaryName(produced, repoName string) string {
	if slices.Contains(goGenericBinNames, produced) {
		return repoName
	}
	return produced
}

// installGoPackage builds pkg@version into a scratch dir on the go bin
// filesystem, names the single produced binary via pickName, and moves it
// into the go bin directory.
func installGoPackage(pkg, version string, pickName func(produced string) string) (string, error) {
	binDir := common.GoBinDir()
	if binDir == "" {
		return "", fmt.Errorf("could not resolve go bin directory")
	}
	if err := os.MkdirAll(binDir, 0o755); err != nil {
		return "", err
	}
	scratch, err := os.MkdirTemp(binDir, goTempPrefix)
	if err != nil {
		return "", err
	}
	defer os.RemoveAll(scratch)

	env := append(os.Environ(), "GOBIN="+scratch)
	if _, stderr, err := goRun("", env, "install", pkg+"@"+version); err != nil {
		return "", goFailure("go install", stderr, err)
	}

	entries, err := os.ReadDir(scratch)
	if err != nil {
		return "", err
	}
	if len(entries) != 1 {
		return "", fmt.Errorf("go install %s produced %d binaries, expected 1", pkg, len(entries))
	}

	name := pickName(entries[0].Name())
	if err := os.Rename(filepath.Join(scratch, entries[0].Name()), filepath.Join(binDir, name)); err != nil {
		return "", err
	}
	return name, nil
}

// goFailure reports the toolchain's own message verbatim when it gave one.
func goFailure(action, stderr string, err error) error {
	if stderr != "" {
		return fmt.Errorf("%s: %s", action, stderr)
	}
	return fmt.Errorf("%s: %w", action, err)
}

// runGo runs the go tool, returning stdout and trimmed stderr. A nil env
// inherits the process environment.
func runGo(dir string, env []string, args ...string) ([]byte, string, error) {
	var stdout, stderr bytes.Buffer
	cmd := exec.Command(common.GoTool, args...)
	cmd.Dir = dir
	cmd.Env = env
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()

	if os.Getenv(common.EnvSATDebug) != "" {
		fmt.Fprint(os.Stderr, stderr.String())
	}
	return stdout.Bytes(), strings.TrimSpace(stderr.String()), err
}
