#!/usr/bin/env bash
# list.sh - List tracked packages across system and sessions

sat_list() {
    local has_content=false

    # Display a tool entry with proper formatting
    _display_tool() {
        local prog="$1" source="$2"
        local display=$(source_display "$source")
        local color=$(source_color "$display")
        local light=$(source_light "$source")
        printf "  ${light}%-20s${C_RESET} [${color}%s${C_RESET}]\n" "$prog" "$display"
    }

    # Show current session tools (if inside a shell)
    if [[ -n "$SAT_SESSION" && -f "$SAT_SESSION_MANIFEST" ]]; then
        echo "Current session (temporary):"
        while IFS='=' read -r key value; do
            [[ "$key" != "TOOL" ]] && continue
            src=$(grep "^SOURCE_$value=" "$SAT_SESSION_MANIFEST" | cut -d= -f2)
            display=$(source_display "$src")
            color=$(source_color "$display")
            printf "  ${C_DIM}%-20s${C_RESET} [${color}%s${C_RESET}]\n" "$value" "$display"
        done < "$SAT_SESSION_MANIFEST"
        echo ""
        has_content=true
    fi

    # Show active shell tools (from master manifest)
    if [[ -s "$SAT_SHELL_MASTER" ]]; then
        local active_tools=()
        while IFS=: read -r tool src pid; do
            [[ -z "$tool" ]] && continue
            # Only show if PID is alive and not current session
            if kill -0 "$pid" 2>/dev/null && [[ "$pid" != "$SAT_SESSION" ]]; then
                active_tools+=("$tool:$src:$pid")
            fi
        done < "$SAT_SHELL_MASTER"

        if [[ ${#active_tools[@]} -gt 0 ]]; then
            echo "Active shell sessions:"
            for entry in "${active_tools[@]}"; do
                IFS=: read -r tool src pid <<< "$entry"
                display=$(source_display "$src")
                color=$(source_color "$display")
                printf "  ${C_DIM}%-20s${C_RESET} [${color}%s${C_RESET}] ${C_DIM}(pid $pid)${C_RESET}\n" "$tool" "$display"
            done
            echo ""
            has_content=true
        fi
    fi

    # Show system manifest (permanent installs) grouped by source
    if [[ -s "$SAT_MANIFEST" ]]; then
        # Collect entries by normalized source
        declare -A by_source
        declare -a stale=()
        while IFS='=' read -r prog source; do
            [[ -z "$prog" ]] && continue
            if ! command -v "$prog" &>/dev/null; then
                stale+=("$prog")
                continue
            fi
            # Normalize source for grouping (repo:* -> repo, apt/pacman/etc -> system)
            local group="$source"
            case "$source" in
                repo:*) group="repo" ;;
                apt|apk|pacman|dnf|pkg) group="system" ;;
            esac
            by_source[$group]+="$prog=$source"$'\n'
        done < "$SAT_MANIFEST"

        # Print in INSTALL_ORDER, then any remaining
        local printed=()
        for src in "${INSTALL_ORDER[@]}"; do
            [[ -z "${by_source[$src]}" ]] && continue
            while IFS='=' read -r prog source; do
                [[ -z "$prog" ]] && continue
                _display_tool "$prog" "$source"
            done <<< "${by_source[$src]}"
            printed+=("$src")
        done

        # Print sources not in INSTALL_ORDER (go, manual, unknown, etc)
        for src in "${!by_source[@]}"; do
            [[ " ${printed[*]} " =~ " $src " ]] && continue
            while IFS='=' read -r prog source; do
                [[ -z "$prog" ]] && continue
                _display_tool "$prog" "$source"
            done <<< "${by_source[$src]}"
        done

        # Clean stale entries
        if [[ ${#stale[@]} -gt 0 ]]; then
            echo ""
            echo "Cleaning ${#stale[@]} stale entries..."
            for prog in "${stale[@]}"; do
                _sat_manifest_remove "$prog"
            done
        fi
        has_content=true
    fi

    [[ "$has_content" == false ]] && echo "No packages tracked by sat"
}
