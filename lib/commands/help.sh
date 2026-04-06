sat_help() {
    cat << 'EOF'
         ,-.
        / \  `.  __..-,O
       :   \ --''_..-'.'
       |    . .-' `. '.
       :     .     .`.'
        \     `.  /  ..
         \      `.   ' .
          `,       `.   \
         ,|,`.        `-.\
        '.||  ``-...__..-`
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
EOF
}
