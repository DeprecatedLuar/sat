#!/usr/bin/env bash
# untrack.sh - Remove programs from manifest without uninstalling

sat_untrack() {
    for prog in "$@"; do
        if [[ -z "$(_sat_manifest_get "$prog")" ]]; then
            echo "$prog: not tracked"
            continue
        fi
        _sat_manifest_remove "$prog"
        echo "$prog: untracked"
    done
}
