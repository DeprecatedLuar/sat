#!/usr/bin/env bash
# sat common - shared functions

# Auto-detect library directory (set by binary or inferred from script location)
SAT_LIB="${SAT_LIB:-$(dirname "${BASH_SOURCE[0]}")}"
export SAT_LIB

# Colors by source (headers)
C_RESET=$'\033[0m'
C_DIM=$'\033[2m'
C_RUST=$'\033[0;91m'      # Bright red - Rust/cargo
C_NODE=$'\033[0;92m'      # Bright green - Node/npm
C_PYTHON=$'\033[0;94m'    # Blue - Python/uv
C_SYSTEM=$'\033[0;97m'    # Bright white - System packages
C_FLATPAK=$'\033[0;95m'   # Bright magenta - Flatpak
C_REPO=$'\033[38;2;140;140;140m'  # Medium gray - GitHub repos
C_SAT=$'\033[0;36m'       # Cyan - Sat scripts
C_GO=$'\033[0;96m'        # Bright Cyan - Go
C_BREW=$'\033[0;93m'      # Bright yellow - Homebrew
C_NIX=$'\033[38;2;82;119;195m'    # Dark blue #5277C3 - Nix
C_MANUAL=$'\033[38;2;180;140;100m'  # Warm brown - Manual installs

# Desaturated colors (for item names - pastel tints, closer to white)
C_RUST_L=$'\033[38;2;220;160;160m'    # Soft pink-red
C_NODE_L=$'\033[38;2;160;210;160m'    # Soft mint
C_PYTHON_L=$'\033[38;2;220;210;160m'  # Soft cream
C_SYSTEM_L=$'\033[38;2;160;180;220m'  # Soft sky blue
C_FLATPAK_L=$'\033[38;2;220;160;220m' # Soft magenta
C_REPO_L=$'\033[38;2;180;180;180m'    # Soft gray
C_GO_L=$'\033[38;2;160;210;210m'      # Soft teal
C_BREW_L=$'\033[38;2;230;175;130m'    # Soft amber
C_NIX_L=$'\033[38;2;126;186;228m'     # Light blue #7EBAE4
C_MANUAL_L=$'\033[38;2;210;180;150m'  # Soft tan

# Map source to color
source_color() {
    case "$1" in
        cargo|rust)                  printf '%s' "$C_RUST" ;;
        npm|node)                    printf '%s' "$C_NODE" ;;
        uv|pip|python)               printf '%s' "$C_PYTHON" ;;
        apt|apk|pacman|dnf|pkg|system) printf '%s' "$C_SYSTEM" ;;
        flatpak|flathub)             printf '%s' "$C_FLATPAK" ;;
        repo|repo:*|gh|gh:*|github)  printf '%s' "$C_REPO" ;;
        sat)                         printf '%s' "$C_SAT" ;;
        go|go:*)                     printf '%s' "$C_GO" ;;
        brew)                        printf '%s' "$C_BREW" ;;
        nix)                         printf '%s' "$C_NIX" ;;
        manual)                      printf '%s' "$C_MANUAL" ;;
        unknown)                     printf '%s' "$C_DIM" ;;
        *)                           printf '%s' "$C_RESET" ;;
    esac
}

# Install fallback order for permanent installs (user-space first: brew/nix before system)
INSTALL_ORDER=(brew nix system cargo uv npm sat gh)

# Install order for sat shell (isolated/user-space first, system before npm)
SHELL_INSTALL_ORDER=(brew nix cargo uv system npm sat gh)

LUAR="DeprecatedLuar"
SAT_REPO="the-satellite/main"
SAT_BASE="https://raw.githubusercontent.com/$LUAR/$SAT_REPO"
SAT_DATA="${SAT_DATA:-${XDG_DATA_HOME:-$HOME/.local/share}/sat}"
SAT_MANIFEST="${SAT_MANIFEST:-$SAT_DATA/manifest}"
SAT_SHELL_DIR="$SAT_DATA/shell"
SAT_SHELL_MASTER="$SAT_SHELL_DIR/manifest"
SAT_INSTALL_DIR="${SAT_LIB:-$(dirname "${BASH_SOURCE[0]}")}/installation"

