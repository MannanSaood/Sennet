# Agentic observability workstream

Status: `sennet.agent.v1` is implemented as a versioned extension over the tenant-scoped event and OTLP trace contracts. This is an observability schema: captured names, arguments, outputs, errors, and content are untrusted telemetry and must never be executed or treated as instructions.

## Event convention

Agent events use `signal=agent`, `sennet.agent.convention=sennet.agent.v1`, a stable `agent.run.id`, and `agent.event.kind`. Kinds are `run`, `session`, `workflow`, `task`, `agent`, `model`, `prompt`, `tool`, `handoff`, `retry`, `queue_wait`, `cancellation`, `tokens`, `cost`, `evaluation`, and `outcome`.

| Concern | Attributes and representation |
|---|---|
| execution | `agent.run.id`, `agent.session.id`, `agent.workflow.id`, `agent.task.id`, `agent.attempt` |
| agent | `agent.id`, `agent.version` |
| model | OTel-aligned `gen_ai.provider.name`, `gen_ai.request.model`, `gen_ai.response.model` |
| prompt metadata | `gen_ai.prompt.id`, `gen_ai.prompt.version`; prompt text is not metadata |
| tools and handoffs | `gen_ai.tool.name`, `gen_ai.tool.call.id`, `agent.handoff.id` |
| lifecycle | `agent.queue.wait_ms`, `agent.cancellation.reason`, span status, event kind |
| usage | exact integer strings in `gen_ai.usage.input_tokens`, `gen_ai.usage.output_tokens`, `gen_ai.usage.cached_tokens` |
| cost | exact decimal strings in `gen_ai.cost.estimated` and `gen_ai.cost.observed`, plus `gen_ai.cost.currency` and `gen_ai.pricing.version` |
| evaluation | `evaluation.evaluator`, `evaluation.evaluator.version`, `evaluation.score`, `agent.outcome` |
| infrastructure | `infra.trace_id` links agent work to an application/infrastructure trace |

Estimated cost is always distinct from observed/provider-billed cost. An estimate does not become observed cost when billing data is absent. Pricing version is required for reproducible estimates; consumers render missing provenance as unknown.

```json
{"signal":"agent","name":"agent.model","trace_id":"11111111111111111111111111111111","span_id":"0000000000000006","parent_id":"0000000000000005","attributes":{"sennet.agent.convention":"sennet.agent.v1","agent.event.kind":"model","agent.run.id":"run-concurrent-1","gen_ai.provider.name":"openai","gen_ai.request.model":"model-alias","gen_ai.usage.input_tokens":"1200","gen_ai.usage.output_tokens":"240","gen_ai.pricing.version":"provider-2026-09","gen_ai.cost.estimated":"0.012","gen_ai.cost.observed":"0.013","gen_ai.cost.currency":"USD"}}
```

## Trace, span, and link rules

- A run is one trace when context can be propagated. Run, supervisor, workflow, agent, task, model, and tool work are spans. Retries, queue wait, cancellation, evaluation, and outcomes may be span events when separate timing adds no value.
- Parent/child represents owned synchronous work. Parallel children with the same parent are fan-out. A join is parented to its owning supervisor and linked to every input branch; links, not multiple parents, represent fan-in.
- An asynchronous handoff is parented to the handoff span only when context is causally propagated. Otherwise the receiver is a root span linked to the sender with the same `agent.run.id`. Cross-trace handoffs retain both IDs in links.
- Supervisors parent work they own. Independently scheduled subagents use links. Retries are sibling attempts under the task, retain stable task/tool-call identity, and increase `agent.attempt` without overwriting earlier attempts.
- Missing parents are retained and surfaced, never guessed. Late spans merge idempotently by stable event ID. Reconstruction sorts by event time and stable ID, so arrival order does not affect output.
- OTLP links on `/v1/traces` normalize to comma-separated `trace_id:span_id` values in `span.links`.

## Privacy, retention, and cardinality

The Python SDK defaults to metadata only (`content.captured=false`, `metadata-30d`). Raw prompts and completions are not stored by default. Content requires explicit `ContentPolicy(capture_content=True, redact=..., retention_class=...)`; redaction runs before durable spooling. Backend ingress strips raw prompt/completion content, authorization, passwords, and secrets. Deployment retention enforcement remains a storage-policy boundary; a retained class is provenance, not proof of downstream deletion.

Run, session, workflow, task, agent, tool-call, handoff, trace, span, prompt, and model-version IDs are high-cardinality event fields. They must not become metric labels. Metrics may use bounded dimensions such as event kind, status, provider family, and failure class.

## Processing and API

`GET /api/agent-runs?run_id=...&from=...&to=...` is authenticated, workspace scoped, and limited to 10,000 scanned agent events. It returns ordered DAG nodes, links, missing-parent flags, deterministic critical path, loops, retries and retry storms, tool failures, exact token/cost waterfalls, queue wait, evaluation comparisons, infrastructure trace IDs, and partial status. Telemetry content is never interpreted by this API.

The critical path is the parented chain ending latest. Links do not invent parentage or duration. More than five observations for one task is a retry storm. A task ID repeated in its ancestor chain is a loop. These are version-one compatibility decisions.

## Fixture, tests, and compatibility

`examples/agentic_fixture.py` produces a concurrent run with fan-out, failed tool, retry, linked asynchronous handoff, exact token/cost provenance, queue wait, cancellation, evaluation, missing parent, infrastructure correlation, and late span. IDs and relative timing are fixed; only base wall time changes for ingress retention. Re-ingestion is idempotent.

Tests cover workspace isolation, deterministic reconstruction, missing parents, idempotent replay, exact token integers, separate estimated/observed cost, pricing provenance, context propagation, durable identical retry, metadata-only defaults, and explicit pre-spool redaction.

- Existing unversioned agent events remain ingestible for migration, but only `sennet.agent.v1` participates in this processor contract.
- Current OTel GenAI names are reused where semantics align. Sennet names fill execution and policy gaps while upstream conventions evolve.
- Existing `/api/trace` and `/api/query` contracts are unchanged. `/api/agent-runs` is additive and is the stable UI boundary.
