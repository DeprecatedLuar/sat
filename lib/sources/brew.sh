#!/usr/bin/env bash
# Homebrew - Self-contained source module

# Search Homebrew
search_brew() {
    local query="$1"
    local response

    # Try formula first
    if response=$(_fetch_json "https://formulae.brew.sh/api/formula/$query.json" "brew formula") && \
       echo "$response" | jq -e '.name' &>/dev/null; then
        echo "$response" | jq -r '"\(.name) \(.versions.stable) - \(.desc // "" | split("\n")[0])"' 2>/dev/null
        return
    fi

    # Try cask if formula not found
    if response=$(_fetch_json "https://formulae.brew.sh/api/cask/$query.json" "brew cask") && \
       echo "$response" | jq -e '.token' &>/dev/null; then
        echo "$response" | jq -r '
            .token + " " + .version + " - " +
            (if (.depends_on | has("macos")) then "(macOS only) " else "" end) +
            (.desc // "" | split("\n")[0])
        ' 2>/dev/null
    fi
}

# Query latest version from Homebrew API (before install)
query_latest_version_brew() {
    local pkg="$1"
    local response

    # Try formula first
    if response=$(_fetch_json "https://formulae.brew.sh/api/formula/$pkg.json" "brew/$pkg") && \
       echo "$response" | jq -e '.versions.stable' &>/dev/null; then
        echo "$response" | jq -r '.versions.stable'
        return
    fi

    # Try cask
    if response=$(_fetch_json "https://formulae.brew.sh/api/cask/$pkg.json" "brew cask/$pkg"); then
        echo "$response" | jq -r '.version // empty'
    fi
}

# Install from Homebrew
install_brew() {
    local tool="$1"

    command -v brew &>/dev/null || return 1

    local output
    local exit_code

    # Try formula first
    if brew info "$tool" &>/dev/null 2>&1; then
        output=$(brew install "$tool" 2>&1)
        exit_code=$?

        if [[ $exit_code -ne 0 ]]; then
            if echo "$output" | grep -q "macOS is required"; then
                echo "Error: $tool requires macOS (brew cask)" >&2
                return 2  # Platform-specific error
            fi
            echo "$output" >&2
        fi
        return $exit_code
    fi

    # Try cask if formula not found
    if brew info --cask "$tool" &>/dev/null 2>&1; then
        output=$(brew install --cask "$tool" 2>&1)
        exit_code=$?

        if [[ $exit_code -ne 0 ]]; then
            if echo "$output" | grep -q "macOS is required"; then
                echo "Error: $tool requires macOS (brew cask)" >&2
                return 2  # Platform-specific error
            fi
            echo "$output" >&2
        fi
        return $exit_code
    fi

    return 1
}

# Get installed version of Homebrew package
get_version_from_brew() {
    local tool="$1"
    command -v brew &>/dev/null || return 1
    brew list --versions "$tool" 2>/dev/null | awk '{print $2}'
}

# Uninstall Homebrew package
uninstall_brew() {
    local pkg="$1"
    _run_quiet brew uninstall "$pkg"
}

# Update Homebrew package
update_brew() {
    local tool="$1"
    _run_quiet brew upgrade "$tool"
}

# Check if brew package is outdated
check_outdated_brew() {
    local tool="$1"
    command -v brew &>/dev/null || return 1

    # Try manifest first
    local current=$(get_source_version "$(manifest_get "$tool")")

    # Fall back to brew outdated if no manifest version
    if [[ -z "$current" ]]; then
        local line=$(brew outdated --verbose 2>/dev/null | grep "^$tool (")
        [[ -z "$line" ]] && return 1
        current=$(echo "$line" | grep -oP '\(\K[^)]+')
    fi
    [[ -z "$current" ]] && return 1

    # Get latest from brew info
    local latest=$(brew info --json=v2 "$tool" 2>/dev/null | \
        jq -r '.formulae[0].versions.stable // .casks[0].version // empty')
    [[ -z "$latest" || "$current" == "$latest" ]] && return 1
    echo "$current $latest"
}
