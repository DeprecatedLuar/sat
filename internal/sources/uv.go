package sources

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"

	"github.com/DeprecatedLuar/sat/internal/common"
	"github.com/DeprecatedLuar/sat/internal/manifest"
)

// UvInstall installs a package via uv tool install
func UvInstall(tool string) error {
	if _, err := exec.LookPath("uv"); err != nil {
		return fmt.Errorf("uv not installed")
	}
	return common.RunQuiet("uv", "tool", "install", tool)
}

// uvToolList runs `uv tool list` and returns its stdout
func uvToolList() string {
	var output bytes.Buffer
	cmd := exec.Command("uv", "tool", "list")
	cmd.Stdout = &output
	cmd.Stderr = nil

	if cmd.Run() != nil {
		return ""
	}
	return output.String()
}

// uvToolEntry is one package block from `uv tool list` output: the package
// itself plus every binary it exposes (a package's binaries can differ from
// its own name, e.g. "kimi-cli" providing "kimi" and "kimi-cli").
type uvToolEntry struct {
	Package string
	Version string
	Bins    []string
}

// parseUvToolListEntries is the single parser for `uv tool list` output -
// every other uv lookup (version, package-name resolution, scan) is built
// on this instead of re-walking the raw text. Format:
//
//	package-name vVERSION
//	- binary1
//	- binary2
func parseUvToolListEntries(output string) []uvToolEntry {
	var entries []uvToolEntry
	var current *uvToolEntry

	for _, line := range strings.Split(output, "\n") {
		if name, ok := strings.CutPrefix(line, "- "); ok {
			if current == nil {
				continue
			}
			current.Bins = append(current.Bins, name)
			continue
		}

		fields := strings.Fields(line)
		if len(fields) < 2 {
			current = nil
			continue
		}
		entries = append(entries, uvToolEntry{
			Package: fields[0],
			Version: strings.TrimPrefix(fields[1], "v"),
		})
		current = &entries[len(entries)-1]
	}
	return entries
}

// parseUvToolList flattens uv tool list output into a single map keyed by
// both each package name and every binary it exposes, so a lookup never
// needs to know in advance whether it has a package name or a binary name.
func parseUvToolList(output string) map[string]string {
	versions := make(map[string]string)
	for _, e := range parseUvToolListEntries(output) {
		versions[e.Package] = e.Version
		for _, bin := range e.Bins {
			versions[bin] = e.Version
		}
	}
	return versions
}

// uvGetPackageName finds the uv package name that owns a given binary,
// or "" if binary isn't found as an entry point of any installed package.
func uvGetPackageName(binary string) string {
	for _, e := range parseUvToolListEntries(uvToolList()) {
		for _, bin := range e.Bins {
			if bin == binary {
				return e.Package
			}
		}
	}
	return ""
}

// uvResolvePackageName finds the uv package name that owns a given binary,
// falling back to name itself when no separate alias is found (name already
// equals the package name, or uv isn't reachable). A package can expose
// entry-point binaries that differ from its own name (e.g. package
// "kimi-cli" providing binaries "kimi" and "kimi-cli"), so every uv
// subcommand that takes a package name must resolve through this first
// rather than assume the sat-tracked tool name is the uv package name.
func uvResolvePackageName(name string) string {
	if pkg := uvGetPackageName(name); pkg != "" {
		return pkg
	}
	return name
}

// UvUninstall removes a uv tool
// Binary name may differ from package name - look it up first
func UvUninstall(pkg, source string) error {
	return common.RunQuiet("uv", "tool", "uninstall", uvResolvePackageName(pkg))
}

// UvGetVersion returns the installed version of a uv tool. tool may be
// either a package name or one of its binaries - parseUvToolList keys by
// both, so no separate resolution step (or extra `uv tool list` call) is
// needed here.
func UvGetVersion(tool string) string {
	return parseUvToolList(uvToolList())[tool]
}

// UvInstalledVersions resolves live versions for every given uv-typed tool
// in a single `uv tool list` call, keyed by whichever name the manifest
// happens to use for each (package or binary).
func UvInstalledVersions(tools []string) map[string]string {
	if _, err := exec.LookPath("uv"); err != nil {
		return nil
	}
	all := parseUvToolList(uvToolList())
	versions := make(map[string]string, len(tools))
	for _, tool := range tools {
		if v, ok := all[tool]; ok && v != "" {
			versions[tool] = v
		}
	}
	return versions
}

// UvUpdate updates a uv tool
func UvUpdate(tool string) error {
	return common.RunQuiet("uv", "tool", "upgrade", uvResolvePackageName(tool))
}

// UvQueryLatestVersion queries the latest version from PyPI
func UvQueryLatestVersion(pkg string) string {
	url := fmt.Sprintf("https://pypi.org/pypi/%s/json", pkg)
	data, err := common.FetchJSON(url, fmt.Sprintf("pypi.org/%s", pkg))
	if err != nil {
		return ""
	}

	var resp pypiPackageResponse
	if err := json.Unmarshal(data, &resp); err != nil {
		return ""
	}

	return resp.Info.Version
}

// UvCheckOutdated checks if a uv tool has updates available
func UvCheckOutdated(tool string) (current, latest string, err error) {
	if _, err := exec.LookPath("uv"); err != nil {
		return "", "", fmt.Errorf("uv not installed")
	}

	if sourceStr := manifest.Get(tool); sourceStr != "" {
		current = manifest.GetSourceVersion(sourceStr)
	}
	if current == "" {
		current = UvGetVersion(tool)
	}
	if current == "" {
		return "", "", fmt.Errorf("package not installed")
	}

	// The binary/tool name can differ from the PyPI package name uv
	// actually installed (see uvResolvePackageName); query PyPI using the
	// real package name so a same-named-but-unrelated PyPI project
	// doesn't get mistaken for this tool's upstream.
	latest = UvQueryLatestVersion(uvResolvePackageName(tool))
	if latest == "" {
		return "", "", fmt.Errorf("failed to query pypi.org")
	}

	if current == latest {
		return "", "", fmt.Errorf("already up to date")
	}

	return current, latest, nil
}

// UvScan scans for installed uv tools
func UvScan() ([]Package, error) {
	if _, err := exec.LookPath("uv"); err != nil {
		return nil, nil
	}

	output := uvToolList()
	if output == "" {
		return nil, nil
	}

	var packages []Package
	for _, e := range parseUvToolListEntries(output) {
		for _, bin := range e.Bins {
			packages = append(packages, Package{
				Name:     bin,
				Source:   common.SourceUV,
				Identity: "",
				Version:  e.Version,
			})
		}
	}

	return packages, nil
}
