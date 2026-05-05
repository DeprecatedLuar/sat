#!/usr/bin/env bash
# source.sh - Install package managers to sat's bin directory

sat_source() {
    local pm="$1"
    [[ -z "$pm" ]] && { echo "Usage: sat source <packagemanager>"; return 1; }

    # Normalize aliases
    case "$pm" in
        gh|github) pm="huber" ;;
        homebrew)  pm="brew" ;;
        python)    pm="uv" ;;
    esac

    # Check if already available
    if command -v "$pm" &>/dev/null; then
        local_path=$(command -v "$pm")
        echo "$pm is already installed at $local_path"
        return 0
    fi

    # Install to sat's bin/sources/
    local sources_dir="$SAT_DATA/bin/sources"
    mkdir -p "$sources_dir"
    mkdir -p "$HOME/.local/bin"

    echo "Installing $pm to $sources_dir..."

    # Fetch and run installer script (installs to default location)
    SOURCE_SCRIPT="$SAT_BASE/packages/sources/$pm.sh"

    if ! source <(curl -sSL "$SOURCE_SCRIPT" | tr -d '\r') 2>/dev/null; then
        echo "Error: Source '$pm' not found or failed to download" >&2
        return 1
    fi

    # Optionally call bootstrap/ensure functions if they exist
    if declare -f bootstrap &>/dev/null; then
        bootstrap
    elif declare -f "_ensure_$pm" &>/dev/null; then
        "_ensure_$pm"
    fi

    # Move installed binary to sat's directory and symlink back
    local installed_path=$(command -v "$pm" 2>/dev/null)
    if [[ -n "$installed_path" ]]; then
        # Move to sat's sources directory
        mv "$installed_path" "$sources_dir/$pm"
        # Create symlink back to original location
        ln -sf "$sources_dir/$pm" "$installed_path"
        echo "✓ Moved $pm to $sources_dir/ and symlinked to $installed_path"
    else
        echo "Warning: $pm was not found after installation" >&2
        return 1
    fi
}
