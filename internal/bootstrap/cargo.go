package bootstrap

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/DeprecatedLuar/sat/internal/common"
)

// rustupInstallScript is rustup's own official installer.
const rustupInstallScript = "https://sh.rustup.rs"

// CargoInstall installs Rust/Cargo via rustup, which places binaries in
// ~/.cargo/bin. Uses -y --no-modify-path to run non-interactively without
// rustup editing shell rc files on our behalf.
func CargoInstall() (installedPath string, err error) {
	script := "curl --proto '=https' --tlsv1.2 -sSf " + rustupInstallScript + " | sh -s -- -y --no-modify-path"
	if err := common.RunQuiet("bash", "-c", script); err != nil {
		return "", fmt.Errorf("rustup install failed: %w", err)
	}

	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("could not determine home directory: %w", err)
	}

	cargoPath := filepath.Join(home, ".cargo", "bin", "cargo")
	if _, err := os.Stat(cargoPath); err != nil {
		return "", fmt.Errorf("rustup ran but %s was not found", cargoPath)
	}

	return cargoPath, nil
}

// CargoRemove removes a sat-bootstrapped cargo via rustup's own official
// uninstaller, which cleanly removes ~/.rustup, ~/.cargo, and every
// installed toolchain (rustup#1072, the FAQ community guides). rustup has
// no knowledge of the copy sat relocated into common.SourcesBinDir(), so
// that orphaned file is removed separately afterward.
func CargoRemove() error {
	home, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("could not determine home directory: %w", err)
	}

	cargoPath := filepath.Join(home, ".cargo", "bin", "cargo")
	if !isSatManaged(cargoPath) {
		return fmt.Errorf("cargo at %s is not managed by sat; remove it manually", cargoPath)
	}

	rustupPath := filepath.Join(home, ".cargo", "bin", "rustup")
	if _, err := os.Stat(rustupPath); err != nil {
		if lookup, lookErr := exec.LookPath("rustup"); lookErr == nil {
			rustupPath = lookup
		} else {
			return fmt.Errorf("rustup not found; cannot cleanly uninstall cargo")
		}
	}

	if err := common.RunQuiet(rustupPath, "self", "uninstall", "-y"); err != nil {
		return fmt.Errorf("rustup self uninstall failed: %w", err)
	}

	relocatedCopy := filepath.Join(common.SourcesBinDir(), "cargo")
	if err := os.Remove(relocatedCopy); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("failed to remove %s: %w", relocatedCopy, err)
	}

	return nil
}
