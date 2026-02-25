#!/usr/bin/env bash
# uninstall.sh - Remove packages installed via sat

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
            SOURCE=$(_sat_manifest_get "$PROGRAM")

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

        printf "Removing %s\n" "$BINARY"
        if pkg_remove "$BINARY" "$SOURCE"; then
            [[ "$TRACKED" == true ]] && _sat_manifest_remove "$BINARY"
            status_ok "$BINARY removed" "$SOURCE"
        else
            status_fail "$BINARY removal failed"
        fi
    done
    hash -r
}
