#!/usr/bin/env bash
# sat install - install programs from various sources

# =============================================================================
# GITHUB INSTALLATION - MODULAR FUNCTIONS
# =============================================================================

# Temp file for subshell communication
_GH_RESULT_FILE="/tmp/sat-gh-result-$$"

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

# --- Method 2: Go install ---
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

# --- Method 3: Python via uv ---
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

# --- Method 4: Binary from GitHub releases ---
install_github_release() {
    local repo_path="$1"
    local repo_name="${repo_path##*/}"
    local os arch base_url tmpdir

    case "$(uname -s)" in
        Linux*)  os="linux" ;;
        Darwin*) os="darwin" ;;
        *)       return 1 ;;
    esac
    case "$(uname -m)" in
        x86_64)  arch="amd64" ;;
        aarch64) arch="arm64" ;;
        armv7l)  arch="arm" ;;
        *)       return 1 ;;
    esac

    base_url="https://github.com/$repo_path/releases/latest/download"
    tmpdir=$(mktemp -d)
    mkdir -p "$HOME/.local/bin"

    local patterns=(
        "$repo_name-$os-$arch"
        "$repo_name-${os}-${arch}.tar.gz"
        "$repo_name-${os}-${arch}.zip"
        "${repo_name}_${os}_${arch}"
        "${repo_name}_${os}_${arch}.tar.gz"
        "$repo_name-$os-x86_64"
        "${repo_name}-${os}-x86_64.tar.gz"
    )

    local asset_name=""
    for pattern in "${patterns[@]}"; do
        if curl -sfIL "$base_url/$pattern" &>/dev/null; then
            asset_name="$pattern"
            break
        fi
    done

    [[ -z "$asset_name" ]] && { rm -rf "$tmpdir"; return 1; }

    curl -sfL -o "$tmpdir/$asset_name" "$base_url/$asset_name" || { rm -rf "$tmpdir"; return 1; }

    case "$asset_name" in
        *.tar.gz|*.tgz)
            tar -xzf "$tmpdir/$asset_name" -C "$tmpdir"
            ;;
        *.zip)
            unzip -q "$tmpdir/$asset_name" -d "$tmpdir" 2>/dev/null
            ;;
        *)
            chmod +x "$tmpdir/$asset_name"
            mv "$tmpdir/$asset_name" "$HOME/.local/bin/$repo_name"
            rm -rf "$tmpdir"
            _gh_set_result "$repo_name" "gh:$repo_path"
            return 0
            ;;
    esac

    local binary
    binary=$(find "$tmpdir" -type f -name "$repo_name" 2>/dev/null | head -1)
    [[ -z "$binary" ]] && binary=$(find "$tmpdir" -type f -executable ! -name "*.sh" 2>/dev/null | head -1)

    if [[ -n "$binary" ]]; then
        chmod +x "$binary"
        mv "$binary" "$HOME/.local/bin/$repo_name"
        rm -rf "$tmpdir"
        _gh_set_result "$repo_name" "gh:$repo_path"
        return 0
    fi

    rm -rf "$tmpdir"
    return 1
}

# --- Method 5: Run install.sh from repo ---
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

# Install from GitHub repo - tries each method in order
# Args: repo_path (owner/repo format)
# Sets: _GH_INSTALLED_BIN, _GH_INSTALL_SOURCE (via temp file)
# Returns: 0 on success, 1 on failure
install_from_github() {
    local repo_path="$1"
    local repo_name="${repo_path##*/}"

    # Clean any stale result file
    rm -f "$_GH_RESULT_FILE"

    # Fetch repo structure once
    local tree
    tree=$(curl -s "https://api.github.com/repos/$repo_path/git/trees/main?recursive=1" | jq -r '.tree[].path' 2>/dev/null)
    [[ -z "$tree" || "$tree" == "null" ]] && \
        tree=$(curl -s "https://api.github.com/repos/$repo_path/git/trees/master?recursive=1" | jq -r '.tree[].path' 2>/dev/null)

    [[ -z "$tree" || "$tree" == "null" ]] && return 1

    # Try each method in order
    install_github_huber "$repo_path" && return 0
    install_github_release "$repo_path" && return 0
    install_github_go "$repo_path" "$tree" && return 0
    install_github_python "$repo_path" "$tree" && return 0
    install_github_script "$repo_path" "$tree" && return 0

    return 1
}

# =============================================================================
# SOURCE-SPECIFIC INSTALLATION
# =============================================================================

