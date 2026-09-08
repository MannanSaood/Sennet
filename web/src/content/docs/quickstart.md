# Quickstart

Configure Firebase login verification (or the documented non-production development session) and start the Go backend and web development server:

```sh
cd backend
go run .
```

```sh
cd web
npm ci
npm run dev
```

Open http://localhost:5173 and sign in. The browser supplies its short-lived login token automatically; Sennet does not issue user-managed API keys.

Send OTLP/HTTP to port 8080 using `/v1/traces`, `/v1/logs`, or `/v1/metrics` with `Authorization: Bearer <login-id-token>`. JSON and protobuf payloads and gzip encoding are supported. Configure a persistent queue in your OpenTelemetry Collector.

For explicit local evaluation data, set `SENNET_SESSION_TOKEN` to the Compose development login session and run `python examples/send_fixture.py` from the repository root. Production clients obtain short-lived tokens by signing in.

Use the streaming compose deployment for PostgreSQL metadata, Kafka ingestion and ClickHouse analytics. Its single replicas are for evaluation; production replication, TLS, backups and capacity tests must be configured separately.
