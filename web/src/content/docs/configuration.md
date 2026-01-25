# Configuration

The Sennet agent is configured via a YAML file located at `/etc/sennet/sennet.yaml`. The `sennet init` command creates this file for you.

## Example Config

```yaml
# /etc/sennet/sennet.yaml

api_key: "sn_live_..."

agent:
  log_level: "info" # debug, info, warn, error
  interface: "eth0" # Network interface to attach to

features:
  flow_tracking: true
  process_monitoring: true
  packet_capture: false # Enable only for debugging

# Advanced eBPF settings
ebpf:
  map_size: 1024
  perf_buffer: 512
```

## Environment Variables

You can override any configuration using environment variables prefixed with `SENNET_`.

- `SENNET_API_KEY`: Override API key
- `SENNET_AGENT_LOG_LEVEL`: Override log level
- `SENNET_FEATURES_FLOW_TRACKING`: Enable/disable flow tracking

## Reloading Configuration

To apply changes, restart the sennet service:

```bash
sudo systemctl restart sennet
```
