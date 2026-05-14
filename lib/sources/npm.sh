#!/usr/bin/env bash
# npm (Node.js) - Self-contained source module

# Search npm registry
search_npm() {
    local query="$1"
    curl -sS "https://registry.npmjs.org/-/v1/search?text=$query&size=10" 2>/dev/null | \
        jq -r '.objects[]? | "\(.package.name) \(.package.version) - \(.package.description // "" | split("\n")[0])"' 2>/dev/null
}

# Query latest version from npm registry (before install)
query_latest_version_npm() {
    local pkg="$1"
    curl -sS "https://registry.npmjs.org/$pkg/latest" 2>/dev/null | \
        jq -r '.version // empty'
}

# Install from npm
# Sets: _NPM_INSTALLED_PKG (actual package name, handles scoped packages)
install_npm() {
    local tool="$1"

    command -v npm &>/dev/null || return 1
    npm show "$tool" >/dev/null 2>&1 || return 1
    _run_quiet npm install -g "$tool" || return 1

    # Find the actual package name from installed binary
    local bin_path=$(command -v "$tool" 2>/dev/null)
    if [[ -L "$bin_path" ]]; then
        # Extract package name from symlink: ../lib/node_modules/@org/pkg/cli.js → @org/pkg
        _NPM_INSTALLED_PKG=$(readlink "$bin_path" | grep -oP 'node_modules/\K[^/]+(/[^/]+)?(?=/)')
    fi

    # Fallback: use tool name if we can't extract package name
    [[ -z "$_NPM_INSTALLED_PKG" ]] && _NPM_INSTALLED_PKG="$tool"
    return 0
}

# Extract actual npm package name from binary name
# Returns: package name (may be scoped like @org/pkg)
get_npm_package_name() {
    local tool="$1"

    # If already looks like a package name, return as-is
    [[ "$tool" =~ ^@|/ ]] && echo "$tool" && return

    # Try to extract from symlink
    local bin_path=$(command -v "$tool" 2>/dev/null)
    if [[ -L "$bin_path" ]]; then
        # Match: @org/pkg for scoped packages, or pkg-name for regular (stops at first /)
        local pkg=$(readlink "$bin_path" | grep -oP 'node_modules/\K(@[^/]+/[^/]+|[^/]+)(?=/)')
        [[ -n "$pkg" ]] && echo "$pkg" && return
    fi

    # Fallback: use tool name
    echo "$tool"
}

# Get installed version of npm package
get_version_from_npm() {
    local tool="$1"
    command -v npm &>/dev/null || return 1

    local pkg_name=$(get_npm_package_name "$tool")

    npm list -g "$pkg_name" --depth=0 --json 2>/dev/null | \
        jq -r --arg pkg "$pkg_name" '.dependencies[$pkg].version // empty'
}

# Uninstall npm package
# Handles both package names (@org/pkg) and binary names (for old manifests)
uninstall_npm() {
    local pkg="$1"

    # Try as package name first
    if npm uninstall -g "$pkg" 2>/dev/null; then
        return 0
    fi

    # Fallback: if it's a binary name, find the package
    local bin_path=$(command -v "$pkg" 2>/dev/null)
    if [[ -L "$bin_path" ]]; then
        local pkg_name=$(readlink "$bin_path" | grep -oP 'node_modules/\K[^/]+(/[^/]+)?(?=/)')
        [[ -n "$pkg_name" ]] && npm uninstall -g "$pkg_name"
    fi
}

# Update npm package
# Handles both package names and binary names
update_npm() {
    local tool="$1"

    # If it looks like a package name (has @ or /), use directly
    if [[ "$tool" =~ ^@|/ ]]; then
        _run_quiet npm update -g "$tool"
        return
    fi

    # Otherwise, try to find the package name from binary
    local bin_path=$(command -v "$tool" 2>/dev/null)
    if [[ -L "$bin_path" ]]; then
        local pkg_name=$(readlink "$bin_path" | grep -oP 'node_modules/\K[^/]+(/[^/]+)?(?=/)')
        if [[ -n "$pkg_name" ]]; then
            _run_quiet npm update -g "$pkg_name"
            return
        fi
    fi

    # Fallback: try tool name
    _run_quiet npm update -g "$tool"
}

# Check if npm package is outdated
# Handles both package names and binary names
check_outdated_npm() {
    local tool="$1"
    command -v npm &>/dev/null || return 1

    # Get package name (might be scoped)
    local pkg_name=$(get_npm_package_name "$tool")

    # Try manifest first, fall back to npm outdated
    local current=$(get_source_version "$(manifest_get "$tool")")
    if [[ -z "$current" ]]; then
        local json=$(npm outdated -g --json 2>/dev/null)
        current=$(echo "$json" | jq -r --arg t "$pkg_name" '.[$t].current // empty')
    fi
    [[ -z "$current" ]] && return 1

    # Get latest from npm registry
    local latest=$(curl -sS "https://registry.npmjs.org/$pkg_name/latest" 2>/dev/null | \
        jq -r '.version // empty')
    [[ -z "$latest" || "$current" == "$latest" ]] && return 1
    echo "$current $latest"
}
