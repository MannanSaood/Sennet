# Deterministic demo fixtures

The consumer-site journey uses `web/src/features/demo/evidenceJourney.ts`. It is always labelled **Demo dataset** and is never imported into an authenticated query surface.

The fixture follows trace `demo-7f34b91a` from a user request through gateway validation, an agent supervisor, parallel risk and route subagents, model and tool calls, payment services, network transit, a risk decision, settlement correction, and the final result. Timings, amounts, identifiers, states, and copy are fixed so screenshots and motion tests remain reproducible.

Backend evaluation fixtures remain separate:

- `examples/send_fixture.py` sends explicit, authenticated evaluation events through the real ingestion API.
- `examples/agentic_fixture.py` provides a deterministic concurrent-agent contract with retries, cancellation, queue wait, cost provenance, and missing-parent conditions.

Neither fixture establishes production throughput, financial correctness, or regulatory compliance.

For development-session ingestion, set `SENNET_WORKSPACE` to the same explicit workspace used by the backend. Production integration credentials already carry their workspace scope and should not rely on this header.
