package sources

import (
	"bytes"
	"os"
	"os/exec"
	"strings"
)

// nixOSScan scans NixOS system packages from declarative configuration
func nixOSScan() ([]Package, error) {
	if _, err := exec.LookPath("nixos-option"); err != nil {
		return nil, nil
	}

	// Query system packages
	var output bytes.Buffer
	cmd := exec.Command("nixos-option", "environment.systemPackages")
	cmd.Stdout = &output
	cmd.Stderr = nil

	if err := cmd.Run(); err != nil {
		return nil, nil
	}

	// Parse output for derivation names
	userPackages := make(map[string]bool)
	for _, line := range strings.Split(output.String(), "\n") {
		// Look for <derivation package-name-version>
		if strings.Contains(line, "<derivation ") {
			start := strings.Index(line, "<derivation ") + 12
			end := strings.Index(line[start:], ">")
			if end != -1 {
				deriv := line[start : start+end]
				// Strip version: package-name-1.2.3 → package-name
				pkg := strings.Split(deriv, "-")[0]
				userPackages[pkg] = true
			}
		}
	}

	if len(userPackages) == 0 {
		return nil, nil
	}

	var packages []Package

	entries, err := os.ReadDir("/run/current-system/sw/bin")
	if err != nil {
		return nil, nil
	}

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		info, err := entry.Info()
		if err != nil || !isExecutable(info) {
			continue
		}

		// Only add if binary name matches a user-installed package
		if userPackages[entry.Name()] {
			version := NixOSGetVersion(entry.Name())
			packages = append(packages, Package{
				Name:     entry.Name(),
				Source:   "nixos",
				Identity: "",
				Version:  version,
			})
		}
	}

	return packages, nil
}
