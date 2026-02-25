#!/usr/bin/env bash
# System package manager installation (apt/pacman/dnf/apk/pkg)

# Install via system package manager
# Returns 0 on success, 1 on failure
install_system() {
    local tool="$1"
    local mgr="$SAT_PKG_MANAGER"

    [[ -z "$mgr" ]] && return 1
    pkg_exists "$tool" "$mgr" || return 1
    pkg_install "$tool" "$mgr"
}
