#!/usr/bin/env bash
# Sat wrapper script installation

install_sat() {
    local tool="$1"

    curl -sSL --fail --head "$SAT_BASE/cargo-bay/programs/${tool}.sh" >/dev/null 2>&1 || return 1
    source <(curl -sSL "$SAT_BASE/internal/fetcher.sh")
    sat_init && sat_run "$tool" &>/dev/null
}
