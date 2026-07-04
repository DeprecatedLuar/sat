# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Overview

`sat` (Satellite) is a universal package installer, cross-ecosystem search tool, and ephemeral shell-session manager. It abstracts package installation across multiple sources (system package managers, cargo, npm, uv, go, brew, nix, flatpak, GitHub releases, AppImage) with automatic fallback, and provides tmux-based temporary environments with auto-cleanup on exit.

## Current State: Go Rewrite In Progress

`sat` is being rewritten from bash (`lib/`) to Go (`cmd/`, `internal/`). **The Go binary is what ships and is committed at `./sat`** — the bash implementation under `lib/` is kept only as a behavioral reference during the port (same manifest format, same fallback semantics) and is not run in production anymore.

Implemented in Go: `install`, `search`, `scan`, `pulverize`, `update`, `uninstall`/`remove`/`rm`, `list`/`ls`.
Still stubs (`cmd/sat/main.go` prints "not implemented"): `shell`, `track`, `untrack`, `outdated`, `info`/`which`/`whereis`, `clone`, `pull`, `deps`/`dependencies`, `source`/`src`. `outdated`'s scan logic already exists inside `update.go` but isn't yet exposed as its own read-only command.

See `implementation-plan.md` for the active phase and `implementation-history.md` for what's already landed — check both before assuming a feature is missing or planned a certain way, they're the source of truth over this file.

## Build & Development

```bash
go build -o sat ./cmd/sat   # binary is committed directly; rebuild and it's live via $PATH
go test ./...
go vet ./...
go test ./internal/sources/ -run TestName   # single test
```

Bash reference implementation (do not add new features here, only consult for parity):
```bash
rm -rf ~/.local/share/sat/lib
ln -s ~/Workspace/dev/sat/lib ~/.local/share/sat/lib   # live-edit symlink, no rebuild needed
./sat --debug search fd   # --debug shows the fallback chain being tried
```

## Architecture (Go)

```
cmd/sat/main.go            # manual switch-based CLI router (no cobra), --debug flag, calls selfheal.Run() first
internal/
  selfheal/                # idempotent startup: creates dirs, manifest, config.toml, os-cache if missing
  config/                  # ~/.config/sat/config.toml — install fallback order, BurntSushi/toml
  manifest/                # source string parsing + system manifest CRUD (master/session manifests not yet ported)
  sources/                 # one file per package manager: Install/Uninstall/GetVersion required;
                            # Search/QueryLatestVersion/CheckOutdated/Update optional per source (not every
                            # source can support all of them, e.g. appimage/go/sat aren't searchable registries)
  sources/github/          # GitHub mechanics (API fetch, release/tree inspection, huber/go/python installers,
                            # fuzzy search+disambiguation); internal/sources/github.go is a thin orchestrator
                            # over this package + AppImage install, kept as two files to avoid an import cycle
  scanner/                 # ecosystem scanning + exclusion-pattern filtering, populates manifest
  commands/                # one file per CLI command, dispatched from main.go's switch
  common/                  # binary source detection, OS-family cache, quiet command execution
  ui/                      # ANSI colors, source-to-display-name mapping, spinners
```

**Design decisions:**
- Manual routing, not cobra — startup performance and simplicity.
- Source contract is intentionally non-uniform: dispatch sites only call the optional methods a given source actually implements.
- Install fallback order lives in user-editable `~/.config/sat/config.toml` (Go-only; bash hardcodes `INSTALL_ORDER`). Sources omitted from the array are skipped entirely, not just deprioritized.
- `SAT_DATA` env var overrides all data paths (used for test isolation).
- AppImage installs also generate a desktop entry/icon (`internal/sources/appimage_desktop.go`) by reading the embedded squashfs via `unsquashfs` at a computed ELF offset — deliberately never executes the AppImage, since `--appimage-extract` can be hijacked by OS-level wrappers (e.g. NixOS's `appimage-run` via binfmt_misc) into launching the app instead of extracting quietly. Optional/non-fatal: warns visibly if `unsquashfs` is missing rather than failing or silently skipping.

## Manifest & Data Storage

Source strings are `source:identity:version`, e.g. `cargo::14.1.0`, `gh:owner/repo:v1.0.0`, `nixos::`.

```
~/.local/share/sat/
├── manifest                    # tool=source:identity:version — permanent installs
├── bin/appimages/              # AppImage storage, symlinked to ~/.local/bin/
│   ├── applications/           # Go-only: .desktop files, symlinked into ~/.local/share/applications/sat-<name>.desktop
│   └── icons/
└── shell/                      # bash-only today: master + per-PID session manifests (see below)
```

A `tool:source` combo lives in exactly one manifest (system XOR session/master); `sat install` promotes a tool from session to system. This routing is controlled by `SAT_MANIFEST_TARGET` (unset = system, `session` = session+master), set by the shell wrapper before calling install — same install logic, different manifest destination.

## Bash Reference (`lib/`)

Kept for behavioral parity during the Go port — not where new features go.

- `sat.bash` / `lib/common.sh` / `lib/manifest.sh` — router, shared utils (`_run_quiet`), manifest API.
- `lib/commands/*.sh` — one file per command; `shell.sh` is a thin wrapper that sets `SAT_MANIFEST_TARGET=session` and delegates to `install.sh`.
- `lib/sources/*.sh` — one installer per package manager, same split as the Go `sources/` package.
- Session lifecycle (not yet ported to Go): start → cache sudo if needed → tmux + XDG override → snapshot configs → run in session; exit → diff snapshot, remove unshared tools, delete session dir; every `sat` invocation also sweeps master manifest for dead PIDs and cleans orphans.

**Bash parser gotcha** (relevant if touching `lib/`): combining `declare -A`, a nested function definition, and a control-flow structure (`while`/`if`) *inside* that nested function is a real bash parser bug, not a logic error. Fix is a flat helper function with explicit params instead of an associative array (see `lib/commands/scan.sh`'s `_scan_dir()`).

## Conventions

- Suppress command output with `RunQuiet()` (Go) / `_run_quiet` (bash) instead of `&>/dev/null` — both stay silent normally but print under `--debug`.
- Debug tracing: `[[ -n "$SAT_DEBUG" ]] && echo "[debug] ..." >&2` (bash) or check `SAT_DEBUG` (Go) — always stderr, `[debug]` prefix, used to show fallback-chain decisions.

## Dependencies

Core: `tmux`, `jq`, `curl`. Recommended: `gh` (authenticated GitHub API). Optional per-source: cargo, uv, npm, go, brew, nix, huber, `unsquashfs` (AppImage desktop entries).
