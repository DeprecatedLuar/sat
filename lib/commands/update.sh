#!/usr/bin/env bash
# update.sh - Update packages installed via sat

_update_tool() {
    local tool="$1"
    local source
    source=$(manifest_get "$tool")

    if [[ -z "$source" ]]; then
        printf "[${C_CROSS}] %-25s ${C_DIM}not tracked${C_RESET}\n" "$tool"
        return 1
    fi

    local ok=0
    case "$source" in
        brew)
            _run_quiet brew upgrade "$tool" && ok=1
            ;;
        cargo)
            _run_quiet cargo install "$tool" && ok=1
            ;;
        nix)
            _run_quiet nix-env -iA "nixpkgs.$tool" && ok=1
            ;;
        apt)
            _run_quiet apt-get install --only-upgrade -y "$tool" && ok=1
            ;;
        pacman)
            _run_quiet pacman -S --noconfirm "$tool" && ok=1
            ;;
        apk)
            _run_quiet apk upgrade "$tool" && ok=1
            ;;
        dnf)
            _run_quiet dnf upgrade -y "$tool" && ok=1
            ;;
        uv)
            _run_quiet uv tool upgrade "$tool" && ok=1
            ;;
        npm)
            _run_quiet npm update -g "$tool" && ok=1
            ;;
        go:*)
            _run_quiet go install "${source#go:}@latest" && ok=1
            ;;
        gh:*)
            command -v huber &>/dev/null || {
                printf "[${C_CROSS}] %-25s ${C_DIM}huber required${C_RESET}\n" "$tool"
                return 1
            }
            _run_quiet huber update "${source#gh:}" && ok=1
            ;;
        sat|repo:*)
            printf "[${C_CROSS}] %-25s ${C_DIM}cannot update source '%s'${C_RESET}\n" "$tool" "$source"
            return 1
            ;;
        *)
            printf "[${C_CROSS}] %-25s ${C_DIM}unknown source '%s'${C_RESET}\n" "$tool" "$source"
            return 1
            ;;
    esac

    if [[ $ok -eq 1 ]]; then
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
