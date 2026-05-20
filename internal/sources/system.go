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
	// Search result limits
	MaxSearchResults = 30

	// Debug output
	DebugPrefix = "[debug]"
	EnvSATDebug = "SAT_DEBUG"

	// Package manager names
	PkgMgrAPT    = "apt"
	PkgMgrPacman = "pacman"
	PkgMgrAPK    = "apk"
	PkgMgrDNF    = "dnf"
	PkgMgrUnknown = "unknown"

	// Version parsing constants
	UpgradablePrefix = "upgradable from: "
	UpgradablePrefixLen = 17
)

// Install installs a package via the system package manager
func Install(tool string) error {
	mgr := common.GetPkgManager()
	if mgr == "" || mgr == PkgMgrUnknown {
		return fmt.Errorf("no system package manager detected")
	}

	// Check if package exists in repo
	if !pkgExistsInRepo(tool, mgr) {
		return fmt.Errorf("package %s not found in %s repositories", tool, mgr)
	}

	// Install using common helper
	if err := common.PkgInstall(tool, mgr); err != nil {
		return fmt.Errorf("failed to install %s via %s: %w", tool, mgr, err)
	}

	return nil
}

// pkgExistsInRepo checks if package exists in repository (not just installed)
func pkgExistsInRepo(pkg, mgr string) bool {
	var cmd *exec.Cmd

	switch mgr {
	case PkgMgrAPT:
		cmd = exec.Command("apt-cache", "show", pkg)
	case PkgMgrPacman:
		cmd = exec.Command("pacman", "-Si", pkg)
	case PkgMgrAPK:
		// apk search returns packages matching pattern
		cmd = exec.Command("apk", "search", "-e", pkg)
	case PkgMgrDNF:
		cmd = exec.Command("dnf", "info", pkg)
	default:
		return false
	}

	// Suppress output
	cmd.Stdout = nil
	cmd.Stderr = nil
	return cmd.Run() == nil
}

// Uninstall removes a system package
func Uninstall(pkg, source string) error {
	mgr := common.GetPkgManager()
	if mgr == "" || mgr == PkgMgrUnknown {
		return fmt.Errorf("no system package manager detected")
	}

	switch mgr {
	case PkgMgrAPT:
		return common.RunQuiet("sudo", "apt", "remove", "--purge", "-y", pkg)
	case PkgMgrPacman:
		return common.RunQuiet("sudo", "pacman", "-Rs", "--noconfirm", pkg)
	case PkgMgrAPK:
		return common.RunQuiet("sudo", "apk", "del", pkg)
	case PkgMgrDNF:
		return common.RunQuiet("sudo", "dnf", "remove", "-y", pkg)
	default:
		return fmt.Errorf("unsupported package manager: %s", mgr)
	}
}

// Update updates a system package
func Update(tool string) error {
	mgr := common.GetPkgManager()
	if mgr == "" || mgr == PkgMgrUnknown {
		return fmt.Errorf("no system package manager detected")
	}

	switch mgr {
	case PkgMgrAPT:
		return common.RunQuiet("sudo", "apt-get", "install", "--only-upgrade", "-y", tool)
	case PkgMgrPacman:
		return common.RunQuiet("sudo", "pacman", "-S", "--noconfirm", tool)
	case PkgMgrAPK:
		return common.RunQuiet("sudo", "apk", "upgrade", tool)
	case PkgMgrDNF:
		return common.RunQuiet("sudo", "dnf", "upgrade", "-y", tool)
	default:
		return fmt.Errorf("unsupported package manager: %s", mgr)
	}
}

// GetVersion returns the installed version of a system package
func GetVersion(tool string) string {
	mgr := common.GetPkgManager()
	if mgr == "" {
		return ""
	}

	var cmd *exec.Cmd
	var output bytes.Buffer

	switch mgr {
	case PkgMgrAPT:
		cmd = exec.Command("dpkg", "-l", tool)
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

	case PkgMgrPacman:
		cmd = exec.Command("pacman", "-Q", tool)
		cmd.Stdout = &output
		cmd.Stderr = nil
		if cmd.Run() != nil {
			return ""
		}
		// Output: "tool version"
		fields := strings.Fields(strings.TrimSpace(output.String()))
		if len(fields) >= 2 {
			return fields[1]
		}

	case PkgMgrAPK:
		cmd = exec.Command("apk", "info", "-e", tool)
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

	case PkgMgrDNF:
		cmd = exec.Command("dnf", "list", "installed", tool)
		cmd.Stdout = &output
		cmd.Stderr = nil
		if cmd.Run() != nil {
			return ""
		}
		// Parse dnf output
		lines := strings.Split(output.String(), "\n")
		for _, line := range lines {
			if strings.HasPrefix(line, tool) {
				fields := strings.Fields(line)
				if len(fields) >= 2 {
					return fields[1]
				}
			}
		}
	}

	return ""
}

