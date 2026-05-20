#!/usr/bin/env bash
# scan.sh - Scan ecosystem directories and add found packages to manifest

# ============================================================================
# Configuration Constants
# ============================================================================

# Default paths (with env var overrides)
readonly DEFAULT_CARGO_HOME="$HOME/.cargo"
readonly DEFAULT_NPM_PREFIX="$HOME/.npm-global"
readonly CARGO_BIN_SUBDIR="bin"
readonly NPM_BIN_SUBDIR="bin"

# Fixed directory paths
readonly UV_TOOLS_DIR="$HOME/.local/share/uv/tools"
readonly NIX_PROFILE_BIN="$HOME/.nix-profile/bin"
readonly LOCAL_BIN_DIR="$HOME/.local/bin"
readonly APPIMAGE_DIR="$HOME/.local/share/sat/bin/appimages"
readonly SAT_MANAGED_PREFIX="$HOME/.local/share/sat/"

# Brew list filter
readonly BREW_BIN_GREP_PATTERN='/bin/'

# AppImage zsync extraction
readonly ZSYNC_PATTERN='gh-releases-zsync\|\K[^|]+\|[^|]+'
readonly ZSYNC_SEPARATOR='|'
readonly ZSYNC_REPLACEMENT='/'

# Source type names
readonly SRC_CARGO="cargo"
readonly SRC_NPM="npm"
readonly SRC_UV="uv"
readonly SRC_GO="go"
readonly SRC_BREW="brew"
readonly SRC_NIX="nix"
readonly SRC_FLATPAK="flatpak"
readonly SRC_APPIMAGE="appimage"
readonly SRC_UNKNOWN="unknown"

# Return codes
readonly SUCCESS=0
readonly SKIPPED=1

source "$SAT_LIB/sources/flatpak.sh"
source "$SAT_LIB/sources/cargo.sh"
source "$SAT_LIB/sources/npm.sh"
source "$SAT_LIB/sources/brew.sh"
source "$SAT_LIB/sources/nix.sh"
source "$SAT_LIB/sources/go.sh"
source "$SAT_LIB/sources/uv.sh"
source "$SAT_LIB/sources/system.sh"
source "$SAT_LIB/sources/github.sh"
source "$SAT_LIB/sources/appimage.sh"

# Scan modules
source "$SAT_LIB/scan/system.sh"
source "$SAT_LIB/scan/exclusion.sh"

