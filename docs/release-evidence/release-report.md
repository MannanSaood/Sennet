# Integration release decision — 2026-09-14

Decision: **not production ready**. The integrated branch is locally ready for backend, SDK, and bounded SQLite evaluation. An evaluation deployment is blocked on a reproducible Compose run because Docker Desktop did not expose a daemon on this host. Production promotion is blocked by unexecuted distributed failure/restore evidence, Linux collector validation, unresolved finance governance requirements, and current browser-suite failures.

## Integrated history and contract disposition

The branch starts at `f464029` and preserves the required integration order: distributed data plane (`7b79bec`, `320d69f`, `f63da21`), security control plane (`77ec8cb`, `21b2a6b`, merged by `3a0ce43`), collector (`0da402d`), query/alerting (`bd63e29`), investigation UI (`286330f`, `853f45b`), agentic observability (`6cb477b`), and finance observability (`596997e`, `f464029`). No parallel schema was introduced.

Review rejected two false integration paths. Development-session authentication could not resolve a human after the security merge, and streaming CI omitted the required session environment. The branch now performs an idempotent, development-only human bootstrap and tests workspace isolation. The agent's packet `trace` command synthesized non-Linux events and depended on event maps removed by the portable counter-only collector; it now fails explicitly on every platform instead of presenting fabricated telemetry.

## Measured capacity

The opt-in harness in `backend/platform/release_load_test.go` used SQLite WAL with `synchronous=FULL`, 12,000 records, 70% tenant skew, eight tenants plus the skew tenant, 1,200 services, 12,000 unique attribute values, 128/512/2,048-byte attribute payload classes, batches of 200, and eight concurrent query workers.

| Measure | Result |
|---|---:|
| Accepted / durable / stored | 12,000 / 12,000 / 12,000 |
| Rejected | 1 intentionally invalid record |
| Duplicate attempts / duplicate rows | 600 / 0 |
| Lost | 0 |
| Sustained ingest | 1,318.43 events/s |
| Ingest batch p95 / p99 | 437.312 / 859.676 ms |
| Query p95 / p99 | 4,363.13 / 6,894.004 ms |
| Heap allocation delta | 816,970,968 bytes |

These are local SQLite measurements only. Kafka/ClickHouse accepted, durable, stored, duplicate, rejected, and lost counters were not measured because the Docker daemon was unavailable. No distributed throughput or high-availability claim is authorized.

## Reliability disposition

Unit/in-process evidence passes gateway ambiguous append retry, duplicate delivery, consumer restart before offset commit, poison dead-letter and corrected replay, storage outage offset retention, credential expiry/replay/clock-skew boundaries, and archive idempotence. A fresh SQLite close/copy/open restore test preserves the expected scoped event.

Gateway process crash against a real broker, broker outage, ClickHouse outage, real consumer kill/restart, real poison topic flow, filesystem disk-full behavior, replica loss, Kafka retention exhaustion, and distributed backup restoration remain untested. Unit doubles are not promoted to deployment evidence.

## Environment

- Intel Core i7-8665U, 8 logical processors; Windows amd64.
- Go 1.25.1, Rust 1.90.0/Cargo 1.90.0, Node 22.13.0/npm 11.3.0, Python 3.13.1.
- Docker CLI 29.7.2 and Compose 5.5.0 were installed; no Docker engine pipe became available.
- Evaluation Kafka configuration: 12 partitions, replication 1, minimum ISR 1, 3-day source retention and 7-day dead-letter retention.
- Production template: 48 partitions by default, replication 3, minimum ISR 2, 7-day source/dead-letter retention.
- ClickHouse local/cluster DDL: 30-day TTL, ReplacingMergeTree/ReplicatedReplacingMergeTree; payload compression is engine-default and was not measured. OTLP exporter uses gzip.
- Resource cost was not measured; no cloud deployment ran.

## Classification

| Class | Items |
|---|---|
| Locally ready | Backend unit/vet/build, Python SDK, agent unit/clippy (warnings remain), SQLite durability/load/restore, dependency audit at zero known advisories |
| Evaluation deployment ready | Source/configuration foundation only; promotion blocked until Compose streaming smoke passes on a running daemon |
| Production blocker | Distributed chaos and restore evidence; latest browser suite 6 pass/3 fail/1 skip; Linux/eBPF runtime matrix; finance audit/retention/source-completeness controls; no production object archive; dependency topology and HA unproven |
| Untested | Real gateway crash, broker/storage outage, replica loss, disk full, Kafka replay/retention exhaustion, ARM64 runtime, privileged eBPF attach, deployment cost |
| Deferred capability | Packet/drop/process tracing, signed auto-upgrade with rollback, notification provider, distributed query admission/monitor lease, production object archive, finance ledger/compliance decisions |

## Next highest-value milestone

Run the exact Compose stack on a stable Linux Docker host, add automated fault injection around Kafka and ClickHouse, and capture stage-separated counters plus resource telemetry throughout recovery. This closes the largest evidence gap shared by durability, capacity, and evaluation-deployment readiness.
