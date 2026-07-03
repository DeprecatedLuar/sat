// Package github holds the GitHub-specific mechanics used by the "gh"
// install source: REST/GraphQL fetching, repo tree/release inspection,
// the huber/go/python install methods, and fuzzy-match search. It has no
// dependency on the parent internal/sources package (in particular, no
// knowledge of AppImage) - internal/sources/github.go is the orchestrator
// that composes this package's pieces with sources.AppImageInstall.
package github

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"regexp"
	"sort"
	"strings"

	"github.com/DeprecatedLuar/sat/internal/common"
)

// releaseAssetSkipSuffixes are release asset extensions that are never
// binaries (archives, checksums, docs) and should be filtered out.
var releaseAssetSkipSuffixes = []string{".zip", ".tar.gz", ".sig", ".sha256", ".txt", ".md"}

// releaseResponse is the subset of GitHub's releases/latest response this
// package needs (tag name for version queries, asset names for deriving
// candidate binary names).
type releaseResponse struct {
	TagName string `json:"tag_name"`
	Assets  []struct {
		Name string `json:"name"`
	} `json:"assets"`
}

// githubTreeResponse is the subset of GitHub's git/trees response needed
// to list a repository's file paths.
type githubTreeResponse struct {
	Tree []struct {
		Path string `json:"path"`
	} `json:"tree"`
}

// APIGet fetches JSON from the GitHub REST API for the given endpoint
// (e.g. "repos/owner/repo/releases/latest"). Prefers the authenticated gh
// CLI, falling back to an unauthenticated curl-equivalent request when gh
// is unavailable or not logged in.
func APIGet(endpoint string) ([]byte, error) {
	if _, err := exec.LookPath("gh"); err == nil {
		if authCheck := exec.Command("gh", "auth", "status"); authCheck.Run() == nil {
			output, err := exec.Command("gh", "api", endpoint).Output()
			if err != nil {
				if os.Getenv("SAT_DEBUG") != "" {
					fmt.Fprintf(os.Stderr, "[debug] gh api %s failed: %v\n", endpoint, err)
				}
				return nil, fmt.Errorf("gh api request failed: %w", err)
			}

			if !json.Valid(output) {
				if os.Getenv("SAT_DEBUG") != "" {
					fmt.Fprintf(os.Stderr, "[debug] invalid JSON from GitHub API\n")
				}
				return nil, fmt.Errorf("invalid JSON from GitHub API")
			}

			return output, nil
		}
	}

	url := "https://api.github.com/" + endpoint
	data, err := common.FetchJSON(url, "GitHub API: "+endpoint)
	if err != nil {
		return nil, err
	}

	if !json.Valid(data) {
		return nil, fmt.Errorf("invalid JSON from GitHub API")
	}

	return data, nil
}

// LatestReleaseTag returns the latest release tag for repo (owner/repo).
func LatestReleaseTag(repo string) (string, error) {
	data, err := APIGet("repos/" + repo + "/releases/latest")
	if err != nil {
		return "", err
	}

	var release releaseResponse
	if err := json.Unmarshal(data, &release); err != nil {
		return "", err
	}

	return release.TagName, nil
}

// FetchTree lists every file path in repo's default branch tree, trying
// "main" then falling back to "master" (lib/sources/github.sh:55). Falls
// back on any failure of the "main" attempt too, not just an empty
// result - most commonly a 404 because the repo's default branch is
// actually "master" (e.g. BurntSushi/ripgrep).
func FetchTree(repo string) ([]string, error) {
	if tree, err := fetchTreeRef(repo, "main"); err == nil && len(tree) > 0 {
		return tree, nil
	}
	return fetchTreeRef(repo, "master")
}

func fetchTreeRef(repo, ref string) ([]string, error) {
	data, err := APIGet("repos/" + repo + "/git/trees/" + ref + "?recursive=1")
	if err != nil {
		return nil, err
	}

	var resp githubTreeResponse
	if err := json.Unmarshal(data, &resp); err != nil {
		return nil, err
	}

	paths := make([]string, 0, len(resp.Tree))
	for _, t := range resp.Tree {
		paths = append(paths, t.Path)
	}
	return paths, nil
}

// releaseBinaryCache is an in-memory per-process cache, mirroring bash's
// _RELEASE_CACHE associative array (lib/sources/github.sh:11).
var releaseBinaryCache = map[string][]string{}

// releaseAssetPlatformSuffixRe strips platform/arch suffixes from release
// asset names to derive candidate binary names, mirroring bash's sed
// expression in _get_release_binaries (lib/sources/github.sh:94).
var releaseAssetPlatformSuffixRe = regexp.MustCompile(`-(linux|darwin|windows|amd64|arm64|x86_64|aarch64|i686|armv7l).*$`)

// deriveReleaseBinaryNames filters out non-binary assets (archives,
// checksums, docs), strips platform/arch suffixes, and dedupes+sorts the
// result.
func deriveReleaseBinaryNames(assets []struct {
	Name string `json:"name"`
}) []string {
	seen := map[string]bool{}
	var bins []string
	for _, asset := range assets {
		skip := false
		for _, suf := range releaseAssetSkipSuffixes {
			if strings.HasSuffix(asset.Name, suf) {
				skip = true
				break
			}
		}
		if skip {
			continue
		}

		bin := releaseAssetPlatformSuffixRe.ReplaceAllString(asset.Name, "")
		if bin == "" || seen[bin] {
			continue
		}
		seen[bin] = true
		bins = append(bins, bin)
	}
	sort.Strings(bins)
	return bins
}

// getReleaseBinaries returns candidate binary names derived from repo's
// latest release assets (archives/checksums/docs filtered out, platform
// suffixes stripped, deduped and sorted), cached per-process. Mirrors
// bash's _get_release_binaries (lib/sources/github.sh:75).
func getReleaseBinaries(repo string) ([]string, error) {
	if cached, ok := releaseBinaryCache[repo]; ok {
		return cached, nil
	}

	data, err := APIGet("repos/" + repo + "/releases/latest")
	if err != nil {
		return nil, err
	}

	var release releaseResponse
	if err := json.Unmarshal(data, &release); err != nil {
		return nil, err
	}

	bins := deriveReleaseBinaryNames(release.Assets)
	releaseBinaryCache[repo] = bins
	return bins, nil
}
