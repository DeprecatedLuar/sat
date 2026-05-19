#!/usr/bin/env bash
# uv/Python - Self-contained source module

# Search PyPI
search_uv() {
    local query="$1"
    local response

    response=$(_fetch_json "https://pypi.org/pypi/$query/json" "PyPI search") || return 0
    if echo "$response" | jq -e '.info' &>/dev/null; then
        echo "$response" | jq -r '"\(.info.name) \(.info.version) - \(.info.summary // "" | split("\n")[0])"' 2>/dev/null
    fi
}

# Query latest version from PyPI (before install)
query_latest_version_uv() {
    local pkg="$1"
    local response

    response=$(_fetch_json "https://pypi.org/pypi/$pkg/json" "PyPI/$pkg") || return 1
    echo "$response" | jq -r '.info.version // empty'
}

# Install from PyPI via uv
install_uv() {
    local tool="$1"

    command -v uv &>/dev/null || return 1
    _run_quiet uv tool install "$tool"
}

# Get installed version of uv tool
get_version_from_uv() {
    local tool="$1"
    command -v uv &>/dev/null || return 1
    uv tool list 2>/dev/null | grep -oP "^${tool} \K\S+"
}

# Install from GitHub repo with Python detection
# Returns: binary name via stdout on success
install_uv_github() {
    local repo_path="$1"
    local tree="$2"
    local repo_name="${repo_path##*/}"

    command -v uv &>/dev/null || return 1
    echo "$tree" | grep -qE '^(pyproject.toml|setup.py|setup.cfg)$' || return 1

    _run_quiet uv tool install "git+https://github.com/$repo_path" || return 1
    echo "$repo_name"
    return 0
}

# Uninstall uv tool
# Binary name may differ from package name - look it up first
uninstall_uv() {
    local pkg="$1"
    # uv tool list format: "package-name vX.X.X\n- binary1\n- binary2"
    local uv_pkg=$(uv tool list 2>/dev/null | grep -B1 "^- $pkg\$" | head -1 | cut -d' ' -f1)
    _run_quiet uv tool uninstall "${uv_pkg:-$pkg}"
}

# Update uv tool
update_uv() {
    local tool="$1"
    _run_quiet uv tool upgrade "$tool"
}

# Check if uv tool is outdated
check_outdated_uv() {
    local tool="$1"
    command -v uv &>/dev/null || return 1

    # Try manifest first, fall back to uv tool list
    local current=$(get_source_version "$(manifest_get "$tool")")
    [[ -z "$current" ]] && current=$(uv tool list 2>/dev/null | grep -oP "^${tool} \K\S+")
    [[ -z "$current" ]] && return 1

    local response latest
    response=$(_fetch_json "https://pypi.org/pypi/$tool/json" "PyPI/$tool") || return 1
    latest=$(echo "$response" | jq -r '.info.version // empty')
    [[ -z "$latest" || "$current" == "$latest" ]] && return 1
    echo "$current $latest"
}
