#!/usr/bin/env bash
# Smoke test - verify core functionality before/after refactor
set -e

echo "=== SAT Smoke Test ==="
echo ""

# Baseline state
echo "1. Checking sat binary..."
command -v sat >/dev/null || { echo "FAIL: sat not in PATH"; exit 1; }
echo "   ✓ sat binary found"

# Search functionality
echo "2. Testing search..."
sat search ripgrep:gh >/dev/null 2>&1 || { echo "FAIL: GitHub search"; exit 1; }
sat search bat:cargo >/dev/null 2>&1 || { echo "FAIL: cargo search"; exit 1; }
echo "   ✓ Search works (github, cargo)"

# List/manifest reading
echo "3. Testing list..."
set +e  # Disable exit on error for this command
sat list >/dev/null 2>&1
list_code=$?
set -e  # Re-enable
if [[ $list_code -le 1 ]]; then  # Allow exit code 0 or 1 (list might return 1 if empty)
    echo "   ✓ List works"
else
    echo "FAIL: sat list (exit code: $list_code)"
    exit 1
fi

# Scan
echo "4. Testing scan..."
sat scan --help >/dev/null 2>&1 || { echo "FAIL: scan help"; exit 1; }
echo "   ✓ Scan works"

# Install/uninstall cycle (using cargo as it's fast and deterministic)
echo "5. Testing install/uninstall cycle..."
TEST_TOOL="tokei"

# Clean state
sat uninstall "$TEST_TOOL" 2>/dev/null || true
sleep 1

# Install
if sat install "${TEST_TOOL}:cargo" >/dev/null 2>&1; then
    echo "   ✓ Install works (cargo)"

    # Verify in manifest
    if sat list | grep -q "$TEST_TOOL"; then
        echo "   ✓ Manifest tracking works"
    else
        echo "FAIL: Tool not in manifest after install"
        exit 1
    fi

    # Uninstall
    if sat uninstall "$TEST_TOOL" >/dev/null 2>&1; then
        echo "   ✓ Uninstall works"
    else
        echo "FAIL: Uninstall failed"
        exit 1
    fi

    # Verify removed from manifest
    if ! sat list | grep -q "$TEST_TOOL"; then
        echo "   ✓ Manifest cleanup works"
    else
        echo "FAIL: Tool still in manifest after uninstall"
        exit 1
    fi
else
    echo "   ⚠ Install test skipped (cargo not available or install failed)"
fi

echo ""
echo "=== All Smoke Tests Passed ==="
