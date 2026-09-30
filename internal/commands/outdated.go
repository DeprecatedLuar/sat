package commands

import "fmt"

const outdatedUsage = "usage: sat outdated [<tool> ...] [--cargo|--brew|--nix|--apt|--gh|--appimage|--flatpak|--npm|--uv|--sat]"

// HandleOutdated lists tracked tools with a newer version available. It only
// reads: nothing is updated and no prompt is shown.
func HandleOutdated(args []string, version, repo string) error {
	tools, sourceFilter, _, err := parseTargetArgs(args, outdatedUsage, false)
	if err != nil {
		return err
	}
	reconcileDrift()

	outdated, matched, err := scanOutdated(tools, sourceFilter, version, repo)
	if err != nil || !matched {
		return err
	}
	if len(outdated) == 0 {
		fmt.Println(upToDateMessage)
		return nil
	}
	printOutdated(outdated)
	return nil
}
