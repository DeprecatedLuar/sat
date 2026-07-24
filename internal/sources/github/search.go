package github

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"regexp"
	"strings"

	"github.com/DeprecatedLuar/sat/internal/common"
)

// githubSearchLimit is the number of candidates fetched per GraphQL
// repository search (matches bash's search_github default limit).
const githubSearchLimit = 10

// githubSearchResponse represents GitHub GraphQL search response.
type githubSearchResponse struct {
	Data struct {
		Search struct {
			Nodes []struct {
				NameWithOwner   string `json:"nameWithOwner"`
				Description     string `json:"description"`
				StargazerCount  int    `json:"stargazerCount"`
				PrimaryLanguage *struct {
					Name string `json:"name"`
				} `json:"primaryLanguage"`
				LatestRelease *struct {
					TagName string `json:"tagName"`
				} `json:"latestRelease"`
			} `json:"nodes"`
		} `json:"search"`
	} `json:"data"`
}

// githubCandidate is a single repository result from a GraphQL search,
// structured for both display (Search) and best-match resolution
// (SearchBestMatch).
type githubCandidate struct {
	NameWithOwner string
	Description   string
	Language      string
	LatestTag     string
	Stars         int
}

// githubGraphQLSearch runs a GraphQL repository search over the gh CLI and
// returns structured candidates. Returns (nil, nil) - not an error - when
// gh is unavailable or unauthenticated, mirroring bash's silent-skip
// behavior (lib/sources/github.sh:150).
func githubGraphQLSearch(query string, limit int) ([]githubCandidate, error) {
	if _, err := exec.LookPath("gh"); err != nil {
		return nil, nil
	}

	if err := exec.Command("gh", "auth", "status").Run(); err != nil {
		if os.Getenv(common.EnvSATDebug) != "" {
			fmt.Fprintf(os.Stderr, "[debug] gh not authenticated, skipping GitHub search\n")
		}
		return nil, nil
	}

	graphqlQuery := fmt.Sprintf(`{
  search(query: "%s", type: REPOSITORY, first: %d) {
    nodes {
      ... on Repository {
        nameWithOwner
        description
        stargazerCount
        primaryLanguage {
          name
        }
        latestRelease {
          tagName
        }
      }
    }
  }
}`, query, limit)

	cmd := exec.Command("gh", "api", "graphql", "-f", "query="+graphqlQuery)
	var output, errOutput bytes.Buffer
	cmd.Stdout = &output
	cmd.Stderr = &errOutput

	if err := cmd.Run(); err != nil {
		if os.Getenv(common.EnvSATDebug) != "" {
			fmt.Fprintf(os.Stderr, "[debug] GitHub GraphQL query failed: %v\n", err)
		}
		return nil, nil
	}

	var resp githubSearchResponse
	if err := json.Unmarshal(output.Bytes(), &resp); err != nil {
		return nil, nil
	}

	candidates := make([]githubCandidate, 0, len(resp.Data.Search.Nodes))
	for _, node := range resp.Data.Search.Nodes {
		lang := ""
		if node.PrimaryLanguage != nil {
			lang = node.PrimaryLanguage.Name
		}
		tag := ""
		if node.LatestRelease != nil {
			tag = node.LatestRelease.TagName
		}
		candidates = append(candidates, githubCandidate{
			NameWithOwner: node.NameWithOwner,
			Description:   node.Description,
			Language:      lang,
			LatestTag:     tag,
			Stars:         node.StargazerCount,
		})
	}

	return candidates, nil
}

// Search searches GitHub repositories using GraphQL and formats each
// result as a single display line.
func Search(query string) ([]string, error) {
	candidates, err := githubGraphQLSearch(query, githubSearchLimit)
	if err != nil || candidates == nil {
		return nil, err
	}

	results := make([]string, 0, len(candidates))
	for _, c := range candidates {
		version := "(no releases)"
		if c.LatestTag != "" {
			version = c.LatestTag
		}

		desc := c.Description
		if desc == "" {
			desc = "(no description)"
		} else {
			desc = strings.Split(desc, "\n")[0]
			if len(desc) > 50 {
				desc = desc[:50]
			}
		}

		results = append(results, fmt.Sprintf("%s %s - %s", c.NameWithOwner, version, desc))
	}

	return results, nil
}

