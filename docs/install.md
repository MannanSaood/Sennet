# Installation

## Web access

Configure the frontend Firebase project values and the backend Firebase Admin identity, start the gateway/query roles, then open the application and sign in with email, Google, or GitHub. The browser refreshes its Firebase login token automatically. Users do not create, copy, rotate, or manage Sennet API keys.

The frontend `VITE_FIREBASE_API_KEY` value is Firebase's public project configuration identifier; it is not a Sennet access credential. Never expose the Firebase Admin service account or `SENNET_OPERATOR_TOKEN` to the browser.

## Local evaluation

The Compose stack uses `SENNET_AUTH_MODE=development` with a fixed test session supplied through `SENNET_DEVELOPMENT_SESSION_TOKEN`. This mode is rejected when `SENNET_ENV=production`. Set the same value as `SENNET_SESSION_TOKEN` when running `examples/send_fixture.py` or `tests/streaming_smoke.py`.

## Collector limitation

The existing Rust agent still uses its historical key-shaped enrollment configuration and is not compatible with the login-only gateway. A browser/device enrollment and renewable workload-session flow must replace it before agent deployment. This workstream deliberately does not claim that migration is complete.

See [DEPLOY.md](DEPLOY.md) for Docker, Kubernetes, role, datastore, and recovery instructions.