// QueryLatestVersion queries the latest available version (not implemented for system packages)
// System packages don't have a simple "latest version" query without updating repos
func QueryLatestVersion(tool string) string {
	return "" // Not implemented for system packages
}

// CheckOutdated checks if a system package has updates available
// Returns current version, latest version, and error
func CheckOutdated(tool string) (current, latest string, err error) {
	mgr := common.GetPkgManager()
	if mgr == "" {
		return "", "", fmt.Errorf("no package manager detected")
	}

	// Try to get current version from manifest first
	if sourceStr := manifest.Get(tool); sourceStr != "" {
		current = manifest.GetSourceVersion(sourceStr)
	}

	// Only apt has reliable per-package outdated checks without sudo
	if mgr == PkgMgrAPT {
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
	}

	// For other package managers, return error (no reliable check without sudo)
	return "", "", fmt.Errorf("outdated check not supported for %s", mgr)
}

// Search searches for packages in the system repository
func Search(query string) ([]string, error) {
	mgr := common.GetPkgManager()
	if mgr == "" {
		return nil, fmt.Errorf("no package manager detected")
	}

	switch mgr {
	case PkgMgrAPT:
		return searchApt(query)
	case PkgMgrPacman:
		return searchPacman(query)
	case PkgMgrAPK:
		return searchApk(query)
	case PkgMgrDNF:
		return searchDnf(query)
	default:
		return nil, fmt.Errorf("search not supported for %s", mgr)
	}
}

func searchApt(query string) ([]string, error) {
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

func searchPacman(query string) ([]string, error) {
	var output bytes.Buffer
	cmd := exec.Command("pacman", "-Ss", query)
	cmd.Stdout = &output
	cmd.Stderr = nil

	if err := cmd.Run(); err != nil {
		return nil, err
	}

	results := []string{}
	lines := strings.Split(output.String(), "\n")

	for i := 0; i < len(lines); i++ {
		line := strings.TrimSpace(lines[i])
		if line == "" {
			continue
		}

		// Package lines start without leading whitespace
		if len(line) > 0 && line[0] != ' ' {
			// Parse: "repo/package version"
			parts := strings.Fields(line)
			if len(parts) < 2 {
				continue
			}

			// Split "repo/package"
			slashParts := strings.Split(parts[0], "/")
			if len(slashParts) < 2 {
				continue
			}

			name := slashParts[1]
			version := parts[1]

			// Get description from next line
			desc := ""
			if i+1 < len(lines) {
				nextLine := lines[i+1]
				if len(nextLine) > 0 && (nextLine[0] == ' ' || nextLine[0] == '\t') {
					desc = strings.TrimSpace(nextLine)
					i++ // Skip next line
				}
			}

			if name != "" && version != "" && desc != "" {
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

func searchApk(query string) ([]string, error) {
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

func searchDnf(query string) ([]string, error) {
	var output bytes.Buffer
	cmd := exec.Command("dnf", "search", query)
	cmd.Stdout = &output
	cmd.Stderr = nil

	if err := cmd.Run(); err != nil {
		return nil, err
	}

	results := []string{}
	lines := strings.Split(output.String(), "\n")

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "=") || strings.HasPrefix(line, "Last metadata") {
			continue
		}

		// Parse: "package.arch : description"
		if strings.Contains(line, " : ") {
			parts := strings.SplitN(line, " : ", 2)
			if len(parts) != 2 {
				continue
			}

			// Extract package name (strip arch)
			pkgWithArch := strings.TrimSpace(parts[0])
			dotParts := strings.Split(pkgWithArch, ".")
			name := pkgWithArch
			if len(dotParts) > 1 {
				// Keep all but last part (which is arch)
				name = strings.Join(dotParts[:len(dotParts)-1], ".")
			}

			desc := strings.TrimSpace(parts[1])
			result := fmt.Sprintf("%s (version varies) - %s", name, desc)
			results = append(results, result)
			if len(results) >= MaxSearchResults {
				break
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

// Debug helper
func init() {
	if os.Getenv(EnvSATDebug) != "" {
		fmt.Fprintf(os.Stderr, "%s system source module loaded\n", DebugPrefix)
	}
}
