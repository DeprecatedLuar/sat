#!/usr/bin/env bash
# System package scanner - Detects explicitly-installed packages per distro

# ============================================================================
# Configuration Constants
# ============================================================================

# Binary directories to scan
readonly BIN_DIRS=("/usr/bin" "/usr/local/bin" "/bin")
readonly APT_BIN_DIRS=("/usr/bin" "/usr/local/bin" "/bin")
readonly DNF_BIN_DIRS=("/usr/bin" "/usr/local/bin")
readonly APK_BIN_DIRS=("/usr/bin" "/usr/local/bin" "/bin")

# System paths
readonly APK_WORLD_FILE="/etc/apk/world"
readonly NIXOS_SYSTEM_BIN="/run/current-system/sw/bin"

# Package manager names
readonly PKG_MGR_PACMAN="pacman"
readonly PKG_MGR_APT="apt"
readonly PKG_MGR_DNF="dnf"
readonly PKG_MGR_APK="apk"
readonly PKG_MGR_NIXOS="nixos"

# Regex patterns
readonly VERSION_STRIP_PATTERN='-[0-9].*'

# AWK patterns
readonly PACMAN_BIN_PATTERN='/\/usr\/(local\/)?bin\/[^\/]+$/ {print $2}'

# ============================================================================
# Main Functions
# ============================================================================

# Main entry point - routes to distro-specific scanner
scan_system_packages() {
    local mgr="$SAT_PKG_MANAGER"
    [[ -z "$mgr" ]] && return 0

    case "$mgr" in
        "$PKG_MGR_PACMAN") _scan_pacman ;;
        "$PKG_MGR_APT")    _scan_apt ;;
        "$PKG_MGR_DNF")    _scan_dnf ;;
        "$PKG_MGR_APK")    _scan_apk ;;
        "$PKG_MGR_NIXOS")  _scan_nixos ;;
        *)                 return 0 ;;
    esac
}

# Pacman (Arch/CachyOS): scan explicitly-installed packages
_scan_pacman() {
    command -v pacman &>/dev/null || return 0

    # Optimized: query all explicit packages at once, then filter for /usr/bin binaries
    # Much faster than calling pacman -Qo for every file in /usr/bin
    pacman -Qeq 2>/dev/null | xargs pacman -Ql 2>/dev/null | \
        awk "$PACMAN_BIN_PATTERN" | \
        while read -r bin; do
            [[ -x "$bin" ]] || continue
            local prog=$(basename "$bin")
            _try_add_tool "$prog" "$PKG_MGR_PACMAN" && ((added++))
        done
}

# Apt (Debian/Ubuntu): scan manually-installed packages
_scan_apt() {
    local manual_packages=$(apt-mark showmanual 2>/dev/null)
    [[ -z "$manual_packages" ]] && return 0

    for bin_dir in "${APT_BIN_DIRS[@]}"; do
        [[ ! -d "$bin_dir" ]] && continue
        for bin in "$bin_dir"/*; do
            [[ ! -x "$bin" ]] && continue
            local prog=$(basename "$bin")

            # Find which package owns this binary
            local package=$(dpkg -S "$bin" 2>/dev/null | head -1 | cut -d: -f1)
            [[ -z "$package" ]] && continue

            # Only add if package is manually installed
            echo "$manual_packages" | grep -qxF "$package" && \
                _try_add_tool "$prog" "$PKG_MGR_APT" && ((added++))
        done
    done
}

# DNF (Fedora/RHEL): scan user-installed packages
_scan_dnf() {
    # dnf history userinstalled is slow and sometimes unreliable
    # Use a faster approach: check installed packages and filter known system groups
    local user_packages=$(dnf repoquery --userinstalled 2>/dev/null | sed "s/$VERSION_STRIP_PATTERN//")
    [[ -z "$user_packages" ]] && return 0

    for bin_dir in "${DNF_BIN_DIRS[@]}"; do
        [[ ! -d "$bin_dir" ]] && continue
        for bin in "$bin_dir"/*; do
            [[ ! -x "$bin" ]] && continue
            local prog=$(basename "$bin")

            # Find which package owns this binary
            local package=$(rpm -qf "$bin" 2>/dev/null | sed "s/$VERSION_STRIP_PATTERN//")
            [[ -z "$package" ]] && continue

            # Only add if package is user-installed
            echo "$user_packages" | grep -qxF "$package" && \
                _try_add_tool "$prog" "$PKG_MGR_DNF" && ((added++))
        done
    done
}

# APK (Alpine): scan explicitly-installed packages
_scan_apk() {
    # /etc/apk/world contains explicitly-installed packages
    [[ ! -f "$APK_WORLD_FILE" ]] && return 0
    local world_packages=$(cat "$APK_WORLD_FILE" 2>/dev/null)
    [[ -z "$world_packages" ]] && return 0

    for bin_dir in "${APK_BIN_DIRS[@]}"; do
        [[ ! -d "$bin_dir" ]] && continue
        for bin in "$bin_dir"/*; do
            [[ ! -x "$bin" ]] && continue
            local prog=$(basename "$bin")

            # Find which package owns this binary
            local package=$(apk info --who-owns "$bin" 2>/dev/null | awk '{print $NF}' | sed 's/ is owned by //')
            [[ -z "$package" ]] && continue

            # Strip version from package name (package-1.2.3 → package)
            package=$(echo "$package" | sed "s/$VERSION_STRIP_PATTERN//")

            # Only add if package is in world file
            echo "$world_packages" | grep -qxF "$package" && \
                _try_add_tool "$prog" "$PKG_MGR_APK" && ((added++))
        done
    done
}

# NixOS: scan system packages from configuration
_scan_nixos() {
    command -v nixos-option &>/dev/null || return 0

    local user_packages=$(nixos-option environment.systemPackages 2>/dev/null | \
        grep -oP '<derivation \K[^>]+' | \
        awk -F'-[0-9]' '{print $1}' | \
        sort -u)

    if [[ -n "$user_packages" && -d "$NIXOS_SYSTEM_BIN" ]]; then
        for bin in "$NIXOS_SYSTEM_BIN"/*; do
            [[ ! -x "$bin" ]] && continue
            local prog=$(basename "$bin")
            # Only add if binary name matches a user-installed package
            echo "$user_packages" | grep -qxF "$prog" && \
                _try_add_tool "$prog" "$PKG_MGR_NIXOS" && ((added++))
        done
    fi
}
