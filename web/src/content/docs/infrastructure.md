# Infrastructure walkthrough

Follow application and network evidence without collapsing distinct signal types into one chart.

## Set the investigation scope

Choose a time range, timezone, environment, and service in the workspace scope bar. These values persist in the URL and remain shared when you change visualization. The backend derives tenant scope from the authenticated session.

## Move from volume to evidence

Use **Traces** for request duration and parent/child context, **Logs** for recorded application events, **Metrics** for server-side aggregates, **Network flows** for observed transmission, and **Service map** for evidence-backed dependencies.

> A topology edge is shown only when the backend finds a parent or explicit source/destination relationship. Spatial proximity never implies causality.

## Handle collection gaps

Collector unavailable, exporter queueing, stale query results, and zero matched events are separate states. Do not interpret an empty result as healthy traffic.
