package bootstrap

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/DeprecatedLuar/sat/internal/common"
)

// nixInstallerURL is the NixOS-org-adopted modern installer (a fork of
// Determinate Systems' nix-installer). Unlike the legacy nixos.org/nix/install
// script, it keeps an install receipt (nixReceiptPath) and a copy of itself
// (nixInstallerBinPath), enabling a genuine, safe, one-command uninstall
// instead of manual `rm -rf /nix` directory surgery.
const nixInstallerURL = "https://artifacts.nixos.org/nix-installer"

// nixInstallerBinPath and nixReceiptPath are where the modern installer
// keeps itself and its install receipt, both required for a safe uninstall.
const (
	nixInstallerBinPath = "/nix/nix-installer"
	nixReceiptPath      = "/nix/receipt.json"
)

// nixosMarkerFile indicates a declarative NixOS system, where Nix is
// already present as the OS's own package manager and must never be
// installed or removed by sat's bootstrap.
const nixosMarkerFile = "/etc/NIXOS"

// NixInstall installs Nix via the modern nix-installer (multi-user/daemon
// by default), which places the binary under ~/.nix-profile/bin. Alpine
// needs GNU coreutils first, since BusyBox's cp lacks options the
// installer relies on.
func NixInstall() (installedPath string, err error) {
	if err := common.EnsureOSInfo(); err == nil && common.GetDistroFamily() == common.DistroFamilyAlpine {
		if err := common.PkgInstall("coreutils", common.PkgManagerAPK); err != nil {
			return "", fmt.Errorf("failed to install coreutils (required by the Nix installer on Alpine): %w", err)
		}
	}

	script := "curl -sSfL " + nixInstallerURL + " | sh -s -- install --no-confirm"
	if err := common.RunQuiet("bash", "-c", script); err != nil {
		return "", fmt.Errorf("Nix install failed: %w", err)
	}

	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("could not determine home directory: %w", err)
	}

	nixPath := filepath.Join(home, ".nix-profile", "bin", "nix")
	if _, err := os.Stat(nixPath); err != nil {
		return "", fmt.Errorf("Nix installer ran but %s was not found", nixPath)
	}

	return nixPath, nil
}

// NixRemove removes a sat-bootstrapped Nix via the modern nix-installer's
// own official uninstaller. Refuses outright on a declarative NixOS system
// (defense-in-depth alongside NixInstall's own guard) and unless the
// installer's receipt confirms Nix was actually installed by it, since
// /nix is a shared, top-level directory that must never be touched
// otherwise.
func NixRemove() error {
	if _, err := os.Stat(nixosMarkerFile); err == nil {
		return fmt.Errorf("nix is managed declaratively on NixOS; edit your NixOS configuration instead")
	}

	if _, err := os.Stat(nixInstallerBinPath); err != nil {
		return fmt.Errorf("%s not found; nix was not installed via sat's modern nix-installer bootstrap", nixInstallerBinPath)
	}
	if _, err := os.Stat(nixReceiptPath); err != nil {
		return fmt.Errorf("%s not found; nix was not installed via sat's modern nix-installer bootstrap", nixReceiptPath)
	}

	if err := common.RunQuiet(nixInstallerBinPath, "uninstall", "--no-confirm"); err != nil {
		return fmt.Errorf("nix-installer uninstall failed: %w", err)
	}

	return nil
}
