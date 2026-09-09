# Security control plane

Status: implemented on `codex/security-control-plane` for backend review. This document is a contract, not a claim that deployment hardening or an external penetration test is complete.

## Ownership and shared API contract

The hierarchy is organization → workspace → resource. Humans join organizations through memberships and receive explicit workspace role assignments. Integrations and workloads are separate subjects with their own workspace role assignments. A principal carries `organization_id`, `workspace_id`, `subject_type`, `subject`, `credential_type`, and `role`.

The existing telemetry contract remains intentionally small: `Principal.Tenant` is a compatibility alias for `WorkspaceID`, and telemetry store/query methods continue to accept that value as their partition key. API payload tenant values are ignored and replaced server-side. No telemetry schema, web code, or agent code is rewritten by this workstream.

Organization creation is an out-of-band bootstrap or explicit migration concern. A workspace administrator cannot create an unrelated organization. `owner` is reserved for provisioning and cannot be delegated through the API.

## Identity and credential classes

| Class | Prefix / source | Lifetime and use | Stored representation |
|---|---|---|---|
| Human external session | Verified Firebase ID token | Firebase lifetime; exact active human, membership, workspace, and role mapping required | No Firebase token stored |
| Human local session | `ses_` | At most 12 hours; issued only to an already-authorized human | SHA-256 digest, metadata, scope, expiry |
| Integration key | `sk_` | Explicit expiry for delegated keys; bootstrap key may be non-expiring | SHA-256 digest, metadata, scope, expiry/revocation |
| Collector enrollment | `enr_` | At most 15 minutes and exactly one successful enrollment | SHA-256 digest plus used/revoked state |
| Collector workload credential | `col_` | 15 minutes; v2-signed ingestion and self-rotation only | SHA-256 digest, workload, predecessor, expiry/revocation |

Secret material is returned only in the successful creation, enrollment, session, or rotation response. List and audit routes expose only IDs, type, prefix, timestamps, role, and status. Database credential hashes are never part of an API model. Rotation revokes the predecessor and inserts the successor in one transaction. Membership revocation and role changes take effect on the next request because authorization is resolved from current records.

Firebase remains fail-closed. Enabling Firebase only enables token verification; it does not create an organization, membership, workspace, or admin role. The request must name an existing workspace with `X-Sennet-Workspace`, and the verified UID must have an active membership and explicit role there.

## Route and role authorization matrix

Legend: R = read, W = write, I = ingest, M = credential/collector management, E = enroll/rotate self, — = denied. `admin (human)` includes owner. `reader` is a compatibility alias for viewer.

| Route / operation | admin (human) | admin (integration) | editor | viewer / reader | ingest integration | collector workload | enrollment |
|---|---:|---:|---:|---:|---:|---:|---:|
| `GET /api/session`, `GET /api/capabilities` | R | R | R | R | R | — | — |
| `POST /api/human-sessions` | W | — | W | W | — | — | — |
| `GET /api/events`, summary, stats, finance | R | R | R | R | — | — | — |
| `POST /api/events`, `/v1/*`, heartbeat | I | I | — | — | I | I | — |
| `GET /api/agents`, dashboards, preferences, alerts, workload identities | R | R | R | R | — | — | — |
| `POST/DELETE` dashboards, preferences, alerts | W | W | W | — | — | — | — |
| `/api/keys` | M | M | — | — | — | — | — |
| `GET /api/organizations`, workspaces, memberships, role assignments | R | R | — | — | — | — | — |
| `POST /api/workspaces`, memberships, role assignments; membership revoke | W | — | — | — | — | — | — |
| `/api/collector-enrollments`, admin collector credential revoke/list | M | M | — | — | — | — | — |
| `POST /api/collectors/enroll` | — | — | — | — | — | — | E |
| `POST /api/collector-credentials/rotate` | M | M | — | — | — | E | — |
| `GET /api/audit-events` | R | R | — | — | — | — | — |

Every mounted authenticated route maps to one operation before request-body decoding or handler dispatch. Unknown routes fail before dispatch. Repository operations derive organization/workspace from the authenticated principal; resource IDs and payload tenant fields cannot select a different scope.

## Request signing and replay prevention

Version 2 signs this canonical UTF-8 value with HMAC-SHA-256 using the presented credential:

```text
METHOD + "\n" + ESCAPED_PATH + "\n" + UNIX_SECONDS + "\n" + NONCE + "\n" + HEX(SHA256(DECODED_BODY))
```

Headers are `X-Sennet-Signature-Version: 2`, `X-Sennet-Timestamp`, `X-Sennet-Nonce`, and `X-Sennet-Signature`. Timestamps have a five-minute skew window. Nonces are 16–128 characters and are consumed after signature verification. The SQL replay store uses `(credential_id, nonce)` uniqueness, expiry cleanup, and a hard 100,000-row bound. PostgreSQL provides cross-process coordination; SQLite is local evaluation only. Replay-store failure is fail-closed.

