# Investigation UI workstream

Status: implemented frontend investigation foundation on `codex/investigation-ui`. The product never substitutes sample telemetry when a backend request is empty or fails.

## UX behavior

The investigation shell keeps time range, timezone, environment, service, tenant display scope, free-text query, trace, cursor, and pause state in the URL. Changing a scope value replaces the current history entry, clears pagination, and changes the React Query key, which aborts obsolete Axios requests through the provided abort signal. The Share action copies the complete URL. Authenticated workspace ownership remains server-derived; the tenant URL value is display/navigation state and is never sent as authority.

Log and event rows use a fixed-height virtual viewport with overscan, selectable columns, redaction badges, loaded-page trace context, event inspection, and backend cursor pagination. A trace drawer traps focus, restores focus on close, supports Escape, and displays missing parents, explicitly late spans, span links, errors, fan-out/fan-in markers, a deterministic end-time critical path approximation, and counts of related logs and metrics returned by the bounded trace request.

The metrics explorer sends only contract-v1 analytical requests. It supports typed operations, grouping, units, formula labels, exact histogram percentiles, exemplars, comparison windows, execution statistics, budget-partial reasons, and an explicit reset-aware rate explanation. Formula labels are not presented as backend algebra because the current contract executes one operation.

Topology is derived only from parent spans and source/destination attributes in the current bounded event page. It supports service filtering, zoom, prefix clustering, a 100-edge visual cap, and an accessible table containing every derived edge. It does not claim a complete workspace graph.

Dashboard composition provides URL-backed variables, synchronized time state, bounded panel placeholders, inspect-query/source actions, and persistence through the existing dashboard resource. The backend currently stores a latest resource and does not expose immutable version history; the UI states that limitation. Monitor and SLO pages read durable definitions and evaluation state. Fleet health distinguishes stale collectors, reported queueing, exporter/gateway errors, and reported storage delay, while explicitly identifying the missing user-facing pipeline-health contract.

Loading, empty, denied, stale, partial, and failed states are shared and visually distinct. Responsive navigation, horizontal data access, focus rings, semantic tables/dialogs, sufficient dark-theme contrast, and `prefers-reduced-motion` behavior are included.

## Browser evidence

Playwright uses explicit route fixtures labelled `browser-fixture`; these fixtures are test-only and cannot become a runtime fallback. Coverage includes URL filtering, cursor pagination, trace drilldown, saved dashboards, analytical partial state, failed requests, keyboard dialog focus/Escape behavior, and mobile layout.

![Desktop investigation](assets/investigation-desktop.png)

![Mobile investigation](assets/investigation-mobile.png)

## Verification

- `npm run lint`: pass.
- `npm run build`: pass; route-level lazy loading plus explicit React, charts, Firebase, and Markdown vendor chunks.
- `npm run test:browser`: 8 passed across Desktop Chrome and Pixel 7 profiles (the desktop project intentionally skips the mobile-only capture case).

Largest final production chunks (uncompressed) are charts 368,421 B, Markdown core 333,942 B, React core 292,272 B, app index 196,819 B, syntax highlighting 169,943 B, and Firebase 109,366 B. No generated chunk exceeds Vite's 500 KB warning threshold. Charts are capped by the backend at 300 points and the UI limits visible topology edges to 100 while retaining the table alternative.

## Remaining visualization and contract gaps

- A complete thousand-service topology needs a server aggregation/cursor contract; page-derived relationships cannot prove absence or completeness.
- Immutable dashboard versions, linked-brush persistence, panel data models, and backend formula algebra are not exposed. The current composition saves the available resource and shares scope variables.
- Trace critical path is approximated from observed parent/end-time evidence. Clock-skew metadata, canonical critical-path output, and first-class span-link arrays are not in the event contract.
- Related logs and metrics are limited to records returned by the bounded trace query. Dedicated correlation endpoints would avoid mixed-signal truncation.
- Protected Prometheus pipeline telemetry has no browser-safe workspace endpoint. Fleet diagnosis therefore uses only reported collector metrics and never invents gateway/storage state.
- Monitor notification delivery is absent by backend design; the UI shows durable evaluation state without claiming that a provider was notified.
- Histogram buckets are not returned by query contract v1; the explorer plots returned percentile summaries rather than fabricating bucket boundaries.
