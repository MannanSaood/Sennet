# Architecture

SDKs and Sennet agents send telemetry through an authenticated ingestion gateway. In streaming mode, the gateway waits for Kafka acknowledgement; a consumer batches writes into ClickHouse and commits offsets afterward. Replayed immutable IDs are deduplicated during queries. PostgreSQL stores credentials, collector inventory and saved settings.

Queries require tenant scope and bounded time ranges, row limits and deadlines. The browser receives paginated observations. Local evaluation can use SQLite with WAL and full synchronization instead of external services.

OpenTelemetry collectors add batching, retries, memory limits and persistent export queues. The Rust agent also keeps a bounded disk outbox. Monitoring data must explicitly distinguish unavailable collection, delayed export and zero observed traffic.

The included compose topology is single-replica evaluation infrastructure. Multi-region capacity, distributed query admission, archive restore and high-availability guarantees require additional deployment validation.
