package sources

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/DeprecatedLuar/sat/internal/common"
	"github.com/DeprecatedLuar/sat/internal/manifest"
)

// npmSearchResponse represents the npm registry search API response
type npmSearchResponse struct {
	Objects []struct {
		Package struct {
			Name        string `json:"name"`
			Version     string `json:"version"`
			Description string `json:"description"`
		} `json:"package"`
	} `json:"objects"`
}

// npmVersionResponse represents the npm registry "latest" version endpoint
type npmVersionResponse struct {
	Version string `json:"version"`
}

// npmScopedPkgRe extracts the package name from a global npm bin symlink
// target, e.g. "../lib/node_modules/@org/pkg/cli.js" -> "@org/pkg", or
// "../lib/node_modules/pkg/cli.js" -> "pkg".
var npmScopedPkgRe = regexp.MustCompile(`node_modules/(@[^/]+/[^/]+|[^/]+)/`)

// ResolveNpmPackageName finds the actual npm package name for a global
// binary, handling scoped packages (@org/pkg) whose package name differs
// from the binary name. Mirrors bash's get_npm_package_name.
func ResolveNpmPackageName(tool string) string {
	if strings.HasPrefix(tool, "@") || strings.Contains(tool, "/") {
		return tool
	}

	binPath, err := exec.LookPath(tool)
	if err != nil {
		return tool
	}

	return resolveNpmPackageNameAtPath(binPath, tool)
}

// resolveNpmPackageNameAtPath is ResolveNpmPackageName for a caller that
// already has the binary's full path (e.g. a directory scan), avoiding a
// redundant PATH lookup.
func resolveNpmPackageNameAtPath(binPath, fallback string) string {
	target, err := os.Readlink(binPath)
	if err != nil {
		return fallback
	}

	if m := npmScopedPkgRe.FindStringSubmatch(target); len(m) > 1 {
		return m[1]
	}

	return fallback
}

// NpmSearch searches the npm registry
func NpmSearch(query string) ([]string, error) {
	url := fmt.Sprintf("https://registry.npmjs.org/-/v1/search?text=%s&size=10", query)
	data, err := common.FetchJSON(url, "npm search")
	if err != nil {
		return nil, err
	}

	var resp npmSearchResponse
	if err := json.Unmarshal(data, &resp); err != nil {
		return nil, err
	}

	results := []string{}
	for _, obj := range resp.Objects {
		pkg := obj.Package
		desc := strings.Split(pkg.Description, "\n")[0]
		if desc == "" {
			desc = "(no description)"
		}
		result := fmt.Sprintf("%s %s - %s", pkg.Name, pkg.Version, desc)
		results = append(results, result)
	}

	return results, nil
}

// NpmInstall installs a package globally via npm. It returns the resolved
// package identity when it differs from tool (scoped packages like
// @org/pkg), or "" when it's the same - matching how gh/appimage populate
// installResult.identity only when there's something extra to record.
func NpmInstall(tool string) (identity string, err error) {
	if _, err := exec.LookPath("npm"); err != nil {
		return "", fmt.Errorf("npm not installed")
	}

	// Fail fast with a clean error instead of npm install's messier output
	if err := exec.Command("npm", "show", tool).Run(); err != nil {
		return "", fmt.Errorf("package not found")
	}

	if err := common.RunQuiet("npm", "install", "-g", tool); err != nil {
		return "", fmt.Errorf("npm install failed")
	}

	resolved := ResolveNpmPackageName(tool)
	if resolved == tool {
		return "", nil
	}
	return resolved, nil
}

// NpmUninstall removes a globally installed npm package. Prefers the
// identity recorded in the manifest (resolved at install time), falling
// back to resolving the package name from the binary, then to the raw
// tool name - mirroring bash's uninstall_npm try-then-fallback order.
func NpmUninstall(tool, sourceStr string) error {
	name := manifest.GetSourceIdentity(sourceStr)
	if name == "" {
		name = ResolveNpmPackageName(tool)
	}

	if common.RunQuiet("npm", "uninstall", "-g", name) == nil {
		return nil
	}

	if name == tool {
		return fmt.Errorf("npm uninstall failed")
	}
	return common.RunQuiet("npm", "uninstall", "-g", tool)
}