// SearchBestMatch resolves a short tool name to an owner/repo, mirroring
// bash's two-tier search_github_best_match (lib/sources/github.sh:182):
// exact repo-name matches rank above substring/word-boundary pattern
// matches. Unlike bash, every same-tier candidate is collected instead of
// returning the first, so ambiguous matches can be disambiguated rather
// than silently picking whatever GitHub returned first.
func SearchBestMatch(query string) (repo, lang string, err error) {
	candidates, err := githubGraphQLSearch(query, githubSearchLimit)
	if err != nil {
		return "", "", err
	}
	if len(candidates) == 0 {
		return "", "", fmt.Errorf("no GitHub repositories found for %q", query)
	}

	patternRe := regexp.MustCompile(`(?i)(^|[-_@/.])` + regexp.QuoteMeta(query) + `($|[-_@/.])`)

	var exact, pattern []githubCandidate
	for _, c := range candidates {
		name := repoBaseName(c.NameWithOwner)
		switch {
		case strings.EqualFold(name, query):
			exact = append(exact, c)
		case patternRe.MatchString(name):
			pattern = append(pattern, c)
		}
	}

	tier := exact
	if len(tier) == 0 {
		tier = pattern
	}
	if len(tier) == 0 {
		return "", "", fmt.Errorf("no GitHub repositories found matching %q", query)
	}

	chosen, err := disambiguateCandidates(query, tier)
	if err != nil {
		return "", "", err
	}
	return chosen.NameWithOwner, chosen.Language, nil
}

// disambiguateCandidates picks the single candidate in a same-confidence
// tier. A single candidate is returned directly. Multiple candidates fall
// back to "exactly one has a release" as a tiebreaker, since a repo with no
// releases can't be installed anyway; if that's still ambiguous (zero or
// more than one has a release), it returns an AmbiguousMatchError so the
// caller can tell the user to disambiguate via an explicit "owner/repo".
func disambiguateCandidates(query string, candidates []githubCandidate) (githubCandidate, error) {
	if len(candidates) == 1 {
		return candidates[0], nil
	}

	var withRelease []githubCandidate
	for _, c := range candidates {
		if c.LatestTag != "" {
			withRelease = append(withRelease, c)
		}
	}
	if len(withRelease) == 1 {
		return withRelease[0], nil
	}

	matches := make([]AmbiguousMatch, len(candidates))
	for i, c := range candidates {
		matches[i] = AmbiguousMatch{NameWithOwner: c.NameWithOwner, Stars: c.Stars}
	}
	return githubCandidate{}, &AmbiguousMatchError{Query: query, Matches: matches}
}

// AmbiguousMatch is one same-confidence candidate reported by
// AmbiguousMatchError, carrying just enough to render a readable list.
type AmbiguousMatch struct {
	NameWithOwner string
	Stars         int
}

// AmbiguousMatchError reports that a short tool name matched more than one
// same-confidence GitHub repository, so the caller must specify one
// explicitly rather than sat guessing.
type AmbiguousMatchError struct {
	Query   string
	Matches []AmbiguousMatch
}

func (e *AmbiguousMatchError) Error() string {
	nameWidth := 0
	for _, m := range e.Matches {
		if len(m.NameWithOwner) > nameWidth {
			nameWidth = len(m.NameWithOwner)
		}
	}

	var b strings.Builder
	fmt.Fprintf(&b, "%d packages matched %q:\n", len(e.Matches), e.Query)
	for _, m := range e.Matches {
		fmt.Fprintf(&b, "  %-*s (%d stars)\n", nameWidth, m.NameWithOwner, m.Stars)
	}
	fmt.Fprint(&b, "\nrun: sat install <owner/repo>")
	return b.String()
}