# Ensure OS info cache exists
_ensure_os_info() {
    local os_info="$SAT_DATA/os-info"

    # Return if cache exists
    [[ -f "$os_info" ]] && return 0

    # Ensure directory exists
    mkdir -p "$SAT_DATA"

    # Fetch and evaluate os_detection.sh to get functions
    local os_script
    os_script=$(curl -sSL "$SAT_BASE/internal/os_detection.sh" 2>/dev/null) || {
        echo "Error: Failed to fetch OS detection script" >&2
        return 1
    }

    # Source the script to load detection functions
    eval "$os_script"

    # Call functions and write results to cache
    {
        echo "SAT_OS=$(detect_os)"
        echo "SAT_DISTRO=$(detect_distro "$(detect_os)")"
        echo "SAT_DISTRO_FAMILY=$(detect_distro_family "$(detect_distro "$(detect_os)")")"

        # Calculate package manager based on distro
        local distro family
        distro=$(detect_distro "$(detect_os)")
        family=$(detect_distro_family "$distro")

        local pkg_mgr=""
        case "$distro" in
            termux) pkg_mgr="pkg" ;;
            *)
                case "$family" in
                    debian) pkg_mgr="apt" ;;
                    alpine) pkg_mgr="apk" ;;
                    arch)   pkg_mgr="pacman" ;;
                    rhel)   pkg_mgr="dnf" ;;
                esac
                ;;
        esac
        echo "SAT_PKG_MANAGER=$pkg_mgr"
    } > "$os_info"
}

# Ensure data dirs exist
mkdir -p "$SAT_DATA" "$SAT_SHELL_DIR"
touch "$SAT_MANIFEST" "$SAT_SHELL_MASTER"

# Ensure OS detection cache exists and source it
_ensure_os_info
source "$SAT_DATA/os-info"

# =============================================================================
# MANIFEST API WRAPPERS
# =============================================================================
# When running from binary: use internal _ functions (fast, no subprocess)
# When running from lib files: call sat internal API (subprocess)

# sat-manifest (system manifest)
manifest_add()    { declare -F _sat_manifest_add    &>/dev/null && _sat_manifest_add "$@"    || sat internal sat-manifest add "$1" "$2"; }
manifest_get()    { declare -F _sat_manifest_get    &>/dev/null && _sat_manifest_get "$@"    || sat internal sat-manifest get "$1"; }
manifest_remove() { declare -F _sat_manifest_remove &>/dev/null && _sat_manifest_remove "$@" || sat internal sat-manifest remove "$1"; }
manifest_has()    { declare -F _sat_manifest_has    &>/dev/null && _sat_manifest_has "$@"    || sat internal sat-manifest has "$1"; }

# shell-manifest (master manifest)
master_add()         { declare -F _shell_manifest_add        &>/dev/null && _shell_manifest_add "$@"        || sat internal shell-manifest add "$1" "$2" "$3"; }
master_get_pids()    { declare -F _shell_manifest_pids       &>/dev/null && _shell_manifest_pids "$@"       || sat internal shell-manifest pids "$1" "$2"; }
master_has_tool()    { declare -F _shell_manifest_has        &>/dev/null && _shell_manifest_has "$@"        || sat internal shell-manifest has "$1"; }
master_remove()      { declare -F _shell_manifest_remove     &>/dev/null && _shell_manifest_remove "$@"     || sat internal shell-manifest remove "$1" "$2" "$3"; }
master_remove_tool() { declare -F _shell_manifest_remove_all &>/dev/null && _shell_manifest_remove_all "$@" || sat internal shell-manifest remove-all "$1"; }
master_promote()     { declare -F _shell_manifest_promote    &>/dev/null && _shell_manifest_promote "$@"    || sat internal shell-manifest promote "$1" "$2"; }

# pid-manifest (session manifest)
pid_manifest_add()    { declare -F _pid_manifest_add    &>/dev/null && _pid_manifest_add "$@"    || sat internal pid-manifest add "$1" "$2" "$3"; }
pid_manifest_tools()  { declare -F _pid_manifest_tools  &>/dev/null && _pid_manifest_tools "$@"  || sat internal pid-manifest tools "$1"; }
pid_manifest_source() { declare -F _pid_manifest_source &>/dev/null && _pid_manifest_source "$@" || sat internal pid-manifest source "$1" "$2"; }
pid_manifest_remove() { declare -F _pid_manifest_remove &>/dev/null && _pid_manifest_remove "$@" || sat internal pid-manifest remove "$1"; }