// NpmUpdate updates a globally installed npm package. identity is the
// resolved package name recorded in the manifest (empty if it matches
// tool), matching the (tool, identity) shape used by GitHubUpdate/
// AppImageUpdate.
func NpmUpdate(tool, identity string) error {
	name := identity
	if name == "" {
		name = ResolveNpmPackageName(tool)
	}
	return common.RunQuiet("npm", "update", "-g", name)
}

// NpmGetVersion returns the installed version of a global npm package.
func NpmGetVersion(tool string) string {
	name := ResolveNpmPackageName(tool)

	var out strings.Builder
	cmd := exec.Command("npm", "list", "-g", name, "--depth=0", "--json")
	cmd.Stdout = &out
	if err := cmd.Run(); err != nil && out.Len() == 0 {
		return ""
	}

	var resp struct {
		Dependencies map[string]struct {
			Version string `json:"version"`
		} `json:"dependencies"`
	}
	if err := json.Unmarshal([]byte(out.String()), &resp); err != nil {
		return ""
	}

	return resp.Dependencies[name].Version
}

// NpmInstalledVersions resolves the live installed version of every given
// npm-tracked tool directly off disk - no `npm` subprocess, and critically
// no npm prefix lookup (`npmBinDir` shells out to `npm config get prefix`,
// ~120ms when NPM_CONFIG_PREFIX is unset, which would dominate a bulk
// reconcile). Each binary's own bin symlink already encodes the path to its
// package directory, so LookPath+Readlink is enough. Only present when the
// answer is known; a tool that can't be resolved is simply absent from the
// result, never mapped to "".
func NpmInstalledVersions(tools []string) map[string]string {
	versions := make(map[string]string, len(tools))
	for _, tool := range tools {
		pkgDir, ok := npmPackageDirFromBinary(tool)
		if !ok {
			continue
		}
		if v := npmReadPackageVersion(pkgDir); v != "" {
			versions[tool] = v
		}
	}
	return versions
}

// npmPackageDirFromBinary resolves a global npm binary's own bin symlink to
// its package directory, e.g. binary "codex" -> symlink target
// "../lib/node_modules/@openai/codex/bin/codex.js" -> package dir
// "<npmBinDir>/../lib/node_modules/@openai/codex".
func npmPackageDirFromBinary(tool string) (string, bool) {
	binPath, err := exec.LookPath(tool)
	if err != nil {
		return "", false
	}
	return npmPackageDirFromBinPath(binPath)
}

// npmPackageDirFromBinPath is npmPackageDirFromBinary's testable core: no
// PATH lookup, just the symlink-following logic against an already-resolved
// binary path. Reuses npmScopedPkgRe (the same regex ResolveNpmPackageName
// matches against) but keeps the matched prefix as a path instead of just
// the captured package name.
func npmPackageDirFromBinPath(binPath string) (string, bool) {
	target, err := os.Readlink(binPath)
	if err != nil {
		return "", false
	}
	loc := npmScopedPkgRe.FindStringSubmatchIndex(target)
	if loc == nil {
		return "", false
	}
	// loc[1] is the end of the full match, which includes the trailing
	// slash after the package name segment.
	return filepath.Join(filepath.Dir(binPath), target[:loc[1]]), true
}

// npmReadPackageVersion reads the "version" field straight out of a global
// npm package's own package.json.
func npmReadPackageVersion(pkgDir string) string {
	data, err := os.ReadFile(filepath.Join(pkgDir, "package.json"))
	if err != nil {
		return ""
	}
	var doc struct {
		Version string `json:"version"`
	}
	if json.Unmarshal(data, &doc) != nil {
		return ""
	}
	return doc.Version
}

