# Finance observability vertical slice

Status: implemented foundation; domain review required. This is operational observability, not a ledger, payment processor, accounting system, tax engine, or regulatory-compliance claim.

## Reference lifecycle and event contract

The reference lifecycle is payment `authorized` → `captured` → `settled`. Events use `finance.schema=sennet.finance.payment.v1` and carry a stable event ID, exact `amount_minor` integer or bounded exact decimal `amount`, three-letter currency, `source_id`, `transaction_id`, positive source `sequence`, event time, `receive_time_ms`, unsigned `correction_version`, provider, account, and state. Workspace identity is derived from authentication and is not trusted from event JSON. Provider and account identify an operational stream; account is part of every derived-state key.

The first reconciliation rule is `payment-auth-capture-set-1`. For each sequence the highest correction version is authoritative. Its conclusions cover duplicate sequence/version pairs, gaps, invalid transitions, corrections, receive-order inversions, events received more than 15 minutes after event time, and settlements without an observed authorization/capture. Every conclusion has a deterministic key over rule version, stream, transaction, kind, and sorted event IDs. It preserves contributing event IDs as provenance.

Source completeness records the highest sequence plus event-time and receive-time watermarks for each workspace/source/account. A watermark reports what was observed; it does not prove the upstream source is complete. Reconciliation is a deterministic derived view and can change when late or corrected events arrive.

## Bounded query contracts for UI consumers

All routes require the existing telemetry-query permission and mandatory bounded time parsing. Processing stops at 1,000 events and returns `partial=true` if more data exists.

- `GET /api/finance/timelines` (the compatibility alias is `/api/finance/reconciliation`)
- `GET /api/finance/queue`
- `GET /api/finance/latency`
- `GET /api/finance/provider-errors`
- `GET /api/finance/dependency-impact`

Responses include schema/rule versions and the non-ledger boundary. Latencies are event-time p50/p95 observations, not provider SLAs. Provider errors and dependency impact require the adapter to set `provider_error` and `dependency`; absence means “not observed,” not healthy.

## AWS Cost Explorer provider

AWS is the only enabled cloud-cost provider. Azure and GCP creation fails explicitly. The provider calls `GetCostAndUsage` over HTTPS using AWS Signature Version 4, daily granularity, `UnblendedCost`, service grouping, and a mandatory `LINKED_ACCOUNT` filter. It follows `NextPageToken`, retains AWS request IDs, records the configured workload identity ID, and persists each page and checkpoint transactionally. Repeating a period replaces the same workspace/provider/source/account/day/service/kind observation instead of adding it. Different accounts cannot overwrite one another.

Required runtime setup:

- Cost Explorer enabled for the payer/management account.
- IAM permission `ce:GetCostAndUsage` scoped through the deployment's workload identity/assumed role policy.
- Metadata: 12-digit linked account, region, source ID, platform workload identity ID, and optional role ARN.
- Short-lived credentials supplied at runtime through `AWS_ACCESS_KEY_ID`, `AWS_SECRET_ACCESS_KEY`, and optional `AWS_SESSION_TOKEN`. They are not fields in persisted JSON or browser contracts. Production should inject these from its workload identity/secret manager; this slice does not implement STS role assumption.

Cost amounts remain exact decimal text. `charge_kind` distinguishes `observed` from AWS `estimate`. Allocation records separately distinguish `observed_charge`, `estimate`, `unallocated_total`, and `allocation`, and include rule ID/version, target, and source keys. Allocation rules are storage contracts only; no allocation policy is presumed.

## Fixture assumptions and review boundaries

Fixtures assume one currency and invariant amount per payment transaction, source sequences start above zero, higher correction versions supersede lower ones at the same sequence, AWS `UnblendedCost` units are currency codes, and Cost Explorer's `Estimated` flag is authoritative for observed-versus-estimate labeling. They test exact values beyond IEEE-754 safe integer precision, pagination, account filtering, lifecycle anomalies, deterministic replay, and account isolation.

Payments/domain review must decide whether authorization increments, partial/multiple captures, partial/refunded/reversed settlements, retries, expiration, disputes, FX, zero/negative amounts, and processor-specific terminal states belong in a future rule version. FinOps review must choose amortized versus unblended cost, credits/refunds/taxes treatment, billing-data finalization windows, organization/payer authorization, allocation policy, and unallocated-total invariants. Security review must validate workload federation and credential rotation. Audit/legal review must set retention and evidentiary requirements. None of these fixtures establishes GAAP/IFRS correctness, tax treatment, PCI scope, settlement finality, or regulatory compliance.