# Try installing from a specific source
# Returns 0 on success, 1 on failure
try_source() {
    local tool="$1" source="$2"

    case "$source" in
        cargo)
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
                local mgr=$(get_pkg_manager)
                if [[ -n "$mgr" ]] && pkg_install "$missing" "$mgr" &>/dev/null; then
                    printf "[${C_CHECK}] %-20s [${C_SYSTEM}system${C_RESET}] ${C_DIM}(build dep)${C_RESET}\n" "$missing"
                    cargo install "$tool" &>/dev/null && return 0
                fi
            fi
            return 1
            ;;
        uv)
            command -v uv &>/dev/null || return 1
            uv tool install "$tool" &>/dev/null
            ;;
        npm)
            command -v npm &>/dev/null || return 1
            npm show "$tool" >/dev/null 2>&1 || return 1
            npm install -g "$tool" &>/dev/null || return 1
            command -v "$tool" &>/dev/null
            ;;
        go)
            command -v go &>/dev/null || return 1
            local go_pkg="$tool"
            [[ "$go_pkg" != *"."* ]] && go_pkg="github.com/$tool"
            go install "${go_pkg}@latest" &>/dev/null
            ;;
        brew)
            command -v brew &>/dev/null || return 1
            brew info "$tool" &>/dev/null 2>&1 || return 1
            brew install "$tool" &>/dev/null
            ;;
        nix)
            command -v nix-env &>/dev/null || return 1
            nix-env -iA "nixpkgs.$tool" &>/dev/null
            ;;
        system)
            local mgr=$(get_pkg_manager)
            [[ -z "$mgr" ]] && return 1
            pkg_exists "$tool" "$mgr" || return 1
            pkg_install "$tool" "$mgr" &>/dev/null
            ;;
        sat)
            curl -sSL --fail --head "$SAT_BASE/cargo-bay/programs/${tool}.sh" >/dev/null 2>&1 || return 1
            source <(curl -sSL "$SAT_BASE/internal/fetcher.sh")
            sat_init && sat_run "$tool" &>/dev/null
            ;;
        gh)
            # Search GitHub for tool, then use unified install logic
            local repo=$(curl -s "https://api.github.com/search/repositories?q=$tool&per_page=1" | jq -r '.items[0].full_name' 2>/dev/null)
            [[ -z "$repo" || "$repo" == "null" ]] && return 1
            install_from_github "$repo"
            ;;
        gh-release)
            # Force GitHub release binary only
            local repo=$(curl -s "https://api.github.com/search/repositories?q=$tool&per_page=1" | jq -r '.items[0].full_name' 2>/dev/null)
            [[ -z "$repo" || "$repo" == "null" ]] && return 1
            install_github_release "$repo"
            ;;
        gh-script)
            # Force install.sh from GitHub repo
            local repo=$(curl -s "https://api.github.com/search/repositories?q=$tool&per_page=1" | jq -r '.items[0].full_name' 2>/dev/null)
            [[ -z "$repo" || "$repo" == "null" ]] && return 1
            local tree=$(curl -s "https://api.github.com/repos/$repo/git/trees/main?recursive=1" | jq -r '.tree[].path' 2>/dev/null)
            [[ -z "$tree" || "$tree" == "null" ]] && \
                tree=$(curl -s "https://api.github.com/repos/$repo/git/trees/master?recursive=1" | jq -r '.tree[].path' 2>/dev/null)
            install_github_script "$repo" "$tree"
            ;;
        *)
            return 1
            ;;
    esac
}

# Bootstrappers - always use sat wrapper
SAT_BOOTSTRAPPERS=(nix homebrew brew rustup)

# Install tool with fallback chain
# Sets: _INSTALL_SOURCE (source that succeeded)
# Returns 0 on success, 1 if all sources fail
install_with_fallback() {
    local tool="$1"
    _INSTALL_SOURCE=""

    # Force sat wrapper for bootstrappers
    for b in "${SAT_BOOTSTRAPPERS[@]}"; do
        if [[ "$tool" == "$b" ]]; then
            local wrapper_name="$tool"
            [[ "$tool" == "brew" ]] && wrapper_name="homebrew"
            if try_source "$wrapper_name" "sat"; then
                _INSTALL_SOURCE="sat"
                return 0
            fi
            return 1
        fi
    done

    for source in "${INSTALL_ORDER[@]}"; do
        try_source "$tool" "$source" &
        spin_with_style "$tool" $! "$source"
        if wait $!; then
            # For gh: get result from temp file
            if [[ "$source" == "gh" ]]; then
                _gh_get_result
                _INSTALL_SOURCE="$_GH_INSTALL_SOURCE"
            else
                _INSTALL_SOURCE=$(detect_source "$tool")
                [[ -z "$_INSTALL_SOURCE" || "$_INSTALL_SOURCE" == "unknown" ]] && _INSTALL_SOURCE="$source"
            fi
            return 0
        fi
    done

    return 1
}

# =============================================================================
# MAIN INSTALL FUNCTION
# =============================================================================

