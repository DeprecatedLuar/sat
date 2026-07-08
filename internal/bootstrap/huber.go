package bootstrap

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"

	"github.com/DeprecatedLuar/sat/internal/common"
)

// huberReleaseBaseURL is innobead/huber's latest-release binary download
// endpoint. Huber ships static binaries per target triple, so this is a
// direct download rather than an upstream shell installer.
const huberReleaseBaseURL = "https://github.com/innobead/huber/releases/latest/download/huber-"

// huberTarget maps the running OS/arch to huber's release target triple,
// mirroring the-satellite's packages/sources/huber.sh. Linux prefers musl
// (static) builds.
func huberTarget() (string, error) {
	var osPart string
	switch runtime.GOOS {
	case "linux":
		osPart = "unknown-linux"
	case "darwin":
		osPart = "apple-darwin"
	default:
		return "", fmt.Errorf("unsupported OS: %s", runtime.GOOS)
	}

	var archPart string
	switch runtime.GOARCH {
	case "amd64":
		archPart = "x86_64"
	case "arm64":
		archPart = "aarch64"
	case "arm":
		archPart = "arm"
	default:
		return "", fmt.Errorf("unsupported architecture: %s", runtime.GOARCH)
	}

	if osPart != "unknown-linux" {
		return archPart + "-" + osPart, nil
	}

	if archPart == "arm" {
		return archPart + "-" + osPart + "-musleabihf", nil
	}
	return archPart + "-" + osPart + "-musl", nil
}

// HuberInstall downloads huber's static release binary directly into
// ~/.local/bin.
func HuberInstall() (installedPath string, err error) {
	target, err := huberTarget()
	if err != nil {
		return "", err
	}

	dest := filepath.Join(common.LocalBin(), "huber")
	if err := os.MkdirAll(filepath.Dir(dest), 0755); err != nil {
		return "", fmt.Errorf("failed to create %s: %w", filepath.Dir(dest), err)
	}

	url := huberReleaseBaseURL + target
	if err := common.RunQuiet("curl", "-sSL", url, "-o", dest); err != nil {
		return "", fmt.Errorf("huber download failed: %w", err)
	}

	if err := os.Chmod(dest, 0755); err != nil {
		return "", fmt.Errorf("failed to make huber executable: %w", err)
	}

	return dest, nil
}

// HuberRemove removes a sat-bootstrapped huber. Huber has no install
// footprint beyond its single binary, so proper cleanup is just reversing
// what Relocate did.
func HuberRemove() error {
	huberPath := filepath.Join(common.LocalBin(), "huber")
	if !isSatManaged(huberPath) {
		return fmt.Errorf("huber at %s is not managed by sat; remove it manually", huberPath)
	}
	return Unrelocate(huberPath)
}
