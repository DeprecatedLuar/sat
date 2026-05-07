#!/usr/bin/env bash
# list.sh - List tracked packages across system and sessions

sat_list() {
    local has_content=false
    local -a filters=()

    # Parse source filters from args
    for arg in "$@"; do
        case "$arg" in
            fpk|flatpak)  filters+=("flatpak") ;;
            img|appimage) filters+=("appimage") ;;
            sys|system)   filters+=("system") ;;
            nix|nixos)    filters+=("nix" "nixos") ;;
            gh|github)    filters+=("github" "repo" "gh") ;;
            npm|node)     filters+=("npm" "node") ;;
            py|python|uv) filters+=("python" "uv") ;;
            cargo|rust)   filters+=("cargo" "rust") ;;
            go)           filters+=("go") ;;
            brew)         filters+=("brew") ;;
            sat)          filters+=("sat") ;;
            manual)       filters+=("manual") ;;
            *)            echo "Unknown source filter: $arg" >&2; return 1 ;;
        esac
    done

    # Check if source matches any filter (or no filters = show all)
    _matches_filter() {
        local source="$1"
        [[ ${#filters[@]} -eq 0 ]] && return 0

        local display=$(source_display "$source")
        local normalized="$source"
        case "$source" in
            repo:*) normalized="repo" ;;
            gh:*)   normalized="gh" ;;
            go:*)   normalized="go" ;;
            unknown:*) normalized="unknown" ;;
            appimage:*) normalized="appimage" ;;
            flatpak:*) normalized="flatpak" ;;
            apt|apk|pacman|dnf|pkg) normalized="system" ;;
        esac

        for filter in "${filters[@]}"; do
            [[ "$filter" == "$normalized" ]] && return 0
            [[ "$filter" == "$display" ]] && return 0
            [[ "$filter" == "$source" ]] && return 0
        done
        return 1
    }

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
        local session_output=""
        while IFS='=' read -r key value; do
            [[ "$key" != "TOOL" ]] && continue
            src=$(grep "^SOURCE_$value=" "$SAT_SESSION_MANIFEST" | cut -d= -f2)
            _matches_filter "$src" || continue
            display=$(source_display "$src")
            color=$(source_color "$display")
            session_output+=$(printf "  ${C_DIM}%-20s${C_RESET} [${color}%s${C_RESET}]\n" "$value" "$display")
        done < "$SAT_SESSION_MANIFEST"

        if [[ -n "$session_output" ]]; then
            echo "Current session (temporary):"
            echo "$session_output"
            echo ""
            has_content=true
        fi
    fi

    # Show active shell tools (from master manifest)
    if [[ -s "$SAT_SHELL_MASTER" ]]; then
        local active_tools=()
        while IFS=: read -r tool src pid; do
            [[ -z "$tool" ]] && continue
            # Only show if PID is alive and not current session
            if kill -0 "$pid" 2>/dev/null && [[ "$pid" != "$SAT_SESSION" ]]; then
                _matches_filter "$src" && active_tools+=("$tool:$src:$pid")
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
            # Skip if doesn't match filter
            _matches_filter "$source" || continue
            # Normalize source for grouping
            local group="$source"
            case "$source" in
                repo:*) group="repo" ;;
                gh:*) group="gh" ;;
                go:*) group="go" ;;
                unknown:*) group="unknown" ;;
                appimage:*) group="appimage" ;;
                flatpak:*) group="flatpak" ;;
                apt|apk|pacman|dnf|pkg) group="system" ;;
            esac
            by_source[$group]+="$prog=$source"$'\n'
        done < "$SAT_MANIFEST"

        # Count packages per source
        declare -A source_counts
        for src in "${!by_source[@]}"; do
            source_counts[$src]=$(echo "${by_source[$src]}" | grep -c "=")
        done

        # Sort sources by count (descending), but always put unknown last
        local sorted_sources=($(for src in "${!source_counts[@]}"; do
            if [[ "$src" == "unknown" ]]; then
                echo "0 $src"  # Force unknown to sort last
            else
                echo "${source_counts[$src]} $src"
            fi
        done | sort -rn | awk '{print $2}'))

        # Display in order of package count
        if [[ ${#by_source[@]} -gt 0 ]]; then
            for src in "${sorted_sources[@]}"; do
                while IFS='=' read -r prog source; do
                    [[ -z "$prog" ]] && continue
                    _display_tool "$prog" "$source"
                done <<< "${by_source[$src]}"
            done
            has_content=true
        fi

        # Clean stale entries
        if [[ ${#stale[@]} -gt 0 ]]; then
            echo ""
            echo "Cleaning ${#stale[@]} stale entries..."
            for prog in "${stale[@]}"; do
                _sat_manifest_remove "$prog"
            done
        fi
    fi

    if [[ "$has_content" == false ]]; then
        if [[ ${#filters[@]} -gt 0 ]]; then
            echo "No packages found for: ${filters[*]}"
        else
            echo "No packages tracked by sat"
        fi
    fi
}
