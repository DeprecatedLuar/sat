#!/usr/bin/env bash
# Cargo/Rust installation - install via cargo

# Install tool via cargo
# Handles missing build dependencies by trying brew then system package manager
# Returns: 0 on success, 1 on failure
install_cargo() {
    local tool="$1"

    command -v cargo &>/dev/null || return 1

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

    return 1
}
