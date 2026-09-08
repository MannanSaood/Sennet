# Sennet observability

Investigate traces, logs, metrics, network flows, multi-agent runs and financial events from one tenant-scoped workspace.

The current application provides real ingestion and querying, time/search controls, trace waterfalls, dependencies derived from observed relationships, saved views and access-key management. All empty, error and partial states remain visible. There is no automatic sample-data fallback.

Start with the [quickstart](/docs/quickstart), then [API contracts](/docs/api) and [architecture](/docs/architecture).

The streaming deployment separates PostgreSQL metadata, Kafka ingestion and ClickHouse analytics. Local SQLite is for development. Production capacity is workload-dependent and has not been established by the repository's unit tests.
