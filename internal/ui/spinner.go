package ui

import (
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/DeprecatedLuar/sat/internal/common"
)

const (
	// Spinner configuration
	SpinnerFrameInterval = 150 * time.Millisecond
	SpinnerFrameCount    = 4
)

var (
	frames      = []string{"|", "/", "-", "\\"}
	frameColors = []string{Rust, Node, Python, Brew}
)

// spinWithStyle renders tool's status line with a rotating colored frame in
// place of check/cross, through the same statusLine layout StatusOK/
// StatusError use, so the line never reflows once the task completes.
// clearWidth is the rendered line's plain-text length, used to blank it on done.
func spinWithStyle(tool string, done <-chan struct{}, sourceStr string, clearWidth int) {
	if os.Getenv(common.EnvSATDebug) != "" {
		return // Skip spinner in debug mode
	}

	ticker := time.NewTicker(SpinnerFrameInterval)
	defer ticker.Stop()

	i := 0
	for {
		select {
		case <-done:
			fmt.Printf("\r%-*s\r", clearWidth, "") // Clear line
			return
		case <-ticker.C:
			marker := frameColors[i] + frames[i] + Reset
			fmt.Print(statusLine(marker, SourceLight(sourceStr), tool, sourceStr))
			i = (i + 1) % SpinnerFrameCount
		}
	}
}

// spinnerLineWidth returns the plain-text rendered width of statusLine's
// layout for tool/sourceStr (no ANSI codes), used to fully blank the line
// once the spinner stops.
func spinnerLineWidth(tool, sourceStr string) int {
	name := TruncateName(tool, ToolNameWidth)
	if len(name) < ToolNameWidth {
		name += strings.Repeat(" ", ToolNameWidth-len(name))
	}
	return len(fmt.Sprintf("[x] %s [%s]", name, SourceDisplay(sourceStr)))
}

// RunWithSpinner executes a function with a spinner or direct output in debug mode
func RunWithSpinner(tool, sourceStr string, fn func() error) error {
	if os.Getenv(common.EnvSATDebug) != "" {
		return fn()
	}

	done := make(chan struct{})
	cleared := make(chan struct{})
	errChan := make(chan error, 1)

	go func() {
		spinWithStyle(tool, done, sourceStr, spinnerLineWidth(tool, sourceStr))
		close(cleared)
	}()

	go func() {
		errChan <- fn()
	}()

	err := <-errChan
	close(done)
	<-cleared // wait for the spinner to finish clearing before the caller prints

	return err
}
