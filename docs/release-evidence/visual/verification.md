# Immersive product verification

Verified on 2026-09-14 from `codex/immersive-product-brand`.

## Automated results

- `npm run lint`: passed.
- `npm run build`: passed; 2,871 modules transformed.
- `npm run test:browser`: 19 passed and 2 intentionally skipped across desktop, 1024 px tablet, and Pixel 7 projects. Coverage includes query/filter state, pagination, trace dialog keyboard navigation and focus restoration, saved dashboard persistence, immutable version restore, partial/failed states, topology, analytical formulas, protected-route no-flash behavior, product navigation, reduced motion, URL visualization restoration, brand asset responses, and document-level overflow.
- Real local backend: the deterministic fixture was accepted durably; the final transmission capture rendered 60 events, one service, zero errors, and 42 ms p95 in the active hour without a failed query state.

## Accessibility and interaction

- Native landmarks, labelled inputs, named controls, chart `role=img` descriptions, a table alternative for flow representations, visible focus outlines, Escape dismissal, and dialog focus restoration are exercised by browser tests.
- `prefers-reduced-motion: reduce` disables route animation, scroll-linked transitions, loading rotation, clip movement, and smooth scrolling without gating interaction.
- Keyboard-only filter execution, pagination, event inspection, and trace-dialog dismissal passed.
- Static contrast spot checks: primary text on canvas 16.57:1; muted text on panel 6.93:1; signal amber on canvas 10.84:1; dark button label on amber 10.67:1.
- Desktop, tablet, and mobile routes reported zero document-level horizontal overflow.
- The final live browser session reported no page errors. Its console contained only the expected development-mode Firebase configuration warning and Vite diagnostics.

## Bundle output

No dependency was added for animation or visualization. Final production output: CSS 109.53 kB raw / 21.12 kB gzip; home 14.92 / 5.21; explorer 50.24 / 15.88; docs 12.68 / 4.57; main index 213.44 / 73.38. The largest existing lazy vendor chunk remains markdown-core at 333.94 / 102.56 kB gzip.

## Backend and media boundaries

- Environment is persisted in dashboard variables and restored from the URL, but current event, summary, and analytical query contracts do not expose an environment predicate. The UI labels it unavailable and does not pretend it was applied.
- Final overview MP4, WebM, captions, and product-derived poster remain assigned to the separate video-production workstream. The component shows a truthful fallback and does not request missing media.
