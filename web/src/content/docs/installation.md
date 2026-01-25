# Installation

Sennet is designed to be a single binary with zero external dependencies. It works on most modern Linux kernels (5.4+).

## Linux Standalone

The easiest way to install Sennet is via our install script:

```bash
curl -sSL https://sennet.dev/install.sh | sudo bash
```

### Manual Installation

If you prefer to install manually, download the binary from the [Releases page](https://github.com/sennet/sennet/releases) and place it in your path.

```bash
wget https://github.com/sennet/sennet/releases/download/v0.1.3/sennet-linux-amd64
chmod +x sennet-linux-amd64
sudo mv sennet-linux-amd64 /usr/local/bin/sennet
```

## Kubernetes (Helm)

Deploying to Kubernetes is done via our Helm chart. This will deploy a DaemonSet to ensure every node is monitored.

```bash
helm repo add sennet https://charts.sennet.dev
helm repo update
helm install sennet sennet/sennet-agent \
  --set apiKey=YOUR_API_KEY
```

## System Requirements

- **OS**: Linux (Ubuntu 20.04+, Debian 11+, Fedora 34+, CentOS 8+)
- **Kernel**: 5.4 or newer (BTF support recommended but not required)
- **CPU**: x86_64 or arm64
- **Memory**: ~100MB per node
