package commands

import (
	"fmt"
)

// HandleUpdate routes between package updates and self-update
func HandleUpdate(args []string, version, repo string) error {
	if len(args) == 0 {
		return fmt.Errorf("Usage: sat update <program> [program2] ...")
	}

	// Special case: sat update sat
	if len(args) == 1 && args[0] == "sat" {
		return HandleSelfUpdate(version, repo)
	}

	// Regular package updates (to be implemented in later phases)
	return updatePackages(args)
}

func updatePackages(args []string) error {
	// TODO: Implement package update logic
	// This will call source-specific update functions based on manifest entries
	return fmt.Errorf("package updates not yet implemented (would update: %v)", args)
}
