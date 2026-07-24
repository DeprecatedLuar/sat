package ui

import (
	"fmt"
	"os"
	"time"

	"github.com/DeprecatedLuar/sat/internal/common"
)

const (
	// Spinner configuration
	SpinnerFrameInterval = 150 * time.Millisecond
	SpinnerClearWidth    = 50
	SpinnerFrameCount    = 4
)

var (
	frames      = []string{"|", "/", "-", "\\"}
	frameColors = []string{Rust, Node, Python, Brew}
)

// spinWithStyle shows a spinner with tool name and source tag
func spinWithStyle(program string, done <-chan struct{}, sourceStr string) {
	if os.Getenv(common.EnvSATDebug) != "" {
		return // Skip spinner in debug mode
	}

	ticker := time.NewTicker(SpinnerFrameInterval)
	defer ticker.Stop()

	pkgColor := SourceLight(sourceStr)
	srcDisplay := SourceDisplay(sourceStr)
	srcColor := SourceColor(srcDisplay)
	i := 0

	for {
		select {
		case <-done:
			fmt.Printf("\r%-*s\r", SpinnerClearWidth, "") // Clear line
			return
		case <-ticker.C:
			fmt.Printf("\r[%s%s%s] %s%s%s [%s%s%s]",
				frameColors[i], frames[i], Reset,
				pkgColor, program, Reset,
				srcColor, srcDisplay, Reset)
			i = (i + 1) % SpinnerFrameCount
		}
	}
}

// RunWithSpinner executes a function with a spinner or direct output in debug mode
func RunWithSpinner(tool, sourceStr string, fn func() error) error {
	if os.Getenv(common.EnvSATDebug) != "" {
		return fn()
	}

	done := make(chan struct{})
	errChan := make(chan error, 1)

	go spinWithStyle(tool, done, sourceStr)

	go func() {
		errChan <- fn()
	}()

	err := <-errChan
	close(done)

	return err
}