sat_scan() {
    echo "Scanning ecosystems..."

    # Try to add tool to manifest (returns 0 on success, 1 if skipped)
    # Args: prog, source_or_full_string
    # If source contains colons, it's already formatted (source:identity:version)
    _try_add_tool() {
        local prog="$1" src_input="$2"

        # Parse source string (might already include identity)
        local source identity version
        source=$(get_source_type "$src_input")
        identity=$(get_source_identity "$src_input")

        # Skip if excluded or already tracked
        is_excluded "$prog" "$source" && return $SKIPPED
        manifest_has "$prog" && return $SKIPPED
        master_has_tool "$prog" && return $SKIPPED

        # Detect identity (for sources that need it)
        if [[ -z "$identity" ]]; then
            case "$source" in
                npm) identity=$(get_npm_package_name "$prog") ;;
            esac
        fi

        # Detect version based on source type
        if [[ -z "$version" ]]; then
            case "$source" in
                npm)     version=$(get_version_from_npm "$prog") ;;
                cargo)   version=$(get_version_from_cargo "$prog") ;;
                brew)    version=$(get_version_from_brew "$prog") ;;
                nix)     version=$(get_version_from_nix "$prog") ;;
                nixos)   version=$(get_version_from_nixos "$prog") ;;
                go)      version=$(get_version_from_go "$prog") ;;
                uv)      version=$(get_version_from_uv "$prog") ;;
                flatpak) version=$(get_version_from_flatpak "$identity") ;;
                appimage) [[ -n "$identity" ]] && version=$(get_version_from_appimage "$identity") ;;
                gh)      [[ -n "$identity" ]] && version=$(get_version_from_github "$identity") ;;
                apt|pacman|apk|dnf) version=$(get_version_from_system "$prog") ;;
            esac
        fi

        # For system package managers: skip child binaries (no version = auxiliary tool)
        # Philosophy: track parent packages only (if you delete parent, children die too)
        case "$source" in
            apt|pacman|apk|dnf)
                [[ -z "$version" ]] && return $SKIPPED
                ;;
        esac

        # Build full source string and add to manifest
        local src_string=$(build_source_string "$source" "$identity" "$version")
        manifest_add "$prog" "$src_string"

        # Display with version
        local color=$(source_color "$(source_display "$source")")
        display_tool_entry "$prog" "$src_string" "${color}+${C_RESET} "
        return $SUCCESS
    }

    # Clean up stale/invalid manifest entries
    _cleanup_manifest() {
        local brew_leaves=""
        command -v brew &>/dev/null && brew_leaves=$(brew leaves 2>/dev/null)

        local pruned=0
        while IFS='=' read -r prog source; do
            [[ -z "$prog" ]] && continue
            local should_prune=false
            local reason=""

            # Check exclusion patterns
            if is_excluded "$prog" "$source"; then
                should_prune=true
                reason="excluded"
            # Check brew deps (not in leaves)
            elif [[ "$source" == "brew" && -n "$brew_leaves" ]]; then
                if ! echo "$brew_leaves" | grep -qxF "$prog"; then
                    should_prune=true
                    reason="brew dep"
                fi
            # Check if managed as symlink in ~/.local/bin (dotfiles, etc.)
            # But exclude sat-managed symlinks (appimages, flatpak wrappers)
            elif [[ -L "$LOCAL_BIN_DIR/$prog" ]]; then
                local target=$(readlink -f "$LOCAL_BIN_DIR/$prog" 2>/dev/null)
                # Skip if symlink points to sat-managed directories
                if [[ ! "$target" =~ ^$SAT_MANAGED_PREFIX ]]; then
                    should_prune=true
                    reason="symlink"
                fi
            fi

            if $should_prune; then
                manifest_remove "$prog"
                printf "  ${C_DIM}- %-20s ($reason)${C_RESET}\n" "$prog"
                ((pruned++))
            fi
        done < "$SAT_MANIFEST"

        return $pruned
    }

    # Scan a directory for binaries from a specific source
    _scan_dir() {
        local src="$1" dir="$2"
        [[ ! -d "$dir" ]] && return
        for bin in "$dir"/*; do
            [[ ! -x "$bin" ]] && continue
            local prog=$(basename "$bin")
            _try_add_tool "$prog" "$src" && ((added++))
        done
    }

    # Detect bin directories using env vars (respect user config)
    local cargo_bin="${CARGO_HOME:-$DEFAULT_CARGO_HOME}/$CARGO_BIN_SUBDIR"

    local npm_bin="${NPM_CONFIG_PREFIX:-$DEFAULT_NPM_PREFIX}/$NPM_BIN_SUBDIR"
    command -v npm &>/dev/null && npm_bin="$(npm config get prefix 2>/dev/null)/$NPM_BIN_SUBDIR"

    local go_bin=""
    command -v go &>/dev/null && go_bin="$(go env GOPATH 2>/dev/null)/$CARGO_BIN_SUBDIR"

    # Clean up manifest before scanning
    _cleanup_manifest
    local pruned=$?

    # Export helpers for scan modules
    export -f is_excluded
    export -f _try_add_tool

    local added=0

    # Scan directory-based sources (explicit mapping)
    _scan_dir "$SRC_CARGO" "$cargo_bin"
    _scan_dir "$SRC_NPM" "$npm_bin"
    _scan_dir "$SRC_UV" "$UV_TOOLS_DIR"
    _scan_dir "$SRC_GO" "$go_bin"

    # Homebrew: query explicit installs only (not deps), get actual binary names
    if command -v brew &>/dev/null; then
        while read -r formula; do
            [[ -z "$formula" ]] && continue
            # Get actual binaries installed by this formula
            while read -r bin; do
                [[ -z "$bin" ]] && continue
                prog=$(basename "$bin")
                _try_add_tool "$prog" "$SRC_BREW" && ((added++))
            done < <(brew list "$formula" 2>/dev/null | grep "$BREW_BIN_GREP_PATTERN")
        done < <(brew leaves 2>/dev/null)
    fi

    # Nix: scan profile bin but exclude nix-* meta-tools
    if [[ -d "$NIX_PROFILE_BIN" ]]; then
        for bin in "$NIX_PROFILE_BIN"/*; do
            [[ ! -x "$bin" ]] && continue
            prog=$(basename "$bin")
            _try_add_tool "$prog" "$SRC_NIX" && ((added++))
        done
    fi

    # System packages: scan explicitly-installed packages per distro
    scan_system_packages

    # Flatpak: scan user-installed apps (not runtimes)
    if command -v flatpak &>/dev/null; then
        while read -r app_id; do
            [[ -z "$app_id" ]] && continue
            # Use last component as prog name (org.gimp.GIMP → gimp)
            prog=$(get_flatpak_shortname "$app_id")
            if _try_add_tool "$prog" "$SRC_FLATPAK:$app_id"; then
                ((added++))
                # Create wrapper for convenient CLI launching
                create_flatpak_wrapper "$prog" "$app_id"
            fi
        done < <(flatpak list --app --columns=application 2>/dev/null)
    fi

    # Helper: Extract GitHub repo from AppImage metadata
    _extract_appimage_repo() {
        local appimage_path="$1"
        # Look for zsync update info embedded in AppImage
        local repo_info=$(strings "$appimage_path" 2>/dev/null | grep -oP "$ZSYNC_PATTERN" | head -1)
        if [[ -n "$repo_info" ]]; then
            # Format: owner|repo-name → owner/repo-name
            echo "$repo_info" | tr "$ZSYNC_SEPARATOR" "$ZSYNC_REPLACEMENT"
            return $SUCCESS
        fi
        return $SKIPPED
    }

    # AppImages: scan sat's appimage directory with repo extraction
    if [[ -d "$APPIMAGE_DIR" ]]; then
        for bin in "$APPIMAGE_DIR"/*; do
            [[ ! -x "$bin" ]] && continue
            prog=$(basename "$bin")

            # Try to extract GitHub repo from AppImage
            local repo=$(_extract_appimage_repo "$bin")
            if [[ -n "$repo" ]]; then
                _try_add_tool "$prog" "$SRC_APPIMAGE:$repo" && ((added++))
            else
                # Fallback: track without repo info
                _try_add_tool "$prog" "$SRC_APPIMAGE" && ((added++))
            fi
        done
    fi

    # Local bin: detect source or mark as unknown with path (skip symlinks)
    if [[ -d "$LOCAL_BIN_DIR" ]]; then
        for bin in "$LOCAL_BIN_DIR"/*; do
            [[ ! -x "$bin" ]] && continue
            [[ -L "$bin" ]] && continue  # Skip symlinks (managed elsewhere)
            prog=$(basename "$bin")
            src=$(detect_source "$prog")
            # If unknown, store the resolved path
            [[ "$src" == "$SRC_UNKNOWN" ]] && src="$SRC_UNKNOWN:$(readlink -f "$bin" 2>/dev/null || echo "$bin")"
            _try_add_tool "$prog" "$src" && ((added++))
        done
    fi

    # Clean up stale flatpak wrappers
    cleanup_flatpak_wrappers

    echo ""
    [[ $pruned -gt 0 ]] && echo "Pruned $pruned entries"
    echo "Added $added packages to manifest"
}
