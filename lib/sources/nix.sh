#!/usr/bin/env bash
# Nix - Self-contained source module

# Search NixOS packages
search_nix() {
    local query="$1"
    curl -sS "https://aWVSALXpZv:X8gPHnzL52wFEekuxsfQ9cSh@nixos-search-7-1733963800.us-east-1.bonsaisearch.net/latest-*/_search" \
        -H "Content-Type: application/json" \
        -d "{\"query\":{\"bool\":{\"should\":[{\"term\":{\"package_attr_name\":{\"value\":\"$query\",\"boost\":10}}},{\"wildcard\":{\"package_attr_name\":\"*$query*\"}}],\"minimum_should_match\":1}},\"size\":20,\"_source\":[\"package_attr_name\",\"package_pversion\",\"package_description\"]}" 2>/dev/null | \
        jq -r '.hits.hits[]._source | "\(.package_attr_name) \(.package_pversion) - \(.package_description // "no description")"' 2>/dev/null | \
        awk -F' ' '!seen[$1]++ {
            # Strip nix metadata from version
            gsub(/-unstable-[0-9]{4}-[0-9]{2}-[0-9]{2}/, "", $2)
            print
        }' | head -10
}

# Install from Nix
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

# Uninstall Nix package
uninstall_nix() {
    local pkg="$1"
    nix-env --uninstall "$pkg" 2>/dev/null || nix profile remove "$pkg"
}
