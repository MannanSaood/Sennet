# Deployment, migration, and recovery

The binary has four explicit roles. `SENNET_ROLE=gateway` serves authenticated HTTP/OTLP ingestion and appends to Kafka. `storage-consumer` reads Kafka, archives raw valid records when configured, writes ClickHouse, and only then commits offsets. `query-control` serves tenant-authorized query/control and dead-letter administration. `all` is the local evaluation role; without Kafka/ClickHouse it uses SQLite and is not a production telemetry path.

## Required configuration

| Variable | Roles | Purpose |
|---|---|---|
| `SENNET_ROLE` | all | `gateway`, `storage-consumer`, `query-control`, or `all` |
| `SENNET_DATABASE_URL` | gateway, query-control, all | PostgreSQL URL; SQLite is allowed only for local `all` |
| `SENNET_KAFKA_BROKERS` | distributed | Comma-separated brokers |
| `SENNET_KAFKA_TOPIC` / `SENNET_KAFKA_DLQ_TOPIC` | distributed | Source and bounded-retention dead-letter topics |
| `SENNET_KAFKA_GROUP` | consumer | Storage consumer group; default `sennet-storage-v1` |
| `SENNET_KAFKA_TLS` / `USER` / `PASSWORD` | distributed | TLS 1.2+ and optional SASL/PLAIN |
| `SENNET_KAFKA_BATCH_EVENTS` / `BATCH_BYTES` / `BATCH_MS` | gateway | Bounded synchronous producer batch controls (500, 1 MiB, 10 ms) |
| `SENNET_KAFKA_WRITE_TIMEOUT_MS` | gateway | Broker acknowledgement deadline; default 10 seconds |
| `SENNET_INGEST_CONCURRENCY` | gateway | Concurrent durable appends; saturation returns 429 |
| `SENNET_CLICKHOUSE_URL` / `USER` / `PASSWORD` | consumer, query | Analytics endpoint and credentials |
| `SENNET_CLICKHOUSE_INIT` | consumer, query | `local` creates evaluation DDL; production must use pre-provisioned `none` |
| `SENNET_ARCHIVE_DIR` | consumer | Optional idempotent local filesystem archive for testing/evaluation |
| `SENNET_OPERATOR_TOKEN` | all | Bearer token for `/internal/metrics`; never expose it to browsers |
| `SENNET_AUTH_MODE` | HTTP roles | `firebase` in production; `development` is allowed only outside production |
| `FIREBASE_SERVICE_ACCOUNT_JSON` / `FIREBASE_SERVICE_ACCOUNT_PATH` | HTTP roles | Firebase Admin credentials used to verify login ID tokens; application-default credentials are also supported |
| `SENNET_DEVELOPMENT_SESSION_TOKEN` / `SENNET_DEVELOPMENT_TENANT` | local evaluation | Fixed local login session and tenant for self-contained tests; forbidden when `SENNET_ENV=production` |
| `SENNET_ALLOWED_ORIGINS` | HTTP roles | Exact browser origins; no wildcard |

`SENNET_ENV=production` rejects SQLite, development login sessions, missing role dependencies, and automatic local ClickHouse schema creation. TLS termination is mandatory outside loopback. Secrets belong in a secret manager.

## Login and tenant identity

Sennet does not issue, list, rotate, or accept user-managed API keys. Users sign in through Firebase Authentication and send the resulting short-lived ID token as the bearer credential. The server verifies that token on every request. A `tenant_id` custom claim selects a shared tenant; otherwise the stable Firebase UID receives a personal tenant. The optional `role` claim may be `admin`, `reader`, or `ingest`; a personal login defaults to administrator of its own tenant.

Gateway and query/control startup fails if Firebase verification cannot initialize. The `development` auth mode is solely for the local Compose/evaluation stack and is rejected in production. `X-Sennet-Signature` and `X-Sennet-Timestamp` are retired API-key-era headers and requests containing either are rejected.

## Kafka contract

The gateway derives `tenant` after authentication; payload tenant values are overwritten. Event IDs and timestamps remain immutable across retries. A request receives success only after synchronous Kafka `acks=all`. A timeout is ambiguous: clients retry with the same IDs. `acks=all` is not replication—production topic defaults in `deploy/kafka-topics-production.sh` require replication factor 3 and `min.insync.replicas=2`, but operators must prove broker placement and failure behavior.

