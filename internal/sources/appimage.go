package sources

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"time"

	"github.com/DeprecatedLuar/sat/internal/common"
	"github.com/DeprecatedLuar/sat/internal/manifest"
	"github.com/DeprecatedLuar/sat/internal/sources/github"
)

// appImageVersionSuffix strips a trailing version segment from a release
// asset filename to derive a clean binary name, e.g. "ripgrep_14.1.0" or
// "ripgrep-v14.1.0" -> "ripgrep". Mirrors bash's
// sed -E 's/[_-]([0-9]+\.[0-9]|v?[0-9]+).*//' (lib/sources/appimage.sh:89).
var appImageVersionSuffix = regexp.MustCompile(`[_-]([0-9]+\.[0-9]|v?[0-9]+).*`)

// githubReleaseAsset is a single asset attached to a GitHub release.
type githubReleaseAsset struct {
	Name               string `json:"name"`
	BrowserDownloadURL string `json:"browser_download_url"`
}

// githubReleaseResponse is the subset of GitHub's releases/latest response
// appimage.go needs.
type githubReleaseResponse struct {
	TagName     string               `json:"tag_name"`
	Message     string               `json:"message"`
	PublishedAt string               `json:"published_at"`
	Assets      []githubReleaseAsset `json:"assets"`
}

// AppImageInstall downloads the latest AppImage release asset for
// repoPath (owner/repo) matching the current architecture, stores it in
// common.AppImagesDir(), and symlinks it into common.LocalBin(). Returns
// the resolved binary name for the caller to record in the manifest as
// appimage:owner/repo.
func AppImageInstall(repoPath string) (binName string, err error) {
	if os.Getenv("SAT_DEBUG") != "" {
		fmt.Fprintf(os.Stderr, "[debug]   trying appimage for %s\n", repoPath)
	}

	archPattern, err := appImageArchPattern()
	if err != nil {
		if os.Getenv("SAT_DEBUG") != "" {
			fmt.Fprintf(os.Stderr, "[debug]   %s\n", err)
		}
		return "", err
	}

	if os.Getenv("SAT_DEBUG") != "" {
		fmt.Fprintf(os.Stderr, "[debug]   fetching releases from GitHub API...\n")
	}
	data, err := github.APIGet("repos/" + repoPath + "/releases/latest")
	if err != nil {
		return "", err
	}

	var release githubReleaseResponse
	if err := json.Unmarshal(data, &release); err != nil {
		return "", fmt.Errorf("failed to parse GitHub release response: %w", err)
	}

	if strings.Contains(strings.ToLower(release.Message), "rate limit") {
		return "", fmt.Errorf("GitHub API rate limit exceeded (authenticate with 'gh auth login' for higher limits)")
	}

	if os.Getenv("SAT_DEBUG") != "" {
		fmt.Fprintf(os.Stderr, "[debug]   found %d release assets, filtering for %s...\n", len(release.Assets), archPattern)
	}
	assetName, assetURL := selectAppImageAsset(release.Assets, archPattern)
	if assetURL == "" {
		if os.Getenv("SAT_DEBUG") != "" {
			fmt.Fprintf(os.Stderr, "[debug]   no AppImage available\n")
		}
		return "", fmt.Errorf("no AppImage release asset found for %s", repoPath)
	}

	if os.Getenv("SAT_DEBUG") != "" {
		fmt.Fprintf(os.Stderr, "[debug]   selected: %s\n", assetName)
	}

	appName := cleanAppImageName(assetName, repoPath)

	appimageDir := common.AppImagesDir()
	appimagePath := filepath.Join(appimageDir, appName)
	symlinkPath := filepath.Join(common.LocalBin(), appName)

	if os.Getenv("SAT_DEBUG") != "" {
		fmt.Fprintf(os.Stderr, "[debug]   downloading to %s...\n", appimagePath)
	}
	if err := os.MkdirAll(appimageDir, 0755); err != nil {
		return "", fmt.Errorf("failed to create appimages directory: %w", err)
	}
	if err := common.RunQuiet("curl", "-L", "-o", appimagePath, assetURL); err != nil {
		if os.Getenv("SAT_DEBUG") != "" {
			fmt.Fprintf(os.Stderr, "[debug]   download failed\n")
		}
		return "", fmt.Errorf("appimage download failed: %w", err)
	}
	if err := os.Chmod(appimagePath, 0755); err != nil {
		return "", fmt.Errorf("failed to make appimage executable: %w", err)
	}

	if os.Getenv("SAT_DEBUG") != "" {
		fmt.Fprintf(os.Stderr, "[debug]   creating symlink %s\n", symlinkPath)
	}
	os.Remove(symlinkPath)
	if err := os.Symlink(appimagePath, symlinkPath); err != nil {
		return "", fmt.Errorf("failed to create symlink: %w", err)
	}

	if os.Getenv("SAT_DEBUG") != "" {
		fmt.Fprintf(os.Stderr, "[debug]   appimage install successful: %s\n", appName)
	}

	InstallDesktopEntry(appimagePath, symlinkPath, appName, repoPath)

	return appName, nil
}

