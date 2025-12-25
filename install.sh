#!/bin/bash
# Sennet Agent Installer
# Usage: curl -sSL https://raw.githubusercontent.com/your-org/sennet/main/install.sh | sudo bash

set -e

# Configuration
REPO="MannanSaood/Sennet"
INSTALL_DIR="/usr/local/bin"
CONFIG_DIR="/etc/sennet"
STATE_DIR="/var/lib/sennet"
SERVICE_NAME="sennet"
BINARY_NAME="sennet"

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m'

info() { echo -e "${BLUE}[INFO]${NC} $1"; }
success() { echo -e "${GREEN}[OK]${NC} $1"; }
warn() { echo -e "${YELLOW}[WARN]${NC} $1"; }
error() { echo -e "${RED}[ERROR]${NC} $1"; exit 1; }

# Parse arguments
DRY_RUN=false
VERSION=""
while [[ $# -gt 0 ]]; do
    case $1 in
        --dry-run|-n)
            DRY_RUN=true
            shift
            ;;
        --version|-v)
            VERSION="$2"
            shift 2
            ;;
        *)
            shift
            ;;
    esac
done

echo ""
echo "  ____                        _   "
echo " / ___|  ___ _ __  _ __   ___| |_ "
echo " \\___ \\ / _ \\ '_ \\| '_ \\ / _ \\ __|"
echo "  ___) |  __/ | | | | | |  __/ |_ "
echo " |____/ \\___|_| |_|_| |_|\\___|\\__|"
echo ""
echo "Sennet Agent Installer"
echo "======================"
echo ""

# Check root
if [[ $EUID -ne 0 ]] && [[ "$DRY_RUN" != "true" ]]; then
    error "This script must be run as root. Use: sudo bash install.sh"
fi

# Detect architecture
ARCH=$(uname -m)
case $ARCH in
    x86_64|amd64)
        ARCH_SUFFIX="linux-amd64"
        ;;
    aarch64|arm64)
        ARCH_SUFFIX="linux-arm64"
        ;;
    *)
        error "Unsupported architecture: $ARCH"
        ;;
esac
info "Detected architecture: $ARCH ($ARCH_SUFFIX)"

# Detect OS
if [[ "$(uname -s)" != "Linux" ]]; then
    error "This installer only supports Linux"
fi
info "Detected OS: Linux"

# Check for required tools
for cmd in curl sha256sum; do
    if ! command -v $cmd &> /dev/null; then
        error "Required command not found: $cmd"
    fi
done

# Get latest version if not specified
if [[ -z "$VERSION" ]]; then
    info "Fetching latest version..."
    VERSION=$(curl -sL "https://api.github.com/repos/${REPO}/releases/latest" | grep '"tag_name":' | sed -E 's/.*"v([^"]+)".*/\1/' || echo "")
    if [[ -z "$VERSION" ]]; then
        error "Failed to fetch latest version. Specify with --version"
    fi
fi
info "Installing version: $VERSION"

# Set download URLs
DOWNLOAD_URL="https://github.com/${REPO}/releases/download/v${VERSION}/${BINARY_NAME}-${ARCH_SUFFIX}"
CHECKSUM_URL="https://github.com/${REPO}/releases/download/v${VERSION}/checksums.txt"

# Create temp directory
TMPDIR=$(mktemp -d)
trap "rm -rf $TMPDIR" EXIT

# Download binary
info "Downloading ${BINARY_NAME}..."
if [[ "$DRY_RUN" == "true" ]]; then
    info "[DRY-RUN] Would download: $DOWNLOAD_URL"
else
    curl -fsSL -o "$TMPDIR/$BINARY_NAME" "$DOWNLOAD_URL" || error "Failed to download binary"
    success "Downloaded binary"
fi

# Download and verify checksum
info "Verifying SHA256 checksum..."
if [[ "$DRY_RUN" == "true" ]]; then
    info "[DRY-RUN] Would verify checksum from: $CHECKSUM_URL"
else
    curl -fsSL -o "$TMPDIR/checksums.txt" "$CHECKSUM_URL" || error "Failed to download checksums"
    
    EXPECTED_HASH=$(grep "${BINARY_NAME}-${ARCH_SUFFIX}" "$TMPDIR/checksums.txt" | awk '{print $1}')
    ACTUAL_HASH=$(sha256sum "$TMPDIR/$BINARY_NAME" | awk '{print $1}')
    
    if [[ "$EXPECTED_HASH" != "$ACTUAL_HASH" ]]; then
        error "Checksum mismatch! Expected: $EXPECTED_HASH, Got: $ACTUAL_HASH"
    fi
    success "Checksum verified"
