package commands

import (
	"fmt"
	"os"
	"os/exec"
	"slices"
	"strings"
)

const (
	cloneUsage = "usage: sat clone [--gh|--gl|--cb] <owner/repo | url> [dest]"

	defaultCloneHost = "github.com"
	cloneURLFormat   = "https://%s/%s.git"
	repoPathSep      = "/"
)

// cloneHostFlags maps each host flag to the domain it selects.
var cloneHostFlags = map[string]string{
	"--github":   "github.com",
	"--gh":       "github.com",
	"--gitlab":   "gitlab.com",
	"--gl":       "gitlab.com",
	"--codeberg": "codeberg.org",
	"--cb":       "codeberg.org",
}

// cloneURLPrefixes mark an input as a full remote URL to pass through as-is.
var cloneURLPrefixes = []string{"https://", "http://", "ssh://", "git@"}

// Clone resolves an owner/repo shorthand (or full URL) and hands it to
// git clone with the terminal attached.
func Clone(args []string) error {
	host, positional, err := parseCloneArgs(args)
	if err != nil {
		return err
	}

	url, err := resolveCloneURL(host, positional[0])
	if err != nil {
		return err
	}

	gitArgs := append([]string{"clone", url}, positional[1:]...)
	cmd := exec.Command("git", gitArgs...)
	cmd.Stdin, cmd.Stdout, cmd.Stderr = os.Stdin, os.Stdout, os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("git clone %s: %w", url, err)
	}
	return nil
}

// parseCloneArgs splits args into the selected host ("" if no flag given)
// and positional arguments (repo, optional dest).
func parseCloneArgs(args []string) (host string, positional []string, err error) {
	for _, arg := range args {
		domain, isHostFlag := cloneHostFlags[arg]
		switch {
		case isHostFlag:
			if host != "" && host != domain {
				return "", nil, fmt.Errorf("conflicting host flags\n%s", cloneUsage)
			}
			host = domain
		case strings.HasPrefix(arg, "-"):
			return "", nil, fmt.Errorf("unknown flag %s\n%s", arg, cloneUsage)
		default:
			positional = append(positional, arg)
		}
	}

	if len(positional) == 0 || len(positional) > 2 {
		return "", nil, fmt.Errorf(cloneUsage)
	}
	return host, positional, nil
}

// resolveCloneURL turns input into a clonable URL. Full URLs pass through
// (a host flag alongside one is contradictory, so it errors); owner/repo
// is expanded against the selected host, defaulting to GitHub.
func resolveCloneURL(host, input string) (string, error) {
	for _, prefix := range cloneURLPrefixes {
		if strings.HasPrefix(input, prefix) {
			if host != "" {
				return "", fmt.Errorf("host flag has no effect on a full URL: %s", input)
			}
			return input, nil
		}
	}

	if !isRepoPath(input) {
		return "", fmt.Errorf("expected owner/repo or a full URL, got %q\n%s", input, cloneUsage)
	}

	if host == "" {
		host = defaultCloneHost
	}
	return fmt.Sprintf(cloneURLFormat, host, strings.TrimSuffix(input, ".git")), nil
}

// isRepoPath reports whether s looks like owner/repo (GitLab subgroups
// allow further segments); every segment must be non-empty.
func isRepoPath(s string) bool {
	parts := strings.Split(s, repoPathSep)
	return len(parts) >= 2 && !slices.Contains(parts, "")
}