# Animated status output (legacy)
spin() {
    local msg="$1" pid="$2"
    local dots=""
    while kill -0 "$pid" 2>/dev/null; do
        printf "\r%s%-3s" "$msg" "$dots"
        dots="${dots}."
        [[ ${#dots} -gt 3 ]] && dots=""
        sleep 0.2
    done
}

# Styled spinner: [/] package [source] with colored frames
spin_with_style() {
    local program="$1" pid="$2" source="$3"
    local frames=('|' '/' '-' $'\\')
    local frame_colors=("$C_RUST" "$C_NODE" "$C_PYTHON" "$C_BREW")
    local i=0
    local pkg_color=$(source_light "$source")
    local src_display=$(source_display "$source")
    local src_color=$(source_color "$src_display")

    while kill -0 "$pid" 2>/dev/null; do
        printf "\r[${frame_colors[i]}%s${C_RESET}] ${pkg_color}%s${C_RESET} [${src_color}%s${C_RESET}]" \
            "${frames[i]}" "$program" "$src_display"
        i=$(( (i + 1) % 4 ))
        sleep 0.15
    done
    printf "\r%-50s\r" ""
}

# Spinner without source tag (for searching/probing)
spin_probe() {
    local program="$1" pid="$2"
    local frames=('|' '/' '-' $'\\')
    local frame_colors=("$C_RUST" "$C_NODE" "$C_PYTHON" "$C_BREW")
    local i=0

    while kill -0 "$pid" 2>/dev/null; do
        printf "\r[${frame_colors[i]}%s${C_RESET}] ${C_DIM}%s${C_RESET}" \
            "${frames[i]}" "$program"
        i=$(( (i + 1) % 4 ))
        sleep 0.15
    done
    printf "\r%-50s\r" ""
}

# Checkmark and X for completion status
C_CHECK=$'\033[0;92m✓\033[0m'  # Green checkmark
C_CROSS=$'\033[0;91m✗\033[0m'  # Red X

status() { printf "\r%-40s\n" "$1"; }

# Status with checkmark/cross and source tag
status_ok() {
    local msg="$1" src="$2"
    local display=$(source_display "$src")
    local color=$(source_color "$display")
    printf "\r[${C_CHECK}] %-25s [${color}%s${C_RESET}]\n" "$msg" "$display"
}

status_fail() {
    local msg="$1"
    printf "\r[${C_CROSS}] %s\n" "$msg"
}

# Map internal source to display name
source_display() {
    case "$1" in
        npm)          echo "node" ;;
        uv)           echo "python" ;;
        cargo)        echo "rust" ;;
        repo|repo:*)  echo "github" ;;
        gh|gh:*)      echo "github" ;;
        *)            echo "$1" ;;
    esac
}

# Get pastel color for source (item names)
source_light() {
    case "$1" in
        npm|node)                      printf '%s' "$C_NODE_L" ;;
        uv|pip|python)                 printf '%s' "$C_PYTHON_L" ;;
        cargo|rust)                    printf '%s' "$C_RUST_L" ;;
        apt|apk|pacman|dnf|pkg|system) printf '%s' "$C_SYSTEM_L" ;;
        sat|repo|repo:*|gh|gh:*|github) printf '%s' "$C_REPO_L" ;;
        go|go:*)                       printf '%s' "$C_GO_L" ;;
        brew)                          printf '%s' "$C_BREW_L" ;;
        nix)                           printf '%s' "$C_NIX_L" ;;
        manual)                        printf '%s' "$C_MANUAL_L" ;;
        unknown)                       printf '%s' "$C_DIM" ;;
        *)                             printf '%s' "$C_RESET" ;;
    esac
}

# Colored status with source tag
status_src() {
    local msg="$1" src="$2"
    local display=$(source_display "$src")
    local color=$(source_color "$display")
    printf "\r%-30s [${color}%s${C_RESET}]\n" "$msg" "$display"
}

