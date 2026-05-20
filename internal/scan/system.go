package scan

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/DeprecatedLuar/sat/internal/common"
)

const (
	// Environment variables
	EnvSATDebug = "SAT_DEBUG"

	// Debug output
	DebugPrefix = "[debug]"
	DebugMessage = "system scanner loaded"

	// Package manager names
	PkgMgrPacman  = "pacman"
	PkgMgrAPT     = "apt"
	PkgMgrDNF     = "dnf"
	PkgMgrAPK     = "apk"
	PkgMgrNix     = "nix"
	PkgMgrUnknown = "unknown"

	// System paths
	OSReleaseFile    = "/etc/os-release"
	APKWorldFile     = "/etc/apk/world"
	NixOSSystemBin   = "/run/current-system/sw/bin"

	// Binary directories
	UsrBin       = "/usr/bin"
	UsrLocalBin  = "/usr/local/bin"
	Bin          = "/bin"

	// Path patterns
	UsrBinPattern       = "/usr/bin/"
	UsrLocalBinPattern  = "/usr/local/bin/"
	BinSlashPattern     = "/bin/"

	// File format constants
	FieldSeparator = ":"
	MinFieldCount  = 2

	// OS release patterns
	NixOSID = "ID=nixos"

	// Pacman command
	PacmanQueryCmd = "pacman -Qeq 2>/dev/null | xargs pacman -Ql 2>/dev/null"

	// Derivation parsing
	DerivationPrefix = "<derivation "
	DerivationPrefixLen = 12
	DerivationSuffix = ">"

	// Version stripping
	VersionSep = "-"
	VersionSepIndex = 0

	// Executable permission mask
	ExecutableMask = 0111
)

var (
	// Binary directories for scanning
	aptBinDirs = []string{UsrBin, UsrLocalBin, Bin}
	dnfBinDirs = []string{UsrBin, UsrLocalBin}
	apkBinDirs = []string{UsrBin, UsrLocalBin, Bin}
)

// FoundPackage represents a discovered system package
type FoundPackage struct {
	Name   string
	Source string
}

// ScanSystemPackages scans for explicitly-installed system packages
// Returns a list of found packages
func ScanSystemPackages() ([]FoundPackage, error) {
	mgr := common.GetPkgManager()
	if mgr == "" || mgr == PkgMgrUnknown {
		return nil, nil
	}

	var packages []FoundPackage
	var err error

	switch mgr {
	case PkgMgrPacman:
		packages, err = scanPacman()
	case PkgMgrAPT:
		packages, err = scanApt()
	case PkgMgrDNF:
		packages, err = scanDNF()
	case PkgMgrAPK:
		packages, err = scanAPK()
	case PkgMgrNix:
		// NixOS uses nixos-option
		if isNixOS() {
			packages, err = scanNixOS()
		}
	}

	if err != nil {
		return nil, err
	}

	return packages, nil
}

// isNixOS checks if running on NixOS
func isNixOS() bool {
	data, err := os.ReadFile(OSReleaseFile)
	if err != nil {
		return false
	}
	return strings.Contains(string(data), NixOSID)
}

// scanPacman scans explicitly-installed Arch packages
func scanPacman() ([]FoundPackage, error) {
	if _, err := exec.LookPath(PkgMgrPacman); err != nil {
		return nil, nil
	}

	// Optimized: query all explicit packages at once, then filter for /usr/bin binaries
	var output bytes.Buffer
	cmd := exec.Command("bash", "-c", PacmanQueryCmd)
	cmd.Stdout = &output

	if err := cmd.Run(); err != nil {
		return nil, nil // Not a fatal error
	}

	packages := []FoundPackage{}
	lines := strings.Split(output.String(), "\n")

	for _, line := range lines {
		fields := strings.Fields(line)
		if len(fields) < MinFieldCount {
			continue
		}

		binPath := fields[1]

		// Filter for /usr/bin or /usr/local/bin binaries
		if !strings.Contains(binPath, UsrBinPattern) && !strings.Contains(binPath, UsrLocalBinPattern) {
			continue
		}
		// Skip if it's a subdirectory (not a direct binary)
		if strings.Count(binPath[strings.LastIndex(binPath, BinSlashPattern)+5:], "/") > 0 {
			continue
		}

		// Check if executable
		info, err := os.Stat(binPath)
		if err != nil || !isExecutable(info) {
			continue
		}

		prog := filepath.Base(binPath)
		packages = append(packages, FoundPackage{Name: prog, Source: PkgMgrPacman})
	}

	return packages, nil
}

