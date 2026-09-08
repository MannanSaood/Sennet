# API and event contracts

All application endpoints require a bearer credential. Roles are `ingest`, `reader` or `admin`. A tenant is derived from the verified credential and overrides any supplied tenant field.

| Endpoint | Function |
|---|---|
| GET /api/session | Verified tenant, subject and role |
| GET /api/agents | Tenant-scoped collector inventory |
| GET /api/events | Bounded event search and cursor pagination |
| POST /api/events | Durable event batch ingestion |
| POST /v1/traces, /v1/logs, /v1/metrics | OTLP/HTTP JSON or protobuf |
| GET/POST/DELETE /api/keys | Admin key metadata, creation and revocation |
| GET/POST/DELETE /api/dashboards | Saved investigation views |
| GET/POST/DELETE /api/alerts | Internal error-count monitors |
| GET /api/finance/reconciliation | Bounded transaction sequence inspection |

Event queries accept `from` and `to` as epoch milliseconds, `signal`, `service`, `trace_id`, `search`, `limit` (1–1000), and `cursor`. Windows must not exceed 30 days. Returned `next_cursor` indicates more results. Charts derived from a page describe that page, not the full data set.

A domain event requires a stable `id`, `time`, `signal`, `service`, `name`, `status`, `duration_ms`, `value` and string-valued `attributes`. Supported signals: trace, log, metric, flow, agent and finance. Trace/span/parent IDs enable causality. Retried events retain both ID and timestamp. Batches contain 1–1000 records and at most 4 MiB decoded bytes.

Finance events require transaction_id, state, a three-letter currency, and an exact decimal amount string in attributes. Sequence and source identifiers support gap inspection. Never encode money as binary floating point. The system is not an accounting ledger.

OTLP metrics currently accept gauge, sum and explicit histogram records. Other metric types return an explicit unsupported error. Collector queues must preserve unacknowledged data; success means the configured durable boundary accepted the batch, not that a query has already indexed it.
