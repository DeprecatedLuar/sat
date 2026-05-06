#!/usr/bin/env bash
# Go - Self-contained source module

# Install tool via go install
install_go() {
    local tool="$1"

    command -v go &>/dev/null || return 1
    local go_pkg="$tool"
    [[ "$go_pkg" != *"."* ]] && go_pkg="github.com/$tool"
    _run_quiet go install "${go_pkg}@latest"
}

# Install from GitHub repo with Go detection
# Returns: binary name via stdout on success
install_go_github() {
    local repo_path="$1"
    local tree="$2"
    local repo_name="${repo_path##*/}"

    command -v go &>/dev/null || return 1
    echo "$tree" | grep -q '^go.mod$' || return 1

    # Lowercase normalization (Go module paths are case-sensitive, GitHub URLs are not)
    repo_path="${repo_path,,}"

    local go_bin go_path go_subdir=""

    # Check cmd/*/main.go pattern
    go_bin=$(echo "$tree" | grep -oP '^cmd/\K[^/]+(?=/main\.go$)' | head -1)
    [[ -n "$go_bin" ]] && go_subdir="cmd"

    # Check {repo_name}/main.go pattern (subpackage)
    [[ -z "$go_bin" ]] && echo "$tree" | grep -q "^${repo_name}/main\.go$" && go_bin="$repo_name"

    # Build go path
    if [[ -n "$go_bin" ]]; then
        go_path="github.com/$repo_path/${go_subdir:+$go_subdir/}$go_bin@latest"
    elif echo "$tree" | grep -q '^main.go$'; then
        go_path="github.com/$repo_path@latest"
    else
        return 1
    fi

    _run_quiet go install "$go_path" || return 1
    echo "${go_bin:-$repo_name}"
    return 0
}

# Uninstall Go binary
uninstall_go() {
    local pkg="$1"
    rm -f "$GOPATH/bin/$pkg" "$HOME/go/bin/$pkg" 2>/dev/null
}
