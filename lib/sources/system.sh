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

# Uninstall system package
# Delegates to cached package manager
uninstall_system() {
    local pkg="$1"
    local mgr="$SAT_PKG_MANAGER"
    [[ -z "$mgr" ]] && return 1
    pkg_remove "$pkg" "$mgr"
}
