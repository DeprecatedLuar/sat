#!/usr/bin/env bash
# Homebrew installation

install_brew() {
    local tool="$1"

    command -v brew &>/dev/null || return 1
    brew info "$tool" &>/dev/null 2>&1 || return 1
    brew install "$tool" &>/dev/null
}
