#!/usr/bin/env bash
# System package scanner - Detects explicitly-installed packages per distro

# Main entry point - routes to distro-specific scanner
scan_system_packages() {
    local mgr="$SAT_PKG_MANAGER"
    [[ -z "$mgr" ]] && return 0

    case "$mgr" in
        pacman) _scan_pacman ;;
        apt)    _scan_apt ;;
        dnf)    _scan_dnf ;;
        apk)    _scan_apk ;;
        nixos)  _scan_nixos ;;
        *)      return 0 ;;
    esac
}

# Pacman (Arch/CachyOS): scan explicitly-installed packages
_scan_pacman() {
    command -v pacman &>/dev/null || return 0

    # Optimized: query all explicit packages at once, then filter for /usr/bin binaries
    # Much faster than calling pacman -Qo for every file in /usr/bin
    pacman -Qeq 2>/dev/null | xargs pacman -Ql 2>/dev/null | \
        awk '/\/usr\/(local\/)?bin\/[^\/]+$/ {print $2}' | \
        while read -r bin; do
            [[ -x "$bin" ]] || continue
            local prog=$(basename "$bin")
            _try_add_tool "$prog" "pacman" && ((added++))
        done
}

# Apt (Debian/Ubuntu): scan manually-installed packages
_scan_apt() {
    local manual_packages=$(apt-mark showmanual 2>/dev/null)
    [[ -z "$manual_packages" ]] && return 0

    for bin_dir in /usr/bin /usr/local/bin /bin; do
        [[ ! -d "$bin_dir" ]] && continue
        for bin in "$bin_dir"/*; do
            [[ ! -x "$bin" ]] && continue
            local prog=$(basename "$bin")

            # Find which package owns this binary
            local package=$(dpkg -S "$bin" 2>/dev/null | head -1 | cut -d: -f1)
            [[ -z "$package" ]] && continue

            # Only add if package is manually installed
            echo "$manual_packages" | grep -qxF "$package" && \
                _try_add_tool "$prog" "apt" && ((added++))
        done
    done
}

# DNF (Fedora/RHEL): scan user-installed packages
_scan_dnf() {
    # dnf history userinstalled is slow and sometimes unreliable
    # Use a faster approach: check installed packages and filter known system groups
    local user_packages=$(dnf repoquery --userinstalled 2>/dev/null | sed 's/-[0-9].*//')
    [[ -z "$user_packages" ]] && return 0

    for bin_dir in /usr/bin /usr/local/bin; do
        [[ ! -d "$bin_dir" ]] && continue
        for bin in "$bin_dir"/*; do
            [[ ! -x "$bin" ]] && continue
            local prog=$(basename "$bin")

            # Find which package owns this binary
            local package=$(rpm -qf "$bin" 2>/dev/null | sed 's/-[0-9].*//')
            [[ -z "$package" ]] && continue

            # Only add if package is user-installed
            echo "$user_packages" | grep -qxF "$package" && \
                _try_add_tool "$prog" "dnf" && ((added++))
        done
    done
}

# APK (Alpine): scan explicitly-installed packages
_scan_apk() {
    # /etc/apk/world contains explicitly-installed packages
    [[ ! -f /etc/apk/world ]] && return 0
    local world_packages=$(cat /etc/apk/world 2>/dev/null)
    [[ -z "$world_packages" ]] && return 0

    for bin_dir in /usr/bin /usr/local/bin /bin; do
        [[ ! -d "$bin_dir" ]] && continue
        for bin in "$bin_dir"/*; do
            [[ ! -x "$bin" ]] && continue
            local prog=$(basename "$bin")

            # Find which package owns this binary
            local package=$(apk info --who-owns "$bin" 2>/dev/null | awk '{print $NF}' | sed 's/ is owned by //')
            [[ -z "$package" ]] && continue

            # Strip version from package name (package-1.2.3 → package)
            package=$(echo "$package" | sed 's/-[0-9].*//')

            # Only add if package is in world file
            echo "$world_packages" | grep -qxF "$package" && \
                _try_add_tool "$prog" "apk" && ((added++))
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

    if [[ -n "$user_packages" && -d "/run/current-system/sw/bin" ]]; then
        for bin in /run/current-system/sw/bin/*; do
            [[ ! -x "$bin" ]] && continue
            local prog=$(basename "$bin")
            # Only add if binary name matches a user-installed package
            echo "$user_packages" | grep -qxF "$prog" && \
                _try_add_tool "$prog" "nixos" && ((added++))
        done
    fi
}
