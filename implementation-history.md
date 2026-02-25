<!-- implementation-history.md -->
<!-- Completed phases moved from implementation-plan.md in chronological order.
Use this to understand the evolution and building blocks of this project.

FORMAT REQUIREMENTS:
- Keep entries concise - focus on WHAT was implemented, not HOW
- Avoid code snippets and technical implementation details
- No emojis
- Structure: brief description of what was accomplished and why
- This is a historical record, not a technical reference
-->

# Implementation History

## Phase 1: Repo Restructure

Reorganized repository into modular structure with separate command and installation directories. Created lib/commands/ for high-level commands (install, search, shell) while keeping lib/installation/ for package manager modules (9 installers: brew, cargo, github, go, nix, npm, sat, system, uv). Updated all source paths across binary and library files to reference new locations. This improves maintainability and parallel development.

---

## Phase 2: OS Detection Caching

Eliminated network dependency for OS detection by replacing remote curl calls with one-time cached file. Added _ensure_os_info() function that fetches os_detection.sh on first run, caches 4 variables (SAT_OS, SAT_DISTRO, SAT_DISTRO_FAMILY, SAT_PKG_MANAGER) to ~/.local/share/sat/os-info, then uses cached values on subsequent runs. Removed get_pkg_manager() function and replaced all calls with direct variable references. System now works offline after initial setup and executes faster.

---

## Phase 3: Library Bootstrap System

Transformed sat binary into pure router with automatic library fetching. Added SAT_LIBRARY constant pointing to XDG data directory and _ensure_lib() function that validates library presence by checking for common.sh. When missing, fetches entire lib/ folder from GitHub main branch using tarball API with clear progress messages and error handling. Created sat pull command to force-refresh library. Updated all router source paths to use SAT_LIBRARY variable. Binary now auto-downloads its dependencies on first run while supporting offline operation once cached.

---

## Phase 4: Shell Session Path Isolation

Fixed tmux rcfile to use SAT_LIBRARY instead of deprecated SAT_LIB variable for sourcing library files. Discovered and resolved XDG path collision issue where shell sessions overriding XDG_DATA_HOME caused library and manifests to be written to temporary session directories instead of permanent locations. Made SAT_LIBRARY and SAT_DATA respect XDG standards but be overridable, with resolved values exported to shell sessions before spawning tmux. This ensures manifest tracking works correctly inside sessions while maintaining XDG compliance and supporting dev workflows with custom library paths.