sat_install() {
    local DEFAULT_SOURCE=""
    local -a SPECS=()

    for arg in "$@"; do
        case "$arg" in
            --system|--sys) DEFAULT_SOURCE="system" ;;
            --rust|--rs)    DEFAULT_SOURCE="cargo" ;;
            --python|--py)  DEFAULT_SOURCE="uv" ;;
            --node|--js)    DEFAULT_SOURCE="npm" ;;
            --go)           DEFAULT_SOURCE="go" ;;
            --brew)         DEFAULT_SOURCE="brew" ;;
            --nix)          DEFAULT_SOURCE="nix" ;;
            --gh|--github)  DEFAULT_SOURCE="gh" ;;
            *)              SPECS+=("$arg") ;;
        esac
    done

    # Helper: add to system manifest (promotes from master if exists there)
    _track_install() {
        local tool="$1" src="$2"
        if master_has_tool "$tool"; then
            master_promote "$tool" "$src"
            printf "  ${C_DIM}(promoted from shell session)${C_RESET}\n"
        else
            manifest_add "$tool" "$src"
        fi
    }

    for SPEC in "${SPECS[@]}"; do
        parse_tool_spec "$SPEC"
        local PROGRAM="$_TOOL_NAME"
        local FORCE_SOURCE="${_TOOL_SOURCE:-$DEFAULT_SOURCE}"

        # Direct GitHub repo path (owner/repo)
        if [[ "$PROGRAM" == */* ]]; then
            local REPO_PATH="$PROGRAM"
            local REPO_NAME="${PROGRAM##*/}"

            install_from_github "$REPO_PATH" &
            spin_probe "$REPO_NAME" $!

            if wait $! && _gh_get_result; then
                _track_install "$_GH_INSTALLED_BIN" "$_GH_INSTALL_SOURCE"
                status_ok "$_GH_INSTALLED_BIN" "$_GH_INSTALL_SOURCE"
            else
                status_fail "$REPO_NAME not found"
            fi
            continue
        fi

        # Already installed check
        if [[ -z "$FORCE_SOURCE" ]] && command -v "$PROGRAM" &>/dev/null; then
            local existing_src=$(detect_source "$PROGRAM")
            if master_has_tool "$PROGRAM"; then
                local display=$(source_display "$existing_src")
                local color=$(source_color "$display")
                printf "%-30s [${color}%s${C_RESET}]\n" "$PROGRAM (shell session)" "$display"
                master_promote "$PROGRAM" "$existing_src"
                printf "  ${C_DIM}Promoted to system manifest${C_RESET}\n"
                continue
            fi
            local display=$(source_display "$existing_src")
            local color=$(source_color "$display")
            printf "%-30s [${color}%s${C_RESET}]\n" "$PROGRAM already installed" "$display"
            printf "  ${C_DIM}Use $PROGRAM:sys :brew :nix :rs :py :js :go to force${C_RESET}\n"
            continue
        fi

        # Forced source
        if [[ -n "$FORCE_SOURCE" ]]; then
            # gh-script needs tty for tmux - run synchronously
            if [[ "$FORCE_SOURCE" == "gh-script" ]]; then
                if try_source "$PROGRAM" "$FORCE_SOURCE"; then
                    local src="$FORCE_SOURCE"
                    local bin_name="$PROGRAM"
                    _gh_get_result && bin_name="$_GH_INSTALLED_BIN" && src="$_GH_INSTALL_SOURCE"
                    _track_install "$bin_name" "$src"
                    status_ok "$bin_name" "$src"
                else
                    status_fail "$PROGRAM not found in $(source_display "$FORCE_SOURCE")"
                fi
                continue
            fi

            # Other sources - background with spinner
            try_source "$PROGRAM" "$FORCE_SOURCE" &
            spin_with_style "$PROGRAM" $! "$FORCE_SOURCE"
            if wait $!; then
                local src="$FORCE_SOURCE"
                local bin_name="$PROGRAM"
                if [[ "$FORCE_SOURCE" == "gh" ]] && _gh_get_result; then
                    src="$_GH_INSTALL_SOURCE"
                    bin_name="$_GH_INSTALLED_BIN"
                fi
                _track_install "$bin_name" "$src"
                status_ok "$bin_name" "$src"
            else
                status_fail "$PROGRAM not found in $(source_display "$FORCE_SOURCE")"
            fi
            continue
        fi

        # Fallback chain
        if install_with_fallback "$PROGRAM"; then
            local installed_name="$PROGRAM"
            [[ -n "$_GH_INSTALLED_BIN" ]] && installed_name="$_GH_INSTALLED_BIN"
            _track_install "$installed_name" "$_INSTALL_SOURCE"
            status_ok "$installed_name" "$_INSTALL_SOURCE"
            _GH_INSTALLED_BIN=""
        else
            status_fail "$PROGRAM not found"
        fi
    done
}
