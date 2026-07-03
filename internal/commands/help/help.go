package help

import (
	"github.com/DeprecatedLuar/gohelp-luar"
)

// rocketArt is the sat banner. Text() only auto-indents its first line, so
// every subsequent line carries an explicit 2-space prefix to match.
const rocketArt = `     ,-.
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
       /||\
      //||\\
     // || \\
  __//__||__\\__
  '--------------'`

// Run renders sat's help output. args is the full CLI argument list
// (e.g. os.Args[1:]) so gohelp can route "sat help <topic>".
func Run(args []string) {
	root := gohelp.NewPage("sat", "universal package installer, search tool, and temporary environment manager").
		Text(rocketArt).
		Usage("sat <command>").
		Section("Commands",
			gohelp.Item("install|i <pkg>", "Install package(s) with optional source"),
			gohelp.Item("source|src <pm>", "Install a package manager (huber, cargo, brew, nix)"),
			gohelp.Item("search <program>", "Find package across sources (--all raw, --wrap full)"),
			gohelp.Item("uninstall|rm <prog>", "Remove program installed via sat"),
			gohelp.Item("shell <tool>", "Temp shell with tools, auto-cleanup on exit (requires tmux)"),
			gohelp.Item("deps", "Install sat dependencies (tmux, wget, curl, jq)"),
			gohelp.Item("info <program>", "Source, path, version, shadowed installs (alias: which)"),
			gohelp.Item("list|ls", "List tracked packages (auto-cleans stale entries)"),
			gohelp.Item("track <program>", "Add existing program to manifest for sat management"),
			gohelp.Item("untrack <program>", "Remove from manifest without uninstalling"),
			gohelp.Item("scan", "Scan ecosystem dirs and add all found packages"),
			gohelp.Item("pulverize", "Clear entire manifest (with confirmation)"),
			gohelp.Item("outdated", "Show available updates for tracked packages"),
			gohelp.Item("pull", "Refresh sat library from GitHub"),
			gohelp.Item("clone <repo> [dest]", "Clone your repo"),
		).
		Section("Source syntax (install/shell)",
			gohelp.Item("pkg:sys", "System package manager (apt/pacman/etc)"),
			gohelp.Item("pkg:brew", "Homebrew"),
			gohelp.Item("pkg:nix", "Nix profile"),
			gohelp.Item("pkg:rs :rust", "Cargo (Rust)"),
			gohelp.Item("pkg:py :python", "uv (Python)"),
			gohelp.Item("pkg:js :node", "npm (Node)"),
			gohelp.Item("pkg:go", "go install"),
		).
		Text("Examples: sat install fd:rs bat:rs ripgrep:rs   |   sat shell hyperfine:brew cowsay:sys jq")

	gohelp.Run(args, root)
}
