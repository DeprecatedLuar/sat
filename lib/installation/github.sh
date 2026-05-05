#!/usr/bin/env bash
# GitHub installation methods - install from GitHub repos

# Temp file for subshell communication
_GH_RESULT_FILE="/tmp/sat-gh-result-$$"

# Fetch repo tree structure (tries main, then master)
_fetch_tree() {
    local repo="$1"
    local tree

    # Use gh CLI if authenticated, else curl
    if command -v gh &>/dev/null && gh auth status &>/dev/null 2>&1; then
        tree=$(gh api "repos/$repo/git/trees/main?recursive=1" 2>/dev/null | jq -r '.tree[].path' 2>/dev/null)
        [[ -z "$tree" || "$tree" == "null" ]] && \
            tree=$(gh api "repos/$repo/git/trees/master?recursive=1" 2>/dev/null | jq -r '.tree[].path' 2>/dev/null)
    else
        tree=$(curl -s "https://api.github.com/repos/$repo/git/trees/main?recursive=1" | jq -r '.tree[].path' 2>/dev/null)
        [[ -z "$tree" || "$tree" == "null" ]] && \
            tree=$(curl -s "https://api.github.com/repos/$repo/git/trees/master?recursive=1" | jq -r '.tree[].path' 2>/dev/null)
    fi

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

    _run_quiet huber install "$repo_path" || return 1

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

    _run_quiet uv tool install "git+https://github.com/$repo_path" || return 1
    _gh_set_result "$repo_name" "uv"
    return 0
}

# --- Method 4: AppImage (portable GUI apps) ---
# Downloads AppImage from GitHub releases and installs to sat's bin directory
install_github_appimage() {
    local repo_path="$1"
    local repo_name="${repo_path##*/}"

    [[ -n "$SAT_DEBUG" ]] && echo "[debug]   trying appimage for $repo_path" >&2

    # Detect platform
    local arch=$(uname -m)
    local arch_pattern=""
    case "$arch" in
        x86_64)  arch_pattern="(amd64|x86_64)" ;;
        aarch64) arch_pattern="(arm64|aarch64)" ;;
        armv7l)  arch_pattern="(armhf|armv7)" ;;
        *)
            [[ -n "$SAT_DEBUG" ]] && echo "[debug]   unsupported architecture: $arch" >&2
            return 1
            ;;
    esac

    # Fetch latest release assets (use gh CLI if authenticated, else curl)
    [[ -n "$SAT_DEBUG" ]] && echo "[debug]   fetching releases from GitHub API..." >&2
    local release_data
    if command -v gh &>/dev/null && gh auth status &>/dev/null 2>&1; then
        [[ -n "$SAT_DEBUG" ]] && echo "[debug]   using authenticated gh CLI" >&2
        release_data=$(gh api "repos/$repo_path/releases/latest" 2>/dev/null)
    else
        [[ -n "$SAT_DEBUG" ]] && echo "[debug]   using unauthenticated curl (may hit rate limits)" >&2
        release_data=$(curl -s "https://api.github.com/repos/$repo_path/releases/latest")
    fi

    # Check for rate limit error
    if echo "$release_data" | jq -e '.message' 2>/dev/null | grep -q "rate limit"; then
        echo "Error: GitHub API rate limit exceeded" >&2
        echo "Tip: Authenticate with 'gh auth login' for higher limits" >&2
        return 1
    fi
    local assets=$(echo "$release_data" | jq -r '.assets[]? | select(.name | endswith(".AppImage")) | .name + "|" + .browser_download_url')

    if [[ -z "$assets" ]]; then
        [[ -n "$SAT_DEBUG" ]] && echo "[debug]   no AppImage assets found" >&2
        return 1
    fi

    [[ -n "$SAT_DEBUG" ]] && echo "[debug]   found AppImage assets, filtering for $arch_pattern..." >&2

    # Filter for matching architecture
    local asset_url=$(echo "$assets" | grep -iE "$arch_pattern" | head -1 | cut -d'|' -f2)
    local asset_name=$(echo "$assets" | grep -iE "$arch_pattern" | head -1 | cut -d'|' -f1)

    # Fallback: if no arch-specific match, try any AppImage (assumes x86_64)
    if [[ -z "$asset_url" ]]; then
        [[ -n "$SAT_DEBUG" ]] && echo "[debug]   no arch-specific match, trying any AppImage..." >&2
        asset_url=$(echo "$assets" | head -1 | cut -d'|' -f2)
        asset_name=$(echo "$assets" | head -1 | cut -d'|' -f1)
    fi

    if [[ -z "$asset_url" ]]; then
        [[ -n "$SAT_DEBUG" ]] && echo "[debug]   no AppImage available" >&2
        return 1
    fi

    [[ -n "$SAT_DEBUG" ]] && echo "[debug]   selected: $asset_name" >&2

    # Extract clean app name from filename
    local name="${asset_name%.AppImage}"
    local app_name=$(echo "$name" | sed -E 's/[_-]([0-9]+\.[0-9]|v?[0-9]+).*//')

    # Fallback to repo name if extraction failed
    [[ -z "$app_name" ]] && app_name="$repo_name"

    # Lowercase normalization
    app_name="${app_name,,}"

    # Install to sat's AppImage directory
    local appimage_dir="$HOME/.local/share/sat/bin/appimages"
    local appimage_path="$appimage_dir/$app_name"
    local symlink_path="$HOME/.local/bin/$app_name"

    [[ -n "$SAT_DEBUG" ]] && echo "[debug]   downloading to $appimage_path..." >&2
    mkdir -p "$appimage_dir"
    _run_quiet curl -L -o "$appimage_path" "$asset_url" || {
        [[ -n "$SAT_DEBUG" ]] && echo "[debug]   download failed" >&2
        return 1
    }
    chmod +x "$appimage_path"

    # Symlink to PATH
    [[ -n "$SAT_DEBUG" ]] && echo "[debug]   creating symlink $symlink_path" >&2
    ln -sf "$appimage_path" "$symlink_path"

    [[ -n "$SAT_DEBUG" ]] && echo "[debug]   appimage install successful: $app_name" >&2
    _gh_set_result "$app_name" "appimage:$repo_path"
    return 0
}

