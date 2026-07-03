package github

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"golang.org/x/term"
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
		if os.Getenv("SAT_DEBUG") != "" {
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
		if os.Getenv("SAT_DEBUG") != "" {
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

	chosen := disambiguateCandidates(query, tier)
	return chosen.NameWithOwner, chosen.Language, nil
}

// disambiguateCandidates picks one candidate from a same-confidence tier.
// A single candidate is returned directly, no prompt. Multiple candidates
// are ranked by stars (descending); interactively, the user is asked to
// choose; non-interactively (piped/scripted, stdin isn't a TTY), the
// top-starred candidate is auto-picked and the alternatives are logged to
// stderr so scripted use never blocks.
func disambiguateCandidates(query string, candidates []githubCandidate) githubCandidate {
	if len(candidates) == 1 {
		return candidates[0]
	}

	sort.Slice(candidates, func(i, j int) bool {
		return candidates[i].Stars > candidates[j].Stars
	})

	if !term.IsTerminal(int(os.Stdin.Fd())) {
		alts := make([]string, 0, len(candidates)-1)
		for _, c := range candidates[1:] {
			alts = append(alts, c.NameWithOwner)
		}
		fmt.Fprintf(os.Stderr, "[info] multiple matches for %q, picked %s (%d stars); also matched: %s\n",
			query, candidates[0].NameWithOwner, candidates[0].Stars, strings.Join(alts, ", "))
		return candidates[0]
	}

	fmt.Fprintf(os.Stderr, "Multiple GitHub repositories match %q:\n", query)
	for i, c := range candidates {
		desc := c.Description
		if desc == "" {
			desc = "(no description)"
		}
		fmt.Fprintf(os.Stderr, "  %d) %s (%d stars) - %s\n", i+1, c.NameWithOwner, c.Stars, desc)
	}
	fmt.Fprintf(os.Stderr, "Choose [1-%d] (default 1): ", len(candidates))

	line, _ := bufio.NewReader(os.Stdin).ReadString('\n')
	if n, err := strconv.Atoi(strings.TrimSpace(line)); err == nil && n >= 1 && n <= len(candidates) {
		return candidates[n-1]
	}
	return candidates[0]
}
