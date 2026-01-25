# Quick Start

Get up and running with Sennet in less than 60 seconds.

## Installation

Run the following command to install the Sennet agent:

```bash
curl -sSL https://sennet.dev/install.sh | sudo bash
```

This script will:
1. Detect your OS and Architecture.
2. Download the latest binary.
3. Install a systemd service (if available).

## Configuration

Once installed, initialize the agent with your API key:

```bash
sudo sennet init
```

You will be prompted to enter your API key, which you can generate in the [Dashboard](/dashboard).

## Verify Installation

Check that the agent is running and collecting data:

```bash
sudo sennet status
```

You should see:
> Sennet Agent v0.1.3 is running (PID: 1234)
> Connected to Control Plane
> eBPF Probes attached: 4
