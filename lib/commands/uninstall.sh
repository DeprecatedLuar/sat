#!/usr/bin/env bash
# uninstall.sh - Remove packages installed via sat

# Source all source modules for uninstall functions
source "$SAT_LIB/sources/github.sh"
source "$SAT_LIB/sources/appimage.sh"
source "$SAT_LIB/sources/cargo.sh"
source "$SAT_LIB/sources/npm.sh"
source "$SAT_LIB/sources/go.sh"
source "$SAT_LIB/sources/brew.sh"
source "$SAT_LIB/sources/nix.sh"
source "$SAT_LIB/sources/uv.sh"
source "$SAT_LIB/sources/sat.sh"
source "$SAT_LIB/sources/system.sh"
source "$SAT_LIB/sources/flatpak.sh"

sat_uninstall() {
    for arg in "$@"; do
        # Parse tool:source syntax (handles aliases like :py, :rs, :sys)
        parse_tool_spec "$arg"
        local PROGRAM="$_TOOL_NAME"
        local EXPLICIT_SOURCE="$_TOOL_SOURCE"

        local SOURCE TRACKED=true BINARY="$PROGRAM"

        # If explicit source specified, use it directly
        if [[ -n "$EXPLICIT_SOURCE" ]]; then
            SOURCE="$EXPLICIT_SOURCE"
            TRACKED=false
        else
            # Manifest lookup
            SOURCE=$(manifest_get "$PROGRAM")

            # If not found directly, search by repo name (for gh:user/repo entries)
            if [[ -z "$SOURCE" ]]; then
                local match=$(grep "=gh:.*/${PROGRAM}$" "$SAT_MANIFEST" 2>/dev/null | head -1)
                if [[ -n "$match" ]]; then
                    BINARY="${match%%=*}"
                    SOURCE="${match#*=}"
                fi
            fi

            # Fallback: detect source from filesystem
            if [[ -z "$SOURCE" ]]; then
                if ! command -v "$PROGRAM" &>/dev/null; then
                    status "$PROGRAM not found"
                    continue
                fi
                SOURCE=$(detect_source "$PROGRAM")
                if [[ -z "$SOURCE" || "$SOURCE" == "unknown" ]]; then
                    status "$PROGRAM source unknown, can't remove"
                    continue
                fi
                TRACKED=false
            fi
        fi

        local _result
        if [[ -n "$SAT_DEBUG" ]]; then
            printf "Removing %s\n" "$BINARY"
            pkg_remove "$BINARY" "$SOURCE"; _result=$?
        else
            pkg_remove "$BINARY" "$SOURCE" >/dev/null 2>&1 &
            spin_with_style "$BINARY" $! "$(get_source_type "$SOURCE")"
            wait $!; _result=$?
        fi

        if [[ $_result -eq 0 ]]; then
            [[ "$TRACKED" == true ]] && manifest_remove "$BINARY"
            status_ok "$BINARY removed" "$SOURCE"
        else
            status_fail "$BINARY removal failed"
        fi
    done
    hash -r
}