// NpmQueryLatestVersion queries the latest version from the npm registry.
func NpmQueryLatestVersion(pkg string) string {
	url := fmt.Sprintf("https://registry.npmjs.org/%s/latest", pkg)
	data, err := common.FetchJSON(url, fmt.Sprintf("npm/%s", pkg))
	if err != nil {
		return ""
	}

	var resp npmVersionResponse
	if err := json.Unmarshal(data, &resp); err != nil {
		return ""
	}

	return resp.Version
}

// NpmCheckOutdated checks if an npm package has updates available.
// identity is the resolved package name from the manifest (empty if it
// matches tool), matching GitHubCheckOutdated/AppImageCheckOutdated's
// (tool, identity) shape.
func NpmCheckOutdated(tool, identity string) (current, latest string, err error) {
	if _, err := exec.LookPath("npm"); err != nil {
		return "", "", fmt.Errorf("npm not installed")
	}

	name := identity
	if name == "" {
		name = ResolveNpmPackageName(tool)
	}

	// Try manifest first, fall back to npm outdated
	if sourceStr := manifest.Get(tool); sourceStr != "" {
		current = manifest.GetSourceVersion(sourceStr)
	}
	if current == "" {
		var out strings.Builder
		cmd := exec.Command("npm", "outdated", "-g", "--json")
		cmd.Stdout = &out
		_ = cmd.Run() // npm outdated exits non-zero when it finds outdated packages
		var resp map[string]struct {
			Current string `json:"current"`
		}
		if json.Unmarshal([]byte(out.String()), &resp) == nil {
			current = resp[name].Current
		}
	}
	if current == "" {
		return "", "", fmt.Errorf("package not installed")
	}

	latest = NpmQueryLatestVersion(name)
	if latest == "" {
		return "", "", fmt.Errorf("failed to query npm registry")
	}

	if current == latest {
		return "", "", fmt.Errorf("already up to date")
	}

	return current, latest, nil
}

// npmBinDir returns npm's global bin directory.
func npmBinDir() string {
	if prefix := os.Getenv("NPM_CONFIG_PREFIX"); prefix != "" {
		return filepath.Join(prefix, "bin")
	}

	if _, err := exec.LookPath("npm"); err == nil {
		var out strings.Builder
		cmd := exec.Command("npm", "config", "get", "prefix")
		cmd.Stdout = &out
		if cmd.Run() == nil {
			if prefix := strings.TrimSpace(out.String()); prefix != "" {
				return filepath.Join(prefix, "bin")
			}
		}
	}

	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".npm-global", "bin")
}

// NpmScan scans the npm global bin directory for installed packages.
// Every entry there is a symlink into node_modules, and a single package
// can install multiple binaries (e.g. netlify-cli installs both `netlify`
// and `ntl`) - so binaries are resolved to their real package name and
// deduped, keeping the first binary name encountered as the manifest key
// and recording the resolved package name as Identity when it differs.
func NpmScan() ([]Package, error) {
	dir := npmBinDir()
	if dir == "" || !dirExists(dir) {
		return nil, nil
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, nil
	}

	seen := make(map[string]bool)
	var packages []Package

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		info, err := entry.Info()
		if err != nil || !isExecutable(info) {
			continue
		}

		prog := entry.Name()
		pkgName := resolveNpmPackageNameAtPath(filepath.Join(dir, prog), prog)
		if seen[pkgName] {
			continue
		}
		seen[pkgName] = true

		identity := ""
		if pkgName != prog {
			identity = pkgName
		}

		packages = append(packages, Package{
			Name:     prog,
			Source:   common.SourceNPM,
			Identity: identity,
			Version:  NpmGetVersion(prog),
		})
	}

	return packages, nil
}

