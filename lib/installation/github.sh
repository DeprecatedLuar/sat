#!/usr/bin/env bash
# GitHub installation methods - install from GitHub repos

# Temp file for subshell communication
_GH_RESULT_FILE="/tmp/sat-gh-result-$$"

# Fetch repo tree structure (tries main, then master)
_fetch_tree() {
    local repo="$1"
    local tree
    tree=$(curl -s "https://api.github.com/repos/$repo/git/trees/main?recursive=1" | jq -r '.tree[].path' 2>/dev/null)
    [[ -z "$tree" || "$tree" == "null" ]] && \
        tree=$(curl -s "https://api.github.com/repos/$repo/git/trees/master?recursive=1" | jq -r '.tree[].path' 2>/dev/null)
    echo "$tree"
}

# Write result to temp file (for subshell communication)
_gh_set_result() {
    local bin="$1" src="$2"
    echo "BIN=$bin" > "$_GH_RESULT_FILE"
    echo "SRC=$src" >> "$_GH_RESULT_FILE"
}

# Read result from temp file
_gh_get_result() {
    [[ -f "$_GH_RESULT_FILE" ]] || return 1
    source "$_GH_RESULT_FILE"
    _GH_INSTALLED_BIN="$BIN"
    _GH_INSTALL_SOURCE="$SRC"
    rm -f "$_GH_RESULT_FILE"
}

# --- Method 1: Huber (binary release manager) ---
# Huber is REQUIRED for release binary installs - provides tracking, removal, updates
install_github_huber() {
    local repo_path="$1"
    local repo_name="${repo_path##*/}"

    command -v huber &>/dev/null || return 1

    huber install "$repo_path" &>/dev/null || return 1

    # Huber installs to ~/.huber/bin - symlink to ~/.local/bin for PATH access
    local huber_bin="$HOME/.huber/bin/$repo_name"
    if [[ -x "$huber_bin" ]]; then
        mkdir -p "$HOME/.local/bin"
        ln -sf "$huber_bin" "$HOME/.local/bin/$repo_name"
        _gh_set_result "$repo_name" "gh:$repo_path"
        return 0
    fi

    return 1
}

# --- Method 2: Go install (build from source) ---
install_github_go() {
    local repo_path="$1"
    local tree="$2"
    local repo_name="${repo_path##*/}"

    command -v go &>/dev/null || return 1
    echo "$tree" | grep -q '^go.mod$' || return 1

    local go_bin go_path

    # Check cmd/*/main.go pattern
    go_bin=$(echo "$tree" | grep -oP '^cmd/\K[^/]+(?=/main\.go$)' | head -1)

    # Check {repo_name}/main.go pattern (subpackage)
    [[ -z "$go_bin" ]] && echo "$tree" | grep -q "^${repo_name}/main\.go$" && go_bin="$repo_name"

    # Build go path
    if [[ -n "$go_bin" ]]; then
        go_path="github.com/$repo_path/$go_bin@latest"
    elif echo "$tree" | grep -q '^main.go$'; then
        go_path="github.com/$repo_path@latest"
    else
        return 1
    fi

    go install "$go_path" &>/dev/null || return 1
    _gh_set_result "${go_bin:-$repo_name}" "go:github.com/$repo_path"
    return 0
}

# --- Method 3: Python via uv (build from source) ---
install_github_python() {
    local repo_path="$1"
    local tree="$2"
    local repo_name="${repo_path##*/}"

    command -v uv &>/dev/null || return 1
    echo "$tree" | grep -qE '^(pyproject.toml|setup.py|setup.cfg)$' || return 1

    uv tool install "git+https://github.com/$repo_path" &>/dev/null || return 1
    _gh_set_result "$repo_name" "uv"
    return 0
}

# --- Method 4: Run install.sh from repo ---
# Snapshots ~/.local/bin before/after to detect installed binary name
install_github_script() {
    local repo_path="$1"
    local tree="$2"
    local repo_name="${repo_path##*/}"

    echo "$tree" | grep -q '^install.sh$' || return 1

    local install_url="https://raw.githubusercontent.com/$repo_path/main/install.sh"
    curl -sfI "$install_url" &>/dev/null || \
        install_url="https://raw.githubusercontent.com/$repo_path/master/install.sh"

    # Verify URL is reachable
    curl -sfI "$install_url" &>/dev/null || return 1

    # Snapshot all relevant bin dirs before install
    local bin_dirs=("$HOME/.local/bin" "$HOME/bin" "$HOME/.cargo/bin" "/usr/local/bin")
    local before=$(for d in "${bin_dirs[@]}"; do ls -1 "$d" 2>/dev/null; done | sort -u)

    # Run install script
    if curl -sfL "$install_url" | bash &>/dev/null; then
        # Compare to find new binaries
        local after=$(for d in "${bin_dirs[@]}"; do ls -1 "$d" 2>/dev/null; done | sort -u)
        local new_bin=$(comm -13 <(echo "$before") <(echo "$after") | head -1)

        # Use detected binary name, fallback to repo name
        local bin_name="${new_bin:-$repo_name}"
        _gh_set_result "$bin_name" "gh:$repo_path"
        return 0
    fi
    return 1
}

# =============================================================================
# GITHUB INSTALLATION - ORCHESTRATOR
# =============================================================================

# Install from GitHub - unified entry point
# Args: input (tool name or owner/repo), method (auto|release|script)
# Sets: _GH_INSTALLED_BIN, _GH_INSTALL_SOURCE (via temp file)
# Returns: 0 on success, 1 on failure
install_github() {
    local input="$1"
    local method="${2:-auto}"

    local repo lang tree

    # Clean any stale result file
    rm -f "$_GH_RESULT_FILE"

    # Handle direct repo path vs search
    if [[ "$input" == */* ]]; then
        repo="$input"
    else
        local gh_data=$(search_github "$input" 1)
        repo=$(echo "$gh_data" | jq -r '.items[0].full_name // empty')
        lang=$(echo "$gh_data" | jq -r '.items[0].language // empty')
        [[ -z "$repo" ]] && return 1
    fi

    # Fetch tree (needed for script and language detection)
    tree=$(_fetch_tree "$repo")
    [[ -z "$tree" || "$tree" == "null" ]] && return 1

    case "$method" in
        auto)
            # Full fallback: huber → language → script
            install_github_huber "$repo" && return 0
            case "$lang" in
                Go)     install_github_go "$repo" "$tree" && return 0 ;;
                Python) install_github_python "$repo" "$tree" && return 0 ;;
            esac
            install_github_script "$repo" "$tree" && return 0
            return 1
            ;;
        release)
            command -v huber &>/dev/null || { echo "huber required for :rel installs (sat source huber)" >&2; return 1; }
            install_github_huber "$repo"
            ;;
        script)
            install_github_script "$repo" "$tree"
            ;;
        *)
            return 1
            ;;
    esac
}
