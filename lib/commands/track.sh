#!/usr/bin/env bash
# track.sh - Add existing programs to manifest for sat management

sat_track() {
    for prog in "$@"; do
        if [[ -n "$(_sat_manifest_get "$prog")" ]]; then
            echo "$prog: already tracked"
            continue
        fi
        bin=$(command -v "$prog" 2>/dev/null)
        if [[ -z "$bin" ]]; then
            echo "$prog: not found"
            continue
        fi
        src=$(detect_source "$prog")
        [[ -z "$src" || "$src" == "unknown" ]] && { echo "$prog: unknown source, skipping"; continue; }
        _sat_manifest_add "$prog" "$src"
        display=$(source_display "$src")
        color=$(source_color "$display")
        printf "%-20s [${color}%s${C_RESET}] tracked\n" "$prog" "$display"
    done
}
