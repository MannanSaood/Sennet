# Configuration Reference

The Sennet agent is configured via a YAML file located at `/etc/sennet/config.yaml`.

## Quick Start

Minimal configuration:

```yaml
server_url: "https://your-server.example.com"
api_key: "sk_your_api_key_here"
```

## Full Configuration

```yaml
# Sennet Agent Configuration
# Location: /etc/sennet/config.yaml

# ============================================================
# REQUIRED SETTINGS
# ============================================================

# Control plane server URL (required)
# The URL of your Sennet backend server
server_url: "https://sennet.example.com"

# API key for authentication (required)
# Generate with: sennet-server keygen --name "MyAgent"
api_key: "sk_xxxxxxxxxxxxxxxxxxxx"

# ============================================================
# OPTIONAL SETTINGS  
# ============================================================

# Logging level
# Options: trace, debug, info, warn, error
# Default: info
log_level: "info"

# Network interface to monitor
# If not specified, auto-detects the interface with the default route
# interface: "eth0"

# Heartbeat interval in seconds
# How often the agent sends metrics to the control plane
# Default: 30
heartbeat_interval_secs: 30

# State directory for agent identity
# Default: /var/lib/sennet
state_dir: "/var/lib/sennet"
```

## Configuration Options

### `server_url` (required)

The URL of your Sennet control plane server.

| Type | Default | Example |
|------|---------|---------|
| `string` | - | `https://sennet.example.com` |

### `api_key` (required)

API key for authenticating with the control plane. Generate one using:

```bash
sennet-server keygen --name "Production-Agent-1"
```

| Type | Default | Example |
|------|---------|---------|
| `string` | - | `sk_abc123...` |

### `log_level`

Controls the verbosity of logging.

| Type | Default | Options |
|------|---------|---------|
| `string` | `info` | `trace`, `debug`, `info`, `warn`, `error` |

### `interface`

Network interface to attach eBPF programs to. If not specified, the agent auto-detects the interface with the default route.

| Type | Default | Example |
|------|---------|---------|
| `string` | auto | `eth0`, `ens5`, `enp0s3` |

### `heartbeat_interval_secs`

How often (in seconds) the agent sends metrics to the control plane.

| Type | Default | Range |
|------|---------|-------|
| `u64` | `30` | 5-300 |

### `state_dir`

Directory where the agent stores its identity (UUID) and state.

| Type | Default |
|------|---------|
| `string` | `/var/lib/sennet` |

## Environment Variables

Configuration can also be set via environment variables (override file settings):

| Variable | Config Key |
|----------|------------|
| `SENNET_SERVER_URL` | `server_url` |
| `SENNET_API_KEY` | `api_key` |
| `RUST_LOG` | `log_level` |

Example:

```bash
export SENNET_API_KEY="sk_xxxxxxxxxxxx"
export SENNET_SERVER_URL="https://sennet.example.com"
sudo -E /usr/local/bin/sennet
```

## Example Configurations

### Minimal Production

```yaml
server_url: "https://sennet.yourcompany.com"
api_key: "sk_prod_xxxxx"
```

### Debug Mode

```yaml
server_url: "https://sennet.yourcompany.com"
api_key: "sk_debug_xxxxx"
log_level: "debug"
heartbeat_interval_secs: 10
```

### Specific Interface

```yaml
server_url: "https://sennet.yourcompany.com"
api_key: "sk_xxxxx"
interface: "ens192"
log_level: "info"
```

## Validating Configuration

The agent validates configuration on startup. Check logs for errors:

```bash
sudo journalctl -u sennet | head -20
```

Common validation errors:

- `Missing required field: server_url` - Add the server URL
- `Missing required field: api_key` - Add your API key
- `Invalid URL` - Check the server_url format
- `File not found` - Create `/etc/sennet/config.yaml`
