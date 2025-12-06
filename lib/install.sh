#!/usr/bin/env bash
# sat install - install programs from various sources

# Source installation modules
source "$SAT_INSTALL_DIR/github.sh"
source "$SAT_INSTALL_DIR/cargo.sh"
source "$SAT_INSTALL_DIR/system.sh"
source "$SAT_INSTALL_DIR/brew.sh"
source "$SAT_INSTALL_DIR/uv.sh"
source "$SAT_INSTALL_DIR/npm.sh"
source "$SAT_INSTALL_DIR/go.sh"
source "$SAT_INSTALL_DIR/nix.sh"
source "$SAT_INSTALL_DIR/sat.sh"
source "$SAT_LIB/search.sh"

# =============================================================================
# SOURCE-SPECIFIC INSTALLATION
# =============================================================================

# Try installing from a specific source
# Returns 0 on success, 1 on failure
try_source() {
    local tool="$1" source="$2"

    case "$source" in
        cargo)  install_cargo "$tool" ;;
        uv)     install_uv "$tool" ;;
        npm)    install_npm "$tool" ;;
        go)     install_go "$tool" ;;
        brew)   install_brew "$tool" ;;
        nix)    install_nix "$tool" ;;
        system) install_system "$tool" ;;
        sat)    install_sat "$tool" ;;
        gh)
            # Search GitHub for tool, get repo + language
            local gh_data=$(search_github "$tool" 1)
            local repo=$(echo "$gh_data" | jq -r '.items[0].full_name // empty')
            local lang=$(echo "$gh_data" | jq -r '.items[0].language // empty')
            [[ -z "$repo" ]] && return 1
            install_from_github "$repo" "$lang"
            ;;
        gh-release)
            # Force GitHub release binary via Huber (requires huber)
            command -v huber &>/dev/null || { echo "huber required for :rel installs (sat install huber)" >&2; return 1; }
            local repo=$(search_github "$tool" 1 | jq -r '.items[0].full_name // empty')
            [[ -z "$repo" ]] && return 1
            install_github_huber "$repo"
            ;;
        gh-script)
            # Force install.sh from GitHub repo
            local repo=$(search_github "$tool" 1 | jq -r '.items[0].full_name // empty')
            [[ -z "$repo" ]] && return 1
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

# Install tool with fallback chain
# Sets: _INSTALL_SOURCE (source that succeeded)
# Returns 0 on success, 1 if all sources fail
install_with_fallback() {
    local tool="$1"
    _INSTALL_SOURCE=""

    for source in "${INSTALL_ORDER[@]}"; do
        try_source "$tool" "$source" >/dev/null 2>&1 &
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

    # Helper: route manifest writes based on context
    # SAT_MANIFEST_TARGET=session → session + master manifest (temporary)
    # SAT_MANIFEST_TARGET unset   → system manifest (permanent)
    _track_install() {
        local tool="$1" src="$2"

        if [[ "$SAT_MANIFEST_TARGET" == "session" ]]; then
            # Shell session: track in session + master manifest
            pid_manifest_add "$SAT_SESSION" "$tool" "$src"
            master_add "$tool" "$src" "$SAT_SESSION"
        elif master_has_tool "$tool"; then
            # Permanent install but tool exists in session: promote it
            master_promote "$tool" "$src"
            printf "  ${C_DIM}(promoted from shell session)${C_RESET}\n"
        else
            # Permanent install: system manifest
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
            try_source "$PROGRAM" "$FORCE_SOURCE" >/dev/null 2>&1 &
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
