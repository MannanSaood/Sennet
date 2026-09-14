# Investigating a distributed trace

Start from an event row, open its detail panel, and follow the bounded server trace rather than a page-local approximation.

## Read the waterfall

Span width represents duration. Position represents event time within the returned trace. Critical-path emphasis is supplied by the server contract. Missing parents, late spans, clock skew, and span links remain visible.

## Keep causal claims bounded

Parent identifiers and span links are evidence. Matching timestamps, names, or service adjacency are correlation only. Sennet does not draw causal edges from timing coincidence.

## Continue the investigation

Use correlated logs and metrics, then apply the service as a workspace filter. Save the view when the scope explains the incident; saving writes a real backend dashboard resource.
