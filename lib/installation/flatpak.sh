#!/usr/bin/env bash
# Flatpak installation

install_flatpak() {
    local tool="$1"
    command -v flatpak &>/dev/null || return 1

    # If it looks like an app ID (has dots), use directly
    if [[ "$tool" == *.*.* ]]; then
        flatpak install -y flathub "$tool"
        return $?
    fi

    # Search using existing function and extract app ID
    local result=$(search_flatpak "$tool" | head -1)
    [[ -z "$result" ]] && return 1

    local app_id=$(echo "$result" | awk '{print $1}')
    flatpak install -y flathub "$app_id"
}
