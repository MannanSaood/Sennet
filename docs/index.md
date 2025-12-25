# Sennet

**Network Observability with eBPF**

Sennet is a lightweight network monitoring agent that uses eBPF for high-performance packet analysis. Deploy agents across your infrastructure and monitor traffic in real-time through beautiful Grafana dashboards.

## Features

- 🚀 **eBPF-powered** - Kernel-level packet inspection with minimal overhead
- 📊 **Real-time metrics** - Packets/sec, bytes, drop rates, anomalies
- 🔄 **Self-updating** - Agents automatically upgrade when new versions are released  
- 🛡️ **Secure** - API key authentication, TLS communication
- 📈 **Grafana integration** - Pre-built dashboards for visualization

## Quick Start

```bash
curl -sSL https://raw.githubusercontent.com/your-org/sennet/main/install.sh | sudo bash
```

## Requirements

- Linux kernel 5.15+
- x86_64 or ARM64 architecture
- Root privileges (for eBPF)

## Documentation

- [Installation Guide](install.md) - Detailed setup instructions
- [Configuration Reference](config_reference.md) - All configuration options
- [Deployment Guide](DEPLOY.md) - Backend deployment

## Architecture

```
┌─────────────────┐     Heartbeat     ┌─────────────────┐
│   Sennet Agent  │ ───────────────▶  │  Control Plane  │
│   (eBPF + Rust) │                   │  (Go + SQLite)  │
└────────┬────────┘                   └────────┬────────┘
         │                                     │
         │ /metrics                            │ /metrics
         ▼                                     ▼
┌─────────────────────────────────────────────────────────┐
│                    Grafana Dashboard                     │
└─────────────────────────────────────────────────────────┘
```

## License

MIT License - See [LICENSE](https://github.com/your-org/sennet/blob/main/LICENSE) for details.
