#!/usr/bin/env bash
# Python/uv installation

install_uv() {
    local tool="$1"

    command -v uv &>/dev/null || return 1
    _run_quiet uv tool install "$tool"
}
