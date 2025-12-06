#!/usr/bin/env bash
# Python/uv installation

install_uv() {
    local tool="$1"

    command -v uv &>/dev/null || return 1
    uv tool install "$tool" &>/dev/null
}
