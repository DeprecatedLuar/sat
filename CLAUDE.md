# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Overview

`sat` (Satellite) is a universal package installer, search tool, and temporary environment manager. It abstracts package installation across multiple sources with automatic fallback, provides cross-ecosystem package search, and ephemeral shell sessions with auto-cleanup.

## Architecture

```
sat (router + offline commands)
├── sat_uninstall()        # Local package removal
├── sat_list()             # Show tracked packages (system + sessions)
├── sat_scan()             # Scan ecosystem dirs (skips session tools)
└── [ROUTER]               # Routes to library for online commands

lib/
├── common.sh              # Shared utilities, manifests, cleanup functions
├── install.sh             # sat_install() - orchestrates installation
├── search.sh              # sat_search() + per-ecosystem search functions
├── shell.sh               # sat_shell() - ephemeral environment
└── installation/          # Modular installers (one per package manager)
    ├── brew.sh            # install_brew()
    ├── cargo.sh           # install_cargo()
    ├── github.sh          # install_from_github(), language-based routing
    ├── go.sh              # install_go()
    ├── nix.sh             # install_nix()
    ├── npm.sh             # install_npm()
    ├── sat.sh             # install_sat()
    ├── system.sh          # install_system()
    └── uv.sh              # install_uv()
```

### Binary vs Library Split
- **Binary**: Offline-capable commands (list, scan, uninstall, track, which, info)
- **Library**: Internet-dependent commands (install, search, shell)

### Installation Fallback Chain
Permanent installs (system-first for stability):
1. system (apt/pacman/dnf/apk)
2. brew
3. nix
4. cargo
5. uv (Python)
6. npm
7. repo (GitHub install.sh)
8. sat (wrapper scripts)

Shell installs (isolated-first):
1. brew, nix, cargo, uv (user-space)
2. system
3. npm, repo, sat

## Data Storage

```
~/.local/share/sat/
├── manifest                    # System manifest (permanent): tool=source
└── shell/
    ├── manifest                # Master manifest (sessions): tool:source:pid
    └── $PID/                   # Session folder
        ├── manifest            # Session manifest: TOOL=x, SOURCE_x=y
        ├── snapshot-before     # Config snapshot before session
        └── snapshot-after      # Config snapshot after session

/tmp/sat-$PID/                  # XDG override (ephemeral)
└── {config,data,cache,state}
```

### Manifest Types
| Manifest | Location | Format | Purpose |
|----------|----------|--------|---------|
| System | `sat/manifest` | `tool=source` | Permanent installs via `sat install` |
| Master | `sat/shell/manifest` | `tool:source:pid` | All active session tools |
| Session | `sat/shell/$PID/manifest` | `TOOL=x`, `SOURCE_x=y` | Per-session details |

### Key Rules
- **System vs Master isolation**: A `tool:source` combo lives in ONE manifest only
- **Promotion**: `sat install` moves tool from master → system
- **sat scan**: Skips any tool in master manifest (prevents pollution)
- **Cleanup**: Master manifest is source of truth for orphan cleanup

## Session Lifecycle

### Starting a shell
1. Check for tmux
2. Cache sudo if any `:sys` tools requested
3. Create session dir + XDG temp
4. Take config snapshot
5. Spawn tmux with rcfile that installs tools
6. Tools written to session manifest + master manifest

### Clean exit (`exit`)
1. `shell_cleanup()` runs in shell.sh
2. Check each tool: other sessions using it? promoted to system?
3. Remove tools not shared/promoted
4. Clean configs via snapshot diff
5. Delete session dir + XDG temp

### Orphan cleanup (crash/kill)
1. `cleanup_orphaned_sessions()` runs on every sat command
2. Scan master manifest for dead PIDs
3. Cache sudo if system packages need removal
4. For each dead PID: `cleanup_session()` handles everything

## Key Functions

