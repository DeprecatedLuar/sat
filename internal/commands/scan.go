package commands

import (
	"fmt"

	"github.com/DeprecatedLuar/sat/internal/scanner"
)

// Scan scans ecosystems and populates the manifest
// Thin orchestrator - delegates to scanner package for core logic
func Scan() error {
	fmt.Println("Scanning ecosystems...")

	// Run scanner
	result, err := scanner.ScanAll()
	if err != nil {
		return err
	}

	// Display results
	fmt.Println()
	if result.Added == 0 && result.Pruned == 0 {
		fmt.Println("Manifest is up to date (no changes)")
	} else {
		if result.Pruned > 0 {
			fmt.Printf("Pruned %d entries\n", result.Pruned)
		}
		fmt.Printf("Added %d packages to manifest\n", result.Added)
	}

	return nil
}
