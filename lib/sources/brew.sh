#!/usr/bin/env bash
# Homebrew - Self-contained source module

# Search Homebrew
search_brew() {
    local query="$1"

    # Try formula first
    local info=$(curl -sS "https://formulae.brew.sh/api/formula/$query.json" 2>/dev/null)
    if echo "$info" | jq -e '.name' &>/dev/null; then
        echo "$info" | jq -r '"\(.name) \(.versions.stable) - \(.desc // "" | split("\n")[0])"' 2>/dev/null
        return
    fi

    # Try cask if formula not found
    info=$(curl -sS "https://formulae.brew.sh/api/cask/$query.json" 2>/dev/null)
    if echo "$info" | jq -e '.token' &>/dev/null; then
        echo "$info" | jq -r '
            .token + " " + .version + " - " +
            (if (.depends_on | has("macos")) then "(macOS only) " else "" end) +
            (.desc // "" | split("\n")[0])
        ' 2>/dev/null
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

# Uninstall Homebrew package
uninstall_brew() {
    local pkg="$1"
    brew uninstall "$pkg"
}
