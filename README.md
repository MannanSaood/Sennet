# Sennet

Sennet connects application traces, logs, metrics, agent executions and financial workflow events in one self-hosted investigation workspace.

The repository now includes a tenant-scoped control plane, OTLP/HTTP ingestion (JSON and protobuf), a durable local development store, a Kafka-to-ClickHouse streaming path, a React investigation UI, and a Linux network agent. **Production capacity and high availability must be measured on your deployment.** The compose stack is a single-replica evaluation environment.

## Run locally

Requirements: Go 1.24+, Node 22+, Python 3 (optional fixture generator).

Generate a random bootstrap credential and put it in your shell as `INIT_API_KEY`; use `sk_` followed by at least 32 random hexadecimal characters. Do not commit it.

```powershell
$env:INIT_API_KEY = 'sk_' + [guid]::NewGuid().ToString('N') + [guid]::NewGuid().ToString('N')
$env:SENNET_AUTH_MODE = 'development'
$env:SENNET_DEVELOPMENT_SESSION_TOKEN = 'replace-with-a-local-random-token'
$env:SENNET_DEVELOPMENT_TENANT = 'local'
cd backend
go run .
```

In a second terminal:

```sh
$env:VITE_SENNET_DEVELOPMENT_SESSION_TOKEN = $env:SENNET_DEVELOPMENT_SESSION_TOKEN
$env:VITE_SENNET_DEVELOPMENT_TENANT = $env:SENNET_DEVELOPMENT_TENANT
cd web
npm ci
npm run dev
```

Open http://localhost:5173. The development-only session token is accepted only when `SENNET_AUTH_MODE=development`; production builds ignore the matching Vite variables. The development server proxies requests to localhost:8080, and the backend binds to loopback by default. Create an **ingestion-only** key in Workspace for collectors; use a reader key for read-only users.

For optional, clearly identified evaluation data, set `SENNET_API_KEY` to an ingestion key and run:

```sh
python examples/send_fixture.py
```

## Streaming evaluation stack

Set `INIT_API_KEY`, `SENNET_POSTGRES_PASSWORD` and `SENNET_CLICKHOUSE_PASSWORD` to random values (hexadecimal passwords avoid URL escaping in the compose DSN), then:

```sh
docker compose -f deploy/compose.yaml up --build --wait
```

Open http://localhost:8088. Data persists in Docker volumes. PostgreSQL stores identities and configuration; Kafka provides durable ingestion; the consumer batches observations into ClickHouse. Browser queries use parameterized tenant filters. Never expose the internal database or broker ports publicly.

For managed production services, configure PostgreSQL TLS, Kafka TLS/SASL, ClickHouse HTTPS, a TLS ingress, topic replication/minimum ISR, storage replicas, backups and tenant budgets. `SENNET_ENV=production` rejects startup unless PostgreSQL, Kafka and ClickHouse are configured. The shipped compose file does not establish HA or a throughput guarantee.

## Collect signals

- OTLP/HTTP: `/v1/traces`, `/v1/logs`, `/v1/metrics`; bearer ingestion key; JSON or protobuf; gzip supported.
- Domain events: `POST /api/events`, 1–1,000 records, up to 4 MiB decoded; stable IDs required for replay.
- Collector example: [deploy/otel-collector.yaml](deploy/otel-collector.yaml), with a persistent exporter queue. Set its endpoint and scoped key explicitly.
- Python instrumentation: [sdk/python/sennet.py](sdk/python/sennet.py) provides nested spans, context propagation, decimal financial events and a disk outbox.
- Linux network collector: build `agent`, configure it, and run `sudo sennet start`. `sennet top` displays real local rates/history; `sennet top --json` prints a snapshot. Windows does not support local eBPF collection.

```yaml
# /etc/sennet/config.yaml
api_key: "sk_REPLACE_WITH_INGEST_KEY"
server_url: "https://your-sennet-server.example"
interface: "eth0"
heartbeat_interval_secs: 30
state_dir: "/var/lib/sennet"
log_level: "info"
```

The agent spools unacknowledged observations (20 MiB cap), reports unavailable collection explicitly, uses network deadlines and preserves queued records across restarts. Optional socket/drop probes using kernel-specific layouts are disabled by default; enabling `SENNET_EXPERIMENTAL_KERNEL_PROBES=true` requires validation against your kernel. Do not mistake BTF detection for verified CO-RE portability.

## Interfaces and limits

The workspace includes searchable/paginated explorers, service dependencies, trace waterfalls, saved views, scoped key creation/revocation and internal error-count monitors. Monitors evaluate every 30 seconds in bounded batches; no outbound messages are sent. Explorers clearly label page-based summaries and partial trace results. Financial events preserve decimal strings; this is operational observability, not a ledger.

Team administration, commercial billing and direct cloud cost-provider integrations are not enabled in the new application. Legacy implementations remain isolated from the production routes. This prevents unimplemented integrations or global legacy credentials from being represented as working capabilities.

## Verification and migration

```sh
cd backend && go test ./...
cd ../agent && cargo test --locked
cd ../web && npm run lint && npm run build
```

See [implementation status](docs/implementation-status.md), [deployment and migration](docs/DEPLOY.md), and [architecture](docs/platform-architecture-proposal.md). Legacy SQLite data and keys are **not automatically assigned to a new tenant**. Keep the old database for controlled export and explicit ownership mapping; do not copy global credentials into shared production.
