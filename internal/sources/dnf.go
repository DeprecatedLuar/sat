package sources

import (
	"bytes"
	"fmt"
	"os/exec"
	"strings"

	"github.com/DeprecatedLuar/sat/internal/common"
)

// dnfInstall installs a package via dnf
func dnfInstall(tool string) error {
	if !dnfPkgExists(tool) {
		return fmt.Errorf("package %s not found in dnf repositories", tool)
	}
	return common.PkgInstall(tool, "dnf")
}

// dnfUninstall removes a dnf package
func dnfUninstall(pkg string) error {
	return common.RunQuiet("sudo", "dnf", "remove", "-y", pkg)
}

// dnfUpdate updates a dnf package
func dnfUpdate(tool string) error {
	return common.RunQuiet("sudo", "dnf", "upgrade", "-y", tool)
}

// dnfGetVersion returns the installed version of a dnf package
func dnfGetVersion(tool string) string {
	var output bytes.Buffer
	cmd := exec.Command("dnf", "list", "installed", tool)
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
	return ""
}

// parseDnfListInstalled parses `dnf list installed` (no argument = every
// installed package) output into a map keyed by package name (the ".arch"
// suffix dnf reports alongside the name is stripped). Lines with fewer than
// 3 fields (e.g. the "Installed Packages" header) are not the
// Name.Arch/Version/Repo shape and are skipped.
func parseDnfListInstalled(output string) map[string]string {
	versions := make(map[string]string)
	for _, line := range strings.Split(output, "\n") {
		fields := strings.Fields(line)
		if len(fields) < 3 {
			continue
		}
		name := fields[0]
		if idx := strings.Index(name, "."); idx != -1 {
			name = name[:idx]
		}
		versions[name] = fields[1]
	}
	return versions
}

// dnfInstalledVersions resolves live versions for every given dnf-tracked
// tool in a single `dnf list installed` call.
func dnfInstalledVersions(tools []string) map[string]string {
	if _, err := exec.LookPath("dnf"); err != nil {
		return nil
	}
	var output bytes.Buffer
	cmd := exec.Command("dnf", "list", "installed")
	cmd.Stdout = &output
	if cmd.Run() != nil {
		return nil
	}
	all := parseDnfListInstalled(output.String())
	versions := make(map[string]string, len(tools))
	for _, tool := range tools {
		if v, ok := all[tool]; ok && v != "" {
			versions[tool] = v
		}
	}
	return versions
}

// dnfPkgExists checks if package exists in dnf repositories
func dnfPkgExists(pkg string) bool {
	cmd := exec.Command("dnf", "info", pkg)
	cmd.Stdout = nil
	cmd.Stderr = nil
	return cmd.Run() == nil
}

// dnfSearch searches for packages in dnf repositories
func dnfSearch(query string) ([]string, error) {
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

// dnfOwnerOf resolves the package owning a binary, via `rpm -qf`
func dnfOwnerOf(binPath string) string {
	var output bytes.Buffer
	cmd := exec.Command("rpm", "-qf", binPath)
	cmd.Stdout = &output
	cmd.Stderr = nil

	if cmd.Run() != nil {
		return ""
	}

	// Strip version: package-1.2.3-4.fc36 → package
	return strings.Split(strings.TrimSpace(output.String()), "-")[0]
}

// dnfScan scans user-installed Fedora/RHEL packages
func dnfScan() ([]Package, error) {
	// Get user-installed packages
	var output bytes.Buffer
	cmd := exec.Command("dnf", "repoquery", "--userinstalled")
	cmd.Stdout = &output

	if err := cmd.Run(); err != nil {
		return nil, nil
	}

	userPackages := make(map[string]bool)
	for _, line := range strings.Split(output.String(), "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		// Strip version: package-1.2.3-4.fc36 → package
		pkg := strings.Split(line, "-")[0]
		userPackages[pkg] = true
	}

	if len(userPackages) == 0 {
		return nil, nil
	}

	binDirs := []string{"/usr/bin", "/usr/local/bin"}
	return scanBinDirsOwnedBy(binDirs, PkgMgrDNF, userPackages, dnfOwnerOf, dnfGetVersion), nil
}
