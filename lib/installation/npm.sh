#!/usr/bin/env bash
# Node/npm installation

install_npm() {
    local tool="$1"

    command -v npm &>/dev/null || return 1
    npm show "$tool" >/dev/null 2>&1 || return 1
    _run_quiet npm install -g "$tool" || return 1
    command -v "$tool" &>/dev/null
}
