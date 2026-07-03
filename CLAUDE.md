# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Overview

`sat` (Satellite) is a universal package installer, search tool, and temporary environment manager. It abstracts package installation across multiple sources with automatic fallback, provides cross-ecosystem package search, and ephemeral shell sessions with auto-cleanup.

## Current State: Bash → Go Migration

**Current implementation:** Bash (fully functional in `sat` binary + `lib/` directory)
**Target implementation:** Go (detailed in `implementation-plan.md`)

The bash implementation is production-ready and actively maintained. The Go rewrite is planned to improve performance, testability, and maintainability while preserving all functionality. See `implementation-plan.md` for the phase-by-phase migration strategy.

## Build & Development

**Bash implementation (current):**
```bash
# Install locally
cp sat ~/.local/bin/sat && chmod +x ~/.local/bin/sat

# Development workflow with live lib updates
rm -rf ~/.local/share/sat/lib
ln -s ~/Workspace/dev/sat/lib ~/.local/share/sat/lib

# Run commands directly from repo
./sat install ripgrep
./sat --debug search fd  # Debug mode shows fallback chain
```

**Go implementation (planned):**
```bash
# Once Phase 1+ complete:
go build -o sat ./cmd/sat
go test ./...
go vet ./...
```

Binary changes require copying to `~/.local/bin/sat`. Library changes (in symlinked `lib/`) are immediately active.

## Architecture (Go Target)

**Planned Go structure** (see `implementation-plan.md` for implementation phases):

```
cmd/sat/
  main.go                   # CLI router with manual switch statement (no cobra)
internal/
  manifest/
    source.go              # Source string parsing (build/get type/identity/version)
    manifest.go            # System manifest operations
    master.go              # Shell master manifest (multi-session tracking)
    session.go             # Per-PID session manifest
    track.go               # Context-aware install tracking (routes via SAT_MANIFEST_TARGET)
    paths.go               # Manifest file paths with SAT_DATA override
  sources/
    system.go              # Distro package managers (pacman, apt, dnf, apk)
    cargo.go               # Rust crates.io
    npm.go                 # Node packages
    pypi.go                # PyPI search (uv install/uninstall/update not yet ported, see lib/sources/uv.sh)
    goinstall.go           # Go packages
    brew.go                # Homebrew
    nix.go                 # Nix packages
    flatpak.go             # Flatpak apps with wrapper generation
    github.go              # GitHub releases (huber, appimage, go, python methods)
    appimage.go            # AppImage extraction and installation
  commands/
    install.go             # sat install with fallback chain
    search.go              # Parallel multi-ecosystem search
    list.go                # Display tracked packages
    scan.go                # Scan ecosystems and populate manifest
    uninstall.go           # Remove packages
    update.go              # Update packages
    outdated.go            # Check for updates
    shell.go               # Ephemeral tmux sessions
    track.go               # Manual manifest add/remove
    info.go                # Package metadata display
  scan/
    exclusion.go           # Infrastructure pattern filtering
    system.go              # Per-distro system package scanning
  common/
    detect.go              # Binary source detection, tool spec parsing
    os.go                  # OS detection cache, package existence checks
    run.go                 # Quiet command execution, JSON fetching
    cleanup.go             # Orphaned session cleanup
  ui/
    colors.go              # ANSI color constants (~30 lines)
    ui.go                  # Source mapping + display formatting (~100 lines)
    spinner.go             # Animation logic (~60 lines)
```

**Key Go design decisions:**
- Manual CLI routing (no cobra) for simplicity and startup performance
- Context-aware manifest routing via `SAT_MANIFEST_TARGET` env var (same as bash)
- Source contract (free functions per source, e.g. `CargoInstall`/`AptSearch`, dispatched via `switch` in `commands/`): a required core (`Install`/`Uninstall`/`GetVersion`) present on nearly every source, plus optional per-source capabilities (`Search`, `QueryLatestVersion`, `CheckOutdated`, `Update`) that not all sources implement — mirrors bash, where e.g. `query_latest_version_X` only exists for sources with a cheap pre-install registry lookup (npm/cargo/uv/brew/github), and `search_X` doesn't exist for appimage/go/sat since they aren't independently discoverable registries. Not every source needs every method; dispatch sites only call what a given source actually has.
- Parallel search via goroutines + sync.WaitGroup
- No daemon/background process - all commands are synchronous
- XDG paths with `SAT_DATA` override for testing