Collector workload requests require v2. Legacy version 1 HMAC remains supported for integrations: it signs little-endian timestamp bytes followed by the decoded body. Partial headers, mixed nonce/version-1 requests, unsupported versions, stale timestamps, wrong methods, wrong paths, and changed bodies fail authentication. Legacy signing is compatibility-only and should be retired after client migration.

## Quotas and proxy identity

`Quota` is a process-independent interface. The default SQL implementation atomically accounts fixed-window usage in PostgreSQL (or SQLite for local evaluation) using a scope key containing both organization and workspace. `ScopedKey` is the required constructor for future control-plane caches and quotas, preventing equal resource IDs in different organizations/workspaces from colliding.

The source limiter remains bounded and local. Forwarding headers are ignored unless the immediate peer is in `SENNET_TRUSTED_PROXIES`, a comma-separated list of exact IPs or CIDRs validated at startup. For a trusted peer, `X-Forwarded-For` is walked right-to-left and the first untrusted address is selected. `X-Real-IP` and malformed entries are not trusted. Production deployments must set this list to the actual load-balancer addresses, not broad client networks.

## Audit export

`GET /api/audit-events?limit=…&cursor=…` returns at most 200 events, newest first, within the authenticated organization and workspace. The opaque cursor binds the last `(time_ms,id)` position. Audit writes are append-only; SQLite and PostgreSQL install update/delete rejection triggers. Exported details recursively redact credential, key, token, secret, password, authorization, and hash fields, and bound long strings. Control/resource mutations and all credential lifecycle transitions record actor, action, target, scope, and non-secret metadata.

Operationally immutable retention still requires append-only backups or a write-once external sink: a database owner can disable triggers. That deployment control is outside this branch.

## Legacy migration and quarantine

Run the backend executable offline against the metadata database:

```text
sennet-backend -db <dsn> -migrate-legacy -ownership-map ownership.json
```

The JSON file is an array of:

```json
{
  "legacy_tenant": "old-tenant",
  "organization_id": "org-approved",
  "organization_name": "Approved Organization",
  "workspace_id": "ws-approved",
  "workspace_name": "Production"
}
```

There are no wildcards and no global/default owner. Existing v2 workspaces count as established ownership. Explicitly mapped telemetry/resources move to the named workspace; mapped legacy key hashes become scoped integration credentials with their prior expiry/revocation state. Records with no ownership, conflicting mappings, or invalid roles are removed from active tables and inserted into `platform_quarantine`. Quarantined legacy key payloads omit credential hashes. The entire migration is transactional and emits a machine-readable count report. Back up and rehearse before production use.

## Adversarial verification

`security_control_test.go` creates two organizations and covers cross-organization telemetry events, agents, dashboards, preferences, alerts, integration keys, organizations, workspaces, memberships, role assignments, workload identities, collector credentials, and audit events. It also tests payload tenant override, cross-tenant revoke/write attempts, cache-key separation, privilege escalation, expiration, one-time enrollment, unsigned collector rejection, method/path binding, replay, rotation revocation, legacy HMAC compatibility, trusted-proxy spoofing, shared SQL quota accounting, audit redaction/immutability/pagination, explicit legacy ownership mapping, and orphan quarantine.

Verification on 2026-09-10:

- `go test ./...`: passed for every backend package.
- `go vet ./...`: passed with no findings.
- `CGO_ENABLED=1 go test -race ./...`: passed in Ubuntu 24.04 under WSL with GCC and Go 1.27.1. Every backend package passed; packages without tests were reported explicitly.

## Threat boundaries and remaining review

- TLS termination, service-to-database authentication, PostgreSQL roles, and secret-manager/KMS integration are deployment boundaries. HMAC does not replace authenticated TLS.
- SHA-256 credential digests depend on high-entropy generated tokens. Human passwords are never accepted or stored here.
- Database owners can inspect digests, disable triggers, or rewrite ownership. Production must use least-privilege application roles, immutable external audit export, backup controls, and administrative separation.
- The fixed-window SQL quota is a correctness boundary, not a complete abuse policy. Review per-plan limits, NAT behavior, storage growth, and denial-of-service characteristics under load.
- Review owner bootstrap/recovery, last-admin protection, session revocation UI, workload federation/mTLS, nonce-store capacity alerts, key rotation automation, PostgreSQL migration locking, and security-event observability before general availability.
- Run a dedicated cryptographic/protocol review, database privilege review, infrastructure threat model, and external penetration test. Docker/PostgreSQL integration and sustained adversarial load remain release gates.
