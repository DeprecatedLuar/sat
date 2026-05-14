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
├── common.sh              # Shared utilities, UI, cleanup functions, _run_quiet()
├── manifest.sh            # Unified manifest API (low-level + high-level)
├── commands/
│   ├── install.sh         # sat_install() - uses track_install() from manifest.sh
│   ├── search.sh          # sat_search() + per-ecosystem search functions
│   ├── shell.sh           # sat_shell() - thin wrapper around sat_install
│   ├── internal.sh        # Manifest internals (_sat_manifest_*, API router)
│   └── ...                # pull, update, clone, list, scan, uninstall, track, etc.
└── sources/               # Modular installers (one per package manager)
    ├── github.sh          # install_from_github(), AppImage, language-based routing
    └── ...                # brew, cargo, go, nix, npm, sat, system, uv, flatpak
```

### Key Concepts

**Shell as Thin Wrapper:**
Shell delegates to `sat_install`, only handling:
- Isolation (tmux session, XDG overrides, session directories)
- Context (sets `SAT_MANIFEST_TARGET=session` for manifest routing)
- Cleanup (removes tools and configs on exit)

**Binary vs Library Split:**
- **Binary**: Offline-capable commands (list, scan, uninstall, track, which, info) + router + `_ensure_lib()`
- **Library**: Internet-dependent commands (install, search, shell, pull, update, clone)

**Installation Fallback Chain:**
- Permanent: brew → nix → system → cargo → uv → npm → sat → gh
- Shell: brew/nix/cargo/uv → system → npm/repo/sat

## Data Storage

```
~/.local/share/sat/
├── manifest                    # System manifest: tool=source
├── bin/appimages/              # AppImage storage (symlinked to ~/.local/bin/)
└── shell/
    ├── manifest                # Master manifest: tool:source:pid
    └── $PID/
        ├── manifest            # Session manifest: TOOL=x, SOURCE_x=y
        └── snapshot-*          # Config snapshots

/tmp/sat-$PID/                  # XDG override (ephemeral)
```

### Manifest Types

| Manifest | Location | Format | Purpose |
|----------|----------|--------|---------|
| System | `sat/manifest` | `tool=source` | Permanent installs via `sat install` |
| Master | `sat/shell/manifest` | `tool:source:pid` | All active session tools |
| Session | `sat/shell/$PID/manifest` | `TOOL=x`, `SOURCE_x=y` | Per-session details |

**Key Rules:**
- A `tool:source` combo lives in ONE manifest only (system XOR master)
- `sat install` promotes tool from master → system
- `sat scan` skips tools in master manifest (prevents pollution)
- Master manifest is source of truth for orphan cleanup

## Session Lifecycle

**Manifest Routing:**
`SAT_MANIFEST_TARGET` env var controls where `_track_install()` writes:
- Unset (default): System manifest (permanent)
- `session`: Session + master manifest (temporary)

Shell sets `SAT_MANIFEST_TARGET=session` before calling `sat_install`, routing the same installation logic to different manifests based on context.

**Starting:** Check tmux → cache sudo if `:sys` tools → create dirs + XDG temp → snapshot configs → spawn tmux with rcfile that sets env vars and calls `sat_install`

**Clean exit:** `shell_cleanup()` checks each tool (shared with other sessions? promoted?), removes unshared tools, cleans configs via snapshot diff, deletes session dir

**Orphan cleanup:** `cleanup_orphaned_sessions()` runs on every sat command, scans master manifest for dead PIDs, runs `cleanup_session()` for each

## Manifest API Architecture

All manifest operations are centralized in `lib/manifest.sh`, providing a clean two-layer API:

**Low-level API** (direct manifest access):
- `manifest_add(tool, source)`, `manifest_get(tool)`, `manifest_has(tool)`, `manifest_remove(tool)`
- `master_add(tool, source, pid)`, `master_promote(tool, source)`, `master_has_tool(tool)`, etc.
- `pid_manifest_add(pid, tool, source)`, `pid_manifest_tools(pid)`, etc.

**High-level API** (context-aware):
- `track_install(tool, source)` - routes to appropriate manifest based on `SAT_MANIFEST_TARGET`

All functions auto-detect context (binary vs remote) and route appropriately.

## Internal Manifest Implementation

The binary exposes `sat internal <subcommand>` for manifest manipulation. Lib files use this API when running remotely (e.g., inside tmux sessions) to ensure all manifest operations go through a single source of truth.

**Architecture:**
```
Binary (sat)
├── _sat_manifest_*()      # Internal functions (fast, in-process)
├── _shell_manifest_*()
├── _pid_manifest_*()
└── internal)              # CLI API entry point

