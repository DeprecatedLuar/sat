#!/usr/bin/env bash
# untrack.sh - Remove programs from manifest without uninstalling

sat_untrack() {
    for prog in "$@"; do
        if [[ -z "$(manifest_get "$prog")" ]]; then
            echo "$prog: not tracked"
            continue
        fi
        manifest_remove "$prog"
        echo "$prog: untracked"
    done
}