// NpmManifestIssues examines every currently-tracked npm manifest entry and
// reports duplicates, without mutating anything - the caller (scanner's
// CleanupManifest) applies prune/repair and handles display, the same
// data-in/data-out contract NpmScan follows for new entries. Detects
// multiple binaries resolving to the same package (e.g. netlify-cli
// tracked twice, as both `netlify` and `ntl`) - all but the first-tracked
// are reported for pruning. Missing or stale versions are no longer
// repaired here - the internal/drift package reconciles every recorded
// version (empty or wrong) against a live query, superseding what used to
// be a scan-only, empty-version-only backfill.
func NpmManifestIssues() ManifestIssues {
	var issues ManifestIssues

	entries, err := manifest.All()
	if err != nil {
		return issues
	}

	seen := make(map[string]bool)

	for _, e := range entries {
		if manifest.GetSourceType(e.Source) != common.SourceNPM {
			continue
		}

		pkgName := ResolveNpmPackageName(e.Tool)
		if seen[pkgName] {
			issues.Prune = append(issues.Prune, PrunedEntry{Tool: e.Tool, Reason: "duplicate of another npm binary"})
			continue
		}
		seen[pkgName] = true
	}

	return issues
}

// npmPackageDocument is the registry document for one package. Only the
// latest version's fields matter for a lookup, but the registry returns
// every version, so the map is indexed by dist-tags.
type npmPackageDocument struct {
	Name     string            `json:"name"`
	DistTags map[string]string `json:"dist-tags"`
	Versions map[string]struct {
		Description string          `json:"description"`
		Deprecated  string          `json:"deprecated"`
		Homepage    string          `json:"homepage"`
		Bin         json.RawMessage `json:"bin"`
		Repository  json.RawMessage `json:"repository"`
	} `json:"versions"`
}

// NpmLookup resolves an exact package name. npm's `bin` field states
// exactly what lands on PATH, which is decisive: many popular tool names
// are squatted by npm packages that install nothing at all.
func NpmLookup(name string) (LookupResult, error) {
	data, err := common.FetchJSON("https://registry.npmjs.org/"+name, "npm lookup")
	if err != nil {
		return LookupResult{}, ErrNoMatch
	}

	var doc npmPackageDocument
	if err := json.Unmarshal(data, &doc); err != nil {
		return LookupResult{}, ErrNoMatch
	}
	if doc.Name != name {
		return LookupResult{}, ErrNoMatch
	}

	version, ok := doc.Versions[doc.DistTags["latest"]]
	if !ok {
		return LookupResult{}, ErrNoMatch
	}

	result := LookupResult{
		Name:        doc.Name,
		Version:     doc.DistTags["latest"],
		Description: version.Description,
		Repo:        npmRepositoryURL(version.Repository),
		Homepage:    version.Homepage,
		Bins:        npmBinaryNames(version.Bin, doc.Name),
		BinsKnown:   true,
	}
	if version.Deprecated != "" {
		result.Dead, result.DeadReason = true, "deprecated: "+version.Deprecated
	}
	return result, nil
}

// npmBinaryNames normalizes npm's `bin` field, which is either a string
// (one executable named after the package) or a name->path map.
func npmBinaryNames(raw json.RawMessage, pkgName string) []string {
	if len(raw) == 0 || string(raw) == "null" {
		return nil
	}

	var asMap map[string]string
	if err := json.Unmarshal(raw, &asMap); err == nil {
		names := make([]string, 0, len(asMap))
		for name := range asMap {
			names = append(names, name)
		}
		sort.Strings(names)
		return names
	}

	var asString string
	if err := json.Unmarshal(raw, &asString); err == nil && asString != "" {
		return []string{pkgName}
	}
	return nil
}

// npmRepositoryURL normalizes npm's `repository` field, which is either a
// bare string or an object carrying the url.
func npmRepositoryURL(raw json.RawMessage) string {
	if len(raw) == 0 {
		return ""
	}

	var asObj struct {
		URL string `json:"url"`
	}
	if err := json.Unmarshal(raw, &asObj); err == nil && asObj.URL != "" {
		return asObj.URL
	}

	var asString string
	if err := json.Unmarshal(raw, &asString); err == nil {
		return asString
	}
	return ""
}
