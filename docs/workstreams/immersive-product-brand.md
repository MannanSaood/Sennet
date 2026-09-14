# Immersive product and brand workstream

## Objective

Replace Sennet's split placeholder/generic visual system with one authored experience across the consumer narrative, authentication, workspace, visualization studio, dashboards, documentation, favicons, and metadata.

## Implemented system

- Original evidence-route identity and complete SVG asset family.
- Cinematic consumer narrative following a deterministic labelled demo trace across request, gateway, agents, model/tool calls, services, network, risk, settlement, and result.
- Motion tokens for timing, easing, distance, opacity, and stagger; viewport-gated scroll focus; reduced-motion overrides.
- URL-addressable visualization registry with signal compatibility for line, area, stacked, distribution, heat map, tunnel, radar, pipeline, transmission, agent workflow, finance lifecycle, and topology modes. Transmission offers bounded telemetry-derived tracks, zoom, collapse, selection, event-boundary positioning, synchronized details, and a minimap. Causal views require explicit source/destination dimensions and render an unavailable explanation when those fields are absent.
- Bounded data inspection, JSON download, fullscreen, descriptions, normalized-radar disclosure, and explicit unavailable states.
- Grid dashboard editor with templates, real backend panel queries, reorder, compact/wide sizing, duplication, deletion, edit/presentation modes, synchronized variables, and backend version persistence.
- Documentation IA, search, active navigation, on-page contents, previous/next links, copy controls, callouts, responsive code, and product-video handoff contract.
- Intentional desktop, tablet, and mobile systems for product, docs, auth, and consumer pages.

## Contract boundaries

No backend semantics were changed. Large-result aggregation remains server-side and query budgets remain bounded. Dashboards persist through the existing `/api/dashboards` resource and immutable version contract. Topology and causal language remain evidence-backed. Finance is operational observability, not a ledger or compliance control.

The current event, summary, and analytical query contracts do not expose an environment field. The shared environment value is URL-addressable and persisted with dashboards, but the UI now identifies it as unavailable and keeps results workspace-wide instead of disguising it as free-text search. A backend environment predicate is the remaining dependency for applying that variable.

The consumer narrative uses only `web/src/features/demo/evidenceJourney.ts` and always labels it as Demo. Authenticated routes never substitute this fixture. Full details are in `docs/deterministic-demo-fixtures.md`.

## Video production handoff

The two-minute overview page and reusable video component are ready. Final footage must be recorded honestly from the working product and dropped into `web/public/media/docs/` using the filenames in that folder's README. Until then, the page shows its poster, chapters, transcript, duration, and exact source paths without issuing requests for missing video files.

## Verification evidence

Screenshots and browser evidence are stored under `docs/release-evidence/visual/`. Final lint, build, browser, responsive, reduced-motion, keyboard, console, overflow, route restoration, and asset results are recorded there after execution.
