package sources

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
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
// reports what's stale, without mutating anything - the caller (scanner's
// CleanupManifest) applies prune/repair and handles display, the same
// data-in/data-out contract NpmScan follows for new entries. Two issues are
// detected, both stemming from before npm's own resolution logic existed:
//   - duplicates: multiple binaries resolving to the same package (e.g.
//     netlify-cli tracked twice, as both `netlify` and `ntl`) - all but the
//     first-tracked are reported for pruning.
//   - repairs: entries with no version on record because they predate
//     version lookup - scan only ever adds new entries, so a stale blank
//     version is otherwise never revisited.
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

		if manifest.GetSourceVersion(e.Source) != "" {
			continue
		}

		version := NpmGetVersion(e.Tool)
		if version == "" {
			continue
		}

		identity := manifest.GetSourceIdentity(e.Source)
		if identity == "" && pkgName != e.Tool {
			identity = pkgName
		}

		issues.Repair = append(issues.Repair, RepairedEntry{
			Tool:      e.Tool,
			NewSource: manifest.BuildSourceString(common.SourceNPM, identity, version),
		})
	}

	return issues
}
