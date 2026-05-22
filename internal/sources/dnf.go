package sources

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/DeprecatedLuar/sat/internal/common"
)

// DnfInstall installs a package via dnf
func DnfInstall(tool string) error {
	if !DnfPkgExists(tool) {
		return fmt.Errorf("package %s not found in dnf repositories", tool)
	}
	return common.PkgInstall(tool, "dnf")
}

// DnfUninstall removes a dnf package
func DnfUninstall(pkg string) error {
	return common.RunQuiet("sudo", "dnf", "remove", "-y", pkg)
}

// DnfUpdate updates a dnf package
func DnfUpdate(tool string) error {
	return common.RunQuiet("sudo", "dnf", "upgrade", "-y", tool)
}

// DnfGetVersion returns the installed version of a dnf package
func DnfGetVersion(tool string) string {
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

// DnfPkgExists checks if package exists in dnf repositories
func DnfPkgExists(pkg string) bool {
	cmd := exec.Command("dnf", "info", pkg)
	cmd.Stdout = nil
	cmd.Stderr = nil
	return cmd.Run() == nil
}

// DnfSearch searches for packages in dnf repositories
func DnfSearch(query string) ([]string, error) {
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

// DnfScan scans user-installed Fedora/RHEL packages
func DnfScan() ([]Package, error) {
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

	var packages []Package
	binDirs := []string{"/usr/bin", "/usr/local/bin"}

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
			pkgCmd := exec.Command("rpm", "-qf", binPath)
			pkgCmd.Stdout = &pkgOutput
			pkgCmd.Stderr = nil

			if pkgCmd.Run() != nil {
				continue
			}

			// Parse output and strip version
			pkgLine := strings.TrimSpace(pkgOutput.String())
			pkgName := strings.Split(pkgLine, "-")[0]

			if userPackages[pkgName] {
				version := DnfGetVersion(entry.Name())
				packages = append(packages, Package{
					Name:     entry.Name(),
					Source:   "dnf",
					Identity: "",
					Version:  version,
				})
			}
		}
	}

	return packages, nil
}
