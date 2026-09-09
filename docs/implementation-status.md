# Platform rebuild status — 2026-09-08

This supersedes the implementation-status column in the original engineering audit. The original file remains a record of the baseline findings. This is a substantial working foundation, not evidence of Datadog feature parity or production capacity.

## What now runs

The backend entry point now uses `backend/platform`, with authenticated tenant-scoped control and query operations. SQLite is the local evaluation store; PostgreSQL holds control metadata in the streaming deployment. Telemetry can pass through Kafka to ClickHouse. Ingestion acknowledges a database commit locally or Kafka's configured broker acknowledgement in streaming mode. Consumers commit offsets only after an analytics write; retries retain event identity. HTTP remains the transport/control interface—removing APIs would not itself solve durability or scale.

OTLP/HTTP accepts JSON and protobuf traces, logs, gauge/sum/histogram metrics, with gzip and bounded requests. Unsupported metric types fail explicitly. The event contract supports agent spans and financial lifecycle events. The Python SDK propagates nested trace context and persists an outbox; the Rust collector persists bounded metric observations before export. Heartbeats update inventory separately from telemetry export.

The website and workspace have been rebuilt around real queries: historical search, range filters, pagination, full-window counts/p95/time buckets, service distribution, trace waterfalls, dependency inspection, agent inventory, financial sequence inspection, saved views and internal threshold monitors. The active backend no longer exposes user-managed API-key operations; access is through verified login sessions. Empty and failed collection are explicit. The TUI displays real counters, measured rates and bounded sparklines, with pause and terminal cleanup; unsupported platforms do not fabricate traffic.

## Audit disposition

“Implemented” refers to the new entry point and contracts, not to unexposed legacy packages. “Partial” identifies remaining engineering work; “contained” means an unsafe or fake feature was removed from the active surface rather than completed.

