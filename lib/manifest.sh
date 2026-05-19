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
# MANIFEST FUNCTIONS
# =============================================================================

# Ensure manifest file exists and is writable
_ensure_manifest() {
    local manifest="$1"
    local dir=$(dirname "$manifest")

    if [[ ! -d "$dir" ]]; then
        if ! mkdir -p "$dir" 2>/dev/null; then
            echo "Error: Cannot create directory: $dir" >&2
            echo "Check permissions and disk space." >&2
            return 1
        fi
    fi

    if [[ ! -f "$manifest" ]]; then
        if ! touch "$manifest" 2>/dev/null; then
            echo "Error: Cannot create manifest: $manifest" >&2
            echo "Check permissions and disk space." >&2
            return 1
        fi
    fi

    if [[ ! -w "$manifest" ]]; then
        echo "Error: Manifest not writable: $manifest" >&2
        echo "Check file permissions." >&2
        return 1
    fi

    return 0
}

# System manifest (tool=source:identity:version)
manifest_add() {
    _ensure_manifest "$SAT_MANIFEST" || return 1
    sed -i "/^$1=/d" "$SAT_MANIFEST" 2>/dev/null
    if ! echo "$1=$2" >> "$SAT_MANIFEST"; then
        echo "Error: Failed to add $1 to manifest" >&2
        return 1
    fi
}

manifest_get() {
    [[ -f "$SAT_MANIFEST" ]] || return 0
    grep "^$1=" "$SAT_MANIFEST" 2>/dev/null | cut -d= -f2
}

manifest_remove() {
    [[ -f "$SAT_MANIFEST" ]] || return 0
    if ! sed -i "/^$1=/d" "$SAT_MANIFEST" 2>/dev/null; then
        echo "Error: Failed to remove $1 from manifest" >&2
        return 1
    fi
}

manifest_has() {
    [[ -f "$SAT_MANIFEST" ]] || return 1
    grep -q "^$1=" "$SAT_MANIFEST" 2>/dev/null
}

# Master manifest (tool:source:identity:version:pid)
master_add() {
    _ensure_manifest "$SAT_SHELL_MASTER" || return 1
    if ! echo "$1:$2:$3" >> "$SAT_SHELL_MASTER"; then
        echo "Error: Failed to add $1 to shell manifest" >&2
        return 1
    fi
}

master_get_pids() {
    [[ -f "$SAT_SHELL_MASTER" ]] || return 0
    grep "^$1:$2:" "$SAT_SHELL_MASTER" 2>/dev/null | cut -d: -f3
}

master_has_tool() {
    [[ -f "$SAT_SHELL_MASTER" ]] || return 1
    grep -q "^$1:" "$SAT_SHELL_MASTER" 2>/dev/null
}

master_remove() {
    [[ -f "$SAT_SHELL_MASTER" ]] || return 0
    if ! sed -i "\|^$1:$2:$3\$|d" "$SAT_SHELL_MASTER" 2>/dev/null; then
        echo "Error: Failed to remove $1 from shell manifest" >&2
        return 1
    fi
}

master_remove_tool() {
    [[ -f "$SAT_SHELL_MASTER" ]] || return 0
    if ! sed -i "\|^$1:|d" "$SAT_SHELL_MASTER" 2>/dev/null; then
        echo "Error: Failed to remove all $1 entries from shell manifest" >&2
        return 1
    fi
}

master_promote() {
    local tool="$1" src="$2"
    [[ -f "$SAT_SHELL_MASTER" ]] && sed -i "\|^${tool}:${src}:|d" "$SAT_SHELL_MASTER" 2>/dev/null
    manifest_has "$tool" || manifest_add "$tool" "$src"
}

# Session manifest (TOOL=x, SOURCE_x=source:identity:version)
pid_manifest_add() {
    local pid="$1" tool="$2" src="$3"
    local manifest_path="$SAT_SHELL_DIR/$pid/manifest"

    if ! mkdir -p "$SAT_SHELL_DIR/$pid" 2>/dev/null; then
        echo "Error: Cannot create session directory: $SAT_SHELL_DIR/$pid" >&2
        return 1
    fi

    if ! echo "TOOL=$tool" >> "$manifest_path" || ! echo "SOURCE_$tool=$src" >> "$manifest_path"; then
        echo "Error: Failed to write to session manifest: $manifest_path" >&2
        return 1
    fi
}

pid_manifest_tools() {
    [[ -f "$SAT_SHELL_DIR/$1/manifest" ]] || return 0
    grep "^TOOL=" "$SAT_SHELL_DIR/$1/manifest" 2>/dev/null | cut -d= -f2
}

pid_manifest_source() {
    [[ -f "$SAT_SHELL_DIR/$1/manifest" ]] || return 0
    grep "^SOURCE_$2=" "$SAT_SHELL_DIR/$1/manifest" 2>/dev/null | cut -d= -f2
}

pid_manifest_remove() {
    if ! rm -rf "$SAT_SHELL_DIR/$1" "/tmp/sat-$1" 2>/dev/null; then
        echo "Error: Failed to remove session data for PID $1" >&2
        return 1
    fi
}

# =============================================================================
# HIGH-LEVEL API
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
