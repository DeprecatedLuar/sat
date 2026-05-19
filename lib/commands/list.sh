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
        local light=$(source_light "$source")
        display_tool_entry "$prog" "$source" "$light"
    }

    # Show current session tools (if inside a shell)
    if [[ -n "$SAT_SESSION" && -f "$SAT_SESSION_MANIFEST" ]]; then
        local session_output=""
        while IFS='=' read -r key value; do
            [[ "$key" != "TOOL" ]] && continue
            src=$(grep "^SOURCE_$value=" "$SAT_SESSION_MANIFEST" | cut -d= -f2)
            _matches_filter "$src" || continue
            session_output+=$(display_tool_entry "$value" "$src" "${C_DIM}")
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
                display_tool_entry "$tool" "$src" "${C_DIM}" " ${C_DIM}(pid $pid)${C_RESET}"
            done
            echo ""
            has_content=true
        fi
    fi

    # Show system manifest (permanent installs) grouped by source
    if [[ -s "$SAT_MANIFEST" ]]; then
        declare -a stale=()
        declare -a entries=()

        # Collect all entries with normalized source for grouping
        while IFS='=' read -r prog source; do
            [[ -z "$prog" ]] && continue
            if ! command -v "$prog" &>/dev/null; then
                stale+=("$prog")
                continue
            fi
            _matches_filter "$source" || continue

            # Normalize source for grouping
            local group="$source"
            case "$source" in
                npm:*) group="node" ;;
                nixos:*) group="nixos" ;;
                repo:*) group="repo" ;;
                gh:*) group="gh" ;;
                go:*) group="go" ;;
                unknown:*) group="unknown" ;;
                appimage:*) group="appimage" ;;
                flatpak:*) group="flatpak" ;;
                apt|apk|pacman|dnf|pkg) group="system" ;;
            esac
            entries+=("$group|$prog|$source")
        done < "$SAT_MANIFEST"

        if [[ ${#entries[@]} -gt 0 ]]; then
            # Count packages per source
            declare -A counts
            for entry in "${entries[@]}"; do
                local src="${entry%%|*}"
                ((counts[$src]++))
            done

            # Prefix each entry with padded count for sorting
            declare -a sorted=()
            for entry in "${entries[@]}"; do
                local src="${entry%%|*}"
                local count="${counts[$src]}"
                # Force unknown to sort last
                [[ "$src" == "unknown" ]] && count=0
                printf -v padded "%06d" "$count"
                sorted+=("$padded|$entry")
            done

            # Sort by count (descending) and display
            while IFS='|' read -r _ group prog source; do
                _display_tool "$prog" "$source"
            done < <(printf '%s\n' "${sorted[@]}" | sort -rn)
            has_content=true
        fi

        # Clean stale entries
        if [[ ${#stale[@]} -gt 0 ]]; then
            echo ""
            echo "Cleaning ${#stale[@]} stale entries..."
            for prog in "${stale[@]}"; do
                manifest_remove "$prog"
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
