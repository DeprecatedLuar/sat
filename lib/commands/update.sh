#!/usr/bin/env bash
# update.sh - Update packages installed via sat

# Source all source modules
source "$SAT_LIB/sources/brew.sh"
source "$SAT_LIB/sources/cargo.sh"
source "$SAT_LIB/sources/flatpak.sh"
source "$SAT_LIB/sources/go.sh"
source "$SAT_LIB/sources/nix.sh"
source "$SAT_LIB/sources/npm.sh"
source "$SAT_LIB/sources/uv.sh"
source "$SAT_LIB/sources/system.sh"
source "$SAT_LIB/sources/github.sh"
source "$SAT_LIB/sources/appimage.sh"
source "$SAT_LIB/sources/sat.sh"

_update_tool() {
    local tool="$1"
    local source
    source=$(manifest_get "$tool")

    if [[ -z "$source" ]]; then
        printf "[${C_CROSS}] %-25s ${C_DIM}not tracked${C_RESET}\n" "$tool"
        return 1
    fi

    local src_type="${source%%:*}"  # Extract source type (before colon)
    local metadata="${source#*:}"   # Extract metadata (after colon)

    # If source has no colon, metadata is empty
    [[ "$metadata" == "$source" ]] && metadata=""

    # Check for non-updatable sources first
    case "$src_type" in
        sat|repo)
            printf "[${C_CROSS}] %-25s ${C_DIM}cannot update source '%s'${C_RESET}\n" "$tool" "$source"
            return 1
            ;;
    esac

    # Route to appropriate update function with spinner
    local result
    case "$src_type" in
        brew)
            run_with_spinner "$tool" "$src_type" update_brew "$tool"
            result=$?
            ;;
        cargo)
            run_with_spinner "$tool" "$src_type" update_cargo "$tool"
            result=$?
            ;;
        nix)
            run_with_spinner "$tool" "$src_type" update_nix "$tool"
            result=$?
            ;;
        apt|pacman|apk|dnf)
            run_with_spinner "$tool" "$src_type" update_system "$tool"
            result=$?
            ;;
        uv)
            run_with_spinner "$tool" "$src_type" update_uv "$tool"
            result=$?
            ;;
        npm)
            run_with_spinner "$tool" "$src_type" update_npm "$tool"
            result=$?
            ;;
        go)
            run_with_spinner "$tool" "$src_type" update_go "$tool" "$metadata"
            result=$?
            ;;
        gh)
            run_with_spinner "$tool" "$src_type" update_github "$tool" "$metadata"
            result=$?
            ;;
        flatpak)
            run_with_spinner "$tool" "$src_type" update_flatpak "$tool" "$metadata"
            result=$?
            ;;
        appimage)
            run_with_spinner "$tool" "$src_type" update_appimage "$tool" "$metadata"
            result=$?
            ;;
        *)
            printf "[${C_CROSS}] %-25s ${C_DIM}unknown source '%s'${C_RESET}\n" "$tool" "$source"
            return 1
            ;;
    esac

    if [[ $result -eq 0 ]]; then
        status_ok "$tool" "$source"
    else
        status_fail "$tool" "$source"
        return 1
    fi
}

sat_update() {
    [[ $# -eq 0 ]] && { echo "Usage: sat update <program> [program2] ..."; return 1; }
    for tool in "$@"; do
        _update_tool "$tool"
    done
}
