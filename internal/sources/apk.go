package sources

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"regexp"
	"strings"

	"github.com/DeprecatedLuar/sat/internal/common"
)

// ApkInstall installs a package via apk
func ApkInstall(tool string) error {
	if !ApkPkgExists(tool) {
		return fmt.Errorf("package %s not found in apk repositories", tool)
	}
	return common.PkgInstall(tool, "apk")
}

// ApkUninstall removes an apk package
func ApkUninstall(pkg string) error {
	return common.RunQuiet("sudo", "apk", "del", pkg)
}

// ApkUpdate updates an apk package
func ApkUpdate(tool string) error {
	return common.RunQuiet("sudo", "apk", "upgrade", tool)
}

// ApkGetVersion returns the installed version of an apk package
func ApkGetVersion(tool string) string {
	var output bytes.Buffer
	cmd := exec.Command("apk", "info", "-e", tool)
	cmd.Stdout = &output
	cmd.Stderr = nil

	if cmd.Run() != nil {
		return ""
	}

	// Output: "tool-version"
	line := strings.TrimSpace(output.String())
	re := regexp.MustCompile(`^` + regexp.QuoteMeta(tool) + `-(.+)$`)
	if matches := re.FindStringSubmatch(line); len(matches) > 1 {
		return matches[1]
	}
	return ""
}

// ApkPkgExists checks if package exists in apk repositories
func ApkPkgExists(pkg string) bool {
	// apk search returns packages matching pattern
	cmd := exec.Command("apk", "search", "-e", pkg)
	cmd.Stdout = nil
	cmd.Stderr = nil
	return cmd.Run() == nil
}

// ApkSearch searches for packages in apk repositories
func ApkSearch(query string) ([]string, error) {
	var output bytes.Buffer
	cmd := exec.Command("apk", "search", "-v", query)
	cmd.Stdout = &output
	cmd.Stderr = nil

	if err := cmd.Run(); err != nil {
		return nil, err
	}

	results := []string{}
	lines := strings.Split(output.String(), "\n")

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		// Parse: "package-version"
		// Find first hyphen followed by digit
		re := regexp.MustCompile(`^(.+?)-(\d.+)$`)
		if matches := re.FindStringSubmatch(line); len(matches) == 3 {
			name := matches[1]
			version := matches[2]
			result := fmt.Sprintf("%s %s - (no description)", name, version)
			results = append(results, result)
			if len(results) >= MaxSearchResults {
				break
			}
		}
	}

	return results, nil
}

// apkOwnerOf resolves the package owning a binary, via `apk info --who-owns`
func apkOwnerOf(binPath string) string {
	var output bytes.Buffer
	cmd := exec.Command("apk", "info", "--who-owns", binPath)
	cmd.Stdout = &output
	cmd.Stderr = nil

	if cmd.Run() != nil {
		return ""
	}

	fields := strings.Fields(strings.TrimSpace(output.String()))
	if len(fields) == 0 {
		return ""
	}

	// Strip version: package-1.2.3 → package
	return strings.Split(fields[len(fields)-1], "-")[0]
}

// ApkScan scans explicitly-installed Alpine packages
func ApkScan() ([]Package, error) {
	// Read /etc/apk/world for explicitly-installed packages
	data, err := os.ReadFile("/etc/apk/world")
	if err != nil {
		return nil, nil
	}

	worldPackages := make(map[string]bool)
	for _, line := range strings.Split(string(data), "\n") {
		pkg := strings.TrimSpace(line)
		if pkg != "" {
			worldPackages[pkg] = true
		}
	}

	if len(worldPackages) == 0 {
		return nil, nil
	}

	binDirs := []string{"/usr/bin", "/usr/local/bin", "/bin"}
	return scanBinDirsOwnedBy(binDirs, "apk", worldPackages, apkOwnerOf, ApkGetVersion), nil
}
