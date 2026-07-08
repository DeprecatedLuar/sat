package sources

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os/exec"
	"regexp"
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

// uvGetPackageName finds the uv package name that owns a given binary
func uvGetPackageName(binary string) string {
	lines := strings.Split(uvToolList(), "\n")
	var currentPkg string

	for _, line := range lines {
		if name, ok := strings.CutPrefix(line, "- "); ok {
			if name == binary && currentPkg != "" {
				return currentPkg
			}
			continue
		}
		if fields := strings.Fields(line); len(fields) > 0 {
			currentPkg = fields[0]
		}
	}

	return ""
}

// UvUninstall removes a uv tool
// Binary name may differ from package name - look it up first
func UvUninstall(pkg, source string) error {
	uvPkg := uvGetPackageName(pkg)
	if uvPkg == "" {
		uvPkg = pkg
	}
	return common.RunQuiet("uv", "tool", "uninstall", uvPkg)
}

// UvGetVersion returns the installed version of a uv tool
func UvGetVersion(tool string) string {
	re := regexp.MustCompile(`^` + regexp.QuoteMeta(tool) + ` v(\S+)`)
	lines := strings.Split(uvToolList(), "\n")
	for _, line := range lines {
		if matches := re.FindStringSubmatch(line); len(matches) > 1 {
			return matches[1]
		}
	}
	return ""
}

// UvUpdate updates a uv tool
func UvUpdate(tool string) error {
	return common.RunQuiet("uv", "tool", "upgrade", tool)
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

	latest = UvQueryLatestVersion(tool)
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
	var currentPkg, currentVersion string

	for _, line := range strings.Split(output, "\n") {
		if name, ok := strings.CutPrefix(line, "- "); ok {
			if currentPkg == "" {
				continue
			}
			packages = append(packages, Package{
				Name:     name,
				Source:   "uv",
				Identity: "",
				Version:  currentVersion,
			})
			continue
		}

		fields := strings.Fields(line)
		if len(fields) < 2 {
			currentPkg = ""
			continue
		}
		currentPkg = fields[0]
		currentVersion = strings.TrimPrefix(fields[1], "v")
	}

	return packages, nil
}