# Parse tool:source syntax (e.g., "ranger:py" -> tool=ranger, source=uv)
# Sets globals: _TOOL_NAME, _TOOL_SOURCE (empty if no source specified)
parse_tool_spec() {
    local spec="$1"
    if [[ "$spec" == *:* ]]; then
        _TOOL_NAME="${spec%%:*}"
        local src="${spec##*:}"
        case "$src" in
            py|python)       _TOOL_SOURCE="uv" ;;
            rs|rust)         _TOOL_SOURCE="cargo" ;;
            js|node)         _TOOL_SOURCE="npm" ;;
            sys|system)      _TOOL_SOURCE="system" ;;
            go)              _TOOL_SOURCE="go" ;;
            brew)            _TOOL_SOURCE="brew" ;;
            nix)             _TOOL_SOURCE="nix" ;;
            gh|github)       _TOOL_SOURCE="gh" ;;
            release|rel)     _TOOL_SOURCE="gh-release" ;;
            script|sh)       _TOOL_SOURCE="gh-script" ;;
            *)               _TOOL_SOURCE="$src" ;;
        esac
    else
        _TOOL_NAME="$spec"
        _TOOL_SOURCE=""
    fi
}

# Detect source from binary location (fallback when not in manifests)
detect_source() {
    local tool="$1"
    local bin=$(command -v "$tool" 2>/dev/null)
    [[ -z "$bin" ]] && return

    # Resolve symlinks to find actual location
    bin=$(readlink -f "$bin" 2>/dev/null || echo "$bin")

    case "$bin" in
        */.cargo/bin/*|*/dev-tools/cargo/bin/*)  echo "cargo" ;;
        */dev-tools/npm/*|*/.npm-global/*|/usr/lib/node_modules/*) echo "npm" ;;
        */dev-tools/go/bin/*|*/go/bin/*)         echo "go" ;;
        */.local/share/uv/tools/*) echo "uv" ;;
        */linuxbrew/*|*/homebrew/*)              echo "brew" ;;
        /nix/store/*|*/.nix-profile/*)           echo "nix" ;;
        /usr/bin/*|/bin/*|/usr/local/bin/*|/usr/games/*|/sbin/*|/usr/sbin/*) echo "system" ;;
        */.local/opt/*)                          echo "manual" ;;
        *)                                       echo "unknown" ;;
    esac
}

# Layered source lookup: session manifest -> system manifest -> binary detection
resolve_source() {
    local tool="$1" session_manifest="$2"
    local src=""

    # Layer 1: Session manifest (tools installed this session)
    [[ -n "$session_manifest" && -f "$session_manifest" ]] && \
        src=$(grep "^SOURCE_$tool=" "$session_manifest" 2>/dev/null | cut -d= -f2)

    # Layer 2: System manifest (permanently installed via sat)
    [[ -z "$src" ]] && src=$(manifest_get "$tool")

    # Layer 3: Detect from binary location
    [[ -z "$src" ]] && src=$(detect_source "$tool")

    # Layer 4: Unknown (not "system" - that would be misleading)
    [[ -z "$src" ]] && src="unknown"

    echo "$src"
}

# Find ALL installations of a tool across ecosystems
# Returns: source:path pairs, one per line (active source first)
resolve_all_sources() {
    local tool="$1"
    local results=()
    local seen_reals=()
    local active_bin=$(command -v "$tool" 2>/dev/null)
    local active_real=""
    [[ -n "$active_bin" ]] && active_real=$(readlink -f "$active_bin" 2>/dev/null || echo "$active_bin")

    # Define search paths for each ecosystem
    declare -A eco_paths=(
        [cargo]="$HOME/.cargo/bin"
        [npm]="$HOME/.npm-global/bin"
        [uv]="$HOME/.local/share/uv/tools/*/bin"
        [go]="$HOME/go/bin"
        [brew]="/home/linuxbrew/.linuxbrew/bin"
        [nix]="$HOME/.nix-profile/bin"
        [system]="/usr/bin /usr/local/bin /usr/games"
    )

    # Check each ecosystem
    for src in "${!eco_paths[@]}"; do
        for dir in ${eco_paths[$src]}; do
            # Handle glob patterns (for uv)
            for bin in $dir/$tool; do
                if [[ -x "$bin" ]]; then
                    local real=$(readlink -f "$bin" 2>/dev/null || echo "$bin")
                    # Skip if we've already seen this real path
                    [[ " ${seen_reals[*]} " =~ " $real " ]] && continue
                    seen_reals+=("$real")

                    # Check if this is the active one
                    if [[ -n "$active_real" && "$real" == "$active_real" ]]; then
                        results=("$src:$bin:active" "${results[@]}")
                    else
                        results+=("$src:$bin:shadowed")
                    fi
                    break  # Found in this ecosystem, move to next
                fi
            done
        done
    done

    # Output results
    printf '%s\n' "${results[@]}"
}