common.sh (wrappers)
├── manifest_*()           # Check for _* functions, else call API
├── master_*()
└── pid_manifest_*()

lib/*.sh
└── Call wrapper functions (works in binary context and remote)
```

When binary runs: wrappers use `_*` functions directly (no subprocess)
When lib runs in tmux: `_*` functions don't exist → fall back to `sat internal ...`

**Future:** Library designed to be hosted remotely (`curl | bash`), with binary-only local install. Architecture already supports this — wrappers auto-detect context.

## Code Patterns & Best Practices

### Output Suppression

Use `_run_quiet` (defined in `common.sh`) instead of `&>/dev/null`:

```bash
_run_quiet cargo install "$tool"   # suppressed normally, visible with --debug
# NOT: cargo install "$tool" &>/dev/null
```

### Debug Output

When `SAT_DEBUG` is set (via `sat --debug`), output diagnostic info to stderr:

```bash
[[ -n "$SAT_DEBUG" ]] && echo "[debug] trying appimage for $repo_path" >&2
```

**Conventions:**
- `[debug]` prefix, indented for call hierarchy
- Write to stderr (`>&2`)
- Show fallback chain and key decisions
- Export `SAT_DEBUG` for propagation

### Bash Parser Limitations

**Critical bug** when combining:
1. `declare -A` (associative arrays)
2. Nested function definitions
3. Control structures (`while`, `if`) inside nested functions

```bash
# ❌ Syntax error:
func() {
    declare -A arr=(["key"]="val")
    nested() {
        while [[ condition ]]; do  # ← Parser bug!
            ...
        done
    }
}

# ✅ Solution: Helper functions with explicit mappings
func() {
    nested() { ... }  # Define FIRST

    _process() {
        local key="$1" val="$2"
        # Explicit mapping, no arrays
    }

    _process "cargo" "$cargo_bin"
}
```

Example: `lib/commands/scan.sh` uses `_scan_dir()` helper instead of `declare -A scan_dirs`.

### DRY Principles

Extract repeated code into helpers:

```bash
# lib/commands/scan.sh
_try_add_tool() {
    local prog="$1" src="$2"
    is_excluded "$prog" "$src" && return 1
    [[ -n "$(_sat_manifest_get "$prog")" ]] && return 1
    _shell_manifest_has "$prog" && return 1
    _sat_manifest_add "$prog" "$src"
    # ... display logic
}

_try_add_tool "$prog" "$src" && ((added++))  # Used everywhere
```

### Function Organization

In files with nested functions:
1. Define helper functions FIRST (before complex data structures)
2. Keep helpers small and focused (single responsibility)
3. Use explicit parameter passing over globals

## GitHub Install Methods

Automatic fallback chain (`install_github` with `method=auto`):
1. **Huber** - Prebuilt release binaries
2. **AppImage** - Portable GUI apps from releases
3. **Language-based routing** - Go → `go install`, Python → `uv tool install`
4. **Script** - Run `install.sh` if present

**AppImage installation:**
- Platform detection (x86_64, aarch64, armv7l)
- Downloads to `~/.local/share/sat/bin/appimages/`
- Symlinks to `~/.local/bin/` with cleaned name (lowercase, version stripped)
- Manifest: `tool=appimage:owner/repo`

**GitHub API:**
- Uses `gh api` when authenticated (higher rate limits)
- Falls back to `curl` when unauthenticated
- Search functions in `search.sh` reused by both `sat search` and install

## Search System

`sat search` queries multiple ecosystems in parallel (system, flatpak, brew, nix, cargo, pypi, npm, github). Each has dedicated search logic in `lib/search.sh`. Color-coded output with version normalization.

## Development

**Library development symlink:**
```bash
rm -rf ~/.local/share/sat/lib
ln -s ~/Workspace/dev/sat/lib ~/.local/share/sat/lib
```

Changes to lib files are immediately active. Run `sat pull` to restore production library.

## Dependencies

**Core:** `tmux`, `jq`, `curl`
**Recommended:** `gh` (GitHub CLI) for authenticated API access
**Optional:** cargo, uv, npm, go, brew, nix, huber

Run `sat deps` to install core dependencies.
