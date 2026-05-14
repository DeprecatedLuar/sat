#!/usr/bin/env bash
# uv/Python - Self-contained source module

# Search PyPI
search_uv() {
    local query="$1"
    local info=$(curl -sS "https://pypi.org/pypi/$query/json" 2>/dev/null)
    if echo "$info" | jq -e '.info' &>/dev/null; then
        echo "$info" | jq -r '"\(.info.name) \(.info.version) - \(.info.summary // "" | split("\n")[0])"' 2>/dev/null
    fi
}

# Query latest version from PyPI (before install)
query_latest_version_uv() {
    local pkg="$1"
    curl -sS "https://pypi.org/pypi/$pkg/json" 2>/dev/null | \
        jq -r '.info.version // empty'
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
    uv tool uninstall "${uv_pkg:-$pkg}"
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

    local latest=$(curl -sS "https://pypi.org/pypi/$tool/json" 2>/dev/null | \
        jq -r '.info.version // empty')
    [[ -z "$latest" || "$current" == "$latest" ]] && return 1
    echo "$current $latest"
}
