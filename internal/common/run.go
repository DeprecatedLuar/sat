package common

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"strconv"
	"syscall"
	"unsafe"
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

// FetchJSON fetches JSON data from a URL with proper headers
// debugCtx is used for debug logging context (e.g., "GitHub API: /repos/...")
func FetchJSON(url, debugCtx string) ([]byte, error) {
	if os.Getenv(EnvSATDebug) != "" && debugCtx != "" {
		fmt.Fprintf(os.Stderr, "%s %s: %s\n", DebugPrefix, debugCtx, url)
	}

	// Create request with proper User-Agent header
	// Many APIs (like crates.io) require a User-Agent or block requests
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	// Set User-Agent to mimic curl
	req.Header.Set("User-Agent", "sat/1.0 (https://github.com/DeprecatedLuar/sat)")

	client := &http.Client{}
	resp, err := client.Do(req)
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

// winsize structure for ioctl
type winsize struct {
	Row    uint16
	Col    uint16
	Xpixel uint16
	Ypixel uint16
}

// GetTerminalWidth returns the terminal width using syscalls
func GetTerminalWidth() (int, error) {
	// Try syscall (most reliable)
	ws := &winsize{}
	retCode, _, errno := syscall.Syscall(syscall.SYS_IOCTL,
		uintptr(syscall.Stdout),
		uintptr(syscall.TIOCGWINSZ),
		uintptr(unsafe.Pointer(ws)))

	if int(retCode) == 0 && ws.Col > 0 {
		return int(ws.Col), nil
	}

	// Fallback to COLUMNS env var
	if cols := os.Getenv("COLUMNS"); cols != "" {
		if width, err := strconv.Atoi(cols); err == nil && width > 0 {
			return width, nil
		}
	}

	// Debug output if detection failed
	if os.Getenv(EnvSATDebug) != "" {
		fmt.Fprintf(os.Stderr, "%s terminal width detection failed (errno: %v), using default 80\n", DebugPrefix, errno)
	}

	// Default fallback
	return 80, nil
}
