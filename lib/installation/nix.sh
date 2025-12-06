#!/usr/bin/env bash
# Nix installation

install_nix() {
    local tool="$1"

    command -v nix-env &>/dev/null || return 1
    nix-env -iA "nixpkgs.$tool" &>/dev/null
}
