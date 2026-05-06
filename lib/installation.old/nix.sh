#!/usr/bin/env bash
# Nix installation

install_nix() {
    local tool="$1"

    command -v nix-env &>/dev/null || return 1

    # Skip on NixOS - packages should be managed declaratively via system config
    if [[ -f /etc/os-release ]] && grep -q '^ID=nixos' /etc/os-release; then
        [[ -n "$SAT_DEBUG" ]] && echo "[debug]   skipping nix-env on NixOS (use system config)" >&2
        return 1
    fi

    _run_quiet nix-env -iA "nixpkgs.$tool"
}
