package sources

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/DeprecatedLuar/sat/internal/common"
)

const (
	PacmanBinPath      = "/usr/bin"
	PacmanLocalBinPath = "/usr/local/bin"
)

// pacmanInstall installs a package via pacman
func pacmanInstall(tool string) error {
	if !pacmanPkgExists(tool) {
		return fmt.Errorf("package %s not found in pacman repositories", tool)
	}
	return common.PkgInstall(tool, "pacman")
}

// pacmanUninstall removes a pacman package
func pacmanUninstall(pkg string) error {
	return common.RunQuiet("sudo", "pacman", "-Rs", "--noconfirm", pkg)
}

// pacmanUpdate updates a pacman package
func pacmanUpdate(tool string) error {
	return common.RunQuiet("sudo", "pacman", "-S", "--noconfirm", tool)
}

// pacmanGetVersion returns the installed version of a pacman package
func pacmanGetVersion(tool string) string {
	var output bytes.Buffer
	cmd := exec.Command("pacman", "-Q", tool)
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
	return ""
}

// pacmanPkgExists checks if package exists in pacman repositories
func pacmanPkgExists(pkg string) bool {
	cmd := exec.Command("pacman", "-Si", pkg)
	cmd.Stdout = nil
	cmd.Stderr = nil
	return cmd.Run() == nil
}

// pacmanSearch searches for packages in pacman repositories
func pacmanSearch(query string) ([]string, error) {
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

// pacmanScan scans explicitly-installed Arch packages
func pacmanScan() ([]Package, error) {
	if _, err := exec.LookPath("pacman"); err != nil {
		return nil, nil
	}

	// Optimized: query all explicit packages at once, then filter for /usr/bin binaries
	var output bytes.Buffer
	cmd := exec.Command("bash", "-c", "pacman -Qeq 2>/dev/null | xargs pacman -Ql 2>/dev/null")
	cmd.Stdout = &output

	if err := cmd.Run(); err != nil {
		return nil, nil
	}

	var packages []Package
	lines := strings.Split(output.String(), "\n")

	for _, line := range lines {
		fields := strings.Fields(line)
		if len(fields) < 2 {
			continue
		}

		binPath := fields[1]

		// Filter for /usr/bin or /usr/local/bin binaries
		if !strings.Contains(binPath, "/usr/bin/") && !strings.Contains(binPath, "/usr/local/bin/") {
			continue
		}
		// Skip if it's a subdirectory (not a direct binary)
		if strings.Count(binPath[strings.LastIndex(binPath, "/bin/")+5:], "/") > 0 {
			continue
		}

		// Check if executable
		info, err := os.Stat(binPath)
		if err != nil || info.Mode()&0111 == 0 {
			continue
		}

		prog := binPath[strings.LastIndex(binPath, "/")+1:]
		version := pacmanGetVersion(prog)

		packages = append(packages, Package{
			Name:     prog,
			Source:   PkgMgrPacman,
			Identity: "",
			Version:  version,
		})
	}

	return packages, nil
}
