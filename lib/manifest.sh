#!/usr/bin/env bash
# sat manifest API - unified interface for all manifest operations

# =============================================================================
# SOURCE STRING FORMAT HELPERS
# =============================================================================
# Format: source:identity:version
# Examples:
#   cargo::14.1.0              (no identity, just version)
#   gh:owner/repo:v1.0.0       (identity=owner/repo, version=v1.0.0)
#   flatpak:app.id:1.2.3       (identity=app.id, version=1.2.3)

# Build source string from components
# Args: source [identity] [version]
# Returns: source:identity:version (with empty fields preserved)
build_source_string() {
    local source="$1" identity="$2" version="$3"
    echo "${source}:${identity}:${version}"
}

# Parse source string into components
# Args: source_string
# Outputs: Three lines: source, identity, version
parse_source_string() {
    local source_string="$1"
    local source identity version
    IFS=: read -r source identity version <<< "$source_string"
    echo "$source"
    echo "$identity"
    echo "$version"
}

# Extract just the source type (first field)
get_source_type() {
    local source_string="$1"
    echo "${source_string%%:*}"
}

# Extract identity (second field, may be empty)
get_source_identity() {
    local source_string="$1"
    [[ "$source_string" != *:* ]] && return  # No colons = no identity
    local temp="${source_string#*:}"   # Remove first field
    echo "${temp%%:*}"                 # Get second field
}

# Extract version (third field, may be empty)
get_source_version() {
    local source_string="$1"
    [[ "$source_string" != *:*:* ]] && return  # Less than 3 fields = no version
    local temp="${source_string#*:}"   # Remove first field
    temp="${temp#*:}"                  # Remove second field
    echo "$temp"                       # Return third field
}

# =============================================================================
# LOW-LEVEL MANIFEST API (Wrappers)
# =============================================================================
# Auto-detect context: use internal _* functions (binary) or subprocess (remote)

# sat-manifest (system manifest: tool=source:identity:version)
manifest_add()    { declare -F _sat_manifest_add    &>/dev/null && _sat_manifest_add "$@"    || sat internal sat-manifest add "$1" "$2"; }
manifest_get()    { declare -F _sat_manifest_get    &>/dev/null && _sat_manifest_get "$@"    || sat internal sat-manifest get "$1"; }
manifest_remove() { declare -F _sat_manifest_remove &>/dev/null && _sat_manifest_remove "$@" || sat internal sat-manifest remove "$1"; }
manifest_has()    { declare -F _sat_manifest_has    &>/dev/null && _sat_manifest_has "$@"    || sat internal sat-manifest has "$1"; }

# shell-manifest (master manifest: tool:source:pid)
master_add()         { declare -F _shell_manifest_add        &>/dev/null && _shell_manifest_add "$@"        || sat internal shell-manifest add "$1" "$2" "$3"; }
master_get_pids()    { declare -F _shell_manifest_pids       &>/dev/null && _shell_manifest_pids "$@"       || sat internal shell-manifest pids "$1" "$2"; }
master_has_tool()    { declare -F _shell_manifest_has        &>/dev/null && _shell_manifest_has "$@"        || sat internal shell-manifest has "$1"; }
master_remove()      { declare -F _shell_manifest_remove     &>/dev/null && _shell_manifest_remove "$@"     || sat internal shell-manifest remove "$1" "$2" "$3"; }
master_remove_tool() { declare -F _shell_manifest_remove_all &>/dev/null && _shell_manifest_remove_all "$@" || sat internal shell-manifest remove-all "$1"; }
master_promote()     { declare -F _shell_manifest_promote    &>/dev/null && _shell_manifest_promote "$@"    || sat internal shell-manifest promote "$1" "$2"; }

# pid-manifest (session manifest: TOOL=x, SOURCE_x=y)
pid_manifest_add()    { declare -F _pid_manifest_add    &>/dev/null && _pid_manifest_add "$@"    || sat internal pid-manifest add "$1" "$2" "$3"; }
pid_manifest_tools()  { declare -F _pid_manifest_tools  &>/dev/null && _pid_manifest_tools "$@"  || sat internal pid-manifest tools "$1"; }
pid_manifest_source() { declare -F _pid_manifest_source &>/dev/null && _pid_manifest_source "$@" || sat internal pid-manifest source "$1" "$2"; }
pid_manifest_remove() { declare -F _pid_manifest_remove &>/dev/null && _pid_manifest_remove "$@" || sat internal pid-manifest remove "$1"; }

# =============================================================================
# HIGH-LEVEL MANIFEST API
# =============================================================================

# Track installation in appropriate manifest based on context
# Args: tool, source, [identity], [version]
# Context: SAT_MANIFEST_TARGET=session → session install (temporary)
#          SAT_MANIFEST_TARGET unset   → permanent install (system)
track_install() {
    local tool="$1" source="$2" identity="$3" version="$4"

    # Build full source string (format: source:identity:version)
    local src=$(build_source_string "$source" "$identity" "$version")

    if [[ "$SAT_MANIFEST_TARGET" == "session" ]]; then
        # Shell session: track in session + master manifest
        pid_manifest_add "$SAT_SESSION" "$tool" "$src"
        master_add "$tool" "$src" "$SAT_SESSION"
    elif master_has_tool "$tool"; then
        # Permanent install but tool exists in session: promote it
        master_promote "$tool" "$src"
        printf "  ${C_DIM}(promoted from shell session)${C_RESET}\n"
    else
        # Permanent install: system manifest
        manifest_add "$tool" "$src"
    fi
}
