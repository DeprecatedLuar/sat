#!/usr/bin/env bash
# Homebrew installation

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
