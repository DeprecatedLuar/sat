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
		return PacmanInstall(tool)
	case PkgMgrAPT:
		return AptInstall(tool)
	case PkgMgrDNF:
		return DnfInstall(tool)
	case PkgMgrAPK:
		return ApkInstall(tool)
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
		return PacmanUninstall(pkg)
	case PkgMgrAPT:
		return AptUninstall(pkg)
	case PkgMgrDNF:
		return DnfUninstall(pkg)
	case PkgMgrAPK:
		return ApkUninstall(pkg)
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
		return PacmanUpdate(tool)
	case PkgMgrAPT:
		return AptUpdate(tool)
	case PkgMgrDNF:
		return DnfUpdate(tool)
	case PkgMgrAPK:
		return ApkUpdate(tool)
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
		return PacmanGetVersion(tool)
	case PkgMgrAPT:
		return AptGetVersion(tool)
	case PkgMgrDNF:
		return DnfGetVersion(tool)
	case PkgMgrAPK:
		return ApkGetVersion(tool)
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
		return AptCheckOutdated(tool)
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
		return PacmanSearch(query)
	case PkgMgrAPT:
		return AptSearch(query)
	case PkgMgrDNF:
		return DnfSearch(query)
	case PkgMgrAPK:
		return ApkSearch(query)
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
		return PacmanScan()
	case PkgMgrAPT:
		return AptScan()
	case PkgMgrDNF:
		return DnfScan()
	case PkgMgrAPK:
		return ApkScan()
	case PkgMgrNix:
		// NixOS system packages (declarative, read-only)
		return NixOSScan()
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
