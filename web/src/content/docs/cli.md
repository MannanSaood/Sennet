# CLI Reference

The `sennet` command line interface is your primary tool for managing the agent and viewing live stats.

## Commands

### `init`
Initializes the agent configuration.
```bash
sudo sennet init
```

### `top`
display top processes and flows sorted by bandwidth usage (like `htop`).
```bash
sudo sennet top
```
**Flags:**
- `-i, --interface`: Select interface (default: auto)
- `--sort`: Sort by `rx`, `tx`, or `total`

### `status`
Show the current health and connection status of the agent.
```bash
sudo sennet status
```

### `inspect`
Dump raw eBPF map data for debugging.
```bash
sudo sennet inspect --map flows
```

### `version`
Print version information.
```bash
sennet version
```
