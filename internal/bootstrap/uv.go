package bootstrap

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/DeprecatedLuar/sat/internal/common"
)

// uvInstallScript is astral.sh's own official uv installer.
const uvInstallScript = "https://astral.sh/uv/install.sh"

// UvInstall installs uv (Python package manager) via its official
// installer, which places the binary in ~/.local/bin.
func UvInstall() (installedPath string, err error) {
	if err := common.RunQuiet("bash", "-c", "curl -LsSf "+uvInstallScript+" | sh"); err != nil {
		return "", fmt.Errorf("uv install failed: %w", err)
	}

	uvPath := filepath.Join(common.LocalBin(), "uv")
	if _, err := os.Stat(uvPath); err != nil {
		return "", fmt.Errorf("uv installer ran but %s was not found", uvPath)
	}

	return uvPath, nil
}

// UvRemove removes a sat-bootstrapped uv. uv has no official self-uninstall
// (astral-sh/uv#9871 is still open), so proper cleanup is just reversing
// what Relocate did.
func UvRemove() error {
	uvPath := filepath.Join(common.LocalBin(), "uv")
	if !isSatManaged(uvPath) {
		return fmt.Errorf("uv at %s is not managed by sat; remove it manually", uvPath)
	}
	return Unrelocate(uvPath, "uvx", "uvw")
}
