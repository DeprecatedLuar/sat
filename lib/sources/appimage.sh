#!/usr/bin/env bash
# AppImage - Self-contained source module for portable Linux apps

# Search relies on GitHub search (no separate AppImage registry)
# Use search_github() from github.sh

# Shared GitHub API wrapper (avoid duplication)
_appimage_gh_api() {
    local endpoint="$1"
    if command -v gh &>/dev/null && gh auth status &>/dev/null 2>&1; then
        gh api "$endpoint" 2>/dev/null
    else
        curl -sS "https://api.github.com/$endpoint"
    fi
}

# Install AppImage from GitHub releases
install_appimage() {
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

    # Fetch latest release assets
    [[ -n "$SAT_DEBUG" ]] && echo "[debug]   fetching releases from GitHub API..." >&2
    local release_data=$(_appimage_gh_api "repos/$repo_path/releases/latest")

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

    # Return the installed binary name via stdout (for caller to capture)
    echo "$app_name"
    return 0
}

# Uninstall AppImage
uninstall_appimage() {
    local pkg="$1"
    rm -f "$HOME/.local/bin/$pkg"                      # Remove symlink
    rm -f "$HOME/.local/share/sat/bin/appimages/$pkg"  # Remove AppImage
}

# Update AppImage (re-download latest)
update_appimage() {
    local tool="$1"
    local repo="$2"  # Format: owner/repo from manifest appimage:owner/repo

    # Remove old version
    uninstall_appimage "$tool"

    # Re-download latest
    install_appimage "$repo"
}
