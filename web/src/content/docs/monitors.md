# Creating a monitor

Monitors are persisted definitions evaluated by the backend. Notification success is never shown before provider acknowledgement.

## Choose a definition

Create an error threshold for a bounded evaluation window or an availability SLO with an explicit target and window. Editor or administrator access is required.

## Read the state

Pending evaluation, healthy, and firing are separate states. The workspace shows the last checked time and observed value or burn calculation returned by the backend.

## Understand notification limits

The notification surface reports durable outbox state and whether a provider is configured. An unconfigured provider is an unavailable integration—not a successful send.
