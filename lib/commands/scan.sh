#!/usr/bin/env bash
# scan.sh - Scan ecosystem directories and add found packages to manifest

sat_scan() {
    echo "Scanning ecosystems..."

    # Helper functions (defined first to avoid bash parser issues)
    is_excluded() {
        local prog="$1" src="$2"
        # Global exclusions
        case "$prog" in
            .*|_*|*-config|*-settings) return 0 ;;
        esac
        # Per-source exclusions
        case "$src" in
            cargo) [[ "$prog" == cargo-* || "$prog" == clippy-driver || "$prog" == rust[!u]* || "$prog" == rls ]] && return 0 ;;
            nix)   [[ "$prog" == nix || "$prog" == nix-* ]] && return 0 ;;
            nixos)
                # NixOS/Nix system tools
                case "$prog" in
                    nix|nix-*|nixos-*) return 0 ;;
                esac
                # Desktop environment infrastructure (services/daemons, not apps)
                case "$prog" in
                    # XFCE services
                    xfce4-panel|xfce4-session|xfce4-power-manager|xfce4-settings|xfdesktop|xfwm4|xfce4-screensaver) return 0 ;;
                    # GNOME services
                    gnome-keyring|gnome-settings-daemon|gnome-session) return 0 ;;
                    # KDE/Plasma services
                    plasma-*|kwin*) return 0 ;;
                esac
                # X11/system utilities
                case "$prog" in
                    xinit|xinput|xlsclients|xprop|xrandr|xrdb|xset|xsetroot|xterm) return 0 ;;
                esac
                ;;
            *)
                # Git infrastructure
                [[ "$prog" == git-* || "$prog" == scalar || "$prog" == trash-* ]] && return 0
                # Language servers (infrastructure)
                case "$prog" in
                    gopls|rust-analyzer|typescript-language-server|pyright) return 0 ;;
                esac
                # sat internal dependencies
                [[ "$prog" == huber ]] && return 0
                ;;
        esac
        return 1
    }

    # Try to add tool to manifest (returns 0 on success, 1 if skipped)
    _try_add_tool() {
        local prog="$1" src="$2"
        is_excluded "$prog" "$src" && return 1
        [[ -n "$(_sat_manifest_get "$prog")" ]] && return 1
        _shell_manifest_has "$prog" && return 1

        _sat_manifest_add "$prog" "$src"
        local display=$(source_display "$src")
        local color=$(source_color "$display")
        printf "  ${color}+${C_RESET} %-20s [${color}%s${C_RESET}]\n" "$prog" "$display"
        return 0
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
            elif [[ -L "$HOME/.local/bin/$prog" ]]; then
                should_prune=true
                reason="symlink"
            fi

            if $should_prune; then
                _sat_manifest_remove "$prog"
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
    local cargo_bin="${CARGO_HOME:-$HOME/.cargo}/bin"

    local npm_bin="${NPM_CONFIG_PREFIX:-$HOME/.npm-global}/bin"
    command -v npm &>/dev/null && npm_bin="$(npm config get prefix 2>/dev/null)/bin"

    local go_bin=""
    command -v go &>/dev/null && go_bin="$(go env GOPATH 2>/dev/null)/bin"

    # Clean up manifest before scanning
    _cleanup_manifest
    local pruned=$?

    local added=0

    # Scan directory-based sources (explicit mapping)
    _scan_dir "cargo" "$cargo_bin"
    _scan_dir "npm" "$npm_bin"
    _scan_dir "uv" "$HOME/.local/share/uv/tools"
    _scan_dir "go" "$go_bin"

    # Homebrew: query explicit installs only (not deps), get actual binary names
    if command -v brew &>/dev/null; then
        while read -r formula; do
            [[ -z "$formula" ]] && continue
            # Get actual binaries installed by this formula
            while read -r bin; do
                [[ -z "$bin" ]] && continue
                prog=$(basename "$bin")
                _try_add_tool "$prog" "brew" && ((added++))
            done < <(brew list "$formula" 2>/dev/null | grep '/bin/')
        done < <(brew leaves 2>/dev/null)
    fi

    # Nix: scan profile bin but exclude nix-* meta-tools
    if [[ -d "$HOME/.nix-profile/bin" ]]; then
        for bin in "$HOME/.nix-profile/bin"/*; do
            [[ ! -x "$bin" ]] && continue
            prog=$(basename "$bin")
            _try_add_tool "$prog" "nix" && ((added++))
        done
    fi

    # NixOS: scan system packages from configuration (only user-installed, not deps)
    if command -v nixos-option &>/dev/null; then
        local user_packages=$(nixos-option environment.systemPackages 2>/dev/null | \
            grep -oP '<derivation \K[^>]+' | \
            awk -F'-[0-9]' '{print $1}' | \
            sort -u)

        if [[ -n "$user_packages" && -d "/run/current-system/sw/bin" ]]; then
            for bin in /run/current-system/sw/bin/*; do
                [[ ! -x "$bin" ]] && continue
                prog=$(basename "$bin")
                # Only add if binary name matches a user-installed package
                echo "$user_packages" | grep -qxF "$prog" && _try_add_tool "$prog" "nixos" && ((added++))
            done
        fi
    fi

    # Flatpak: scan user-installed apps (not runtimes)
    if command -v flatpak &>/dev/null; then
        while read -r app_id; do
            [[ -z "$app_id" ]] && continue
            # Use last component as prog name (org.gimp.GIMP → gimp)
            prog=$(echo "$app_id" | awk -F. '{print tolower($NF)}')
            _try_add_tool "$prog" "flatpak:$app_id" && ((added++))
        done < <(flatpak list --app --columns=application 2>/dev/null)
    fi

    # AppImages: scan sat's appimage directory
    local appimage_dir="$HOME/.local/share/sat/bin/appimages"
    if [[ -d "$appimage_dir" ]]; then
        for bin in "$appimage_dir"/*; do
            [[ ! -x "$bin" ]] && continue
            prog=$(basename "$bin")
            _try_add_tool "$prog" "appimage" && ((added++))
        done
    fi

    # Local bin: detect source or mark as unknown with path (skip symlinks)
    if [[ -d "$HOME/.local/bin" ]]; then
        for bin in "$HOME/.local/bin"/*; do
            [[ ! -x "$bin" ]] && continue
            [[ -L "$bin" ]] && continue  # Skip symlinks (managed elsewhere)
            prog=$(basename "$bin")
            src=$(detect_source "$prog")
            # If unknown, store the resolved path
            [[ "$src" == "unknown" ]] && src="unknown:$(readlink -f "$bin" 2>/dev/null || echo "$bin")"
            _try_add_tool "$prog" "$src" && ((added++))
        done
    fi

    echo ""
    [[ $pruned -gt 0 ]] && echo "Pruned $pruned entries"
    echo "Added $added packages to manifest"
}
