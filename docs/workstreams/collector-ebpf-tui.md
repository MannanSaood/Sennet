# Collector, eBPF, exporter, upgrade, and TUI workstream

## Supported collection matrix

| OS | Architecture | Kernel | Collection | Evidence |
|---|---|---|---|---|
| Linux | x86_64 | 5.8+ | TC ingress/egress packet and byte counters | Compiled in CI; privileged verifier/attach test still required |
| Linux | aarch64 | 5.8+ | TC ingress/egress packet and byte counters | Cross-compiled in CI; privileged arm64 attach test still required |
| Linux | other | any | Unsupported | Startup capability error |
| non-Linux | any | any | Unsupported | Unit test proves no zero-valued fallback |

The host must provide BPF syscalls, a mounted bpffs at `/sys/fs/bpf`, `clsact`,
and sufficient privilege. Container deployments normally need `CAP_BPF`,
`CAP_NET_ADMIN`, and (depending on kernel policy) `CAP_PERFMON` or root. A
distribution may disable required BPF features even on a nominally supported
kernel; load failure is explicit and includes the verifier/loader error.

## Portability and truthfulness

The shipped BPF object reads only `TcContext::len()`. The former `kfree_skb`,
netfilter, and socket kprobes used guessed context/structure offsets and were
removed. Drops and process-attributed flows therefore report `unsupported`;
they are not inferred or replaced with synthetic traffic. Reintroduction
requires generated CO-RE relocations plus verifier/fixture evidence on every
supported kernel/architecture cell.

## Recovery and observability

One daemon owns `/sys/fs/bpf/sennet` under `/run/sennet.lock`. Maps are staged
under a process-specific pin and renamed into place. Pin/load errors are fatal
to collection and stale optional pins are not interpreted by the current BPF
object. Privileged crash-between-pin-and-rename, incompatible-map, and bpffs
restart tests remain mandatory before production rollout.

The durable spool is capped at 20 MiB. Full-disk and cap failures increment
explicit loss and log the failed observation; map-read failures set collection
to unavailable. `exporter-health.json` is atomically replaced and reports queue
bytes, oldest record age, retries, rejected records, explicit loss, last
successful export, and collection status/error. Shutdown permits a 15-second
drain and retains remaining durable records.

Configuration reload validates the complete candidate and constructs its HTTP
client before swapping it in; failure retains the old configuration. Original
arguments and inherited environment are retained by `exec` restart.

## Upgrade boundary

Unattended upgrade remains disabled unless `SENNET_ALLOW_AUTO_UPGRADE=true`.
The current updater verifies a release SHA-256 checksum but does **not** yet
verify a public-key signature or run a post-restart health handshake. Treat the
manual `upgrade` command as staged-but-not-production-approved until signed
manifests, rollback, and health-confirmation tests are added.

## TUI and tests

The TUI provides Overview, Flows, Drops, and Exporter tabs with numeric,
arrow/h-l navigation, pause, resize handling, compact minimum-size messaging,
and q/Esc/Ctrl-C exit. Unsupported tabs say so explicitly. `--json` exposes the
same four surfaces. Terminal cleanup uses an RAII guard; privileged Linux PTY,
panic-unwind, and fixture snapshot tests remain unexecuted in the Windows
development environment.

## Operational limits

- Counters are cumulative per daemon/map lifetime and reset after map replacement.
- No IPv4/IPv6 tuple, process attribution, or kernel drop reason is collected.
- Ring-buffer loss is not applicable because the portable object has no ring buffers.
- The outbox sends at most 20 files or 10 seconds of work per pass.
- Delivery is at least once; an ambiguous HTTP failure can produce duplicates.
- Rejected records are quarantined locally and are not retried automatically.
