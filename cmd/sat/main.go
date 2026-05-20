package main

import (
	"fmt"
	"os"

	"github.com/DeprecatedLuar/sat/internal/commands"
	"github.com/DeprecatedLuar/sat/internal/selfheal"
)

const (
	// Environment variables
	EnvSATDebug = "SAT_DEBUG"

	// Debug flag value
	DebugValue = "1"

	// Global flags
	FlagDebug = "--debug"
)

var version = "dev"

func main() {
	// Ensure sat is properly initialized
	if err := selfheal.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "sat: initialization failed: %v\n", err)
		os.Exit(1)
	}

	args := os.Args[1:]

	// Handle global flags
	debug := false
	filteredArgs := []string{}
	for _, arg := range args {
		if arg == FlagDebug {
			debug = true
			os.Setenv(EnvSATDebug, DebugValue)
		} else {
			filteredArgs = append(filteredArgs, arg)
		}
	}
	args = filteredArgs

	if len(args) == 0 {
		printHelp()
		return
	}

	command := args[0]
	commandArgs := args[1:]

	_ = debug // Will be used in later phases
	_ = commandArgs

	switch command {
	case "install":
		notImplemented("install")
	case "search":
		notImplemented("search")
	case "uninstall":
		notImplemented("uninstall")
	case "shell":
		notImplemented("shell")
	case "list":
		notImplemented("list")
	case "track":
		notImplemented("track")
	case "untrack":
		notImplemented("untrack")
	case "scan":
		if err := commands.Scan(); err != nil {
			fmt.Fprintf(os.Stderr, "sat: scan failed: %v\n", err)
			os.Exit(1)
		}
	case "outdated":
		notImplemented("outdated")
	case "update":
		notImplemented("update")
	case "info":
		notImplemented("info")
	case "clone":
		notImplemented("clone")
	case "pull":
		notImplemented("pull")
	case "deps":
		notImplemented("deps")
	case "source":
		notImplemented("source")
	case "version":
		fmt.Println("sat version", version)
	case "help", "--help", "-h":
		printHelp()
	default:
		fmt.Fprintf(os.Stderr, "sat: unknown command: %s\n", command)
		fmt.Fprintln(os.Stderr, "Run 'sat help' for usage.")
		os.Exit(1)
	}
}

func notImplemented(cmd string) {
	fmt.Printf("not implemented: %s\n", cmd)
}

func printHelp() {
	help := `     ,-.
    / \  ` + "`" + `.  __..-,O
   :   \ --''_..-'.'
   |    . .-' ` + "`" + `. '.
   :     .     .` + "`" + `.'
    \     ` + "`" + `.  /  ..
     \      ` + "`" + `.   ' .
      ` + "`" + `,       ` + "`" + `.   \
     ,|,` + "`" + `.        ` + "`" + `-.\
    '.||  ` + "``" + `-...__..-` + "`" + `
     |  |
     |__|
     /||\    Usage: sat <command>
    //||\\
   // || \\
__//__||__\\__
'--------------'

Commands:
  install|i <pkg>     - Install package(s) with optional source
  source|src <pm>     - Install a package manager (huber, cargo, brew, nix)
  search <program>    - Find package across sources (--all raw, --wrap full)
  uninstall|rm <prog> - Remove program installed via sat
  shell <tool>        - Temp shell with tools, auto-cleanup on exit (requires tmux)
  deps                - Install sat dependencies (tmux, wget, curl, jq)
  info <program>      - Source, path, version, shadowed installs (alias: which)
  list|ls             - List tracked packages (auto-cleans stale entries)
  track <program>     - Add existing program to manifest for sat management
  untrack <program>   - Remove from manifest without uninstalling
  scan                - Scan ecosystem dirs and add all found packages
  outdated            - Show available updates for tracked packages
  pull                - Refresh sat library from GitHub
  clone <repo> [dest] - Clone your repo

Source syntax (install/shell):
  pkg:sys             - System package manager (apt/pacman/etc)
  pkg:brew            - Homebrew
  pkg:nix             - Nix profile
  pkg:rs :rust        - Cargo (Rust)
  pkg:py :python      - uv (Python)
  pkg:js :node        - npm (Node)
  pkg:go              - go install

Examples:
  sat install fd:rs bat:rs ripgrep:rs
  sat shell hyperfine:brew cowsay:sys jq
`
	fmt.Print(help)
}
