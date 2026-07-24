package common

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"

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

// OS info cache structure. Populated once via EnsureOSInfo (guarded by
// osInfoOnce), then only ever read — safe for concurrent readers (e.g. the
// goroutines search.go dispatches per source) without per-access locking.
var (
	osType       string
	distro       string
	distroFamily string
	pkgManager   string

	osInfoOnce sync.Once
	osInfoErr  error
)

// EnsureOSInfo loads or creates the OS detection cache. Safe to call from
// multiple goroutines; detection runs exactly once.
func EnsureOSInfo() error {
	osInfoOnce.Do(func() {
		osInfoErr = ensureOSInfo()
	})
	return osInfoErr
}

func ensureOSInfo() error {
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
		if v, ok := strings.CutPrefix(line, CacheVarOSType); ok {
			osType = v
		} else if v, ok := strings.CutPrefix(line, CacheVarDistro); ok {
			distro = v
		} else if v, ok := strings.CutPrefix(line, CacheVarDistroFamily); ok {
			distroFamily = v
		} else if v, ok := strings.CutPrefix(line, CacheVarPkgManager); ok {
			pkgManager = v
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
		if v, ok := strings.CutPrefix(line, OSReleaseID); ok {
			id = strings.Trim(v, `"`)
		} else if v, ok := strings.CutPrefix(line, OSReleaseIDLike); ok {
			idLike = strings.Trim(v, `"`)
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

// GetPkgManager returns the detected package manager
func GetPkgManager() string {
	return pkgManager
}

// GetDistro returns the detected distro
// GetDistroFamily returns the detected distro family
func GetDistroFamily() string {
	return distroFamily
}