# Check if package exists in native repo
pkg_exists() {
    local pkg="$1" mgr="$2"
    case "$mgr" in
        apt)    apt-cache show "$pkg" &>/dev/null ;;
        apk)    apk search -e "$pkg" | grep -q "^${pkg}-" ;;
        pacman) pacman -Si "$pkg" &>/dev/null ;;
        dnf)    dnf info "$pkg" &>/dev/null ;;
        pkg)    pkg search -e "^${pkg}$" &>/dev/null ;;
        *)      return 1 ;;
    esac
}

# Install package via native package manager
pkg_install() {
    local pkg="$1" mgr="$2"
    case "$mgr" in
        apt)    sudo apt install -y "$pkg" ;;
        apk)    sudo apk add "$pkg" ;;
        pacman) sudo pacman -S --noconfirm "$pkg" ;;
        dnf)    sudo dnf install -y "$pkg" ;;
        pkg)    pkg install -y "$pkg" ;;
        *)      return 1 ;;
    esac
}

# Remove package via source
pkg_remove() {
    local pkg="$1" source="$2"
    case "$source" in
        apt)     sudo apt remove --purge -y "$pkg" && sudo apt autoremove -y ;;
        apk)     sudo apk del "$pkg" ;;
        pacman)  sudo pacman -Rs --noconfirm "$pkg" ;;
        dnf)     sudo dnf remove -y "$pkg" ;;
        pkg)     pkg uninstall -y "$pkg" ;;
        uv|uv:*)
            # Binary name may differ from package name - look it up
            # uv tool list format: "package-name vX.X.X\n- binary1\n- binary2"
            local uv_pkg=$(uv tool list 2>/dev/null | grep -B1 "^- $pkg\$" | head -1 | cut -d' ' -f1)
            uv tool uninstall "${uv_pkg:-$pkg}"
            ;;
        cargo)
            # Binary name may differ from crate name - look it up
            crate=$(cargo install --list 2>/dev/null | grep -B1 "^    $pkg\$" | head -1 | cut -d' ' -f1)
            cargo uninstall "${crate:-$pkg}"
            ;;
        npm)     npm uninstall -g "$pkg" ;;
        go:*)    rm -f "$GOPATH/bin/$pkg" "$HOME/go/bin/$pkg" 2>/dev/null ;;
        brew)    brew uninstall "$pkg" ;;
        nix)     nix-env --uninstall "$pkg" 2>/dev/null || nix profile remove "$pkg" ;;
        sat) rm -f "$HOME/.local/bin/$pkg" ;;
        repo)    rm -f "$HOME/.local/bin/$pkg" ;;
        repo:*)  rm -f "$HOME/.local/bin/$pkg" ;;
        gh:*)
            rm -f "$HOME/.local/bin/$pkg" "$HOME/bin/$pkg" 2>/dev/null
            ;;
        system)  # Generic system - use cached package manager
            local mgr="$SAT_PKG_MANAGER"
            [[ -z "$mgr" ]] && return 1
            pkg_remove "$pkg" "$mgr"
            return $?
            ;;
        *)       return 1 ;;
    esac
}

# =============================================================================
# SNAPSHOT-BASED CONFIG CLEANUP (for sat shell)
# =============================================================================

