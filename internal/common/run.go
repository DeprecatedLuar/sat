package common

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
)

const (
	// Environment variables
	EnvSATDebug = "SAT_DEBUG"

	// Debug output
	DebugPrefix = "[debug]"
)

// RunQuiet executes a command, suppressing output unless SAT_DEBUG is set
func RunQuiet(name string, args ...string) error {
	cmd := exec.Command(name, args...)

	if os.Getenv(EnvSATDebug) != "" {
		// Show output in debug mode
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		return cmd.Run()
	}

	// Suppress output in normal mode
	return cmd.Run()
}

// FetchJSON fetches JSON data from a URL
// debugCtx is used for debug logging context (e.g., "GitHub API: /repos/...")
func FetchJSON(url, debugCtx string) ([]byte, error) {
	if os.Getenv(EnvSATDebug) != "" && debugCtx != "" {
		fmt.Fprintf(os.Stderr, "%s %s: %s\n", DebugPrefix, debugCtx, url)
	}

	resp, err := http.Get(url)
	if err != nil {
		return nil, fmt.Errorf("http request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("http %d: %s", resp.StatusCode, resp.Status)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	return body, nil
}
