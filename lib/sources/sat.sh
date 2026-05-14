#!/usr/bin/env bash
# sat wrapper scripts - Self-contained source module

# Install sat wrapper script
install_sat() {
    local tool="$1"

    curl -sSL --fail --head "$SAT_BASE/cargo-bay/programs/${tool}.sh" >/dev/null 2>&1 || return 1
    source <(curl -sSL "$SAT_BASE/internal/fetcher.sh")
    sat_init && sat_run "$tool" &>/dev/null
}

# Uninstall sat wrapper script
uninstall_sat() {
    local pkg="$1"
    rm -f "$HOME/.local/bin/$pkg"
}

# Update sat wrapper script (not supported)
update_sat() {
    return 1  # Sat wrapper scripts cannot be updated in place
}

# Check if sat wrapper is outdated (not supported)
check_outdated_sat() {
    return 1  # Sat wrapper scripts don't have versions
}
