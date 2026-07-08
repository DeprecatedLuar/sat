package bootstrap

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/DeprecatedLuar/sat/internal/common"
)

// homebrewInstallScript is Homebrew's own official installer, and
// homebrewUninstallScript its counterpart from the same repo.
const (
	homebrewInstallScript   = "https://raw.githubusercontent.com/Homebrew/install/HEAD/install.sh"
	homebrewUninstallScript = "https://raw.githubusercontent.com/Homebrew/install/HEAD/uninstall.sh"
)

// homebrewLinuxPath, homebrewMacArmPath, and homebrewMacX86Path are where
// Homebrew's installer places the binary depending on platform.
const (
	homebrewLinuxPath  = "/home/linuxbrew/.linuxbrew/bin/brew"
	homebrewMacArmPath = "/opt/homebrew/bin/brew"
	homebrewMacX86Path = "/usr/local/bin/brew"
)

// termuxPrefixMarker identifies a Termux environment via $PREFIX, which
// Homebrew's installer does not support.
const termuxPrefixMarker = "com.termux"

// BrewInstall installs Homebrew via its official installer.
func BrewInstall() (installedPath string, err error) {
	if strings.Contains(os.Getenv("PREFIX"), termuxPrefixMarker) {
		return "", fmt.Errorf("Homebrew is not supported on Termux")
	}
	if runtime.GOOS != "linux" && runtime.GOOS != "darwin" {
		return "", fmt.Errorf("Homebrew only supports macOS and Linux")
	}

	script := "NONINTERACTIVE=1 /bin/bash -c \"$(curl -fsSL " + homebrewInstallScript + ")\""
	if err := common.RunQuiet("bash", "-c", script); err != nil {
		return "", fmt.Errorf("Homebrew install failed: %w", err)
	}

	candidates := []string{homebrewLinuxPath, homebrewMacArmPath, homebrewMacX86Path}
	for _, path := range candidates {
		if _, err := os.Stat(path); err == nil {
			return path, nil
		}
	}

	return "", fmt.Errorf("Homebrew installer ran but brew was not found in any known location")
}

// BrewRemove removes a sat-bootstrapped Homebrew via its official
// uninstall.sh (same repo as the installer), which tears down the whole
// Homebrew prefix (Cellar, taps, etc.) rather than just the brew binary.
// The copy sat relocated into common.SourcesBinDir() is orphaned by that
// uninstaller (it only knows about the real prefix), so it's removed
// separately afterward.
func BrewRemove() error {
	candidates := []string{homebrewLinuxPath, homebrewMacArmPath, homebrewMacX86Path}

	var prefix string
	for _, path := range candidates {
		if isSatManaged(path) {
			prefix = filepath.Dir(filepath.Dir(path)) // strip "/bin/brew"
			break
		}
	}
	if prefix == "" {
		return fmt.Errorf("brew is not managed by sat; remove it manually")
	}

	script := "NONINTERACTIVE=1 /bin/bash -c \"$(curl -fsSL " + homebrewUninstallScript + ")\" -- --path " + prefix
	if err := common.RunQuiet("bash", "-c", script); err != nil {
		return fmt.Errorf("Homebrew uninstall failed: %w", err)
	}

	relocatedCopy := filepath.Join(common.SourcesBinDir(), "brew")
	if err := os.Remove(relocatedCopy); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("failed to remove %s: %w", relocatedCopy, err)
	}

	return nil
}