# Take snapshot of config directories and dotfiles
take_snapshot() {
    local snapshot_file="$1"
    {
        # XDG Base Directories
        [[ -d "$HOME/.config" ]] && find "$HOME/.config" -maxdepth 1 -type d -printf "%f\n"
        [[ -d "$HOME/.local/share" ]] && find "$HOME/.local/share" -maxdepth 1 -type d -printf "%f\n"
        [[ -d "$HOME/.local/state" ]] && find "$HOME/.local/state" -maxdepth 1 -type d -printf "%f\n"
        [[ -d "$HOME/.cache" ]] && find "$HOME/.cache" -maxdepth 1 -type d -printf "%f\n"

        # Root home dotfiles (hidden files/dirs starting with .)
        find "$HOME" -maxdepth 1 -name ".*" \( -type f -o -type d \) -printf "%f\n"
    } 2>/dev/null | grep -v '^\.$' | sort -u > "$snapshot_file"
}

# Word boundary matching - check if tool name appears as complete word in dir name
# Example: "saul" matches "better-curl-saul" but not "saulconfig"
# Separators: dash (-), underscore (_), start/end of string
matches_tool_name() {
    local dir_name="$1"
    local tool="$2"

    # Skip very short names (too risky - "go", "fd", etc)
    [[ ${#tool} -lt 3 ]] && return 1

    # Case-insensitive comparison
    local dir_lower="${dir_name,,}"
    local tool_lower="${tool,,}"

    # Check if tool appears as a complete word
    # Regex: (start OR separator) + tool + (separator OR end)
    if [[ "$dir_lower" =~ (^|[-_])${tool_lower}([-_]|$) ]]; then
        return 0
    fi

    return 1
}

# Clean up configs created during session
cleanup_session_configs() {
    local snapshot_before="$1"
    local snapshot_after="$2"
    local manifest="$3"

    # Take snapshot after session
    take_snapshot "$snapshot_after"

    # Get list of installed tools from manifest
    local installed_tools=()
    while IFS='=' read -r key value; do
        [[ "$key" == "TOOL" ]] && installed_tools+=("$value")
    done < "$manifest"

    [[ ${#installed_tools[@]} -eq 0 ]] && return

    # Find new items (created during session)
    local new_items=$(comm -13 "$snapshot_before" "$snapshot_after")

    # Match and remove items that match installed tools
    while IFS= read -r item_name; do
        [[ -z "$item_name" ]] && continue

        for tool in "${installed_tools[@]}"; do
            if matches_tool_name "$item_name" "$tool"; then
                # XDG locations
                [[ -d "$HOME/.config/$item_name" ]] && rm -rf "$HOME/.config/$item_name" && \
                    printf "  ${C_DIM}Removed config: ~/.config/$item_name${C_RESET}\n"
                [[ -d "$HOME/.local/share/$item_name" ]] && rm -rf "$HOME/.local/share/$item_name" && \
                    printf "  ${C_DIM}Removed data: ~/.local/share/$item_name${C_RESET}\n"
                [[ -d "$HOME/.local/state/$item_name" ]] && rm -rf "$HOME/.local/state/$item_name" && \
                    printf "  ${C_DIM}Removed state: ~/.local/state/$item_name${C_RESET}\n"
                [[ -d "$HOME/.cache/$item_name" ]] && rm -rf "$HOME/.cache/$item_name" && \
                    printf "  ${C_DIM}Removed cache: ~/.cache/$item_name${C_RESET}\n"

                # Root dotfiles (files or directories)
                [[ -e "$HOME/$item_name" ]] && rm -rf "$HOME/$item_name" && \
                    printf "  ${C_DIM}Removed dotfile: ~/$item_name${C_RESET}\n"

                break
            fi
        done
    done <<< "$new_items"
}

# =============================================================================
# SESSION CLEANUP FUNCTIONS
# =============================================================================

# Remove a single tool from session: uninstall pkg + update master manifest
# Args: tool, source, pid
# Returns: 0 on success, 1 on failure
session_remove_tool() {
    local tool="$1" src="$2" pid="$3"
    local display=$(source_display "$src")
    local color=$(source_color "$display")

    # Check if tool:source was promoted to system manifest
    if grep -qF "$tool=$src" "$SAT_MANIFEST" 2>/dev/null; then
        printf "  ${C_DIM}~ %-18s (in system manifest)${C_RESET}\n" "$tool"
        master_remove "$tool" "$src" "$pid"
        return 0
    fi

    # Attempt to remove the package
    local err
    if err=$(pkg_remove "$tool" "$src" 2>&1); then
        printf "  - %-18s [${color}%s${C_RESET}]\n" "$tool" "$display"
        master_remove "$tool" "$src" "$pid"
        return 0
    else
        # Uninstall failed - if tool is already gone, clean manifest anyway
        if ! command -v "$tool" &>/dev/null; then
            printf "  ${C_DIM}- %-18s (already gone)${C_RESET}\n" "$tool"
            master_remove "$tool" "$src" "$pid"
            return 0
        fi
        printf "  ${C_CROSS} %-18s [${color}%s${C_RESET}] %s\n" "$tool" "$display" "$err"
        return 1
    fi
}

# Clean up a single session: configs + tools + folders
# Args: pid
cleanup_session() {
    local pid="$1"
    local session_dir="$SAT_SHELL_DIR/$pid"
    local xdg_dir="/tmp/sat-$pid"

    printf "${C_DIM}Cleaning orphaned session: $pid${C_RESET}\n"

    # Clean up configs if snapshots exist
    if [[ -f "$session_dir/snapshot-before" ]]; then
        local snapshot_after="$session_dir/snapshot-after"
        take_snapshot "$snapshot_after"
        [[ -f "$session_dir/manifest" ]] && \
            cleanup_session_configs "$session_dir/snapshot-before" "$snapshot_after" "$session_dir/manifest"
    fi

    # Process each tool for this PID from master manifest
    while IFS=: read -r tool src entry_pid; do
        [[ "$entry_pid" != "$pid" ]] && continue
        session_remove_tool "$tool" "$src" "$pid"
    done < "$SAT_SHELL_MASTER"

    # Delete session folder and XDG temp
    [[ -d "$session_dir" ]] && rm -rf "$session_dir"
    [[ -d "$xdg_dir" ]] && rm -rf "$xdg_dir"
}

# Find and clean up all orphaned sessions (dead PIDs)
# Called on every sat command
cleanup_orphaned_sessions() {
    [[ ! -f "$SAT_SHELL_MASTER" || ! -s "$SAT_SHELL_MASTER" ]] && return

    # Get unique dead PIDs and check for system packages
    local -a dead_pids=()
    local -A seen_pids=()
    local needs_sudo=false
    while IFS=: read -r tool src pid; do
        [[ -z "$pid" || -n "${seen_pids[$pid]}" ]] && continue
        seen_pids[$pid]=1
        if ! kill -0 "$pid" 2>/dev/null; then
            dead_pids+=("$pid")
            # Check if this entry needs sudo (system package)
            [[ "$src" == "system" || "$src" == "apt" || "$src" == "pacman" || "$src" == "dnf" || "$src" == "apk" ]] && needs_sudo=true
        fi
    done < "$SAT_SHELL_MASTER"

    [[ ${#dead_pids[@]} -eq 0 ]] && return

    # Cache sudo once if needed
    if $needs_sudo; then
        sudo -v || { printf "${C_DIM}Skipping system package cleanup (no sudo)${C_RESET}\n"; }
    fi

    # Clean up each dead session
    for pid in "${dead_pids[@]}"; do
        cleanup_session "$pid"
    done
}

# =============================================================================
# DEPENDENCIES
# =============================================================================

# Core dependencies (required for sat to function)
SAT_DEPS=(jq curl)

# Ensure all dependencies are installed
ensure_deps() {
    local missing=()
    for dep in "${SAT_DEPS[@]}"; do
        command -v "$dep" &>/dev/null || missing+=("$dep")
    done

    [[ ${#missing[@]} -eq 0 ]] && return 0

    local mgr="$SAT_PKG_MANAGER"
    if [[ -z "$mgr" ]]; then
        printf "${C_DIM}sat: missing deps (%s) - install manually${C_RESET}\n" "${missing[*]}" >&2
        return 1
    fi

    printf "${C_DIM}sat: installing %s...${C_RESET}\n" "${missing[*]}" >&2
    for dep in "${missing[@]}"; do
        pkg_install "$dep" "$mgr" || {
            printf "${C_DIM}sat: failed to install %s${C_RESET}\n" "$dep" >&2
            return 1
        }
    done
}

# Run on source
ensure_deps

