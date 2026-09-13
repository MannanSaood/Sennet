# Reproducing the local load measurement

From `backend`:

```powershell
$env:GOCACHE = Join-Path (Get-Location) '.gocache'
$env:SENNET_RELEASE_LOAD = '1'
go test -buildvcs=false -run TestReleaseLoadHarness -count=1 -v ./platform
```

The harness uses a fresh temporary SQLite database and fails if accepted and stored IDs diverge. It independently reports accepted, durable, stored, rejected, duplicate attempts, duplicate rows, and lost records, along with payload distribution, cardinality, tenant skew, concurrent query count, throughput, p95/p99 latency, elapsed time, and Go heap allocation.

It does not emulate Kafka or ClickHouse and must not be used as distributed capacity evidence.
