#!/usr/bin/env bash
# pull.sh - Download latest sat version from GitHub

sat_pull() {
    echo "Updating sat..."

    echo "  Refreshing library..."
    rm -rf "$SAT_LIBRARY"
    _ensure_lib

    local bin_path
    bin_path=$(command -v sat)
    if [[ -n "$bin_path" ]]; then
        if [[ -w "$bin_path" ]]; then
            echo "  Updating binary ($bin_path)..."
            if curl -sSL "https://raw.githubusercontent.com/DeprecatedLuar/sat/main/sat" -o "$bin_path.tmp"; then
                chmod +x "$bin_path.tmp" && mv "$bin_path.tmp" "$bin_path"
            else
                rm -f "$bin_path.tmp"
                echo "  Warning: failed to download binary update" >&2
            fi
        else
            echo "  Binary at $bin_path is not writable (try sudo sat pull)" >&2
        fi
    fi

    echo "Done!"
}
