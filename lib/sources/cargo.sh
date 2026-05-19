#!/usr/bin/env bash
# Cargo/Rust - Self-contained source module

# Search crates.io
search_cargo() {
    local query="$1"
    local response

    response=$(_fetch_json "https://crates.io/api/v1/crates?q=$query&per_page=10" "crates.io search") || return 0
    echo "$response" | jq -r '.crates[]? | "\(.name) \(.max_version) - \(.description // "" | split("\n")[0])"' 2>/dev/null
}

# Query latest version from crates.io (before install)
query_latest_version_cargo() {
    local crate="$1"
    local response

    response=$(_fetch_json "https://crates.io/api/v1/crates/$crate" "crates.io/$crate") || return 1
    echo "$response" | jq -r '.crate.newest_version // empty'
}

# Install tool via cargo
# Handles missing build dependencies by trying brew then system package manager
install_cargo() {
    local tool="$1"

    command -v cargo &>/dev/null || return 1

    if [[ -n "$SAT_DEBUG" ]]; then
        cargo install "$tool" && return 0
    else
        local err_file="/tmp/sat-cargo-err-$$"
        if cargo install "$tool" 2>"$err_file"; then
            rm -f "$err_file"
            return 0
        fi

        # Check for missing build tools
        local missing=$(grep -oP "is \`\K[^\`]+(?=\` not installed)" "$err_file" 2>/dev/null)
        rm -f "$err_file"

        if [[ -n "$missing" ]]; then
            printf "\r%-50s\r" ""
            printf "${C_DIM}Build requires %s, installing...${C_RESET}\n" "$missing"

            # Try brew first (no sudo)
            if command -v brew &>/dev/null && brew install "$missing" &>/dev/null; then
                printf "[${C_CHECK}] %-20s [${C_BREW}brew${C_RESET}] ${C_DIM}(build dep)${C_RESET}\n" "$missing"
                cargo install "$tool" &>/dev/null && return 0
            fi

            # Try system package manager
            local mgr="$SAT_PKG_MANAGER"
            if [[ -n "$mgr" ]] && pkg_install "$missing" "$mgr" &>/dev/null; then
                printf "[${C_CHECK}] %-20s [${C_SYSTEM}system${C_RESET}] ${C_DIM}(build dep)${C_RESET}\n" "$missing"
                cargo install "$tool" &>/dev/null && return 0
            fi
        fi
    fi

    return 1
}

# Get installed version of cargo package
get_version_from_cargo() {
    local tool="$1"
    command -v cargo &>/dev/null || return 1
    cargo install --list 2>/dev/null | grep -oP "^${tool} v\K[^ :]+"
}

# Uninstall cargo package
# Binary name may differ from crate name - look it up first
uninstall_cargo() {
    local pkg="$1"
    local crate=$(cargo install --list 2>/dev/null | grep -B1 "^    $pkg\$" | head -1 | cut -d' ' -f1)
    _run_quiet cargo uninstall "${crate:-$pkg}"
}

# Update cargo package (cargo install re-installs/updates)
update_cargo() {
    local tool="$1"
    _run_quiet cargo install "$tool"
}

# Check if cargo package is outdated
check_outdated_cargo() {
    local tool="$1"
    command -v cargo &>/dev/null || return 1

    # Try manifest first, fall back to querying cargo
    local current=$(get_source_version "$(manifest_get "$tool")")
    [[ -z "$current" ]] && current=$(cargo install --list 2>/dev/null | grep -oP "^${tool} v\K[^ :]+")
    [[ -z "$current" ]] && return 1

    local response latest
    response=$(_fetch_json "https://crates.io/api/v1/crates/$tool" "crates.io/$tool") || return 1
    latest=$(echo "$response" | jq -r '.crate.newest_version // empty')
    [[ -z "$latest" || "$current" == "$latest" ]] && return 1
    echo "$current $latest"
}
