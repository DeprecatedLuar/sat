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

# Get installed version of Nix package
get_version_from_nix() {
    local tool="$1"
    command -v nix-env &>/dev/null || return 1
    nix-env -q "$tool" 2>/dev/null | grep -oP "\d[\d.]+"
}

# Uninstall Nix package
uninstall_nix() {
    local pkg="$1"
    _run_quiet nix-env --uninstall "$pkg" || _run_quiet nix profile remove "$pkg"
}

# Update Nix package
update_nix() {
    local tool="$1"
    _run_quiet nix-env -iA "nixpkgs.$tool"
}

# Check if nix package is outdated
# For nixos source: skip (system packages managed declaratively)
# For nix source: check (user packages via nix-env)
check_outdated_nix() {
    local tool="$1"
    local source_type="${2:-nix}"  # Optional: nix or nixos
    command -v nix-env &>/dev/null || return 1

    # Skip nixos source - those are system packages managed declaratively
    # Use 'nixos-rebuild switch' after updating channels for system packages
    if [[ "$source_type" == "nixos" ]]; then
        return 1
    fi

    # Try manifest first, fall back to nix-env query
    local current=$(get_source_version "$(manifest_get "$tool")")
    [[ -z "$current" ]] && current=$(nix-env -q "$tool" 2>/dev/null | grep -oP "\d[\d.]+")
    [[ -z "$current" ]] && return 1

    local latest=$(nix-env -qaA "nixpkgs.$tool" 2>/dev/null | grep -oP "\d[\d.]+")
    [[ -z "$latest" || "$current" == "$latest" ]] && return 1
    echo "$current $latest"
}
