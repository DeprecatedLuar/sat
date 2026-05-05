#!/usr/bin/env bash
# sat installer - https://github.com/DeprecatedLuar/sat

set -e

BIN_DIR="$HOME/.local/bin"
BIN="$BIN_DIR/sat"
SAT_URL="https://raw.githubusercontent.com/DeprecatedLuar/sat/main/sat"

mkdir -p "$BIN_DIR"

echo "Downloading sat..."
curl -sSL "$SAT_URL" -o "$BIN"
chmod +x "$BIN"

# Warn if ~/.local/bin isn't in PATH
if [[ ":$PATH:" != *":$BIN_DIR:"* ]]; then
    echo ""
    echo "Note: $BIN_DIR is not in your PATH."
    echo "Add this to your shell config (~/.bashrc, ~/.zshrc, etc.):"
    echo ""
    echo '  export PATH="$HOME/.local/bin:$PATH"'
    echo ""
fi

echo "sat installed to $BIN"
echo "Run 'sat deps' to install core dependencies (tmux, curl, jq)."
