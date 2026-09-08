# Distributed data plane workstream

Status: implemented foundation; high availability and production object storage are not proven.

## Changed contracts

- Process selection is explicit through `SENNET_ROLE`: `gateway`, `storage-consumer`, `query-control`, or local `all`.
- The gateway remains the tenant trust boundary. It overwrites untrusted tenant values after authentication and keeps HTTP event and OTLP/HTTP transports.
- Streaming success means synchronous Kafka `acks=all`, not ClickHouse persistence. An ambiguous broker response is retried with the identical event IDs.
- Storage delivery is at least once. The consumer commits a fetched batch only after configured archive writes, ClickHouse writes, and poison dead-letter writes all succeed.
- Event IDs and timestamps are immutable. ClickHouse uses tenant/time/ID replacement keys and query-time `FINAL`; repeat delivery/replay is storage-idempotent but not a claim of exactly-once Kafka delivery.
- Partitioning is trace-affine for traced signals, source/account/entity-affine for finance, and tenant/service-affine otherwise. Only records sharing a partition key are ordered.
- Producer admission is bounded. Saturation is a retryable 429; broker/durability failure is a retryable 503. Neither returns success.
- Poison events move to a bounded-retention dead-letter topic with deterministic source-position IDs and reason codes. Publishing the dead letter is itself part of the source-offset commit boundary.
- Dead-letter inspection/replay requires an admin API credential. Corrected replay preserves known tenant and event ID. Repeated replay is safe at storage.
- `/live` is process liveness. `/ready`/`/health` are role-specific dependency readiness. `/internal/metrics` requires `SENNET_OPERATOR_TOKEN`.
- `Archive` is an extension interface. Only an atomic, idempotent local filesystem implementation exists; there is no cloud-shaped fake.
- `SENNET_CLICKHOUSE_INIT=local` creates the evaluation table. Production uses pre-provisioned local replicated and distributed tables with `SENNET_CLICKHOUSE_INIT=none`.

## Failure matrix

| Failure | Retained | Retried/delayed | Rejected | Lost |
|---|---|---|---|---|
| Gateway validation/auth failure | Nothing appended | Client may correct and retry | 400/401/403 | Invalid request only |
| Gateway queue saturated | Client-owned batch | Client retries same IDs after 429 | 429, explicit overload | None if client honors retry |
| Producer byte bound exceeded | Client-owned batch | Client splits the batch | 413 | None |
| Kafka append fails before quorum acknowledgement | Client-owned batch; broker state may be ambiguous | Client retries same IDs after 503 | 503 | None within client retry/retention policy |
| Gateway exits after acknowledged append | Kafka source topic | Consumer proceeds later | None | None within Kafka retention |
| ClickHouse unavailable | Kafka offsets remain uncommitted; archive may already exist | Consumer retries batch | Ingestion continues until broker/admission capacity is exhausted | None within Kafka retention |
| Archive unavailable when configured | Kafka offsets remain uncommitted | Consumer retries; ClickHouse write is delayed | None immediately | None within Kafka retention |
| Consumer exits after ClickHouse/archive write before commit | Kafka plus idempotent archive/analytics rows | Restart redelivers; duplicates collapse by ID | None | None within retention |
| Poison record | Dead-letter envelope and bounded original payload | Valid neighbors wait until dead-letter acknowledgement, then continue | Poison is excluded from analytics | Oversize poison body is intentionally omitted; metadata remains |
| Dead-letter topic unavailable | Source offset remains uncommitted | Entire batch retries | None | None within source retention |
| Replay repeated | DLQ original plus each replay append | Consumer may redeliver | Invalid correction gets 409 | No valid event loss; log may contain duplicates |
| Query/ClickHouse unavailable | Kafka/archive/ClickHouse existing data unaffected | Queries delayed with 503 | Query request | None |
| Kafka retention expires before recovery | ClickHouse/archive copies that completed | No further source replay | New requests may later overload | Unconsumed source records beyond retention |
| Local all-in-one SQLite disk failure | Whatever SQLite committed | Client retries failed transaction | 503 | Subject to local disk durability/backup; not production |

## Verification added

`platform/stream_integration_test.go` covers ambiguous retry after append, duplicate delivery, restart before offset commit, poison quarantine, corrected/repeated replay, idempotent filesystem archive, and unavailable storage retaining offsets. Docker Compose separates the three distributed roles and explicitly provisions both topics. Cluster DDL is supplied but no multi-node failover test is claimed.

Local evidence on 2026-09-08:

- `go test ./...`: pass.
- `go vet ./...`: pass.
- `go test -race ./...`: not executed to completion because this Windows host has no C compiler required by Go's race runtime (`gcc` not found). The Linux CI gate remains configured.
- Compose YAML parse: pass. Docker smoke: not run because Docker is not installed on this host.

## Known limitations

- No production cloud object-storage adapter is included.
- Dead-letter inspection scans a bounded number of topic records and is intended for operator repair, not bulk analytics.
- Consumer lag is the active reader's reported aggregate and has no per-partition history.
- Broker readiness is a bounded network reachability check; append acknowledgements remain the actual durability signal.
- The provided cluster DDL requires deployment-specific ClickHouse Keeper and macros and has not established HA.
- No sustained throughput, regional failover, restore-time, or retention-exhaustion result is claimed.

## Migration notes

Existing all-in-one streaming deployments must split credentials and ports by role, pre-create the dead-letter topic, deploy ClickHouse DDL, and keep the existing `sennet-storage-v1` group only when continuing its offsets is intentional. Run a new group for shadow validation. Gateway clients keep their HTTP/OTLP endpoints and must retain immutable IDs across 429/503 retries. Query clients move to query/control. Operators must protect the metrics token separately from tenant API credentials.
