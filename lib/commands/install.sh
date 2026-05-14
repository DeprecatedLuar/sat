#!/usr/bin/env bash
# sat install - install programs from various sources

# Source all source modules (self-contained: search + install + uninstall)
source "$SAT_LIB/sources/github.sh"
source "$SAT_LIB/sources/cargo.sh"
source "$SAT_LIB/sources/system.sh"
source "$SAT_LIB/sources/brew.sh"
source "$SAT_LIB/sources/uv.sh"
source "$SAT_LIB/sources/npm.sh"
source "$SAT_LIB/sources/go.sh"
source "$SAT_LIB/sources/nix.sh"
source "$SAT_LIB/sources/flatpak.sh"
source "$SAT_LIB/sources/sat.sh"

# =============================================================================
# VERSION DETECTION
# =============================================================================

# Query latest version from registry before install
# Args: tool, source
# Returns: version string (or empty if not available)
query_version_before_install() {
    local tool="$1" source="$2"

    case "$source" in
        npm)    query_latest_version_npm "$tool" ;;
        cargo)  query_latest_version_cargo "$tool" ;;
        uv)     query_latest_version_uv "$tool" ;;
        brew)   query_latest_version_brew "$tool" ;;
        *)      return 1 ;;  # No registry query available
    esac
}

# Detect identity and version after successful install
# Args: tool, source, [cached_version]
# Outputs: identity (line 1), version (line 2)
detect_install_metadata() {
    local tool="$1" source_string="$2" cached_version="${3:-}"
    local identity="" version="$cached_version"

    # Parse source string (might be "gh:owner/repo" or "appimage:owner/repo")
    local source=$(get_source_type "$source_string")
    local existing_identity=$(get_source_identity "$source_string")

    case "$source" in
        npm)
            identity=$(get_npm_package_name "$tool")
            # Use cached version if available, else query installed
            [[ -z "$version" ]] && version=$(get_version_from_npm "$tool")
            ;;
        cargo)
            [[ -z "$version" ]] && version=$(get_version_from_cargo "$tool")
            ;;
        brew)
            [[ -z "$version" ]] && version=$(get_version_from_brew "$tool")
            ;;
        nix)
            [[ -z "$version" ]] && version=$(get_version_from_nix "$tool")
            ;;
        go)
            [[ -z "$version" ]] && version=$(get_version_from_go "$tool")
            ;;
        uv)
            [[ -z "$version" ]] && version=$(get_version_from_uv "$tool")
            ;;
        flatpak)
            # Identity should be set by install_flatpak via _FLATPAK_APP_ID
            identity="${_FLATPAK_APP_ID:-$existing_identity}"
            [[ -n "$identity" && -z "$version" ]] && version=$(get_version_from_flatpak "$identity")
            ;;
        gh|appimage)
            # Identity comes from source string (owner/repo)
            identity="$existing_identity"
            [[ -n "$identity" && -z "$version" ]] && version=$(get_version_from_github "$identity")
            ;;
        apt|pacman|apk|dnf|nixos)
            [[ -z "$version" ]] && version=$(get_version_from_system "$tool")
            ;;
    esac

    echo "$identity"
    echo "$version"
}

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
        flatpak) install_flatpak "$tool" ;;
        system) install_system "$tool" ;;
        sat)    install_sat "$tool" ;;
        gh)          install_github "$tool" "auto" ;;
        gh-release)  install_github "$tool" "release" ;;
        gh-appimage) install_github "$tool" "appimage" ;;
        *)
            return 1
            ;;
    esac
}

