package sources

import (
	"fmt"
	"os"

	"github.com/DeprecatedLuar/sat/internal/common"
)

const (
	// Search result limits
	MaxSearchResults = 30

	// Package manager names (used for routing)
	PkgMgrAPT     = "apt"
	PkgMgrPacman  = "pacman"
	PkgMgrAPK     = "apk"
	PkgMgrDNF     = "dnf"
	PkgMgrNix     = "nix"
	PkgMgrUnknown = "unknown"
)

// Install installs a package via the system package manager
func Install(tool string) error {
	mgr := common.GetPkgManager()
	if mgr == "" || mgr == PkgMgrUnknown {
		return fmt.Errorf("no system package manager detected")
	}

	switch mgr {
	case PkgMgrPacman:
		return pacmanInstall(tool)
	case PkgMgrAPT:
		return aptInstall(tool)
	case PkgMgrDNF:
		return dnfInstall(tool)
	case PkgMgrAPK:
		return apkInstall(tool)
	default:
		return fmt.Errorf("unsupported package manager: %s", mgr)
	}
}

// Uninstall removes a system package
func Uninstall(pkg, source string) error {
	mgr := common.GetPkgManager()
	if mgr == "" || mgr == PkgMgrUnknown {
		return fmt.Errorf("no system package manager detected")
	}

	switch mgr {
	case PkgMgrPacman:
		return pacmanUninstall(pkg)
	case PkgMgrAPT:
		return aptUninstall(pkg)
	case PkgMgrDNF:
		return dnfUninstall(pkg)
	case PkgMgrAPK:
		return apkUninstall(pkg)
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
	case PkgMgrPacman:
		return pacmanUpdate(tool)
	case PkgMgrAPT:
		return aptUpdate(tool)
	case PkgMgrDNF:
		return dnfUpdate(tool)
	case PkgMgrAPK:
		return apkUpdate(tool)
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

	switch mgr {
	case PkgMgrPacman:
		return pacmanGetVersion(tool)
	case PkgMgrAPT:
		return aptGetVersion(tool)
	case PkgMgrDNF:
		return dnfGetVersion(tool)
	case PkgMgrAPK:
		return apkGetVersion(tool)
	default:
		return ""
	}
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

	// Only apt has reliable per-package outdated checks without sudo
	if mgr == PkgMgrAPT {
		return aptCheckOutdated(tool)
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
	case PkgMgrPacman:
		return pacmanSearch(query)
	case PkgMgrAPT:
		return aptSearch(query)
	case PkgMgrDNF:
		return dnfSearch(query)
	case PkgMgrAPK:
		return apkSearch(query)
	default:
		return nil, fmt.Errorf("search not supported for %s", mgr)
	}
}

// SystemScan scans for explicitly-installed system packages
func SystemScan() ([]Package, error) {
	mgr := common.GetPkgManager()
	if mgr == "" || mgr == PkgMgrUnknown {
		return nil, nil
	}

	switch mgr {
	case PkgMgrPacman:
		return pacmanScan()
	case PkgMgrAPT:
		return aptScan()
	case PkgMgrDNF:
		return dnfScan()
	case PkgMgrAPK:
		return apkScan()
	case PkgMgrNix:
		// NixOS system packages (declarative, read-only)
		return nixOSScan()
	default:
		return nil, nil
	}
}

// Debug helper
func init() {
	if os.Getenv(common.EnvSATDebug) != "" {
		fmt.Fprintf(os.Stderr, "%s system source module loaded\n", common.DebugPrefix)
	}
}
