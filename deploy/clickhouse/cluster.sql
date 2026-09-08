-- Requires ClickHouse Keeper, a named cluster macro, and shard/replica macros.
-- This is a topology definition, not proof of high availability.
CREATE TABLE IF NOT EXISTS sennet_events_local ON CLUSTER '{cluster}'
(
    tenant String,
    id String,
    time_ms Int64,
    signal LowCardinality(String),
    service LowCardinality(String),
    trace_id String,
    payload String
)
ENGINE = ReplicatedReplacingMergeTree('/clickhouse/tables/{shard}/sennet_events_local', '{replica}')
ORDER BY (tenant, time_ms, id)
TTL toDateTime(time_ms / 1000) + INTERVAL 30 DAY;

CREATE TABLE IF NOT EXISTS sennet_events ON CLUSTER '{cluster}'
AS sennet_events_local
ENGINE = Distributed('{cluster}', currentDatabase(), sennet_events_local, cityHash64(tenant, id));
