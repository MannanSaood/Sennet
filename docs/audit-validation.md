# Validation after the platform rebuild — 2026-09-08

| Check | Result | Scope and limits |
|---|---|---|
| Backend `go test ./...` | PASS | New tenant, key lifecycle, durability, OTLP, aggregation and financial precision tests plus legacy regression suites. |
| Backend `go vet ./...` | PASS | Static checks; not a penetration test. |
| Backend final binary build | PASS | Windows local evaluation server. |
| Agent `cargo test --locked` | PASS: 42 passed, 1 ignored | Windows execution; Linux-only paths unexecuted. Existing unused/dead-code warnings remain. |
| Python SDK `python -m unittest -v` | PASS: 4 tests | Nested context, durable retry, exact decimals, preservation of application exceptions. |
| Web `npm run lint` | PASS | All prior lint errors resolved. |
| Web `npm run build` | PASS | Route splitting reduced main chunk from 1,571.80 kB to 561.67 kB; main/docs still exceed Vite's 500 kB warning. |
| Local HTTP smoke | PASS | 300 fresh evaluation events acknowledged, queried and aggregated; cursor pagination and financial sequence inspection checked. SQLite mode, not Kafka validation. |
| Restart persistence | PASS | Previously ingested evaluation data remained after local backend restart. |
| Browser authentication and dashboard | PASS | Final server: 300 events, four services, seven errors, 480 ms span p95. Evaluation fixture only. |
| Browser search | PASS | GET /quotes filter yielded 60 events, one service, zero errors, 42 ms p95. |
| Browser trace drilldown | PASS | Four linked spans, durations, shared trace identifier and attributes rendered. |
| Browser console | PASS for exercised flow | No error entries captured in the isolated verification tab. |
| Responsive width check | PASS for measured viewport | At 390 px viewport the document/body widths were 375 px; no horizontal document overflow. Not exhaustive device/accessibility testing. |
| `git diff --check` | PASS | Line-ending normalization warnings only. |

The final regression run exposed nanosecond timestamp rounding in OTLP JSON normalization; the decoder now preserves JSON numbers and the exact-duration regression passes. Empty OTLP attributes are handled safely. Heartbeat inventory no longer duplicates the agent's durable telemetry outbox.

Not executed: Docker PostgreSQL/Kafka/ClickHouse integration (Docker unavailable on this host), CI jobs, race detector, privileged Linux/eBPF/TUI runtime and ARM64 tests, real cloud providers, sustained mixed-load benchmarks, chaos/failover, backup restore, external notifications, independent penetration testing or financial source reconciliation. CI definitions include backend race/vet, Linux agent, web, SDK and streaming smoke checks, but definitions are not execution evidence.

The original audit is historical. See [implementation-status.md](implementation-status.md) for every finding's current disposition and release gates.

## Integration release evidence — 2026-09-14

The integration run supersedes stale counts above. Backend tests passed before the final development-auth reconciliation and were rerun on the final tree. Python SDK: 6 passed. Agent: 45 passed, 1 explicitly ignored; clippy completed with warnings. npm dependency audit initially found 45 vulnerable dependency nodes (2 critical) and, after non-forced lockfile remediation, reports 0. The latest browser run produced 6 passed, 3 failed, 1 skipped; failures are not recorded as passes. The production web build passed and its largest uncompressed chunk was 368.42 kB. Docker/Compose was not executed because Docker Desktop exposed no engine pipe. WSL Linux race execution was unavailable because no Linux distribution is installed; Windows race was unavailable because `gcc` is absent.

The release harness passed with 12,000 accepted/durable/stored, 1 rejected, 600 duplicate attempts, 0 duplicate rows, and 0 lost. Local SQLite sustained ingestion was 1,318.43 events/s; batch p95/p99 was 437.312/859.676 ms and eight-query p95/p99 was 4,363.13/6,894.004 ms. This is local evaluation evidence only. See [release-evidence/release-report.md](release-evidence/release-report.md) for environment, limitations, reliability disposition, and release decision.