## Architecture (Bash Current)

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
├── manifest                    # System manifest: tool=source:identity:version
├── bin/appimages/              # AppImage storage (symlinked to ~/.local/bin/)
└── shell/
    ├── manifest                # Master manifest: tool:source:identity:version:pid
    └── $PID/
        ├── manifest            # Session manifest: TOOL=x, SOURCE_x=source:identity:version
        └── snapshot-*          # Config snapshots

/tmp/sat-$PID/                  # XDG override (ephemeral)
```

### Source String Format

All manifests use structured source strings: `source:identity:version`

**Examples:**
- `cargo::14.1.0` - Cargo package, no identity, version 14.1.0
- `gh:owner/repo:v1.0.0` - GitHub repo, identity=owner/repo, version v1.0.0
- `flatpak:com.app.id:1.2.3` - Flatpak app, identity=app ID, version 1.2.3
- `nixos::` - NixOS package, no identity/version tracked

**Parsing helpers** (in `lib/manifest.sh`):
- `get_source_type(str)` - Extract source type (first field)
- `get_source_identity(str)` - Extract identity (second field)
- `get_source_version(str)` - Extract version (third field)
- `build_source_string(source, identity, version)` - Construct from components

### Manifest Types

| Manifest | Location | Format | Purpose |
|----------|----------|--------|---------|
| System | `sat/manifest` | `tool=source:identity:version` | Permanent installs via `sat install` |
| Master | `sat/shell/manifest` | `tool:source:identity:version:pid` | All active session tools |
| Session | `sat/shell/$PID/manifest` | `TOOL=x`, `SOURCE_x=source:identity:version` | Per-session details |

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
- `track_install(tool, source, [identity], [version])` - routes to appropriate manifest based on `SAT_MANIFEST_TARGET`

**Display helpers** (in `lib/common.sh`):
- `source_display(source_str)` - Convert source type to display name (e.g., `npm` → `node`)
- `source_color(source_str)` - Get ANSI color for source type
- `source_light(source_str)` - Get pastel color for package names
- `display_tool_entry(prog, source_str, [prefix], [suffix])` - Formatted output with version

All functions auto-detect context (binary vs remote) and route appropriately. Display functions parse source strings via `get_source_type()` to extract just the source type.

## Internal Manifest Implementation

The binary exposes `sat internal <subcommand>` for manifest manipulation. Lib files use this API when running remotely (e.g., inside tmux sessions) to ensure all manifest operations go through a single source of truth.

**Architecture:**
```
Binary (sat)
├── _sat_manifest_*()      # Internal functions (fast, in-process)
├── _shell_manifest_*()
├── _pid_manifest_*()
└── internal)              # CLI API entry point

manifest.sh (wrappers)
├── manifest_*()           # Check for _* functions, else call API
├── master_*()
└── pid_manifest_*()

