#!/usr/bin/env bash
# Install script - Execute install.sh from GitHub repos (DISABLED)
#
# This module is extracted but not currently used in the main flow.
# To re-enable: source this file in install.sh and add gh-script to try_source()

# Install via repository install.sh script
_install_script() {
    local repo_path="$1"
    local tree="$2"
    local repo_name="${repo_path##*/}"

    echo "$tree" | grep -q '^install.sh$' || return 1

    local install_url="https://raw.githubusercontent.com/$repo_path/main/install.sh"
    curl -sfI "$install_url" &>/dev/null || \
        install_url="https://raw.githubusercontent.com/$repo_path/master/install.sh"
    curl -sfI "$install_url" &>/dev/null || return 1

    local bin_dirs=("$HOME/.local/bin" "$HOME/bin" "$HOME/.cargo/bin" "/usr/local/bin")
    local before=$(for d in "${bin_dirs[@]}"; do ls -1 "$d" 2>/dev/null; done | sort -u)

    if [[ -n "$SAT_DEBUG" ]]; then
        curl -sfL "$install_url" | bash
    else
        curl -sfL "$install_url" | bash &>/dev/null
    fi

    if [[ $? -eq 0 ]]; then
        local after=$(for d in "${bin_dirs[@]}"; do ls -1 "$d" 2>/dev/null; done | sort -u)
        local new_bin=$(comm -13 <(echo "$before") <(echo "$after") | head -1)
        _gh_set_result "${new_bin:-$repo_name}" "gh:$repo_path"
        return 0
    fi
    return 1
}

# Install from GitHub using install.sh script
install_github_script() {
    local repo="$1"
    local tree=$(_fetch_tree "$repo")
    [[ -z "$tree" || "$tree" == "null" ]] && return 1
    _install_script "$repo" "$tree"
}
