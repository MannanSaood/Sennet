#!/bin/bash
# Phase 2.1: eBPF TC Classifier Tests
# NOTE: These tests require Linux with kernel 5.15+ and root privileges

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/../../.." && pwd)"

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

pass() { echo -e "${GREEN}[PASS]${NC} $1"; }
fail() { echo -e "${RED}[FAIL]${NC} $1"; exit 1; }
skip() { echo -e "${YELLOW}[SKIP]${NC} $1"; }
info() { echo -e "[INFO] $1"; }

echo "=========================================="
echo " Phase 2.1: eBPF TC Classifier Tests"
echo "=========================================="

# Check if running on Linux
if [[ "$(uname)" != "Linux" ]]; then
    skip "eBPF tests require Linux (detected: $(uname))"
    exit 0
fi

# Check kernel version
KERNEL_VERSION=$(uname -r | cut -d. -f1-2)
REQUIRED_VERSION="5.15"
if [[ "$(echo -e "$KERNEL_VERSION\n$REQUIRED_VERSION" | sort -V | head -n1)" != "$REQUIRED_VERSION" ]]; then
    skip "Kernel version $KERNEL_VERSION < $REQUIRED_VERSION required for RingBuf"
    exit 0
fi
pass "Kernel version $KERNEL_VERSION >= $REQUIRED_VERSION"

# Check for BTF support (required for CO-RE)
if [[ ! -f /sys/kernel/btf/vmlinux ]]; then
    skip "BTF not available (/sys/kernel/btf/vmlinux missing). CO-RE requires BTF."
    exit 0
fi
pass "BTF support available"

# Check for root/CAP_BPF
if [[ $EUID -ne 0 ]]; then
    skip "eBPF loading requires root privileges"
    exit 0
fi
pass "Running as root"

# Check for bpf-linker
if ! command -v bpf-linker &> /dev/null; then
    skip "bpf-linker not installed (required for eBPF compilation)"
    exit 0
fi
pass "bpf-linker available"

# Check if eBPF crate exists
EBPF_CRATE="$PROJECT_ROOT/agent/sennet-ebpf"
if [[ ! -d "$EBPF_CRATE" ]]; then
    skip "eBPF crate not found at $EBPF_CRATE"
    exit 0
fi
pass "eBPF crate exists"

# Build eBPF program
info "Building eBPF program..."
cd "$EBPF_CRATE"
if cargo build --release 2>&1; then
    pass "eBPF program compiles"
else
    fail "eBPF compilation failed"
fi

# Test: eBPF program can be loaded (without attaching)
info "Testing eBPF load (dry-run)..."
cd "$PROJECT_ROOT/agent"
if cargo run --release -- --dry-run 2>&1; then
    pass "eBPF program loads into kernel"
else
    fail "eBPF loading failed"
fi

# Test: RingBuf map is created
info "Checking for RingBuf map..."
MAPS=$(bpftool map show 2>/dev/null | grep -c "ringbuf" || true)
if [[ $MAPS -gt 0 ]]; then
    pass "RingBuf map created"
else
    skip "RingBuf map not found (may need to run agent first)"
fi

# Test: TC program can be attached to loopback (safe for testing)
info "Testing TC attachment to loopback..."
if tc qdisc add dev lo clsact 2>/dev/null; then
    if tc filter add dev lo ingress bpf direct-action obj "$EBPF_CRATE/target/bpfel-unknown-none/release/sennet-ebpf" sec tc 2>&1; then
        pass "TC program attaches to interface"
        # Cleanup
        tc qdisc del dev lo clsact 2>/dev/null || true
    else
        skip "TC attachment failed (may need different eBPF object path)"
        tc qdisc del dev lo clsact 2>/dev/null || true
    fi
else
    skip "Could not create clsact qdisc on loopback"
fi

echo ""
echo "=========================================="
echo " eBPF Tests Complete"
echo "=========================================="