# Install tool with fallback chain
# Sets: _INSTALL_SOURCE (source that succeeded), _CACHED_VERSION (from registry query)
# Returns 0 on success, 1 if all sources fail
install_with_fallback() {
    local tool="$1"
    _INSTALL_SOURCE=""
    _CACHED_VERSION=""

    for source in "${INSTALL_ORDER[@]}"; do
        # Query registry for version before attempting install
        local queried_version=$(query_version_before_install "$tool" "$source" 2>/dev/null)
        [[ -n "$SAT_DEBUG" && -n "$queried_version" ]] && \
            printf "${C_DIM}[debug] registry version for %s: %s${C_RESET}\n" "$tool" "$queried_version" >&2

        local _result
        if [[ -n "$SAT_DEBUG" ]]; then
            printf "${C_DIM}[debug] trying %s via %s${C_RESET}\n" "$tool" "$source" >&2
            try_source "$tool" "$source"; _result=$?
        else
            try_source "$tool" "$source" >/dev/null 2>&1 &
            spin_with_style "$tool" $! "$source"
            wait $!; _result=$?
        fi
        if [[ $_result -eq 0 ]]; then
            _CACHED_VERSION="$queried_version"
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
            --flatpak)      DEFAULT_SOURCE="flatpak" ;;
            --gh|--github)  DEFAULT_SOURCE="gh" ;;
            --debug)        export SAT_DEBUG=1 ;;
            *)              SPECS+=("$arg") ;;
        esac
    done

    for SPEC in "${SPECS[@]}"; do
        parse_tool_spec "$SPEC"
        local PROGRAM="$_TOOL_NAME"
        local FORCE_SOURCE="${_TOOL_SOURCE:-$DEFAULT_SOURCE}"

        # Direct GitHub repo path (owner/repo)
        if [[ "$PROGRAM" == */* ]]; then
            local REPO_PATH="$PROGRAM"
            local REPO_NAME="${PROGRAM##*/}"

            # Query latest release version from GitHub
            local cached_version=$(query_latest_version_github "$REPO_PATH" 2>/dev/null)
            [[ -n "$SAT_DEBUG" && -n "$cached_version" ]] && \
                printf "${C_DIM}[debug] GitHub latest release: %s${C_RESET}\n" "$cached_version" >&2

            install_github "$REPO_PATH" "auto" &
            spin_probe "$REPO_NAME" $!

            if wait $! && _gh_get_result; then
                local metadata identity version
                metadata=$(detect_install_metadata "$_GH_INSTALLED_BIN" "$_GH_INSTALL_SOURCE" "$cached_version")
                identity=$(echo "$metadata" | head -1)
                version=$(echo "$metadata" | tail -1)
                track_install "$_GH_INSTALLED_BIN" "$(get_source_type "$_GH_INSTALL_SOURCE")" "$identity" "$version"
                status_ok "$_GH_INSTALLED_BIN" "$_GH_INSTALL_SOURCE"
            else
                status_fail "$REPO_NAME not found"
            fi
            continue
        fi

        # Already installed check
        if [[ -z "$FORCE_SOURCE" ]] && command -v "$PROGRAM" &>/dev/null; then
            if master_has_tool "$PROGRAM"; then
                # Get full source string from master manifest (preserves identity:version)
                local full_source=$(grep "^$PROGRAM:" "$SAT_SHELL_MASTER" 2>/dev/null | head -1 | cut -d: -f2)
                local display=$(source_display "$(get_source_type "$full_source")")
                local color=$(source_color "$display")
                printf "%-30s [${color}%s${C_RESET}]\n" "$PROGRAM (shell session)" "$display"
                master_promote "$PROGRAM" "$full_source"
                printf "  ${C_DIM}Promoted to system manifest${C_RESET}\n"
                continue
            fi
            local existing_src=$(detect_source "$PROGRAM")
            local display=$(source_display "$existing_src")
            local color=$(source_color "$display")
            printf "%-30s [${color}%s${C_RESET}]\n" "$PROGRAM already installed" "$display"
            printf "  ${C_DIM}Use $PROGRAM:sys :brew :nix :fpk :rs :py :js :go :img to force${C_RESET}\n"
            continue
        fi

        # Forced source
        if [[ -n "$FORCE_SOURCE" ]]; then
            # Query registry for version before install
            local cached_version=$(query_version_before_install "$PROGRAM" "$FORCE_SOURCE" 2>/dev/null)
            [[ -n "$SAT_DEBUG" && -n "$cached_version" ]] && \
                printf "${C_DIM}[debug] registry version for %s: %s${C_RESET}\n" "$PROGRAM" "$cached_version" >&2

            # Background with spinner (or synchronous in debug mode)
            local _forced_result
            if [[ -n "$SAT_DEBUG" ]]; then
                printf "${C_DIM}[debug] trying %s via %s${C_RESET}\n" "$PROGRAM" "$FORCE_SOURCE" >&2
                try_source "$PROGRAM" "$FORCE_SOURCE"; _forced_result=$?
            else
                local error_file=$(mktemp)
                try_source "$PROGRAM" "$FORCE_SOURCE" >/dev/null 2>"$error_file" &
                spin_with_style "$PROGRAM" $! "$FORCE_SOURCE"
                wait $!; _forced_result=$?
            fi
            if [[ $_forced_result -eq 0 ]]; then
                local src="$FORCE_SOURCE"
                local bin_name="$PROGRAM"
                if [[ "$FORCE_SOURCE" == "gh" ]] && _gh_get_result; then
                    src="$_GH_INSTALL_SOURCE"
                    bin_name="$_GH_INSTALLED_BIN"
                elif [[ "$FORCE_SOURCE" == "npm" ]] && [[ -n "$_NPM_INSTALLED_PKG" ]]; then
                    bin_name="$_NPM_INSTALLED_PKG"
                fi

                # Detect identity and version (use cached version from registry query)
                local metadata identity version
                metadata=$(detect_install_metadata "$bin_name" "$src" "$cached_version")
                identity=$(echo "$metadata" | head -1)
                version=$(echo "$metadata" | tail -1)

                track_install "$bin_name" "$(get_source_type "$src")" "$identity" "$version"
                status_ok "$bin_name" "$src"
            else
                # Show actual error if available, otherwise generic message
                if [[ -z "$SAT_DEBUG" ]] && [[ -s "$error_file" ]]; then
                    local error_msg=$(cat "$error_file")
                    status_fail "${error_msg#Error: }"  # Strip "Error: " prefix
                else
                    status_fail "$PROGRAM not found in $(source_display "$FORCE_SOURCE")"
                fi
            fi
            [[ -z "$SAT_DEBUG" ]] && rm -f "$error_file"
            continue
        fi

        # Fallback chain
        if install_with_fallback "$PROGRAM"; then
            local installed_name="$PROGRAM"
            [[ -n "$_GH_INSTALLED_BIN" ]] && installed_name="$_GH_INSTALLED_BIN"
            [[ -n "$_NPM_INSTALLED_PKG" ]] && installed_name="$_NPM_INSTALLED_PKG"

            # Detect identity and version (use cached version from registry query)
            local metadata identity version
            metadata=$(detect_install_metadata "$installed_name" "$_INSTALL_SOURCE" "$_CACHED_VERSION")
            identity=$(echo "$metadata" | head -1)
            version=$(echo "$metadata" | tail -1)

            track_install "$installed_name" "$(get_source_type "$_INSTALL_SOURCE")" "$identity" "$version"
            status_ok "$installed_name" "$_INSTALL_SOURCE"
            _GH_INSTALLED_BIN=""
            _NPM_INSTALLED_PKG=""
            _CACHED_VERSION=""
        else
            status_fail "$PROGRAM not found"
        fi
    done
}
