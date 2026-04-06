#!/usr/bin/env bash
# Nix installation

install_nix() {
    local tool="$1"

    command -v nix-env &>/dev/null || return 1
    _run_quiet nix-env -iA "nixpkgs.$tool"
}