Partition keys preserve only scoped ordering:

- traces/agent spans: `tenant:trace:<trace_id>`;
- finance events: `tenant:<source_id>:<account_id>:<entity_id>`;
- other signals: `tenant:service:<service>`.

There is no global ordering guarantee. Producer batches are bounded by event count, serialized bytes, and time. The consumer exports `sennet_storage_consumer_lag` on the authenticated metrics endpoint.

Create topics explicitly. The evaluation script uses one replica and 72-hour source/7-day dead-letter retention. The production-oriented script defaults to 48 partitions, three replicas, two minimum in-sync replicas, and seven-day bounded retention for both topics. Size these from measured event volume and outage objectives.

## ClickHouse and archive

`deploy/clickhouse/local.sql` is the single-node evaluation schema. `cluster.sql` defines `ReplicatedReplacingMergeTree` local tables and a `Distributed` `sennet_events` table; it requires ClickHouse Keeper plus correct cluster, shard, and replica macros. The application writes and queries `sennet_events`, and `FINAL` collapses duplicate tenant/time/ID rows. These definitions do not establish HA.

The `Archive` interface receives raw source records before offset commit. `FileArchive` writes deterministic topic/partition/offset objects atomically and is only a local test/evaluation implementation. No fake cloud adapter exists. If no archive is configured, Kafka retention and ClickHouse are the only telemetry copies; production object-storage integration remains an explicit deployment gap.

## Dead letters and replay

Poison reasons are `invalid_json`, `missing_tenant`, `invalid_event`, and `record_too_large`. The consumer publishes the dead-letter envelope with `acks=all` before committing the source offset. Original values over half the source batch byte bound are omitted and marked non-replayable.

An administrator's current login session can inspect `GET /api/dead-letter?limit=100` on query/control. Correct a record and replay with:

```text
sennet-replay -url http://query-control:8080 -session-token "$SENNET_SESSION_TOKEN" -id <dead-letter-id> -event corrected-event.json
```

The correction must preserve a known original event ID, event timestamp, and gateway-derived tenant. Repeating replay can append more than once to Kafka, but ClickHouse/SQLite storage remains idempotent by tenant and immutable ID. Replay is at-least-once, not exactly-once.

## Readiness, overload, and shutdown

`/live` reports process life. `/ready` and `/health` check dependencies required by that role. The consumer checks Kafka reachability, ClickHouse, and configured archive. SIGTERM/SIGINT stops HTTP admission, gives active requests up to 20 seconds, cancels consumer fetch/retry loops, and closes Kafka clients.

Gateway queue saturation returns 429 with `Retry-After: 1`; failed/ambiguous broker appends return 503. Both instruct clients to retry the same event IDs. A request above the configured producer byte bound receives 413 and must be split. Authenticated Prometheus metrics include queue capacity/in-flight, accepted/rejected events, producer errors, consumer lag/retries, dead letters, and replay counts.

## Migration

1. Provision PostgreSQL, source/dead-letter topics, and ClickHouse DDL before starting roles.
2. Deploy query/control and validate tenant-scoped reads against a shadow ClickHouse table.
3. Deploy the storage consumer with a new consumer group; verify archive objects, duplicate collapse, lag, and poison handling.
4. Configure Firebase login verification on gateway/query roles, sign in, and send HTTP plus OTLP fixtures with the login ID token.
5. Retry an intentionally ambiguous request with identical IDs and compare Kafka, archive, and ClickHouse counts.
6. Switch ingestion, retain rollback routing, and keep the previous databases read-only until restore/replay drills pass.

Do not point production telemetry at SQLite. Changing an ID's timestamp creates a different ClickHouse ordering key and violates the contract.

## Recovery gates

Test consumer termination after analytics append but before offset commit, broker replica loss, ClickHouse/archive unavailability, dead-letter outage, login-session expiry/revocation, full archive volume, replay, backup restore, and retention exhaustion. Docker Compose is a single-node smoke topology. Multi-broker Kafka, ClickHouse Keeper/replicas, sustained load, backup restore, and failure-domain tests remain required before any availability claim.
