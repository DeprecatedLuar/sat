#!/usr/bin/env bash
# sat - Satellite installer (universal package manager)

SAT_LIBRARY="${SAT_LIBRARY:-$HOME/.local/share/sat/lib}"

# Ensure library is present (fetch from GitHub if missing)
_ensure_lib() {
    [[ -f "$SAT_LIBRARY/common.sh" ]] && return 0

    echo "Downloading latest sat version..."
    mkdir -p "$SAT_LIBRARY"

    if ! curl -sSL "https://github.com/DeprecatedLuar/sat/archive/refs/heads/main.tar.gz" | \
         tar xzf - --strip-components=2 -C "$SAT_LIBRARY" "sat-main/lib" 2>/dev/null; then
        echo "Error: Failed to download sat. Check network connection and firewall settings." >&2
        exit 1
    fi
}

_ensure_lib
source "$SAT_LIBRARY/common.sh"
source "$SAT_LIBRARY/manifest.sh"  # Early load for manifest functions and parsing helpers

# Validate critical dependencies before running any commands
_check_dependencies() {
    local missing=()
    local pkg_mgr=""

    # Detect package manager for install suggestions
    if command -v apt &>/dev/null; then
        pkg_mgr="apt"
    elif command -v pacman &>/dev/null; then
        pkg_mgr="pacman"
    elif command -v dnf &>/dev/null; then
        pkg_mgr="dnf"
    elif command -v apk &>/dev/null; then
        pkg_mgr="apk"
    fi

    # Check critical dependencies
    command -v jq &>/dev/null || missing+=("jq")
    command -v curl &>/dev/null || missing+=("curl")

    if [[ ${#missing[@]} -gt 0 ]]; then
        echo "Error: Missing required dependencies: ${missing[*]}" >&2
        echo "" >&2
        echo "sat requires these tools to function. Install them with:" >&2

        case "$pkg_mgr" in
            apt)    echo "  sudo apt install ${missing[*]}" >&2 ;;
            pacman) echo "  sudo pacman -S ${missing[*]}" >&2 ;;
            dnf)    echo "  sudo dnf install ${missing[*]}" >&2 ;;
            apk)    echo "  sudo apk add ${missing[*]}" >&2 ;;
            *)
                echo "  Install packages: ${missing[*]}" >&2
                echo "  Or run: sat deps" >&2
                ;;
        esac

        exit 1
    fi
}

# Check dependencies (skip for deps command itself)
[[ "$1" != "deps" && "$1" != "dependencies" ]] && _check_dependencies

# Strip --debug/--update flags early (before routing)
_args=()
for _arg in "$@"; do
    if [[ "$_arg" == "--debug" ]]; then export SAT_DEBUG=1
    elif [[ "$_arg" == "--update" ]]; then export SAT_UPDATE=1
    else _args+=("$_arg"); fi
done
set -- "${_args[@]}"
unset _arg _args

if [[ "${SAT_UPDATE:-0}" == "1" ]]; then
    source "$SAT_LIBRARY/commands/pull.sh"
    sat_pull
    exit $?
fi

cleanup_orphaned_sessions

# ══════════════════════════════════════════════════════════════════════════════
# ──[ROUTER]────────────────────────────────────────────────────────────────────
# ══════════════════════════════════════════════════════════════════════════════

case "$1" in
    deps|dependencies)
        source "$SAT_LIBRARY/commands/deps.sh"
        sat_deps
        ;;

    install|i)
        shift
        source "$SAT_LIBRARY/commands/install.sh"
        sat_install "$@"
        ;;

    search)
        [[ -z "$2" ]] && { echo "Usage: sat search <program> [--wrap]"; exit 1; }
        shift
        source "$SAT_LIBRARY/commands/search.sh"
        sat_search "$@"
        ;;

    uninstall|remove|rm)
        [[ -z "$2" ]] && { echo "Usage: sat uninstall <program> [program2] ..."; exit 1; }
        shift
        source "$SAT_LIBRARY/commands/uninstall.sh"
        sat_uninstall "$@"
        ;;

    shell)
        shift
        source "$SAT_LIBRARY/commands/shell.sh"
        sat_shell "$@"
        ;;

    list|ls)
        shift
        source "$SAT_LIBRARY/commands/list.sh"
        sat_list "$@"
        ;;

    track)
        [[ -z "$2" ]] && { echo "Usage: sat track <program> [program2] ..."; exit 1; }
        shift
        source "$SAT_LIBRARY/commands/track.sh"
        sat_track "$@"
        ;;

    scan)
        source "$SAT_LIBRARY/commands/scan.sh"
        sat_scan
        ;;

    untrack)
        [[ -z "$2" ]] && { echo "Usage: sat untrack <program> [program2] ..."; exit 1; }
        shift
        source "$SAT_LIBRARY/commands/untrack.sh"
        sat_untrack "$@"
        ;;

    source|src)
        shift
        source "$SAT_LIBRARY/commands/source.sh"
        sat_source "$@"
        ;;

    clone)
        shift
        source "$SAT_LIBRARY/commands/clone.sh"
        sat_clone "$@"
        ;;

    pull)
        source "$SAT_LIBRARY/commands/pull.sh"
        sat_pull
        ;;

    outdated)
        source "$SAT_LIBRARY/commands/outdated.sh"
        sat_outdated
        ;;

    update)
        if [[ "$2" == "sat" ]]; then
            source "$SAT_LIBRARY/commands/pull.sh"
            sat_pull
        else
            shift
            source "$SAT_LIBRARY/commands/update.sh"
            sat_update "$@"
        fi
        ;;

    info|which|whereis)
        [[ -z "$2" ]] && { echo "Usage: sat info <program> [program2] ..."; exit 1; }
        shift
        source "$SAT_LIBRARY/commands/info.sh"
        sat_info "$@"
        ;;

    help|--help|-h|"")
        source "$SAT_LIBRARY/commands/help.sh"
        sat_help
        ;;

    *)
        echo "sat: unknown command '$1'" >&2
        exit 1
        ;;
esac
