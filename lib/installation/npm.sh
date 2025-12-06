#!/usr/bin/env bash
# Node/npm installation

install_npm() {
    local tool="$1"

    command -v npm &>/dev/null || return 1
    npm show "$tool" >/dev/null 2>&1 || return 1
    npm install -g "$tool" &>/dev/null || return 1
    command -v "$tool" &>/dev/null
}