fi

# Install binary
info "Installing to $INSTALL_DIR..."
if [[ "$DRY_RUN" == "true" ]]; then
    info "[DRY-RUN] Would install to: $INSTALL_DIR/$BINARY_NAME"
else
    chmod +x "$TMPDIR/$BINARY_NAME"
    mv "$TMPDIR/$BINARY_NAME" "$INSTALL_DIR/$BINARY_NAME"
    success "Installed $BINARY_NAME to $INSTALL_DIR"
fi

# Create directories
info "Creating directories..."
if [[ "$DRY_RUN" == "true" ]]; then
    info "[DRY-RUN] Would create: $CONFIG_DIR, $STATE_DIR"
else
    mkdir -p "$CONFIG_DIR"
    mkdir -p "$STATE_DIR"
    success "Created directories"
fi

# Create default config if not exists
if [[ ! -f "$CONFIG_DIR/config.yaml" ]]; then
    info "Creating default configuration..."
    if [[ "$DRY_RUN" == "true" ]]; then
        info "[DRY-RUN] Would create config at: $CONFIG_DIR/config.yaml"
    else
        cat > "$CONFIG_DIR/config.yaml" << 'EOF'
# Sennet Agent Configuration
# See: https://github.com/your-org/sennet/docs/config_reference.md

# Server URL (required)
server_url: "https://your-server.example.com"

# API Key (required) - get from: sennet-server keygen --name "MyAgent"
api_key: "YOUR_API_KEY_HERE"

# Log level: trace, debug, info, warn, error
log_level: "info"

# Network interface (optional, auto-detected if not set)
# interface: "eth0"
EOF
        chmod 600 "$CONFIG_DIR/config.yaml"
        success "Created default config (edit $CONFIG_DIR/config.yaml)"
    fi
fi

# Create systemd service
info "Creating systemd service..."
SYSTEMD_SERVICE="/etc/systemd/system/${SERVICE_NAME}.service"

if [[ "$DRY_RUN" == "true" ]]; then
    info "[DRY-RUN] Would create service: $SYSTEMD_SERVICE"
else
    cat > "$SYSTEMD_SERVICE" << EOF
[Unit]
Description=Sennet Network Observability Agent
Documentation=https://github.com/${REPO}
After=network-online.target
Wants=network-online.target

[Service]
Type=simple
ExecStart=${INSTALL_DIR}/${BINARY_NAME}
Restart=always
RestartSec=10
Environment=RUST_LOG=info

# Security hardening
NoNewPrivileges=false
ProtectSystem=strict
ProtectHome=true
ReadWritePaths=${STATE_DIR}
ReadOnlyPaths=${CONFIG_DIR}

# eBPF requires these capabilities
AmbientCapabilities=CAP_BPF CAP_NET_ADMIN CAP_SYS_ADMIN
CapabilityBoundingSet=CAP_BPF CAP_NET_ADMIN CAP_SYS_ADMIN

[Install]
WantedBy=multi-user.target
EOF
    success "Created systemd service"
    
    # Reload and enable service
    systemctl daemon-reload
    systemctl enable "$SERVICE_NAME" 2>/dev/null || true
    success "Enabled $SERVICE_NAME service"
fi

# Summary
echo ""
echo "========================================="
echo " Installation Complete!"
echo "========================================="
echo ""
echo "Next steps:"
echo "  1. Edit configuration: sudo nano $CONFIG_DIR/config.yaml"
echo "  2. Add your API key and server URL"
echo "  3. Start the agent: sudo systemctl start $SERVICE_NAME"
echo "  4. Check status: sudo systemctl status $SERVICE_NAME"
echo "  5. View logs: sudo journalctl -u $SERVICE_NAME -f"
echo ""
info "Binary installed at: $INSTALL_DIR/$BINARY_NAME"
info "Config file at: $CONFIG_DIR/config.yaml"
echo ""

if [[ "$DRY_RUN" == "true" ]]; then
    warn "This was a DRY-RUN. No changes were made."
fi