lib/*.sh
└── Call wrapper functions (works in binary context and remote)
```

**Load order in binary:**
1. `common.sh` - Colors, utilities, display functions
2. `internal.sh` - Internal manifest functions (`_*`)
3. `manifest.sh` - Wrappers + parsing helpers (loaded early for display functions)

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

## Scan System

### Exclusion Architecture

Centralized exclusion logic in `lib/scan/exclusion.sh` prevents infrastructure noise from polluting manifests.

**Default exclusions** (embedded in code):
- Filesystem utilities (xfs_*, btrfs-*, fsck.*, mkfs.*)
- Network/WiFi infrastructure (wpa_*, dhcp*, iw*)
- Bluetooth utilities (bluez-*, bluetooth*)
- System services (systemd*, udev*, polkit*)
- Low-level X11/audio tools (xdg-*, pulse*, alsa-*)

**User customization:**
Users can extend defaults via `~/.config/sat/exclude` (glob patterns):
```bash
# My custom exclusions
gcc*
g++*
steam*
```

**Implementation:**
```bash
is_excluded() {
    local prog="$1" src="$2"
    # Check user + default patterns (glob matching)
    for pattern in "${EXCLUSION_PATTERNS[@]}"; do
        [[ "$prog" == $pattern ]] && return 0
    done
    # Then check hardcoded safety patterns per source
}
```

All scan sources (pacman, apt, cargo, etc.) use `is_excluded()` for consistent filtering.

### OS Detection

Uses `/etc/os-release` with smart fallback for derivative distros.

**Primary detection:** `ID_LIKE` field (e.g., CachyOS sets `ID_LIKE=arch`)
- Automatically supports any distro declaring its base
- No code changes needed for new derivatives

**Fallback:** Direct `ID` matching for base distros without `ID_LIKE`

**Cache:** Results stored in `~/.local/share/sat/os-info`:
```bash
SAT_OS=linux
SAT_DISTRO=cachyos
SAT_DISTRO_FAMILY=arch
SAT_PKG_MANAGER=pacman
```

Detection script lives in separate repo: `the-satellite/internal/os_detection.sh`

### System Package Scanning

**Performance optimization for Arch/pacman:**
```bash
# OLD: Call pacman -Qo for every binary (thousands of calls, minutes)
for bin in /usr/bin/*; do
    package=$(pacman -Qo "$bin")
done

# NEW: Single pipeline (batched query, ~500ms)
pacman -Qeq | xargs pacman -Ql | awk '/\/usr\/bin\// {print $2}' | while read bin; do
    _try_add_tool "$(basename "$bin")" "pacman"
done
```

**Scan strategy:**
- Query explicit packages only (`pacman -Qeq`, `apt-mark showmanual`, etc.)
- Use exclusion patterns to filter infrastructure
- Track actual binaries, not packages (one package → many binaries)

**Per-distro scanners** in `lib/scan/system.sh`:
- `_scan_pacman()` - Arch/CachyOS/Manjaro
- `_scan_apt()` - Debian/Ubuntu derivatives
- `_scan_dnf()` - Fedora/RHEL derivatives
- `_scan_apk()` - Alpine Linux
- `_scan_nixos()` - NixOS system packages

## Migration Strategy (Bash → Go)

**Implementation phases** (detailed in `implementation-plan.md`):

1. **Phase 1-2:** Go skeleton + manifest system (data layer foundation)
2. **Phase 3:** UI layer + common utilities (import-safe, no circular deps)
3. **Phase 4-6:** Source modules (system/cargo/brew/nix → npm/uv/go/flatpak → github/appimage)
4. **Phase 7-11:** Command implementations (install → list/track/info → scan → search → uninstall/update/outdated)
5. **Phase 12:** Shell (tmux ephemeral sessions, most complex)
6. **Phase 13:** Distribution (GitHub Actions, satellite cargo-bay integration)

**Critical constraints:**
- Import order: `manifest` → `pkg/ui` → `internal/common` → `sources` → `commands` (no circular deps)
- Manifest operations MUST go through `internal/manifest` package (single source of truth)
- Source modules implement a required core (Install/Uninstall/GetVersion) plus whichever optional capabilities apply to that source (Search/QueryLatestVersion/CheckOutdated/Update) — not a uniform 7-method interface; partial coverage is intentional per source, matching bash
- Display helpers in `pkg/ui` parse source strings via `manifest.GetSourceType` (never duplicate parsing)
- Debug output to stderr with `[debug]` prefix when `SAT_DEBUG` set
- Use `RunQuiet()` for suppressed output (visible in debug mode)

**Testing during migration:**
- Each phase has explicit success criteria (see implementation-plan.md)
- Bash implementation remains production until Go rewrite is feature-complete
- Both versions share same manifest format (seamless transition)

## Development

**Installation:**
```bash
# One-line install
curl -sSL https://raw.githubusercontent.com/DeprecatedLuar/sat/main/install.sh | bash

# Manual install
cp sat ~/.local/bin/sat
chmod +x ~/.local/bin/sat
```

The binary (`sat`) is a bash script that auto-downloads its library on first run via `_ensure_lib()`.

**Library development symlink:**
```bash
rm -rf ~/.local/share/sat/lib
ln -s ~/Workspace/dev/sat/lib ~/.local/share/sat/lib
```

Changes to lib files are immediately active (no rebuild needed). Binary changes require copying to `~/.local/bin/sat`. Run `sat pull` to restore production library.

## Dependencies

**Core:** `tmux`, `jq`, `curl`
**Recommended:** `gh` (GitHub CLI) for authenticated API access
**Optional:** cargo, uv, npm, go, brew, nix, huber

Run `sat deps` to install core dependencies.
