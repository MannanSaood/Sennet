# Agent commands

- `sennet start`: foreground collector daemon (also the default with no command).
- `sennet init`: configure a local agent interactively.
- `sennet status`: inspect local state.
- `sennet top`: real Linux traffic rates, history and recent drop observations; q/Esc exits, p pauses.
- `sennet top --json`: exact counter strings for scripts.
- `sennet flows`, `sennet trace`, `sennet diagnose`: Linux/Kubernetes diagnostics; kernel-dependent probes are experimental and require environment validation.
- `sennet upgrade`: explicit release update. Unattended updates are disabled by default.

Live eBPF data is unavailable on Windows. The TUI fails clearly instead of substituting mock traffic. Optional kernel probes using layout-dependent fields require `SENNET_EXPERIMENTAL_KERNEL_PROBES=true` and verification on your kernel.