### Manifest helpers (common.sh)
```bash
# System manifest
manifest_add()      # Add tool=source
manifest_get()      # Get source for tool
manifest_remove()   # Remove tool

# Master manifest
master_add()        # Add tool:source:pid
master_has_tool()   # Check if tool exists (any source)
master_remove()     # Remove specific tool:source:pid
master_promote()    # Move tool:source from master → system
```

### Session cleanup (common.sh)
```bash
session_remove_tool()       # Remove one tool: pkg_remove + master_remove
cleanup_session()           # Full session: configs + tools + folders
cleanup_orphaned_sessions() # Find dead PIDs, cleanup each
```

### Snapshot cleanup (common.sh)
```bash
take_snapshot()             # Capture ~/.config, ~/.local/share, etc.
cleanup_session_configs()   # Diff before/after, remove matching tool dirs
```

## Commands

```bash
sat install <pkg>              # Install (promotes from master if exists)
sat install <pkg>:sys          # Force system package manager
sat install <pkg>:brew         # Force homebrew
sat install <pkg>:nix          # Force nix
sat install <pkg>:rs           # Force cargo (alias: :rust)
sat install <pkg>:py           # Force uv/python (alias: :python)
sat install <pkg>:js           # Force npm (alias: :node)
sat install <pkg>:go           # Force go install
sat install <pkg>:gh           # Force GitHub search + install
sat install <pkg>:rel          # Force GitHub release binary (alias: :release)
sat install <pkg>:sh           # Force GitHub install.sh script (alias: :script)
sat install owner/repo         # Direct GitHub repo install

sat shell <pkg>[:source] ...   # Ephemeral shell with tools
sat uninstall <pkg>            # Remove and update manifest
sat list                       # Show sessions + system tools
sat scan                       # Scan ecosystems, skip session tools
sat which <pkg>                # Show all installations across sources
sat info <pkg>                 # Detailed info (source, path, version)
sat track <pkg>                # Add existing program to manifest
sat untrack <pkg>              # Remove from manifest without uninstalling
sat source <pm>                # Install a package manager (huber, cargo, brew, nix)
sat search <pkg>               # Cross-ecosystem package search
sat search <pkg>:gh            # Search specific source (gh, cargo, npm, etc.)
```

## Testing

```bash
# Test shell + cleanup
sat shell cowsay:sys figlet:brew hyperfine:cargo
# ... use tools ...
exit  # or kill terminal

# Verify cleanup
sat list                              # Should show no session tools
sat which cowsay figlet hyperfine     # Should be gone
ls ~/.local/share/sat/shell/          # Session dir gone
ls /tmp/sat-*                         # XDG temp gone

# Test promotion
sat shell ripgrep:cargo
# inside shell:
sat install ripgrep                   # Promotes to system
exit
sat which ripgrep                     # Still installed (system manifest)
```

## Error Handling

- **No silent failures**: All errors visible with context
- **Sudo caching**: Requested once before shell/cleanup if `:sys` packages involved
- **Master-first cleanup**: Handles stale entries even without session dir

## Dependencies

- `tmux` - Required for shell isolation
- `jq` - JSON parsing for API responses
- `curl` - HTTP requests
- Optional: cargo, uv, npm, go, brew, nix for respective sources

Run `sat deps` to install core dependencies.

## Remote Sourcing

The tool sources scripts from a remote repository for OS detection and bootstrapping:
- `SAT_BASE=https://raw.githubusercontent.com/DeprecatedLuar/the-satellite/main`
- `internal/os_detection.sh` - OS/distro detection (sourced in common.sh)
- `internal/fetcher.sh` - Sat wrapper installer
- `cargo-bay/programs/*.sh` - Sat wrapper scripts

## GitHub Install Methods

When installing from GitHub (`sat install owner/repo` or `<pkg>:gh`):

1. **Search** returns repo + language from GitHub API
2. **Huber** tried first (prebuilt release binaries)
3. **Language-based routing** if Huber fails:
   - `Go` → `go install`
   - `Python` → `uv tool install`
4. **Script** fallback - run install.sh if present

Search functions in `search.sh` are reused by both `sat search` and install.
