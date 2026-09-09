# Distributed data plane workstream

Status: implemented foundation; high availability and production object storage are not proven.

## Changed contracts

- Process selection is explicit through `SENNET_ROLE`: `gateway`, `storage-consumer`, `query-control`, or local `all`.
- The gateway remains the tenant trust boundary. It authenticates the control-plane credential classes, resolves explicit organization/workspace ownership and roles, overwrites untrusted tenant values, and keeps HTTP event and OTLP/HTTP transports.
- The merged security-control-plane contract supersedes this workstream's earlier login-only interim design. Human sessions, integration keys, collector enrollment credentials, and workload identities are distinct; secret material is returned once, collector credentials require versioned signing, and Firebase verification remains fail-closed through stored membership and role resolution.
- Streaming success means synchronous Kafka `acks=all`, not ClickHouse persistence. An ambiguous broker response is retried with the identical event IDs.
- Storage delivery is at least once. The consumer commits a fetched batch only after configured archive writes, ClickHouse writes, and poison dead-letter writes all succeed.
- Event IDs and timestamps are immutable. ClickHouse uses tenant/time/ID replacement keys and query-time `FINAL`; repeat delivery/replay is storage-idempotent but not a claim of exactly-once Kafka delivery.
- Partitioning is trace-affine for traced signals, source/account/entity-affine for finance, and tenant/service-affine otherwise. Only records sharing a partition key are ordered.
- Producer admission is bounded. Saturation is a retryable 429; broker/durability failure is a retryable 503. Neither returns success.
- Poison events move to a bounded-retention dead-letter topic with deterministic source-position IDs and reason codes. Publishing the dead letter is itself part of the source-offset commit boundary.
- Dead-letter inspection/replay requires an owner or administrator identity in the scoped workspace. Corrected replay preserves known tenant and event ID. Repeated replay is safe at storage.
- `/live` is process liveness. `/ready`/`/health` are role-specific dependency readiness. `/internal/metrics` requires `SENNET_OPERATOR_TOKEN`.
- `Archive` is an extension interface. Only an atomic, idempotent local filesystem implementation exists; there is no cloud-shaped fake.
- `SENNET_CLICKHOUSE_INIT=local` creates the evaluation table. Production uses pre-provisioned local replicated and distributed tables with `SENNET_CLICKHOUSE_INIT=none`.

## Failure matrix

| Failure | Retained | Retried/delayed | Rejected | Lost |
|---|---|---|---|---|
| Gateway validation/login failure | Nothing appended | Client signs in or refreshes the session, then retries | 400/401/403 | Invalid request only |
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
- `go test -race ./...`: pass on Windows with portable WinLibs GCC 16.2.0 explicitly selected as Go's C compiler. The standalone archive SHA-256 was `C1F52294597C0B73786B2A78EB5D176D89226D2F21875EAB75E783A8B1CEFCC4`.
- Docker Desktop 29.7.2 Compose stack: pass after adding explicit one-shot ownership initialization for the Kafka and filesystem archive named volumes.
- Streaming smoke: pass with 300 events acknowledged by Kafka and subsequently queryable from ClickHouse through the query role.
- Kubernetes validation clients: kubectl 1.37.0 and kind 0.32.0.
- Kubernetes manifests for all four roles render with Kustomize and pass server-side dry-run against a disposable kind 0.32.0 cluster (Kubernetes 1.36.1). External data services were not deployed in kind, so no Kubernetes workload availability result is claimed.

## Known limitations

- No production cloud object-storage adapter is included.
- Dead-letter inspection scans a bounded number of topic records and is intended for operator repair, not bulk analytics.
- Consumer lag is the active reader's reported aggregate and has no per-partition history.
- Broker readiness is a bounded network reachability check; append acknowledgements remain the actual durability signal.
- The provided cluster DDL requires deployment-specific ClickHouse Keeper and macros and has not established HA.
- No sustained throughput, regional failover, restore-time, or retention-exhaustion result is claimed.
- The existing Rust agent was intentionally not changed in these workstreams. The control plane now provides short-lived collector enrollment and workload credentials, but wiring that enrollment/rotation protocol into the agent remains follow-up work.

## Migration notes

Existing all-in-one streaming deployments must split credentials and ports by role, pre-create the dead-letter topic, deploy ClickHouse DDL, and keep the existing `sennet-storage-v1` group only when continuing its offsets is intentional. Run a new group for shadow validation. Follow the explicit ownership migration in `security-control-plane.md`; unmapped legacy records are quarantined instead of assigned globally. `INIT_API_KEY` is bootstrap-only, while normal clients use the credential class appropriate to humans, integrations, collectors, or workloads. Gateway clients keep their HTTP/OTLP endpoints and immutable IDs across 429/503 retries. Operators must protect the metrics token separately from application identities.