// AppImageUninstall removes the symlink and the stored AppImage file.
// Matches bash's rm -f semantics: missing files are not an error.
func AppImageUninstall(pkg, source string) error {
	symlinkPath := filepath.Join(common.LocalBin(), pkg)
	appimagePath := filepath.Join(common.AppImagesDir(), pkg)

	removeDesktopEntry(pkg)

	var firstErr error
	if err := os.Remove(symlinkPath); err != nil && !os.IsNotExist(err) {
		firstErr = err
	}
	if err := os.Remove(appimagePath); err != nil && !os.IsNotExist(err) && firstErr == nil {
		firstErr = err
	}

	return firstErr
}

// AppImageUpdate re-downloads the latest release, replacing the existing
// install. repo is the owner/repo identity from the appimage:owner/repo
// manifest source string.
func AppImageUpdate(tool, repo string) error {
	if err := AppImageUninstall(tool, ""); err != nil {
		return err
	}
	_, err := AppImageInstall(repo)
	return err
}

// AppImageGetVersion returns the latest GitHub release tag for repo.
// AppImages carry no local version metadata, so the "installed version"
// is whatever tag was current when installed/last checked.
func AppImageGetVersion(repo string) string {
	if repo == "" {
		return ""
	}

	data, err := github.APIGet("repos/" + repo + "/releases/latest")
	if err != nil {
		return ""
	}

	var release githubReleaseResponse
	if err := json.Unmarshal(data, &release); err != nil {
		return ""
	}

	return release.TagName
}

// AppImageCheckOutdated checks whether a newer release exists for repo.
// If no version was ever recorded in the manifest, falls back to comparing
// the installed file's mtime against the release's published_at timestamp.
func AppImageCheckOutdated(tool, repo string) (current, latest string, err error) {
	if repo == "" {
		return "", "", fmt.Errorf("no repository identity for %s", tool)
	}

	appimagePath := filepath.Join(common.AppImagesDir(), tool)

	current = manifest.GetSourceVersion(manifest.Get(tool))
	if current == "" {
		if !fileExists(appimagePath) {
			return "", "", fmt.Errorf("appimage not installed")
		}
		current = "local"
	}

	data, err := github.APIGet("repos/" + repo + "/releases/latest")
	if err != nil {
		return "", "", err
	}

	var release githubReleaseResponse
	if err := json.Unmarshal(data, &release); err != nil {
		return "", "", fmt.Errorf("failed to parse GitHub release response: %w", err)
	}
	if release.TagName == "" {
		return "", "", fmt.Errorf("failed to query latest version")
	}

	if current != "local" {
		if current == release.TagName {
			return "", "", fmt.Errorf("already up to date")
		}
		return current, release.TagName, nil
	}

	// No recorded version: fall back to file mtime vs release publish time.
	info, err := os.Stat(appimagePath)
	if err != nil {
		return "", "", fmt.Errorf("appimage not installed")
	}

	publishedAt, err := time.Parse(time.RFC3339, release.PublishedAt)
	if err != nil {
		return "", "", fmt.Errorf("failed to parse release publish time: %w", err)
	}

	if publishedAt.After(info.ModTime()) {
		return "local", release.TagName, nil
	}

	return "", "", fmt.Errorf("already up to date")
}

// appImageArchPattern returns a case-insensitive regex pattern matching
// release asset names for the current architecture, mirroring bash's
// uname -m switch (lib/sources/appimage.sh:39-47).
func appImageArchPattern() (string, error) {
	switch runtime.GOARCH {
	case "amd64":
		return "(?i)(amd64|x86_64)", nil
	case "arm64":
		return "(?i)(arm64|aarch64)", nil
	case "arm":
		return "(?i)(armhf|armv7)", nil
	default:
		return "", fmt.Errorf("unsupported architecture: %s", runtime.GOARCH)
	}
}

// selectAppImageAsset picks the best .AppImage asset for the current
// architecture. Falls back to the first .AppImage asset found if none
// match archPattern (assumes x86_64, matching bash's fallback).
func selectAppImageAsset(assets []githubReleaseAsset, archPattern string) (name, url string) {
	archRe := regexp.MustCompile(archPattern)

	var appimageAssets []githubReleaseAsset
	for _, a := range assets {
		if strings.HasSuffix(a.Name, ".AppImage") {
			appimageAssets = append(appimageAssets, a)
		}
	}
	if len(appimageAssets) == 0 {
		return "", ""
	}

	for _, a := range appimageAssets {
		if archRe.MatchString(a.Name) {
			return a.Name, a.BrowserDownloadURL
		}
	}

	first := appimageAssets[0]
	return first.Name, first.BrowserDownloadURL
}

// cleanAppImageName derives a clean, lowercase binary name from a release
// asset filename, stripping the .AppImage suffix and any trailing version
// segment. Falls back to the repo name if extraction leaves nothing.
func cleanAppImageName(assetName, repoPath string) string {
	name := strings.TrimSuffix(assetName, ".AppImage")

	if loc := appImageVersionSuffix.FindStringIndex(name); loc != nil {
		name = name[:loc[0]]
	}

	if name == "" {
		name = repoPath
		if idx := strings.LastIndex(repoPath, "/"); idx != -1 {
			name = repoPath[idx+1:]
		}
	}

	return strings.ToLower(name)
}

// fileExists checks if a regular file exists at path.
func fileExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && !info.IsDir()
}
