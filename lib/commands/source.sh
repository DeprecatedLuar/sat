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

    # NixOS check for nix
    if [[ "$pm" == "nix" ]] && [[ -f /etc/NIXOS ]]; then
        echo "bruh"
        return 0
    fi

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

    # Fetch and run installer script (installs to default location)
    SOURCE_SCRIPT="$SAT_BASE/packages/sources/$pm.sh"

    if ! source <(curl -sSL "$SOURCE_SCRIPT" | tr -d '\r') 2>/dev/null; then
        echo "Error: Source '$pm' not found or failed to download" >&2
        return 1
    fi

    # Optionally call bootstrap/ensure functions if they exist (suppress output)
    if declare -f bootstrap &>/dev/null; then
        if [[ -n "$SAT_DEBUG" ]]; then
            bootstrap
        else
            bootstrap &>/dev/null
        fi
    elif declare -f "_ensure_$pm" &>/dev/null; then
        if [[ -n "$SAT_DEBUG" ]]; then
            "_ensure_$pm"
        else
            "_ensure_$pm" &>/dev/null
        fi
    fi

    # Define companion binaries for packages that provide multiple binaries
    local -a companions=()
    case "$pm" in
        uv) companions=("uvx" "uvw") ;;
    esac

    # Move installed binary to sat's directory and symlink back
    local installed_path=$(command -v "$pm" 2>/dev/null)
    if [[ -n "$installed_path" ]]; then
        local install_dir=$(dirname "$installed_path")

        # Move primary binary
        mv "$installed_path" "$sources_dir/$pm"
        ln -sf "$sources_dir/$pm" "$installed_path"

        # Move companion binaries if they exist
        local -a moved_companions=()
        for comp in "${companions[@]}"; do
            local comp_path="$install_dir/$comp"
            if [[ -f "$comp_path" ]]; then
                mv "$comp_path" "$sources_dir/$comp"
                ln -sf "$sources_dir/$comp" "$comp_path"
                moved_companions+=("$comp")
            fi
        done

        # Print single consolidated message
        if [[ ${#moved_companions[@]} -gt 0 ]]; then
            echo "✓ $pm (+ ${moved_companions[*]}) → $installed_path"
        else
            echo "✓ $pm → $installed_path"
        fi
    else
        echo "Warning: $pm was not found after installation" >&2
        return 1
    fi
}
