# Configuration reference

Sennet's HTTP and OTLP surfaces use verified login identity. Sennet does not issue user-managed API keys.

## Server identity

| Variable | Purpose |
|---|---|
| `SENNET_AUTH_MODE` | `firebase` in production; `development` only for local evaluation |
| `FIREBASE_SERVICE_ACCOUNT_JSON` | Firebase Admin service-account JSON supplied through a secret manager |
| `FIREBASE_SERVICE_ACCOUNT_PATH` | Alternative path to the Firebase Admin credential file |
| `GOOGLE_APPLICATION_CREDENTIALS` | Application-default credential path where supported |
| `SENNET_DEVELOPMENT_SESSION_TOKEN` | Fixed local session used by Compose tests; rejected in production |
| `SENNET_DEVELOPMENT_TENANT` | Tenant assigned to the development session; default `local` |

Firebase ID tokens may contain `tenant_id` and `role` custom claims. Without `tenant_id`, a user receives `user:<firebase-uid>` as a personal tenant. Roles are `admin`, `reader`, and `ingest`; a personal account defaults to administrator of its own tenant.

See [DEPLOY.md](DEPLOY.md) for Kafka, ClickHouse, archive, role, readiness, and overload settings.

## Agent compatibility

The current distributed gateway no longer accepts the legacy agent's API-key credential. The Rust agent remains unchanged under this workstream's original `agent/` constraint and needs a separate browser/device enrollment plus refresh-token migration before it can connect to this login-only gateway. Do not deploy the legacy agent against this gateway and do not substitute a long-lived login token in its old key field.
