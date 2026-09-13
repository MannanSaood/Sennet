# Query execution and alerting workstream

Status: implemented bounded foundation. This is not an arbitrary query language or a claim of production capacity.

## Versioned query contract

`POST /api/query` accepts contract version `v1`. Every request must specify a trusted authenticated workspace (derived by the server), `signal`, `from`, `to`, one fixed operation, and explicit `max_rows`, `max_bytes`, `max_cpu_ms`, and `timeout_ms` budgets. The maximum range is 30 days, CPU/wall deadline is 15 seconds, result series are capped at 300 buckets, grouping is capped at two allow-listed dimensions, and concurrency uses the query role's bounded admission channel. The supported signals are traces, logs, metrics, flows, agent events, and finance events. Supported operations are `count`, `sum`, `avg`, `min`, `max`, `rate`, `histogram`, `p50`, `p95`, and `p99` subject to signal semantics.

This is a typed allow-list, not SQL, PromQL, or user-provided code. Tenant predicates and time predicates are always inserted by the server. Trace numeric operations use duration. Metric, flow, and agent numeric operations use the event value. Rate requires one named metric counter. Counter samples are event-time sorted and negative deltas are treated as resets. Duplicate event IDs collapse at storage. Histograms return exact nearest-rank p50/p95/p99 for the bounded input. `compare_previous` evaluates the immediately preceding equal-duration window. `step_ms` controls server downsampling; zero selects a step producing at most 300 points. Up to three trace/event exemplar IDs may be requested per point.

Logs and finance events support counts only. In particular, finance decimal amounts remain strings in event attributes and are never converted to floating point by this query contract.

Responses include `rows_scanned`, estimated decoded `bytes_scanned`, elapsed time, effective step, `partial`, and stable partial reasons (`row_budget`, `byte_budget`). Cancellation propagates through HTTP context and database/ClickHouse requests. The ClickHouse adapter applies its existing 10,001-row and 16 MiB server caps in addition to request budgets; a cursor becomes explicit row-budget partial metadata.

An optional bounded `formula` applies one numeric transform (`A`, `A+n`, `A-n`, `A*n`, or `A/n`) to the operation result; arbitrary code, multiple source queries, functions, and division by zero are rejected. Histogram points include ten bounded equal-width buckets with explicit upper bounds and counts in addition to exact p50/p95/p99 values. Formula transforms also apply to bucket bounds.

Investigation-specific reads are authenticated and tenant scoped. `/api/topology` scans at most 100,000 trace records, aggregates service edges before cursor pagination, and returns stable partial metadata. `/api/trace` and `/api/correlations` scan at most 10,000 records for one trace ID and return canonical critical-path, span-link, missing-parent, late-span, fan-in/fan-out, clock-skew, related-log, and related-metric fields. Dashboard writes create immutable snapshots retrievable through `/api/dashboard-versions`. Admin-only `/api/pipeline-health` returns real deployment counters, while `/api/notification-status` returns tenant-filtered outbox acknowledgement counts without claiming a provider is configured.

## Monitor and SLO contract

Monitor definitions are version `v1`. Threshold monitors count error events in a five-minute window. SLO monitors define an availability target and rolling compliance period, with two paired burn-rate conditions. Defaults are 5m/1h at 14.4x for fast burn and 30m/6h at 6x for slow burn. Each pair must breach together before firing, limiting sensitivity to isolated short spikes.

Evaluation runs in `MonitorEvaluator`, independently of API request handlers. The scheduler aligns windows to 30-second boundaries and applies a per-monitor timeout. Definition persistence, evaluation history, current state, and notification outbox are durable SQL tables. An evaluation key hashes workspace, full monitor definition, and window end. Inserting that key, changing state, and enqueueing a transition occur in one transaction. Replaying a window or restarting before re-evaluation therefore cannot enqueue a duplicate transition.

`NotificationOutbox` exposes bounded pending reads and acknowledgement. No email, chat, paging, or webhook provider is implemented and the backend never claims a notification was sent. A future dispatcher must acknowledge its provider before calling `MarkDelivered`.

Empty SLO windows are healthy with zero burn rather than fabricated traffic. Query errors leave durable state unchanged and increment monitor error telemetry.

## Internal telemetry

The protected Prometheus endpoint reports gateway durable acceptance/rejection and saturation, Kafka producer errors, consumer lag/retries, analytics storage write latency, storage queue age, query capacity/in-flight/rejections/cancellation/partials, scanned rows/bytes, query CPU and wall time, dead-letter activity, monitor evaluations/errors/transitions, and monitor scheduling delay. Existing ingress rejection totals remain aggregate; stable per-reason label cardinality is intentionally deferred until the rejection taxonomy is centralized.

## Verification and limits

Fixtures cover event-time ordering, counter resets, duplicate IDs, exact quantiles, empty windows, tenant skew and partial results, cancellation, finance precision guards, SLO burn, repeated evaluation, restart recovery, and outbox deduplication. Backend race tests and vet are release checks.

Local SQLite benchmark on an Intel i7-8665U (Windows amd64, five measured iterations, 10,000 rows, exact p95): 134.8 ms/op, 22.2 MB allocated/op, 220,882 allocations/op. This is a bounded implementation measurement, not a sustained-throughput or production ClickHouse claim.

Remaining limits:

- Query admission is per process, not distributed across query replicas.
- CPU cost is measured process elapsed work, not database engine CPU attribution; bytes are decoded payload estimates in local mode.
- ClickHouse analytics currently materializes a bounded event page in the query process before aggregation. Production scale requires native typed columns/materialized views and distributed cost enforcement.
- Histograms are exact within the bounded scan, not mergeable long-term sketches. Long-range rollups and retention tiers remain future work.
- Monitor scheduling claims at most 100 definitions per tick and has no distributed lease. Replay safety is durable, but only one scheduler should own a workspace until leasing is added.
- Notification dispatch providers, retry policy, routing, and dead-lettering remain outside the interface implemented here.
