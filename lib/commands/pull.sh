#!/usr/bin/env bash
# pull.sh - Download latest sat version from GitHub

sat_pull() {
    rm -rf "$SAT_LIBRARY"
    _ensure_lib
    echo "Done!"
}
