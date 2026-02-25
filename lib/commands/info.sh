#!/usr/bin/env bash
# info.sh - Show detailed information about installed programs

sat_info() {
    for prog in "$@"; do
        bin=$(command -v "$prog" 2>/dev/null)
        if [[ -z "$bin" ]]; then
            echo "$prog: not found"
            continue
        fi
        real=$(readlink -f "$bin" 2>/dev/null || echo "$bin")
        src=$(resolve_source "$prog" "")
        display=$(source_display "$src")
        color=$(source_color "$display")
        tracked=$(_sat_manifest_get "$prog")
        ver=$("$prog" --version 2>/dev/null | head -1 || echo "unknown")

        light=$(source_light "$src")
        printf "${light}%s${C_RESET} [${color}%s${C_RESET}] ${C_DIM}%s${C_RESET}\n" "$prog" "$display" "$ver"
        echo "  path:   $bin"
        [[ "$real" != "$bin" ]] && echo "  target: $real"
        # Show repo for gh:user/repo sources
        [[ "$tracked" == gh:* ]] && echo "  repo:   ${tracked#gh:}"
        [[ -n "$tracked" ]] && echo "  tracked"

        # Show shadowed installations
        all_sources=$(resolve_all_sources "$prog")
        shadowed=$(echo "$all_sources" | grep -v ":active$")
        if [[ -n "$shadowed" ]]; then
            echo "  shadowed:"
            while IFS=: read -r s_src s_path _; do
                [[ -z "$s_src" ]] && continue
                s_display=$(source_display "$s_src")
                s_color=$(source_color "$s_display")
                printf "    ${C_DIM}[${s_color}%s${C_RESET}${C_DIM}] %s${C_RESET}\n" "$s_display" "$s_path"
            done <<< "$shadowed"
        fi
        [[ $# -gt 1 ]] && echo ""
    done
}
