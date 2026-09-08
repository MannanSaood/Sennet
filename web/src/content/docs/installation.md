# Quickstart

Run the Go backend with a random `INIT_API_KEY` and start the web development server:

```sh
cd backend
go run .
```

```sh
cd web
npm ci
npm run dev
```

Open http://localhost:5173. Use the bootstrap key to connect, then create an ingestion-only key under Workspace. Never embed keys in the website bundle.

Send OTLP/HTTP to port 8080 using `/v1/traces`, `/v1/logs`, or `/v1/metrics` with `Authorization: Bearer <ingest-key>`. JSON and protobuf payloads and gzip encoding are supported. Configure a persistent queue in your OpenTelemetry Collector.

For explicit evaluation data, set `SENNET_API_KEY` and run `python examples/send_fixture.py` from the repository root. The workspace shows only ingested data.

Use the streaming compose deployment for PostgreSQL metadata, Kafka ingestion and ClickHouse analytics. Its single replicas are for evaluation; production replication, TLS, backups and capacity tests must be configured separately.
