package common

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/DeprecatedLuar/sat/internal/manifest"
)

const (
	// OS detection cache file
	OSCacheFile = "os-info"

	// OS release file
	OSReleaseFile = "/etc/os-release"

	// File permissions
	DirPermissions = 0755

	// OS release variable names
	OSReleaseID     = "ID="
	OSReleaseIDLike = "ID_LIKE="

	// Cache variable names
	CacheVarOSType       = "SAT_OS="
	CacheVarDistro       = "SAT_DISTRO="
	CacheVarDistroFamily = "SAT_DISTRO_FAMILY="
	CacheVarPkgManager   = "SAT_PKG_MANAGER="

	// OS types
	OSTypeLinux = "linux"

	// Package managers
	PkgManagerPacman  = "pacman"
	PkgManagerAPT     = "apt"
	PkgManagerDNF     = "dnf"
	PkgManagerAPK     = "apk"
	PkgManagerNix     = "nix"
	PkgManagerUnknown = "unknown"

	// Distro families
	DistroFamilyArch   = "arch"
	DistroFamilyDebian = "debian"
	DistroFamilyUbuntu = "ubuntu"
	DistroFamilyFedora = "fedora"
	DistroFamilyRHEL   = "rhel"
	DistroFamilyAlpine = "alpine"
	DistroFamilyNixOS  = "nixos"
)

// OS info cache structure
var (
	osType       string
	distro       string
	distroFamily string
	pkgManager   string
)

// EnsureOSInfo loads or creates the OS detection cache
func EnsureOSInfo() error {
	cacheFile := filepath.Join(manifest.DataDir(), OSCacheFile)

	// Try to load from cache
	if err := loadOSCache(cacheFile); err == nil {
		return nil
	}

	// Cache doesn't exist, detect OS
	if err := detectOS(); err != nil {
		return err
	}

	// Write cache
	return writeOSCache(cacheFile)
}

// loadOSCache reads cached OS info
func loadOSCache(path string) error {
	file, err := os.Open(path)
	if err != nil {
		return err
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if strings.HasPrefix(line, CacheVarOSType) {
			osType = strings.TrimPrefix(line, CacheVarOSType)
		} else if strings.HasPrefix(line, CacheVarDistro) {
			distro = strings.TrimPrefix(line, CacheVarDistro)
		} else if strings.HasPrefix(line, CacheVarDistroFamily) {
			distroFamily = strings.TrimPrefix(line, CacheVarDistroFamily)
		} else if strings.HasPrefix(line, CacheVarPkgManager) {
			pkgManager = strings.TrimPrefix(line, CacheVarPkgManager)
		}
	}

	return scanner.Err()
}

// detectOS performs OS detection (simplified version)
func detectOS() error {
	osType = OSTypeLinux // Simplified - expand later if needed

	// Read /etc/os-release
	file, err := os.Open(OSReleaseFile)
	if err != nil {
		return fmt.Errorf("failed to read %s: %w", OSReleaseFile, err)
	}
	defer file.Close()

	var id, idLike string
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := scanner.Text()
		if strings.HasPrefix(line, OSReleaseID) {
			id = strings.Trim(strings.TrimPrefix(line, OSReleaseID), `"`)
		} else if strings.HasPrefix(line, OSReleaseIDLike) {
			idLike = strings.Trim(strings.TrimPrefix(line, OSReleaseIDLike), `"`)
		}
	}

	distro = id

	// Use ID_LIKE for family, fallback to ID
	if idLike != "" {
		distroFamily = strings.Fields(idLike)[0] // Take first word
	} else {
		distroFamily = id
	}

	// Determine package manager
	switch distroFamily {
	case DistroFamilyArch:
		pkgManager = PkgManagerPacman
	case DistroFamilyDebian, DistroFamilyUbuntu:
		pkgManager = PkgManagerAPT
	case DistroFamilyFedora, DistroFamilyRHEL:
		pkgManager = PkgManagerDNF
	case DistroFamilyAlpine:
		pkgManager = PkgManagerAPK
	case DistroFamilyNixOS:
		pkgManager = PkgManagerNix
	default:
		pkgManager = PkgManagerUnknown
	}

	return nil
}

// writeOSCache writes OS info to cache
func writeOSCache(path string) error {
	file, err := os.Create(path)
	if err != nil {
		return err
	}
	defer file.Close()

	fmt.Fprintf(file, "%s%s\n", CacheVarOSType, osType)
	fmt.Fprintf(file, "%s%s\n", CacheVarDistro, distro)
	fmt.Fprintf(file, "%s%s\n", CacheVarDistroFamily, distroFamily)
	fmt.Fprintf(file, "%s%s\n", CacheVarPkgManager, pkgManager)

	return nil
}

// PkgExists checks if a package exists in the given package manager
func PkgExists(pkg, mgr string) bool {
	var cmd *exec.Cmd

	switch mgr {
	case PkgManagerPacman:
		cmd = exec.Command("pacman", "-Qi", pkg)
	case PkgManagerAPT:
		cmd = exec.Command("dpkg", "-s", pkg)
	case PkgManagerDNF:
		cmd = exec.Command("rpm", "-q", pkg)
	case PkgManagerAPK:
		cmd = exec.Command("apk", "info", "-e", pkg)
	default:
		return false
	}

	return cmd.Run() == nil
}

// PkgInstall installs a package using the given package manager
func PkgInstall(pkg, mgr string) error {
	var cmd *exec.Cmd

	switch mgr {
	case PkgManagerPacman:
		cmd = exec.Command("sudo", "pacman", "-S", "--noconfirm", pkg)
	case PkgManagerAPT:
		cmd = exec.Command("sudo", "apt", "install", "-y", pkg)
	case PkgManagerDNF:
		cmd = exec.Command("sudo", "dnf", "install", "-y", pkg)
	case PkgManagerAPK:
		cmd = exec.Command("sudo", "apk", "add", pkg)
	default:
		return fmt.Errorf("unsupported package manager: %s", mgr)
	}

	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

// PkgRemove removes a package (stub - will be completed in Phase 11)
func PkgRemove(pkg, source string) error {
	return fmt.Errorf("PkgRemove not yet implemented (Phase 11)")
}

// GetPkgManager returns the detected package manager
func GetPkgManager() string {
	return pkgManager
}

// GetDistro returns the detected distro
func GetDistro() string {
	return distro
}

// GetDistroFamily returns the detected distro family
func GetDistroFamily() string {
	return distroFamily
}
