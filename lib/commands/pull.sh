#!/usr/bin/env bash
# pull.sh - Refresh sat library from GitHub

sat_pull() {
    echo "Refreshing sat library from GitHub..."
    rm -rf "$SAT_LIBRARY"
    _ensure_lib
    echo "Library refreshed from GitHub"
}
