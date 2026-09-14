# Observing a multi-agent run

Agent telemetry uses run, task, handoff, retry, cancellation, evaluation, model, and tool identifiers alongside normal trace context.

## Read the run DAG

Parallel branches remain parallel. Loops and retries retain attempt metadata. The critical path measures observed span timing; it does not infer hidden decisions. Queue wait is separated from processing time.

## Inspect token and cost provenance

Input/output tokens, provider, model version, price version, observed cost, and estimated cost are distinct fields. Never present an estimate as a provider charge.

## Find infrastructure correlation

An agent span may link to an application or network trace. Open that link only when the telemetry contains it. Content capture defaults to metadata; deploy a redaction and retention policy before enabling sensitive payload capture.
