# Sennet brand guide

## Direction: the evidence route

The Sennet mark is one routed `S` path crossing four fixed observation points. Signal amber and mineral blue mark transmission from an originating event to resolved evidence. The open construction avoids enclosure, security-shield symbolism, and implied omniscience.

The chosen direction followed exploration of routed S, convergence, layered evidence, orbital signal, split field, and branch-to-result concepts. The routed S produced the clearest small-size silhouette and the strongest connection to the interaction system.

## Construction and spacing

The master symbol uses a 48 × 48 unit grid, a 5-unit round stroke, two 4-unit terminal nodes, and two 2.6-unit observation nodes. Preserve clear space equal to one terminal-node diameter (8 units) on every side. Do not optically close the route or add a containing shape.

Horizontal wordmarks use the mark at 48 units high followed by a custom path-built uppercase wordmark. The React product lockup uses the same symbol with the display typography for efficient UI rendering.

## Minimum size

- Standalone symbol: 16 CSS pixels minimum; use the simplified sidebar/favicon asset below 24 pixels.
- Horizontal wordmark: 96 CSS pixels minimum width.
- Application icon: 32 CSS pixels minimum.
- Social lockup: do not crop inside its 1200 × 630 view box.

## Color

| Token | Value | Role |
|---|---:|---|
| Evidence paper | `#f2efe6` | Primary dark-mode ink, docs background |
| Evidence black | `#0c110e` | Product and consumer background |
| Panel graphite | `#151c17` | Product surfaces |
| Boundary | `#2a352d` | Dividers and structure |
| Signal amber | `#ffb547` | Active route, origin, primary action |
| Mineral blue | `#7dcfff` | Resolved result, secondary series |
| Evaluation lime | `#c8f08a` | Healthy measured state |
| Exception rose | `#e99aae` | Errors and invalid transitions |

Use signal colors semantically. Amber is not a decorative glow; it identifies the active evidence route or action. Blue identifies resolved evidence. Lime and rose retain state meaning.

## Typography

Display headings use a condensed sans-serif stack (`Arial Narrow`, `Roboto Condensed`, Arial fallback), uppercase with tight spacing. Body copy uses Inter or Segoe UI. Data, timings, identifiers, section indices, and control metadata use a monospace stack. Avoid rounded geometric startup typography.

## Correct usage

- Use the dark asset on evidence-black or dark photography.
- Use the light asset on paper and light documentation surfaces.
- Use monochrome only when color reproduction is unavailable.
- Keep the mark level, unskewed, and surrounded by the required clear space.
- Pair motion with real state transitions or route progression.

## Incorrect usage

- Do not add gradients, outer glow, glass panels, shadows, or a shield/hexagon container.
- Do not recolor nodes arbitrarily or use amber/blue as decoration.
- Do not stretch, rotate, outline, or rearrange the path and nodes.
- Do not animate the logo continuously in navigation.
- Do not place the wordmark over low-contrast telemetry.

## Motion behavior

For opening or loading moments, reveal the route once from origin to result over the `--motion-slow` token (760 ms) using `--ease-signal`. Observation points may resolve in a 70 ms stagger. Persistent navigation marks remain still. Under `prefers-reduced-motion`, show the complete mark immediately. Logo motion never delays authentication, navigation, or query interaction.

## Asset inventory

- `web/public/brand/sennet-wordmark-dark.svg`
- `web/public/brand/sennet-wordmark-light.svg`
- `web/public/brand/sennet-symbol-dark.svg`
- `web/public/brand/sennet-symbol-light.svg`
- `web/public/brand/sennet-symbol-mono.svg`
- `web/public/brand/favicon.svg`
- `web/public/brand/sennet-app-icon.svg`
- `web/public/brand/sennet-og-lockup.svg`
- `web/public/brand/sennet-sidebar-mark.svg`

All production assets are clean SVG paths and shapes. The exploratory raster concept sheet is not used by the product.