// scanApt scans manually-installed Debian/Ubuntu packages
func scanApt() ([]FoundPackage, error) {
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

	packages := []FoundPackage{}

	for _, binDir := range aptBinDirs {
		entries, err := os.ReadDir(binDir)
		if err != nil {
			continue
		}

		for _, entry := range entries {
			if entry.IsDir() {
				continue
			}

			binPath := filepath.Join(binDir, entry.Name())
			info, err := entry.Info()
			if err != nil || !isExecutable(info) {
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
				packages = append(packages, FoundPackage{Name: entry.Name(), Source: PkgMgrAPT})
			}
		}
	}

	return packages, nil
}

// scanDNF scans user-installed Fedora/RHEL packages
func scanDNF() ([]FoundPackage, error) {
	// Get user-installed packages
	var output bytes.Buffer
	cmd := exec.Command(PkgMgrDNF, "repoquery", "--userinstalled")
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
		pkg := strings.Split(line, VersionSep)[VersionSepIndex]
		userPackages[pkg] = true
	}

	if len(userPackages) == 0 {
		return nil, nil
	}

	packages := []FoundPackage{}

	for _, binDir := range dnfBinDirs {
		entries, err := os.ReadDir(binDir)
		if err != nil {
			continue
		}

		for _, entry := range entries {
			if entry.IsDir() {
				continue
			}

			binPath := filepath.Join(binDir, entry.Name())
			info, err := entry.Info()
			if err != nil || !isExecutable(info) {
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
			pkgName := strings.Split(pkgLine, VersionSep)[VersionSepIndex]

			if userPackages[pkgName] {
				packages = append(packages, FoundPackage{Name: entry.Name(), Source: PkgMgrDNF})
			}
		}
	}

	return packages, nil
}

// scanAPK scans explicitly-installed Alpine packages
func scanAPK() ([]FoundPackage, error) {
	// Read /etc/apk/world for explicitly-installed packages
	data, err := os.ReadFile(APKWorldFile)
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

	packages := []FoundPackage{}

	for _, binDir := range apkBinDirs {
		entries, err := os.ReadDir(binDir)
		if err != nil {
			continue
		}

		for _, entry := range entries {
			if entry.IsDir() {
				continue
			}

			binPath := filepath.Join(binDir, entry.Name())
			info, err := entry.Info()
			if err != nil || !isExecutable(info) {
				continue
			}

			// Find which package owns this binary
			var pkgOutput bytes.Buffer
			pkgCmd := exec.Command("apk", "info", "--who-owns", binPath)
			pkgCmd.Stdout = &pkgOutput
			pkgCmd.Stderr = nil

			if pkgCmd.Run() != nil {
				continue
			}

			// Parse output and strip version
			output := strings.TrimSpace(pkgOutput.String())
			fields := strings.Fields(output)
			if len(fields) == 0 {
				continue
			}

			pkgName := fields[len(fields)-1]
			// Strip version: package-1.2.3 → package
			pkgName = strings.Split(pkgName, VersionSep)[VersionSepIndex]

			if worldPackages[pkgName] {
				packages = append(packages, FoundPackage{Name: entry.Name(), Source: PkgMgrAPK})
			}
		}
	}

	return packages, nil
}

// scanNixOS scans NixOS system packages from configuration
func scanNixOS() ([]FoundPackage, error) {
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
		if strings.Contains(line, DerivationPrefix) {
			start := strings.Index(line, DerivationPrefix) + DerivationPrefixLen
			end := strings.Index(line[start:], DerivationSuffix)
			if end != -1 {
				deriv := line[start : start+end]
				// Strip version: package-name-1.2.3 → package-name
				pkg := strings.Split(deriv, VersionSep)[VersionSepIndex]
				userPackages[pkg] = true
			}
		}
	}

	if len(userPackages) == 0 {
		return nil, nil
	}

	packages := []FoundPackage{}

	entries, err := os.ReadDir(NixOSSystemBin)
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
			packages = append(packages, FoundPackage{Name: entry.Name(), Source: "nixos"})
		}
	}

	return packages, nil
}

// isExecutable checks if a file has executable permissions
func isExecutable(info os.FileInfo) bool {
	return info.Mode()&ExecutableMask != 0
}

// Debug helper
func init() {
	if os.Getenv(EnvSATDebug) != "" {
		fmt.Fprintf(os.Stderr, "%s %s\n", DebugPrefix, DebugMessage)
	}
}
