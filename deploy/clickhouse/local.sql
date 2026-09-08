CREATE TABLE IF NOT EXISTS sennet_events
(
    tenant String,
    id String,
    time_ms Int64,
    signal LowCardinality(String),
    service LowCardinality(String),
    trace_id String,
    payload String
)
ENGINE = ReplacingMergeTree
ORDER BY (tenant, time_ms, id)
TTL toDateTime(time_ms / 1000) + INTERVAL 30 DAY;
