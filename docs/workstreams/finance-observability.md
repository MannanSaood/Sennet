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

## Internal domain reviews — 2026-09-13

These are engineering readiness reviews against the current implementation and public primary-source guidance. They are not approvals by a payment processor, qualified security assessor, accountant, auditor, lawyer, or regulator. Overall decision: **no-go for production financial decisions or compliance use; acceptable only as an explicitly labeled evaluation slice**.

### Payments lifecycle review — changes required

The three-state rule is internally deterministic for its fixture but is not a generally valid payment lifecycle. Real payment products can support partial authorization, incremental authorization, partial or multiple captures, overcapture, authorization expiry, cancellation, asynchronous processing, refunds, and reversals. Stripe, for example, documents multiple captures against one authorization and separate captured/capturable amounts; it also distinguishes processing, succeeded, canceled, and refund flows. References: [Stripe flexible payment scenarios](https://docs.stripe.com/payments/flexible-payments), [multicapture](https://docs.stripe.com/payments/multicapture), and [PaymentIntent lifecycle](https://docs.stripe.com/payments/paymentintents/lifecycle).

Release blockers:

- The single invariant transaction amount makes legitimate partial authorization/capture look like an error and cannot reconcile cumulative captured or settled quantities.
- `settled` has no provider-specific definition or settlement source identifier. A payment success event is not necessarily bank/acquirer settlement.
- Retry/attempt identity, authorization expiry, cancellation, failure, refund, reversal, dispute, fee, FX rate, and original-versus-presentment/settlement currency are absent.
- Corrections select a sequence version but do not explicitly retract superseded conclusions. Equal correction versions with conflicting payloads need quarantine, not arbitrary selection.
- A partial 1,000-event query is currently allowed to persist a conclusion. No derived state may be committed when `partial=true`.

Exit criteria: choose one named processor/rail and obtain its event/state mapping; add payment, attempt, capture, settlement, and correction identities; define cumulative amount invariants in minor units with currency exponents; model terminal and compensating events; quarantine conflicting duplicates; and approve provider-specific fixtures with the payments owner.

### FinOps and cloud-cost review — changes required

`UnblendedCost` is a valid Cost Explorer metric but is only one view. AWS distinguishes unblended, net unblended, amortized, and net amortized costs, and explains that Cost Explorer can differ from invoices because of timing, rounding, service grouping, discounts, credits, refunds, and taxes. Current-period data may update later than 24 hours, while invoices remain the amount owed. References: [AWS Cost Explorer metric options](https://docs.aws.amazon.com/cli/latest/reference/ce/get-cost-and-usage.html), [Billing versus Cost Explorer](https://docs.aws.amazon.com/cost-management/latest/userguide/differences-billing-data-cost-explorer-data.html), and [Cost Explorer data freshness](https://docs.aws.amazon.com/cost-management/latest/userguide/ce-what-is.html).

Release blockers:

- The metric is fixed to `UnblendedCost`; the record does not preserve metric/basis, charge type, pricing term, invoice period, or data-finalization policy.
- `Estimated=false` is labeled `observed`, but it must not be presented as invoiced or final. Historical Cost Explorer values can still differ from billing data.
- Page checkpoints are written but never read to resume. Final completion, retry generation, request range, and prior-token provenance are not represented.
- Allocation storage has no enforced equation proving `observed/estimated source total = allocated + unallocated`, and no deterministic rule execution exists.
- Each Cost Explorer API page is billable; schedules, backfill bounds, and retry budgets need an owner-approved policy.

Exit criteria: version the cost basis, default reporting view, charge-type treatment, currency policy, finalization window, and re-ingestion cadence; implement resumable generation-aware checkpoints; preserve source totals; enforce exact per-period/account/currency allocation conservation; and reconcile a closed AWS month to both Cost Explorer and the invoice with explained variance.

### Security review — changes required

The metadata/credential separation is sound in principle, and unsupported providers fail explicitly. Production credential handling is not complete.

Release blockers:

- `RoleARN` and workload identity metadata are not used to assume or bind a role. Process-wide environment credentials can serve every configured account and workspace.
- Temporary-session `X-Amz-Security-Token` is sent but is not included in the canonical signed-header set. The implementation needs an AWS conformance test or the maintained AWS SDK credential/signing chain.
- There is no validation that the assumed principal and allowed linked accounts match the workspace/account registration, nor rotation/expiry telemetry.
- Provider error bodies are propagated and require a reviewed redaction policy before exposure or durable logging.

Exit criteria: use workload federation or STS AssumeRole with external-ID/audience controls and short sessions; bind organization/workspace/source/account to an allowed role; use and test the maintained AWS signing/credential provider chain; add rotation, expiry, access-denial, and throttling observability; and complete threat-model and credential-leak tests.

### Audit, retention, and compliance-boundary review — changes required

PCI DSS does not prescribe a universal cardholder-data retention period; it requires retention to be limited to legal, regulatory, and business need, and prohibits sensitive authentication data after authorization. Reference: [PCI SSC FAQ 1318](https://www.pcisecuritystandards.org/faqs/1318/). This repository cannot choose the applicable legal retention periods without deployment jurisdiction, contracts, data classification, and counsel.

Release blockers:

- Finance reconciliation occurs as a side effect of `GET` and can be based on a caller-selected time window. There is no independently scheduled, complete-source processing boundary.
- Finance conclusions and cost provenance lack immutable-delete protection, retention classes, legal-hold handling, archival verification, and deletion evidence.
- Event attributes permit arbitrary adapter metadata; there is no finance-specific denylist preventing PAN, card verification codes, bank credentials, or personal data from entering telemetry.
- Rule deployment approval, rule effective time, reviewer identity, conclusion resolution/retraction, and reproducible replay manifest are absent.
- Thirty-day telemetry retention is a product default, not an approved audit, tax, payment-network, contractual, or legal policy.

Exit criteria: classify every finance attribute; reject prohibited payment data before durable ingest; obtain jurisdiction/contract-specific retention schedules; implement immutable audit and legal-hold/deletion controls; move reconciliation to a complete, replayable processor; version and approve rules with effective intervals; and produce a replay manifest containing input range/watermark, schema/rule versions, event IDs, and output digest.

### Review disposition

| Review | Decision | Required external sign-off |
|---|---|---|
| Payment lifecycle | Changes required | Payments domain owner and selected processor/rail owner |
| Cloud cost/FinOps | Changes required | FinOps owner and AWS payer-account owner |
| Credential security | Changes required | Security architecture/IAM owner; QSA only if PCI scope applies |
| Audit and retention | Changes required | Data governance, audit, privacy, and jurisdiction-qualified counsel |

Until every applicable exit criterion is closed, UI and API consumers must retain the existing “operational observability; not a ledger or compliance assessment” label and must not use these outputs to move money, post journal entries, file taxes, determine invoice liability, or attest compliance.