# --- Method 5: Portable tarballs ---
# TODO: Extract portable .tar.gz releases when AppImage is not available
# Similar to AppImage but with extraction logic to find the binary

# --- Method 6: Run install.sh from repo ---
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
    if [[ -n "$SAT_DEBUG" ]]; then
        curl -sfL "$install_url" | bash
    else
        curl -sfL "$install_url" | bash &>/dev/null
    fi
    if [[ $? -eq 0 ]]; then
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

    [[ -n "$SAT_DEBUG" ]] && echo "[debug]   install_github called: input=$input, method=$method" >&2

    local repo lang tree

    # Clean any stale result file
    rm -f "$_GH_RESULT_FILE"

    # Handle direct repo path vs search
    if [[ "$input" == */* ]]; then
        repo="$input"
        [[ -n "$SAT_DEBUG" ]] && echo "[debug]   direct repo path: $repo" >&2
    else
        [[ -n "$SAT_DEBUG" ]] && echo "[debug]   searching GitHub for: $input" >&2
        local gh_data=$(search_github "$input" 1)
        repo=$(echo "$gh_data" | jq -r '.items[0].full_name // empty')
        lang=$(echo "$gh_data" | jq -r '.items[0].language // empty')
        if [[ -z "$repo" ]]; then
            [[ -n "$SAT_DEBUG" ]] && echo "[debug]   search returned no results" >&2
            return 1
        fi
        [[ -n "$SAT_DEBUG" ]] && echo "[debug]   found repo: $repo (language: $lang)" >&2
    fi

    # Fetch tree (needed for script and language detection)
    [[ -n "$SAT_DEBUG" ]] && echo "[debug]   fetching repo tree..." >&2
    tree=$(_fetch_tree "$repo")
    if [[ -z "$tree" || "$tree" == "null" ]]; then
        [[ -n "$SAT_DEBUG" ]] && echo "[debug]   tree fetch failed" >&2
        return 1
    fi

    case "$method" in
        auto)
            # Full fallback: huber → appimage → language → script
            [[ -n "$SAT_DEBUG" ]] && echo "[debug]   trying huber..." >&2
            install_github_huber "$repo" && return 0
            [[ -n "$SAT_DEBUG" ]] && echo "[debug]   huber failed, trying appimage..." >&2
            install_github_appimage "$repo" && return 0
            [[ -n "$SAT_DEBUG" ]] && echo "[debug]   appimage failed, trying language-based install..." >&2
            case "$lang" in
                Go)     install_github_go "$repo" "$tree" && return 0 ;;
                Python) install_github_python "$repo" "$tree" && return 0 ;;
                "")
                    install_github_go "$repo" "$tree" && return 0
                    install_github_python "$repo" "$tree" && return 0
                    ;;
            esac
            [[ -n "$SAT_DEBUG" ]] && echo "[debug]   language install failed, trying install.sh..." >&2
            install_github_script "$repo" "$tree" && return 0
            return 1
            ;;
        release)
            command -v huber &>/dev/null || { echo "huber required for :rel installs (sat source huber)" >&2; return 1; }
            install_github_huber "$repo"
            ;;
        appimage)
            install_github_appimage "$repo"
            ;;
        script)
            install_github_script "$repo" "$tree"
            ;;
        *)
            return 1
            ;;
    esac
}
