package commands

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	"github.com/DeprecatedLuar/sat/internal/manifest"
)

// Pulverize clears the manifest file after confirmation
func Pulverize() error {
	manifestPath := manifest.ManifestPath()

	// Check if manifest exists
	if _, err := os.Stat(manifestPath); os.IsNotExist(err) {
		fmt.Println("Manifest is already empty.")
		return nil
	}

	// Prompt for confirmation
	fmt.Print("This will clear the entire manifest. Continue? (y/N): ")
	reader := bufio.NewReader(os.Stdin)
	response, err := reader.ReadString('\n')
	if err != nil {
		return fmt.Errorf("failed to read input: %w", err)
	}

	response = strings.TrimSpace(strings.ToLower(response))
	if response != "y" && response != "yes" {
		fmt.Println("Cancelled.")
		return nil
	}

	// Clear the manifest
	if err := os.Remove(manifestPath); err != nil {
		return fmt.Errorf("failed to remove manifest: %w", err)
	}

	// Recreate empty manifest
	if err := os.WriteFile(manifestPath, []byte(""), 0644); err != nil {
		return fmt.Errorf("failed to create empty manifest: %w", err)
	}

	fmt.Println("Manifest pulverized successfully.")
	return nil
}
