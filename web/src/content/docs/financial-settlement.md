# Inspecting financial settlement

The finance surface is operational observability, not a ledger, reconciliation authority, or regulatory control.

## Preserve exact values

Amounts are decimal strings with an explicit currency. Sequence, source identifier, transaction identifier, event time, and receive time support gap and delay inspection without converting money to binary floating point.

## Read lifecycle state

Authorization, capture, settlement, correction, and reconciliation events appear in event time. Duplicates, invalid transitions, gaps, and late arrivals remain visible. Business-specific state rules require domain review.

## Trace an affected dependency

Follow a dependency only when a trace identifier, parent, span link, or explicit source relationship exists. A nearby infrastructure error is supporting context, not proof of cause.
