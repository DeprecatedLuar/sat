// Self-update handler for the sat binary itself, distinct from `sat update`
// (which updates already-installed packages). The version check runs through
// sat's own GitHub machinery so a pending sat release is judged by the same
// rules as every other gh-sourced tool in the manifest; only the install is
// delegated to the-satellite's installer script.
// Project-specific values live in main.go: const githubRepo = "user/repo" and var version = "dev".
// Call site: commands.HandleSelfUpdate(version, githubRepo)
package commands

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/DeprecatedLuar/sat/internal/common"
	"github.com/DeprecatedLuar/sat/internal/sources"
)

const (
	updateDevSentinel = "dev"
	satelliteURL      = "https://raw.githubusercontent.com/DeprecatedLuar/the-satellite/main/satellite.sh"

	// satelliteNoStop opts out of the installer's stop_running_instance step,
	// which pgreps for the binary's name and SIGTERMs every match. That exists
	// to stop a daemon before its binary is replaced; sat is a short-lived CLI
	// updating itself, so the only process it would ever match is this one.
	satelliteNoStop = "SATELLITE_NO_STOP=1"
)

// SelfUpdateCheck reports repo's latest released version when it is newer
// than currentVersion. ok is false for a dev build, a failed GitHub lookup,
// or an already-current binary — a bulk scan skips all three silently rather
// than treating them as errors, matching checkOutdated's ok=false contract.
func SelfUpdateCheck(currentVersion, repo string) (latest string, ok bool) {
	if currentVersion == updateDevSentinel {
		return "", false
	}

	tag := sources.GitHubQueryLatestVersion(repo)
	if tag == "" || !common.VersionIsNewer(tag, currentVersion) {
		return "", false
	}
	return tag, true
}

func HandleSelfUpdate(currentVersion, repo string) error {
	if currentVersion == updateDevSentinel {
		fmt.Println("Development build — skipping update")
		return nil
	}

	fmt.Printf("Current version: %s\n", currentVersion)
	fmt.Println("Checking for updates...")

	latestVersion, ok := SelfUpdateCheck(currentVersion, repo)
	if !ok {
		fmt.Println("Already up to date")
		return nil
	}

	return selfUpdateInstall(latestVersion, repo)
}

// selfUpdateInstall replaces the running binary with repo's latestVersion via
// the-satellite's installer. Split from HandleSelfUpdate so the bulk update
// flow, which already learned latestVersion from its own scan, can install
// without paying for a second lookup.
//
// This does not return on success in the usual sense: the installer replaces
// the executable underneath the running process, so nothing may be sequenced
// after it.
func selfUpdateInstall(latestVersion, repo string) error {
	parts := strings.SplitN(repo, "/", 2)
	if len(parts) != 2 {
		return fmt.Errorf("invalid repo format %q, expected user/repo", repo)
	}
	repoUser, repoName := parts[0], parts[1]

	exe, err := os.Executable()
	if err != nil {
		return fmt.Errorf("could not locate executable: %w", err)
	}
	installDir := filepath.Dir(exe)
	binaryName := filepath.Base(exe)

	fmt.Printf("Updating to %s...\n", latestVersion)

	buildCmd := fmt.Sprintf("go build -ldflags='-s -w' -o %s ./cmd/%s", binaryName, binaryName)
	installCmd := exec.Command("bash", "-c",
		fmt.Sprintf(`bash <(curl -sSL %s) install "%s" "%s" "%s" "%s" "%s" "%s"`,
			satelliteURL, repoName, binaryName, repoUser, repoName, installDir, buildCmd),
	)
	installCmd.Env = append(os.Environ(), satelliteNoStop)
	installCmd.Stdout = os.Stdout
	installCmd.Stderr = os.Stderr
	return installCmd.Run()
}
