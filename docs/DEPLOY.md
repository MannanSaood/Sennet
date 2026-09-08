# Deployment, migration and recovery

## Configuration

| Variable | Purpose |
|---|---|
| SENNET_DATABASE_URL | PostgreSQL URL or local SQLite file; default `./sennet-platform.db` |
| INIT_API_KEY | Random bootstrap credential, `sk_` prefix, at least 24 characters |
| SENNET_BOOTSTRAP_TENANT | Explicit bootstrap tenant, default `local` |
| SENNET_BIND / PORT | Bind address (default loopback) and port (8080) |
| SENNET_ALLOWED_ORIGINS | Exact comma-separated browser origins; no wildcard |
| SENNET_CLICKHOUSE_URL / USER / PASSWORD | Analytical backend endpoint and credentials |
| SENNET_KAFKA_BROKERS / TOPIC | Broker addresses and topic (`sennet-events`) |
| SENNET_KAFKA_TLS / USER / PASSWORD | Broker transport TLS and optional SASL |
| SENNET_ENV | `production` requires PostgreSQL, Kafka and ClickHouse |
| FIREBASE_SERVICE_ACCOUNT_PATH | Optional Firebase server identity; each verified UID owns a private tenant |

Credential values belong in a secret manager or deployment environment. Browser variables must never contain ingestion/admin secrets. Frontend Firebase configuration is optional; self-hosted access-key sign-in works without it.

TLS termination is required when serving outside loopback. Trust only configured ingress hosts. The rate limiter deliberately ignores forwarded headers, so a shared proxy uses a shared pre-auth source budget. Place scalable authenticated quotas at the gateway before raising local defaults. Configure Kafka topics explicitly for production (replication factor >=3, minimum in-sync replicas >=2, sufficient partitions and retention). Required-acks alone does not create replication. The example ClickHouse table is a local ReplacingMergeTree; use appropriately configured replicated/distributed tables in production after validating query and replay semantics.

## Migration

The new server uses `platform_*` tables and a separate default SQLite file. It does not expose legacy global key, cost or dashboard endpoints. Existing legacy keys are not silently promoted. If a legacy INIT_API_KEY is reused, it is deliberately seeded as one bootstrap tenant; rotate it immediately and issue scoped credentials.

1. Back up the legacy database and config; preserve existing files.
2. Inventory owners and resolve unowned records explicitly.
3. Deploy the new stack alongside the old service, initially for an evaluation tenant.
4. Issue new ingestion credentials; validate OTLP and application events with a fixture.
5. Compare query results, ownership and event counts before switching each tenant.
6. Retain rollback routing/config and old read-only backups until acceptance.

Schema version 1 is created transactionally. Future migrations must be additive and versioned; do not mutate historical telemetry in place. Event IDs and event timestamps must remain immutable during retry. Kafka delivery is at least once; query-time `FINAL` deduplicates identical tenant/time/ID records in ClickHouse. Changing an ID's timestamp is a new ordering key and violates the event contract.

## Recovery drills

Back up PostgreSQL using your managed database backup system or `pg_dump` (verify restore into a new database). Back up ClickHouse with a supported native/object-storage backup workflow and test restore against counts and representative queries. Preserve Kafka offsets and sufficient retention for recovery; a backup of metadata alone does not back up telemetry. Local SQLite backup must use the SQLite backup API or a stopped database, not a copy of only the live `.db` file while WAL exists.

Test collector disconnect/reconnect, consumer kill after insert before commit, broker replica loss, ClickHouse unavailability, expired credentials, full collector disk, invalid broker record, restore and replay. Invalid broker records currently block processing for operator repair; no records are silently skipped. A production dead-letter/repair workflow remains necessary before unattended high-volume operation.

## Execution boundaries

The compose evaluation topology and privileged Linux eBPF runtime require Linux/Docker. They are not validated merely by passing Windows tests. Sustained ingest/query throughput, multi-region failover, business-specific financial reconciliation and cloud account integrations require their respective environments and acceptance data. See the implementation status for exact evidence.
