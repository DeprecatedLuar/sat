package github

import (
	"encoding/json"
	"fmt"
	"strings"
)

// lookupSearchResults bounds the repository search used to find an
// exact-name match. Results are star-sorted, so the exact match - if one
// exists - is reachable well within this window.
const lookupSearchResults = 10

// LookupResult is what GitHub knows about a repository matching a tool
// name exactly. It mirrors sources.LookupResult but lives here to keep
// this package free of an import cycle back into sources.
type LookupResult struct {
	Repo        string // owner/repo
	Version     string // latest release tag
	Description string
	URL         string
	Stars       int
	Assets      []string
	Dead        bool
	DeadReason  string
}

// ErrNoMatch reports that no repository is named exactly like the query.
var ErrNoMatch = fmt.Errorf("no exact match")

// Lookup finds the repository whose name matches the query exactly and
// reports whether it ships installable release artifacts.
//
// The exact-name requirement is a real limitation worth knowing about:
// projects whose repository name differs from their binary name (the
// GitHub CLI ships `gh` from cli/cli) are invisible to this lookup, so
// GitHub must never be the only evidence used to decide an install.
func Lookup(name string) (LookupResult, error) {
	endpoint := fmt.Sprintf("search/repositories?q=%s+in:name&sort=stars&per_page=%d",
		name, lookupSearchResults)
	data, err := APIGet(endpoint)
	if err != nil {
		return LookupResult{}, ErrNoMatch
	}

	var search struct {
		Items []struct {
			FullName    string `json:"full_name"`
			Name        string `json:"name"`
			Description string `json:"description"`
			Stars       int    `json:"stargazers_count"`
			Archived    bool   `json:"archived"`
			HTMLURL     string `json:"html_url"`
		} `json:"items"`
	}
	if err := json.Unmarshal(data, &search); err != nil {
		return LookupResult{}, ErrNoMatch
	}

	for _, item := range search.Items {
		if !strings.EqualFold(item.Name, name) {
			continue
		}
		return finishLookup(item.FullName, item.Description, item.HTMLURL, item.Stars, item.Archived)
	}

	return LookupResult{}, ErrNoMatch
}

// LookupByRepo looks up a repository directly by its owner/repo path,
// skipping the by-name search entirely.
//
// This exists because Lookup is name-only: it searches for a repository
// *named* like the query, so a project whose repo name differs from its
// binary name (the GitHub CLI ships `gh` from `cli/cli`) is invisible to
// it. Once another ecosystem (brew, nix, a homepage redirect...) has
// already told the resolver which repo the project actually lives at,
// this fetches that repo's own metadata directly instead of giving up.
func LookupByRepo(repo string) (LookupResult, error) {
	data, err := APIGet("repos/" + repo)
	if err != nil {
		return LookupResult{}, ErrNoMatch
	}

	var info struct {
		FullName    string `json:"full_name"`
		Description string `json:"description"`
		Stars       int    `json:"stargazers_count"`
		Archived    bool   `json:"archived"`
		HTMLURL     string `json:"html_url"`
	}
	if err := json.Unmarshal(data, &info); err != nil {
		return LookupResult{}, ErrNoMatch
	}

	return finishLookup(info.FullName, info.Description, info.HTMLURL, info.Stars, info.Archived)
}

// finishLookup builds a LookupResult from a repository already known to
// exist, checking archival status and release-asset provision - the part
// of the lookup that is identical whether the repo was found by name
// search (Lookup) or by direct path (LookupByRepo).
func finishLookup(fullName, description, htmlURL string, stars int, archived bool) (LookupResult, error) {
	result := LookupResult{
		Repo:        fullName,
		Description: description,
		URL:         htmlURL,
		Stars:       stars,
	}
	if archived {
		result.Dead, result.DeadReason = true, "archived"
		return result, nil
	}

	assets, tag, err := latestReleaseAssets(fullName)
	switch {
	case err != nil:
		result.Dead, result.DeadReason = true, "no releases"
	case len(assets) == 0:
		result.Dead, result.DeadReason = true, "release "+tag+" has no binary assets"
	}
	result.Version, result.Assets = tag, assets
	return result, nil
}

// latestReleaseAssets returns the uploaded asset names of a repo's latest
// release. GitHub's auto-generated source tarballs are not part of
// `assets`, so a non-empty result means the maintainer published real
// artifacts rather than just tagging a commit.
func latestReleaseAssets(repo string) (names []string, tag string, err error) {
	data, err := APIGet("repos/" + repo + "/releases/latest")
	if err != nil {
		return nil, "", err
	}

	var release struct {
		TagName string `json:"tag_name"`
		Assets  []struct {
			Name string `json:"name"`
		} `json:"assets"`
	}
	if err := json.Unmarshal(data, &release); err != nil {
		return nil, "", err
	}

	for _, a := range release.Assets {
		names = append(names, a.Name)
	}
	return names, release.TagName, nil
}

// RepoStars reports the star count of an owner/repo path, whether it could
// be read at all, and whether a failed read was specifically a 404 (the
// repository does not exist) rather than some other failure.
//
// The ok return matters as much as the count: a repository that was
// deleted, renamed or rate-limited is unmeasurable, not unpopular, and
// callers must not treat it as a zero-star loser. notFound is a stronger
// claim than !ok - it means the repo is confirmed dead and belongs in
// PRUNE, whereas every other failure (network, rate limit) must never be
// treated as evidence against the repo.
func RepoStars(repo string) (stars int, ok bool, notFound bool) {
	data, err := APIGet("repos/" + repo)
	if err != nil {
		return 0, false, strings.Contains(err.Error(), "404")
	}

	var info struct {
		Stars int `json:"stargazers_count"`
	}
	if err := json.Unmarshal(data, &info); err != nil {
		return 0, false, false
	}
	return info.Stars, true, false
}
