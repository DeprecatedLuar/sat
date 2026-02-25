#!/usr/bin/env bash
# scan.sh - Scan ecosystem directories and add found packages to manifest

sat_scan() {
    echo "Scanning ecosystems..."

    # Helper functions (defined first to avoid bash parser issues)
    is_from_repo() {
        local bin="$1"
        local real=$(readlink -f "$bin" 2>/dev/null || echo "$bin")
        local dir=$(dirname "$real")
        while [[ "$dir" != "/" && "$dir" != "$HOME" ]]; do
            [[ -d "$dir/.git" ]] && return 0
            dir=$(dirname "$dir")
        done
        return 1
    }

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
            *)     [[ "$prog" == git-* || "$prog" == scalar || "$prog" == trash-* ]] && return 0 ;;
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

    # Detect bin directories dynamically
    local cargo_bin="$HOME/.cargo/bin"
    [[ ! -d "$cargo_bin" ]] && command -v cargo &>/dev/null && cargo_bin="$(dirname "$(command -v cargo)")"

    local npm_bin="$HOME/.npm-global/bin"
    [[ ! -d "$npm_bin" ]] && command -v npm &>/dev/null && npm_bin="$(npm root -g 2>/dev/null)/../bin"

    local go_bin="$HOME/go/bin"
    [[ ! -d "$go_bin" ]] && command -v go &>/dev/null && go_bin="$(go env GOPATH 2>/dev/null)/bin"

    # Get brew leaves for validation
    local brew_leaves=""
    command -v brew &>/dev/null && brew_leaves=$(brew leaves 2>/dev/null)

    # Prune excluded entries and brew deps from manifest
    local pruned=0
    while IFS='=' read -r prog source; do
        [[ -z "$prog" ]] && continue
        local should_prune=false
        if is_excluded "$prog" "$source"; then
            should_prune=true
        elif [[ "$source" == "brew" && -n "$brew_leaves" ]]; then
            # Prune brew entries not in leaves (deps)
            echo "$brew_leaves" | grep -qxF "$prog" || should_prune=true
        fi
        if $should_prune; then
            _sat_manifest_remove "$prog"
            printf "  ${C_DIM}- %-20s (excluded)${C_RESET}\n" "$prog"
            ((pruned++))
        fi
    done < "$SAT_MANIFEST"

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

    # Local bin: check for repo-sourced tools
    if [[ -d "$HOME/.local/bin" ]]; then
        for bin in "$HOME/.local/bin"/*; do
            [[ ! -x "$bin" ]] && continue
            prog=$(basename "$bin")
            is_from_repo "$bin" && _try_add_tool "$prog" "repo" && ((added++))
        done
    fi

    echo ""
    [[ $pruned -gt 0 ]] && echo "Pruned $pruned excluded entries"
    echo "Added $added packages to manifest"
}
