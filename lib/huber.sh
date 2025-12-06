#!/usr/bin/env bash
# huber.sh - GitHub release package manager wrapper

HUBER_DIR="${XDG_CONFIG_HOME:-$HOME/.config}/huber"

# Check if huber is installed
huber_available() {
    command -v huber &>/dev/null
}

# Ensure huber is installed
huber_ensure() {
    huber_available && return 0

    echo "Installing huber (GitHub release manager)..."

    # Check for cargo
    if ! command -v cargo &>/dev/null; then
        echo "Error: cargo required to install huber"
        echo "Run: sat source rust"
        return 1
    fi

    # Check for cmake (required build dep)
    if ! command -v cmake &>/dev/null; then
        echo "Error: cmake required to build huber"
        local mgr=$(get_pkg_manager)
        if [[ -n "$mgr" ]]; then
            echo "Installing cmake..."
            pkg_install cmake "$mgr" || return 1
        else
            echo "Install cmake manually and retry"
            return 1
        fi
    fi

    # Install huber
    cargo install huber || return 1

    # Initialize config dir
    huber --huber-dir "$HUBER_DIR" config save &>/dev/null

    echo "huber installed successfully"
}

# Wrapper: always use our config dir
huber_cmd() {
    huber --huber-dir "$HUBER_DIR" "$@"
}

# Install package via huber
# Args: owner/repo or just repo (assumes user's repo)
huber_install() {
    local pkg="$1"
    huber_ensure || return 1
    huber_cmd install "$pkg"
}

# Uninstall package via huber
huber_uninstall() {
    local pkg="$1"
    huber_available || return 1
    huber_cmd uninstall "$pkg"
}

# List installed packages
huber_list() {
    huber_available || return 1
    huber_cmd show
}

# Check if package is installed
huber_has() {
    local pkg="$1"
    huber_available || return 1
    huber_cmd show 2>/dev/null | grep -q "$pkg"
}