| Findings | Disposition | Implemented remedy and remaining boundary |
|---|---|---|
| SEC-01, SEC-02 | Implemented for new platform | Verified Firebase login identity, server-derived tenant, scoped repositories/queries/resources, and cross-tenant tests. Shared organization membership beyond identity claims is not implemented. |
| SEC-03 | Superseded | User-managed key creation, listing, rotation, signatures, and bootstrap seeding were removed from the active platform. Login token expiry and revocation are delegated to Firebase verification. |
| SEC-04 | Partial | Bounded source and tenant rate buckets ignore spoofed forwarding headers. Rates are per process; distributed quotas, proxy identity and large NAT fleet policy require deployment work. |
| SEC-05 | Partial | Raw/decoded limits, gzip limits, deadlines, tenant deduplication, and explicit rejection of retired API-key signing headers. Production still needs authenticated TLS boundaries. |
| SEC-06 | Contained | Legacy cloud configuration routes are unmounted; encryption utility requires exactly 32 bytes. Historical plaintext requires an explicit secret-manager migration. |
| SEC-07 | Partial | Secret prefix logging removed; credential filenames ignored. Historical credential provenance was not investigated. |
| SEC-08 | Partial | Roles, exact CORS origins, mutation audit records; public debug/metrics removed. No complete audit export or platform self-observability yet. |
| DATA-01, DATA-02 | Implemented foundation | Persisted tenant inventory and historical telemetry; SQL/ClickHouse query contracts. Long-term rollups and per-tenant retention policies remain. |
| DATA-03 | Implemented | Invalid identity and persistence failures cannot receive success. Telemetry uses a separate durable acknowledgement. |
| DATA-04 | Partial | OTLP HTTP JSON/protobuf plus domain ingestion. Native backend OTLP/gRPC, exponential histograms and complete span-event semantics remain. Collector config accepts gRPC and forwards HTTP. |
| DATA-05 | Partial | Bounded outboxes and Kafka producer admission, commit-after-archive/analytics/dead-letter, idempotent retries, consumer lag metrics, poison reason codes, authenticated inspection and corrected replay. Only a local filesystem archive exists; cloud object storage and a lag UI remain. |
| AGENT-01, AGENT-02 | Implemented, Linux validation pending | Start alias; redesigned real-source TUI with explicit unsupported platform error. |
| AGENT-03 | Partial | No zero fallback; map-read errors propagate; blocking disk/HTTP off async executor, deadlines, retry retention and quarantine. Export failure counters and shutdown drain tests remain. |
| AGENT-04 | Partial | Exclusive daemon lock and replacement map pins; pin failures propagate. Restart/load rollback still requires privileged Linux tests. |
| AGENT-05 | Contained, unresolved | Layout-dependent probes gated behind explicit experimental opt-in. CO-RE conversion and kernel/architecture matrix are not complete. |
| AGENT-06 | Partial | Validated config reload, preserve restart arguments, unattended upgrades off by default. Signed staged rollout with rollback/health checks remains. |
| COST-01, COST-03 | Contained, unresolved | Fake connection success removed; cloud-cost routes absent. Real provider ingestion, account-scoped cost allocation and reconciliation remain. |
| COST-02 | Partial | Legacy errors propagate; no checkpointed cloud synchronization. |
| UI-01, UI-02, UI-03, UI-05 | Implemented | Real queries, freshness/error states, corrected hooks, one current credential source and cache reset on identity changes. |
| UI-04 | Partial | Working saved views and monitors. Team invitations, billing and notifications are explicitly unavailable; API-key management was intentionally removed. |
| UI-06 | Partial | Range/filter explorers, waterfall, page-derived dependencies, full-window analytics and saved views. Large graph rendering, custom dashboard composition, flamegraphs, metric algebra and anomaly drilldowns remain. |
| OPS-01 | Partial | Independently deployable gateway, storage-consumer, and query/control roles plus local all-in-one; replicated/distributed ClickHouse DDL and explicit Kafka topic defaults are supplied. Multi-node failure, global admission, and HA remain unproven. |
| OPS-02 | Partial | Versioned initial schema and documented safe cutover. Restore, rolling migration, sustained load and disaster recovery are unproven. |
| QA-01 | Partial | Local regression suites and clean web lint/build; CI adds race/vet, Linux agent and streaming smoke jobs. CI infrastructure jobs have not been executed here. |
| DOC-01 | Implemented for new path | Replaced quickstart/deployment/capability docs with actual configuration and limits. |
| DOMAIN-01 | Partial | Linked agent/tool spans, durable Python SDK, exact decimal finance events and bounded sequence inspection. Evaluation pipelines, redaction policy management, accounting reconciliation and regulated retention remain. |

## Release gates still required

1. Run the Compose streaming smoke test in Docker. The included stack has single replicas and twelve Kafka partitions; it is an evaluation topology. Deploy Kafka replication/minimum in-sync replicas and ClickHouse replicated/distributed tables before claiming high availability.
2. Validate the separated roles under sustained load; add distributed tenant quotas, queue-age and query-cost metrics, lag history, and production object-storage archive/recovery.
3. Benchmark measured event sizes, cardinality and concurrent queries. Record sustained accepted/stored throughput, p95/p99 query latency, recovery time, duplicate rate, resource use and cost. No throughput number is claimed from a 300-event smoke test.
4. Run Linux kernel/verifier, restart/pin lifecycle and terminal tests across supported architectures. Replace layout-dependent probes before enabling them by default.
5. Rehearse backup restoration and schema upgrades with scoped historical data. Keep legacy databases offline until ownership mapping and secret migration are approved and tested.
6. Add organization membership, workload federation, configurable retention, sensitive-content policy, security review and immutable audit export.
7. Implement domain-specific finance source completeness and state machines with users' actual schemas. Current inspection is limited to 1,000 events and does not establish ledger correctness or regulatory compliance.

See `audit-validation.md` for executed checks. Unexecuted gates are not passes.
