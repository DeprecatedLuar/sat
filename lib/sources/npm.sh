#!/usr/bin/env bash
# npm (Node.js) - Self-contained source module

# Search npm registry
search_npm() {
    local query="$1"
    curl -sS "https://registry.npmjs.org/-/v1/search?text=$query&size=10" 2>/dev/null | \
        jq -r '.objects[]? | "\(.package.name) \(.package.version) - \(.package.description // "" | split("\n")[0])"' 2>/dev/null
}

# Install from npm
install_npm() {
    local tool="$1"

    command -v npm &>/dev/null || return 1
    npm show "$tool" >/dev/null 2>&1 || return 1
    _run_quiet npm install -g "$tool" || return 1
    command -v "$tool" &>/dev/null
}

# Uninstall npm package
uninstall_npm() {
    local pkg="$1"
    npm uninstall -g "$pkg"
}
