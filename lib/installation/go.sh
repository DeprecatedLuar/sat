#!/usr/bin/env bash
# Go installation

install_go() {
    local tool="$1"

    command -v go &>/dev/null || return 1
    local go_pkg="$tool"
    [[ "$go_pkg" != *"."* ]] && go_pkg="github.com/$tool"
    go install "${go_pkg}@latest" &>/dev/null
}
