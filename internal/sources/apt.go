package sources

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"regexp"
	"strings"

	"github.com/DeprecatedLuar/sat/internal/common"
	"github.com/DeprecatedLuar/sat/internal/manifest"
)

const (
	UpgradablePrefix    = "upgradable from: "
	UpgradablePrefixLen = 17
)

// AptInstall installs a package via apt
func AptInstall(tool string) error {
	if !AptPkgExists(tool) {
		return fmt.Errorf("package %s not found in apt repositories", tool)
	}
	return common.PkgInstall(tool, "apt")
}

// AptUninstall removes an apt package
func AptUninstall(pkg string) error {
	return common.RunQuiet("sudo", "apt", "remove", "--purge", "-y", pkg)
}

// AptUpdate updates an apt package
func AptUpdate(tool string) error {
	return common.RunQuiet("sudo", "apt-get", "install", "--only-upgrade", "-y", tool)
}

// AptGetVersion returns the installed version of an apt package
func AptGetVersion(tool string) string {
	var output bytes.Buffer
	cmd := exec.Command("dpkg", "-l", tool)
	cmd.Stdout = &output
	cmd.Stderr = nil

	if cmd.Run() != nil {
		return ""
	}

	// Parse dpkg output: "ii  tool  version  description"
	lines := strings.Split(output.String(), "\n")
	for _, line := range lines {
		if strings.HasPrefix(line, "ii") {
			fields := strings.Fields(line)
			if len(fields) >= 3 {
				return fields[2]
			}
		}
	}
	return ""
}

// AptPkgExists checks if package exists in apt repositories
func AptPkgExists(pkg string) bool {
	cmd := exec.Command("apt-cache", "show", pkg)
	cmd.Stdout = nil
	cmd.Stderr = nil
	return cmd.Run() == nil
}

// AptCheckOutdated checks if an apt package has updates available
func AptCheckOutdated(tool string) (current, latest string, err error) {
	// Try to get current version from manifest first
	if sourceStr := manifest.Get(tool); sourceStr != "" {
		current = manifest.GetSourceVersion(sourceStr)
	}

	var output bytes.Buffer
	cmd := exec.Command("apt", "list", "--upgradable")
	cmd.Stdout = &output
	cmd.Stderr = nil

	if cmd.Run() != nil {
		return "", "", fmt.Errorf("failed to check for updates")
	}

	// Parse output for this specific package
	lines := strings.Split(output.String(), "\n")
	for _, line := range lines {
		if strings.HasPrefix(line, tool+"/") {
			// Example: "tool/stable 2.0.0 amd64 [upgradable from: 1.0.0]"
			fields := strings.Fields(line)
			if len(fields) >= 2 {
				latest = fields[1]
			}
			// Extract current from "upgradable from: X.X.X]"
			if idx := strings.Index(line, UpgradablePrefix); idx != -1 {
				rest := line[idx+UpgradablePrefixLen:]
				if endIdx := strings.Index(rest, "]"); endIdx != -1 {
					current = strings.TrimSpace(rest[:endIdx])
				}
			}

			// If we found both versions and they differ, return them
			if current != "" && latest != "" && current != latest {
				return current, latest, nil
			}
		}
	}

	return "", "", fmt.Errorf("no updates available")
}

// AptSearch searches for packages in apt repositories
func AptSearch(query string) ([]string, error) {
	var output bytes.Buffer
	cmd := exec.Command("apt", "search", query)
	cmd.Stdout = &output
	cmd.Stderr = nil

	if err := cmd.Run(); err != nil {
		return nil, err
	}

	results := []string{}
	lines := strings.Split(output.String(), "\n")

	for i := 0; i < len(lines); i++ {
		line := strings.TrimSpace(lines[i])

		// Skip header lines
		if strings.HasPrefix(line, "Sorting") || strings.HasPrefix(line, "Full Text") || line == "" {
			continue
		}

		// Package lines start without whitespace
		if len(line) > 0 && line[0] != ' ' {
			// Parse: "package/stable 1.0.0 arch [installed]"
			slashIdx := strings.Index(line, "/")
			if slashIdx == -1 {
				continue
			}

			name := line[:slashIdx]
			rest := line[slashIdx+1:]
			parts := strings.Fields(rest)

			version := ""
			if len(parts) >= 2 {
				version = parts[1]
				// Strip Debian metadata
				version = stripDebianVersion(version)
			}

			// Get description from next line
			desc := ""
			if i+1 < len(lines) {
				nextLine := strings.TrimSpace(lines[i+1])
				if len(nextLine) > 0 && (nextLine[0] == ' ' || nextLine[0] == '\t') {
					desc = strings.TrimSpace(nextLine)
					i++ // Skip next line since we consumed it
				}
			}

			if name != "" && version != "" {
				result := fmt.Sprintf("%s %s - %s", name, version, desc)
				results = append(results, result)
				if len(results) >= MaxSearchResults {
					break
				}
			}
		}
	}

	return results, nil
}

// stripDebianVersion removes Debian-specific version metadata
func stripDebianVersion(version string) string {
	// Remove epoch (1:)
	if idx := strings.Index(version, ":"); idx != -1 {
		version = version[idx+1:]
	}
	// Remove +dfsg suffix
	version = regexp.MustCompile(`\+dfsg\d*`).ReplaceAllString(version, "")
	// Remove revision (-8build1)
	version = regexp.MustCompile(`-\d+.*$`).ReplaceAllString(version, "")
	return version
}

// AptScan scans manually-installed Debian/Ubuntu packages
func AptScan() ([]Package, error) {
	// Get list of manually installed packages
	var output bytes.Buffer
	cmd := exec.Command("apt-mark", "showmanual")
	cmd.Stdout = &output

	if err := cmd.Run(); err != nil {
		return nil, nil
	}

	manualPackages := make(map[string]bool)
	for _, pkg := range strings.Split(output.String(), "\n") {
		pkg = strings.TrimSpace(pkg)
		if pkg != "" {
			manualPackages[pkg] = true
		}
	}

	if len(manualPackages) == 0 {
		return nil, nil
	}

	var packages []Package
	binDirs := []string{"/usr/bin", "/usr/local/bin", "/bin"}

	for _, binDir := range binDirs {
		entries, err := os.ReadDir(binDir)
		if err != nil {
			continue
		}

		for _, entry := range entries {
			if entry.IsDir() {
				continue
			}

			binPath := binDir + "/" + entry.Name()
			info, err := entry.Info()
			if err != nil || info.Mode()&0111 == 0 {
				continue
			}

			// Find which package owns this binary
			var pkgOutput bytes.Buffer
			pkgCmd := exec.Command("dpkg", "-S", binPath)
			pkgCmd.Stdout = &pkgOutput
			pkgCmd.Stderr = nil

			if pkgCmd.Run() != nil {
				continue
			}

			// Parse output: "package: /path/to/binary"
			pkgLine := strings.TrimSpace(pkgOutput.String())
			parts := strings.SplitN(pkgLine, ":", 2)
			if len(parts) < 1 {
				continue
			}

			pkgName := strings.TrimSpace(parts[0])
			if manualPackages[pkgName] {
				version := AptGetVersion(entry.Name())
				packages = append(packages, Package{
					Name:     entry.Name(),
					Source:   "apt",
					Identity: "",
					Version:  version,
				})
			}
		}
	}

	return packages, nil
}
