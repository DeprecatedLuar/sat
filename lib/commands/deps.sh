#!/usr/bin/env bash
# deps.sh - Install sat dependencies

sat_deps() {
    echo "Installing sat dependencies..."
    echo ""
    DEPS=(tmux wget curl jq)
    source "$SAT_LIBRARY/commands/install.sh"
    for dep in "${DEPS[@]}"; do
        if command -v "$dep" &>/dev/null; then
            printf "  ${C_CHECK} %s\n" "$dep"
        else
            printf "  Installing %s...\n" "$dep"
            sat_install "$dep:sys"
        fi
    done
    echo ""
    echo "sat is ready to use."
}
