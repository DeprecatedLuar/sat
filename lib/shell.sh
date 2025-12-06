#!/usr/bin/env bash
# sat shell - temporary environment with auto-cleanup

# Check if tool:source has other live sessions (for shared cleanup)
master_tool_has_other_live_pids() {
    local tool="$1" src="$2" our_pid="$3"
    [[ ! -f "$SAT_SHELL_MASTER" ]] && return 1
    while IFS=: read -r t s pid; do
        [[ "$t" != "$tool" || "$s" != "$src" ]] && continue
        [[ "$pid" == "$our_pid" ]] && continue
        kill -0 "$pid" 2>/dev/null && return 0
    done < "$SAT_SHELL_MASTER"
    return 1
}

shell_cleanup() {
    local session_pid="$1"
    local session_dir="$2"
    local xdg_dir="$3"
    local session_manifest="$session_dir/manifest"
    local snapshot_before="$session_dir/snapshot-before"
    local snapshot_after="$session_dir/snapshot-after"

    echo ""
    printf "${C_DIM}Cleaning up session $session_pid${C_RESET}\n"

    if [[ ! -f "$session_manifest" ]]; then
        echo "  No tools installed"
        rm -rf "$session_dir" "$xdg_dir"
        return 0
    fi

    local -a session_tools=()
    local -A session_sources=()

    while IFS='=' read -r key value; do
        if [[ "$key" == "TOOL" ]]; then
            session_tools+=("$value")
        elif [[ "$key" == SOURCE_* ]]; then
            local tool="${key#SOURCE_}"
            session_sources["$tool"]="$value"
        fi
    done < "$session_manifest"

    if [[ ${#session_tools[@]} -eq 0 ]]; then
        echo "  Nothing to remove"
        rm -rf "$session_dir" "$xdg_dir"
        return 0
    fi

    local -a to_remove=()
    local -a to_keep=()

    for tool in "${session_tools[@]}"; do
        local src="${session_sources[$tool]}"

        # Check if other sessions are using this tool:source
        if master_tool_has_other_live_pids "$tool" "$src" "$session_pid"; then
            to_keep+=("$tool (other session)")
            continue
        fi

        # Check if tool:source was promoted to system manifest
        if grep -q "^$tool=$src\$" "$SAT_MANIFEST" 2>/dev/null; then
            to_keep+=("$tool (promoted)")
            continue
        fi

        to_remove+=("$tool")
    done

    for kept in "${to_keep[@]}"; do
        printf "  ${C_DIM}~ %s${C_RESET}\n" "$kept"
    done

    if [[ ${#to_remove[@]} -gt 0 ]]; then
        for tool in "${to_remove[@]}"; do
            local src="${session_sources[$tool]}"
            local display=$(source_display "$src")
            local color=$(source_color "$display")

            local err_file="/tmp/sat-cleanup-$$-$tool"
            pkg_remove "$tool" "$src" >"$err_file" 2>&1 &
            spin_probe "$tool" $!
            if wait $!; then
                printf "  - %-18s [${color}%s${C_RESET}]\n" "$tool" "$display"
            else
                local err=$(cat "$err_file" 2>/dev/null | head -1)
                printf "  ${C_CROSS} %-18s [${color}%s${C_RESET}] %s\n" "$tool" "$display" "$err"
            fi
            rm -f "$err_file"
            # Remove from master manifest
            master_remove "$tool" "$src" "$session_pid"
        done
    else
        echo "  Nothing to remove"
    fi

    # Clean up configs created during session
    if [[ -f "$snapshot_before" ]]; then
        take_snapshot "$snapshot_after"
        cleanup_session_configs "$snapshot_before" "$snapshot_after" "$session_manifest"
    fi

    rm -rf "$session_dir" "$xdg_dir"
}

sat_shell() {
    local specs=("$@")

    if ! command -v tmux &>/dev/null; then
        printf "${C_CROSS} sat shell requires tmux for proper isolation.\n"
        echo ""
        echo "Install it with:"
        echo "  sat install tmux"
        echo ""
        return 1
    fi

    if [[ ${#specs[@]} -eq 0 ]]; then
        echo "Usage: sat shell <tool[:source]> [tool2[:source]] ..."
        return 1
    fi

    # Cache sudo if any :sys tools requested (before spawning tmux)
    for spec in "${specs[@]}"; do
        parse_tool_spec "$spec"
        if [[ "$_TOOL_SOURCE" == "system" ]]; then
            sudo -v || { echo "sudo required for system packages"; return 1; }
            break
        fi
    done

    local session_dir="$SAT_SHELL_DIR/$$"
    local manifest="$session_dir/manifest"
    local snapshot_before="$session_dir/snapshot-before"

    mkdir -p "$session_dir"

    local xdg_dir="/tmp/sat-$$"
    mkdir -p "$xdg_dir"/{config,data,cache,state}

    take_snapshot "$snapshot_before"

    local to_install=()
    local already_have=()

    for spec in "${specs[@]}"; do
        parse_tool_spec "$spec"
        local tool="$_TOOL_NAME"
        local forced_src="$_TOOL_SOURCE"

        if [[ -z "$forced_src" ]] && command -v "$tool" &>/dev/null; then
            already_have+=("$spec")
        else
            to_install+=("$spec")
        fi
    done

    local to_install_str="${to_install[*]}"
    local already_have_str="${already_have[*]}"
    local all_specs_str="${specs[*]}"

    local rcfile="$session_dir/rcfile"
    cat > "$rcfile" << 'RCFILE_START'
source ~/.bashrc 2>/dev/null
RCFILE_START

    cat >> "$rcfile" << RCFILE_VARS
export PS1="(sat) \$PS1"
export HISTFILE="$xdg_dir/history"
export SAT_SESSION="$$"

export XDG_CONFIG_HOME="$xdg_dir/config"
export XDG_DATA_HOME="$xdg_dir/data"
export XDG_CACHE_HOME="$xdg_dir/cache"
export XDG_STATE_HOME="$xdg_dir/state"

SAT_SESSION_MANIFEST="$manifest"
SAT_SHELL_MASTER="$SAT_SHELL_MASTER"
SAT_LIB="$SAT_LIB"

SAT_TO_INSTALL=($to_install_str)
SAT_ALREADY_HAVE=($already_have_str)
SAT_ALL_SPECS=($all_specs_str)
RCFILE_VARS

    cat >> "$rcfile" << 'RCFILE_MAIN'

source "$SAT_LIB/common.sh"

INSTALL_ORDER=("${SHELL_INSTALL_ORDER[@]}")

clear
cols=$(tput cols 2>/dev/null || echo 80)
header="────[SAT SHELL]"
pad=$(printf '─%.0s' $(seq 1 $((cols - ${#header}))))
echo -e "\033[1m${header}${pad}\033[0m"
echo ""

for spec in "${SAT_ALREADY_HAVE[@]}"; do
    parse_tool_spec "$spec"
    tool="$_TOOL_NAME"
    src=$(resolve_source "$tool" "")
    display=$(source_display "$src")
    light=$(source_light "$src")
    color=$(source_color "$display")
    printf "[${C_CHECK}] ${light}%-20s${C_RESET} [${color}%s${C_RESET}] ${C_DIM}(already installed)${C_RESET}\n" "$tool" "$display"
done

if [[ ${#SAT_TO_INSTALL[@]} -gt 0 ]]; then
    total=${#SAT_TO_INSTALL[@]}

    for spec in "${SAT_TO_INSTALL[@]}"; do
        parse_tool_spec "$spec"
        printf "[ ] %s\n" "$_TOOL_NAME"
    done

    printf "\033[%dA" "$total"

    for spec in "${SAT_TO_INSTALL[@]}"; do
        parse_tool_spec "$spec"
        tool="$_TOOL_NAME"
        forced_src="$_TOOL_SOURCE"

        if [[ -n "$forced_src" ]]; then
            try_source "$tool" "$forced_src" &
            spin_probe "$tool" $!
            if wait $!; then
                _INSTALL_SOURCE="$forced_src"
                installed=true
            else
                installed=false
            fi
        else
            installed=false
            install_with_fallback "$tool" && installed=true
        fi

        if $installed; then
            display=$(source_display "$_INSTALL_SOURCE")
            light=$(source_light "$_INSTALL_SOURCE")
            color=$(source_color "$display")
            printf "\r[${C_CHECK}] ${light}%-20s${C_RESET} [${color}%s${C_RESET}]\n" "$tool" "$display"
            pid_manifest_add "$SAT_SESSION" "$tool" "$_INSTALL_SOURCE"
            master_add "$tool" "$_INSTALL_SOURCE" "$SAT_SESSION"
        else
            printf "\r[${C_CROSS}] %-20s\n" "$tool"
        fi
    done

    # Track dependencies that got installed
    for spec in "${SAT_TO_INSTALL[@]}"; do
        parse_tool_spec "$spec"
        tool="$_TOOL_NAME"
        if command -v "$tool" &>/dev/null && ! grep -q "^TOOL=$tool$" "$SAT_SESSION_MANIFEST" 2>/dev/null; then
            src=$(resolve_source "$tool" "")
            [[ -z "$src" || "$src" == "unknown" ]] && continue
            display=$(source_display "$src")
            light=$(source_light "$src")
            color=$(source_color "$display")
            printf "[${C_CHECK}] ${light}%-20s${C_RESET} [${color}%s${C_RESET}] (dep)\n" "$tool" "$display"
            pid_manifest_add "$SAT_SESSION" "$tool" "$src"
            master_add "$tool" "$src" "$SAT_SESSION"
        fi
    done
    echo ""
fi

echo "type 'exit' to leave"
echo ""
RCFILE_MAIN

    tmux new-session -s "sat-$$" "bash --rcfile $rcfile"
    shell_cleanup "$$" "$session_dir" "$xdg_dir"
}
