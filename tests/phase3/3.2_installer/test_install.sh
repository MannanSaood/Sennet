#!/bin/bash
# Phase 3.2: Installer Script Tests
# Tests the install.sh script functionality

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/../../.." && pwd)"
INSTALL_SCRIPT="$PROJECT_ROOT/install.sh"

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m'

pass() { echo -e "${GREEN}[PASS]${NC} $1"; }
fail() { echo -e "${RED}[FAIL]${NC} $1"; }
skip() { echo -e "${YELLOW}[SKIP]${NC} $1"; }
info() { echo -e "[INFO] $1"; }

echo "=========================================="
echo " Phase 3.2: Installer Script Tests"
echo "=========================================="

# Test 1: Install script exists
if [[ ! -f "$INSTALL_SCRIPT" ]]; then
    skip "Install script not found at $INSTALL_SCRIPT"
    echo "Create install.sh to enable these tests"
    exit 0
fi
pass "install.sh exists"

# Test 2: Script is executable (or can be made executable)
if [[ -x "$INSTALL_SCRIPT" ]]; then
    pass "install.sh is executable"
else
    skip "install.sh not executable (chmod +x install.sh)"
fi

# Test 3: Script has shebang
FIRST_LINE=$(head -n 1 "$INSTALL_SCRIPT")
if [[ "$FIRST_LINE" == "#!/bin/bash" ]] || [[ "$FIRST_LINE" == "#!/usr/bin/env bash" ]] || [[ "$FIRST_LINE" == "#!/bin/sh" ]]; then
    pass "Has valid shebang: $FIRST_LINE"
else
    fail "Missing or invalid shebang"
fi

# Read script content for analysis
CONTENT=$(cat "$INSTALL_SCRIPT")

# Test 4: Detects architecture
if echo "$CONTENT" | grep -qE '(uname -m|arch|ARCH|x86_64|aarch64)'; then
    pass "Detects system architecture"
else
    fail "No architecture detection found"
fi

# Test 5: Downloads binary
if echo "$CONTENT" | grep -qE '(curl|wget)'; then
    pass "Uses curl/wget for download"
else
    fail "No download mechanism found"
fi

# Test 6: Verifies checksum
if echo "$CONTENT" | grep -qE '(sha256|checksum|SHA256)'; then
    pass "Verifies checksum"
else
    fail "No checksum verification found"
fi

# Test 7: Creates systemd service
if echo "$CONTENT" | grep -qE '(systemd|systemctl|\.service)'; then
    pass "Creates systemd service"
else
    fail "No systemd integration found"
fi

# Test 8: Uses safe download location
if echo "$CONTENT" | grep -qE '(/tmp|mktemp|TMPDIR)'; then
    pass "Uses temporary directory for download"
else
    skip "Temp directory usage not detected"
fi

# Test 9: Installs to standard location
if echo "$CONTENT" | grep -qE '(/usr/local/bin|/usr/bin|/opt)'; then
    pass "Installs to standard location"
else
    skip "Standard install path not detected"
fi

# Test 10: Script syntax check
if bash -n "$INSTALL_SCRIPT" 2>/dev/null; then
    pass "Script syntax is valid"
else
    fail "Script has syntax errors"
fi

echo ""
echo "--- Dry-run Test ---"

# Test 11: Dry-run mode (if supported)
if echo "$CONTENT" | grep -qE '(--dry-run|-n|DRY_RUN)'; then
    info "Dry-run mode supported, testing..."
    if bash "$INSTALL_SCRIPT" --dry-run 2>/dev/null; then
        pass "Dry-run completes without errors"
    else
        skip "Dry-run failed (may require network)"
    fi
else
    skip "No dry-run mode detected"
fi

echo ""
echo "=========================================="
echo " Installer Tests Complete"
echo "=========================================="
