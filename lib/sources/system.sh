#!/usr/bin/env bash
# System package manager - Self-contained source module

# =============================================================================
# SEARCH FUNCTIONS
# =============================================================================

# Search apt (Debian/Ubuntu)
search_system_apt() {
    local query="$1"
    apt search "$query" 2>/dev/null | grep -v "^Sorting\|^Full Text" | \
        awk '/^[^ ]/ {
            slash_pos = index($0, "/")
            name = substr($0, 1, slash_pos-1)
            rest = substr($0, slash_pos+1)
            split(rest, parts, " ")
            version = parts[2]

            # Strip Debian metadata (epoch, +dfsg, -revision)
            sub(/^[0-9]+:/, "", version)     # Remove epoch (1:)
            sub(/\+dfsg[0-9]*/, "", version) # Remove +dfsg2
            sub(/-[0-9]+.*$/, "", version)   # Remove -8build1

            getline desc
            gsub(/^[[:space:]]+/, "", desc)
            if (name && version && desc)
                print name, version, "-", desc
        }' | head -30
}

# Search pacman (Arch)
search_system_pacman() {
    local query="$1"
    pacman -Ss "$query" 2>/dev/null | awk '
        /^[^ ]/ {
            split($0, parts, "/")
            split(parts[2], pkg, " ")
            name = pkg[1]
            version = pkg[2]
            getline desc
            gsub(/^[[:space:]]+/, "", desc)
            if (desc) print name, version, "-", desc
        }' | head -30
}

# Search apk (Alpine)
search_system_apk() {
    local query="$1"
    apk search -v "$query" 2>/dev/null | head -30 | \
        awk '{
            match($0, /-[0-9]/)
            if (RSTART > 0) {
                name = substr($0, 1, RSTART-1)
                version = substr($0, RSTART+1)
                print name, version, "- (no description)"
            }
        }'
}

# Search dnf (Fedora/RHEL)
search_system_dnf() {
    local query="$1"
    dnf search "$query" 2>/dev/null | grep -v "^=" | grep -v "^Last metadata" | \
        awk -F' : ' '
            /\./ && NF==2 {
                split($1, parts, ".")
                name = parts[1]
                desc = $2
                print name, "(version varies)", "-", desc
            }
        ' | head -30
}

# Search system package manager (router)
search_system() {
    local query="$1"
    local mgr="$SAT_PKG_MANAGER"
    [[ -z "$mgr" ]] && return 1

    case "$mgr" in
        apt)    search_system_apt "$query" ;;
        pacman) search_system_pacman "$query" ;;
        apk)    search_system_apk "$query" ;;
        dnf)    search_system_dnf "$query" ;;
    esac
}

# =============================================================================
# INSTALL/UNINSTALL FUNCTIONS
# =============================================================================

# Install via system package manager
# Returns 0 on success, 1 on failure
install_system() {
    local tool="$1"
    local mgr="$SAT_PKG_MANAGER"

    [[ -z "$mgr" ]] && return 1
    pkg_exists "$tool" "$mgr" || return 1
    pkg_install "$tool" "$mgr"
}

# Get installed version of system package
get_version_from_system() {
    local tool="$1"
    local mgr="$SAT_PKG_MANAGER"
    [[ -z "$mgr" ]] && return 1

    case "$mgr" in
        apt)
            dpkg -l "$tool" 2>/dev/null | awk '/^ii/ {print $3}'
            ;;
        pacman)
            pacman -Q "$tool" 2>/dev/null | awk '{print $2}'
            ;;
        apk)
            apk info -e "$tool" 2>/dev/null | grep -oP "${tool}-\K[0-9][\S]+"
            ;;
        dnf)
            dnf list installed "$tool" 2>/dev/null | awk '/^'"$tool"'/ {print $2}'
            ;;
        *)
            return 1
            ;;
    esac
}

# Uninstall system package
# Delegates to cached package manager
uninstall_system() {
    local pkg="$1"
    local mgr="$SAT_PKG_MANAGER"
    [[ -z "$mgr" ]] && return 1
    pkg_remove "$pkg" "$mgr"
}

# Update system package
update_system() {
    local tool="$1"
    local mgr="$SAT_PKG_MANAGER"
    [[ -z "$mgr" ]] && return 1

    case "$mgr" in
        apt)    _run_quiet sudo apt-get install --only-upgrade -y "$tool" ;;
        pacman) _run_quiet sudo pacman -S --noconfirm "$tool" ;;
        apk)    _run_quiet sudo apk upgrade "$tool" ;;
        dnf)    _run_quiet sudo dnf upgrade -y "$tool" ;;
        *)      return 1 ;;
    esac
}

# Check if system package is outdated
check_outdated_system() {
    local tool="$1"
    local mgr="$SAT_PKG_MANAGER"
    [[ -z "$mgr" ]] && return 1

    # Try manifest first
    local current=$(get_source_version "$(manifest_get "$tool")")

    case "$mgr" in
        apt)
            # Fall back to apt if no manifest version
            if [[ -z "$current" ]]; then
                local line=$(apt list --upgradable 2>/dev/null | grep "^$tool/")
                [[ -z "$line" ]] && return 1
                current=$(echo "$line" | grep -oP 'upgradable from: \K[^]]+' | tr -d ']')
            fi
            [[ -z "$current" ]] && return 1

            # Get latest from apt
            local latest=$(apt list --upgradable 2>/dev/null | grep "^$tool/" | awk '{print $2}')
            [[ -n "$latest" ]] || return 1
            [[ "$current" == "$latest" ]] && return 1
            echo "$current $latest"
            ;;
        *)
            # pacman/apk/dnf don't have reliable per-tool outdated checks without sudo
            return 1
            ;;
    esac
}
