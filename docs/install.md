# Installation

This guide covers installing the Sennet agent on your Linux servers.

## Requirements

- Linux kernel **5.15+** (for eBPF support)
- Architecture: **x86_64** or **ARM64**
- **Root privileges** (required for eBPF programs)

## Quick Install (One-Liner)

To install and configure the agent in one go:

```bash
# Replace sk_... with your actual API key
curl -sSL https://raw.githubusercontent.com/MannanSaood/Sennet/main/install.sh | sudo bash -s -- --api-key "sk_your_key_here"
```

This will:
1. ✅ Install the agent binary
2. ✅ Automatically configure it with your API key
3. ✅ Start the service immediately

## Installation Methods

### Option 1: Flags

```bash
curl -sSL https://raw.githubusercontent.com/MannanSaood/Sennet/main/install.sh | sudo bash -s -- \
  --api-key "sk_your_key" \
  --server-url "https://sennet.onrender.com"
```

### Option 2: Environment Variables

```bash
export SENNET_API_KEY="sk_your_key"
curl -sSL https://raw.githubusercontent.com/MannanSaood/Sennet/main/install.sh | sudo -E bash
```

### Option 3: Interactive/Manual (Default)

If you run without arguments, you'll need to edit the config manually:

```bash
curl -sSL https://raw.githubusercontent.com/MannanSaood/Sennet/main/install.sh | sudo bash
sudo nano /etc/sennet/config.yaml
sudo systemctl restart sennet
```

# For ARM64  
curl -LO https://github.com/your-org/sennet/releases/latest/download/sennet-linux-arm64
```

### 2. Verify Checksum

```bash
curl -LO https://github.com/your-org/sennet/releases/latest/download/checksums.txt
sha256sum -c checksums.txt --ignore-missing
```

### 3. Install

```bash
chmod +x sennet-linux-*
sudo mv sennet-linux-* /usr/local/bin/sennet
```

### 4. Create Configuration

```bash
sudo mkdir -p /etc/sennet /var/lib/sennet
sudo nano /etc/sennet/config.yaml
```

See [Configuration Reference](config_reference.md) for all options.

### 5. Create Systemd Service

```bash
sudo tee /etc/systemd/system/sennet.service << 'EOF'
[Unit]
Description=Sennet Network Observability Agent
After=network-online.target

[Service]
Type=simple
ExecStart=/usr/local/bin/sennet
Restart=always
AmbientCapabilities=CAP_BPF CAP_NET_ADMIN CAP_SYS_ADMIN

[Install]
WantedBy=multi-user.target
EOF

sudo systemctl daemon-reload
sudo systemctl enable --now sennet
```

## Verify Installation

```bash
# Check service status
sudo systemctl status sennet

# View logs
sudo journalctl -u sennet -f

# Check eBPF programs (Linux only)
sudo bpftool prog list
```

## Upgrading

The agent can self-update:

```bash
sudo sennet upgrade
```

Or use the install script again - it will replace the existing binary.

## Uninstalling

```bash
sudo systemctl stop sennet
sudo systemctl disable sennet
sudo rm /etc/systemd/system/sennet.service
sudo rm /usr/local/bin/sennet
sudo rm -rf /etc/sennet /var/lib/sennet
sudo systemctl daemon-reload
```

## Troubleshooting

### "Operation not permitted"

Run with `sudo` - eBPF requires root privileges.

### "BTF not found"

Install kernel headers:
```bash
sudo apt install linux-headers-$(uname -r)
```

### Agent can't connect to backend

1. Check your `config.yaml` has correct `server_url` and `api_key`
2. Verify network connectivity: `curl -I https://your-server.com/health`
3. Check logs: `sudo journalctl -u sennet -f`
