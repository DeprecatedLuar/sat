package sources

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"regexp"
	"sort"
	"strings"

	"github.com/DeprecatedLuar/sat/internal/common"
)

// Lookup is the exact-match counterpart to Search. Search answers "what
// packages look like this?" with fuzzy, display-oriented strings; Lookup
// answers "is there a package named exactly this, and what is it?" with
// the structured metadata the install resolver needs to decide between
// competing sources.
//
// Like CheckOutdated and ManifestIssues, it is an optional part of the
// source contract - only sources whose registry can answer an exact-name
// query with metadata implement XLookup.

// ErrNoMatch reports that a registry has no package under the exact name
// queried. It is a normal outcome, not a failure, so callers distinguish
// it from transport errors rather than treating every miss as an outage.
var ErrNoMatch = fmt.Errorf("no exact match")

// LookupResult is what one ecosystem knows about an exact-name match.
//
// Repo and Homepage are deliberately separate. Grouping candidates by
// project only works on a repository URL, but several registries publish
// only a homepage (nixpkgs has no repo field at all). Keeping both lets
// the resolver merge "the same project described two different ways"
// instead of treating a homepage as a distinct upstream.
type LookupResult struct {
	Name        string // package identity within that ecosystem
	Version     string
	Description string
	Repo        string   // upstream repository URL, when the registry knows it
	Homepage    string   // project homepage, when that is all the registry has
	Bins        []string // executables installed; meaningful only if BinsKnown
	BinsKnown   bool     // whether this registry can report Bins at all
	Dead        bool     // deprecated / yanked / disabled / EOL / archived
	DeadReason  string

	// IsDesktopApp and DesktopAppKnown follow the same needs-a-known-bit
	// pattern as BinsKnown: absence of a signal means UNKNOWN, not "is a
	// CLI tool". Only Flathub currently sets these, from its search
	// response's authoritative `type` field (desktop-application vs
	// console-application/runtime/addon).
	IsDesktopApp    bool
	DesktopAppKnown bool
}

// repoLinkRe matches a GitHub repository path inside arbitrary text, used
// to recover a repo URL from a project homepage.
var repoLinkRe = regexp.MustCompile(`github\.com/([A-Za-z0-9_.-]+)/([A-Za-z0-9_.-]+)`)

// repoLinkNoise are GitHub paths that are never a project's own repo, so a
// homepage linking to them doesn't get mistaken for one.
var repoLinkNoise = map[string]bool{
	"sponsors": true, "features": true, "about": true, "site": true,
	"apps": true, "orgs": true, "login": true, "marketplace": true,
	"en": true, "owner": true, "settings": true, "topics": true,
	"readme": true, "explore": true, "pricing": true, "security": true,
}

// maxHomepageBytes caps how much of a homepage is scanned for repo links.
// The links we want appear in navigation and footers, so reading the whole
// page is unnecessary.
const maxHomepageBytes = 512 << 10

// CanonicalRepo reduces the many spellings a repository URL takes across
// registries to one comparable key. Registries point at release tarballs,
// archive paths, git+ URLs and scp-style remotes, and grouping only works
// if all of them collapse to the same string.
//
// Non-GitHub URLs are normalized but otherwise returned as-is: plenty of
// legitimate projects live elsewhere, and pretending otherwise would drop
// them.
func CanonicalRepo(u string) string {
	if u == "" {
		return ""
	}

	u = strings.TrimPrefix(u, "git+")
	for _, scheme := range []string{"https://", "http://", "git://", "ssh://git@", "git@"} {
		u = strings.TrimPrefix(u, scheme)
	}
	u = strings.TrimSuffix(strings.TrimSuffix(u, "/"), ".git")
	u = strings.ToLower(u)
	u = strings.TrimPrefix(u, "www.")

	// github.com/owner/repo/releases/download/... -> github.com/owner/repo
	if rest, ok := strings.CutPrefix(u, "github.com/"); ok {
		parts := strings.Split(rest, "/")
		if len(parts) >= 2 && parts[0] != "" && parts[1] != "" {
			return "github.com/" + parts[0] + "/" + strings.TrimSuffix(parts[1], ".git")
		}
	}

	return u
}

// IsRepoURL reports whether a canonical upstream is a real repository
// rather than a homepage or a registry tarball. Only repository URLs can
// be used to identify a project across ecosystems.
func IsRepoURL(canonical string) bool {
	return strings.HasPrefix(canonical, "github.com/") &&
		strings.Count(canonical, "/") == 2
}

// RepoFromHomepage fetches a project homepage and returns the GitHub
// repository it links to most often.
//
// This exists because registries like nixpkgs publish only a homepage
// (`https://cli.github.com/`), which makes their package look like a
// different project from every registry that reports the repo
// (`github.com/cli/cli`). Frequency is the signal: a project's own site
// links to its own repo far more than to anyone else's.
//
// It is the fallback path - callers should first try to infer the repo
// from another ecosystem that already knows it, since that costs nothing.
func RepoFromHomepage(homepage string) (string, error) {
	if homepage == "" {
		return "", ErrNoMatch
	}

	req, err := http.NewRequest(http.MethodGet, homepage, nil)
	if err != nil {
		return "", err
	}
	// Some project sites serve a redirect or a stub to unknown agents.
	req.Header.Set("User-Agent", "sat/1.0 (https://github.com/DeprecatedLuar/sat)")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("http %d", resp.StatusCode)
	}

	body, err := io.ReadAll(io.LimitReader(resp.Body, maxHomepageBytes))
	if err != nil {
		return "", err
	}

	counts := map[string]int{}
	for _, m := range repoLinkRe.FindAllStringSubmatch(string(body), -1) {
		owner, repo := m[1], strings.TrimSuffix(m[2], ".git")
		if repoLinkNoise[strings.ToLower(owner)] || owner == "" || repo == "" {
			continue
		}
		counts["github.com/"+strings.ToLower(owner)+"/"+strings.ToLower(repo)]++
	}

	if len(counts) == 0 {
		return "", ErrNoMatch
	}

	best, bestN := "", 0
	keys := make([]string, 0, len(counts))
	for k := range counts {
		keys = append(keys, k)
	}
	sort.Strings(keys) // deterministic winner when two links tie
	for _, k := range keys {
		if counts[k] > bestN {
			best, bestN = k, counts[k]
		}
	}

	if os.Getenv(common.EnvSATDebug) != "" {
		fmt.Fprintf(os.Stderr, "%s resolved homepage %s -> %s (%d links)\n",
			common.DebugPrefix, homepage, best, bestN)
	}
	return best, nil
}

// ResolveRedirect follows a homepage-shaped URL's HTTP redirects and
// returns the canonicalized final destination.
//
// This exists because some projects republish under a second domain that
// 301s to the primary one (discordapp.com -> discord.com, a redirect
// Discord itself publishes). Grouping on the raw hostname would split one
// project into two identity groups; grouping on the redirect target merges
// them back into one, at the cost of trusting the redirect chain -
// two unrelated tools whose homepages both redirect to one corporate
// landing page would be merged too.
func ResolveRedirect(rawURL string) (string, error) {
	if rawURL == "" {
		return "", ErrNoMatch
	}

	req, err := http.NewRequest(http.MethodGet, rawURL, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("User-Agent", "sat/1.0 (https://github.com/DeprecatedLuar/sat)")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	io.Copy(io.Discard, io.LimitReader(resp.Body, 1<<10))

	final := CanonicalRepo(resp.Request.URL.String())
	if final == "" {
		return "", ErrNoMatch
	}
	return final, nil
}
