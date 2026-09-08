# eBPF collection

The Linux agent attaches traffic-control programs for counters and exposes pinned maps to the local TUI. A daemon lock prevents multiple processes from owning the same pin namespace. Map publication errors fail collection rather than silently reading an old map.

Drop and socket probes use kernel-dependent layouts and are disabled by default. BTF capability detection alone does not establish CO-RE compatibility. Enable experimental probes only after validating the verifier, offsets and event accuracy for your kernel. Windows does not execute eBPF collection.
