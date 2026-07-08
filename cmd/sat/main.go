package main

import (
	"fmt"
	"os"

	"github.com/DeprecatedLuar/sat/internal/commands"
	"github.com/DeprecatedLuar/sat/internal/commands/help"
	"github.com/DeprecatedLuar/sat/internal/selfheal"
)

const (
	// Environment variables
	EnvSATDebug = "SAT_DEBUG"

	// Debug flag value
	DebugValue = "1"

	// Global flags
	FlagDebug = "--debug"

	// Repository
	githubRepo = "DeprecatedLuar/sat"
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
		help.Run(args)
		return
	}

	command := args[0]
	commandArgs := args[1:]

	_ = debug // Will be used in later phases
	_ = commandArgs

	switch command {
	case "install", "i":
		if err := commands.Install(commandArgs); err != nil {
			fmt.Fprintf(os.Stderr, "sat: install failed: %v\n", err)
			os.Exit(1)
		}
	case "search":
		if err := commands.Search(commandArgs); err != nil {
			fmt.Fprintf(os.Stderr, "sat: search failed: %v\n", err)
			os.Exit(1)
		}
	case "uninstall", "remove", "rm":
		if err := commands.Uninstall(commandArgs); err != nil {
			fmt.Fprintf(os.Stderr, "sat: uninstall failed: %v\n", err)
			os.Exit(1)
		}
	case "shell":
		notImplemented("shell")
	case "list", "ls":
		if err := commands.List(commandArgs); err != nil {
			fmt.Fprintf(os.Stderr, "sat: list failed: %v\n", err)
			os.Exit(1)
		}
	case "track":
		notImplemented("track")
	case "untrack":
		notImplemented("untrack")
	case "scan":
		if err := commands.Scan(); err != nil {
			fmt.Fprintf(os.Stderr, "sat: scan failed: %v\n", err)
			os.Exit(1)
		}
	case "pulverize":
		if err := commands.Pulverize(); err != nil {
			fmt.Fprintf(os.Stderr, "sat: pulverize failed: %v\n", err)
			os.Exit(1)
		}
	case "outdated":
		notImplemented("outdated")
	case "update":
		if err := commands.HandleUpdate(commandArgs, version, githubRepo); err != nil {
			fmt.Fprintf(os.Stderr, "sat: %v\n", err)
			os.Exit(1)
		}
	case "info", "which", "whereis":
		notImplemented("info")
	case "clone":
		notImplemented("clone")
	case "pull":
		notImplemented("pull")
	case "deps", "dependencies":
		notImplemented("deps")
	case "source", "src":
		if err := commands.Source(commandArgs); err != nil {
			fmt.Fprintf(os.Stderr, "sat: %v\n", err)
			os.Exit(1)
		}
	case "version":
		fmt.Println("sat version", version)
	case "help", "--help", "-h":
		help.Run(args)
	default:
		fmt.Fprintf(os.Stderr, "sat: unknown command: %s\n", command)
		fmt.Fprintln(os.Stderr, "Run 'sat help' for usage.")
		os.Exit(1)
	}
}

func notImplemented(cmd string) {
	fmt.Printf("not implemented: %s\n", cmd)
}

