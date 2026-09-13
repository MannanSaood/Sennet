# Exact verification results — 2026-09-14

| Command/check | Result |
|---|---|
| `go test -buildvcs=false -count=1 ./...` | PASS: cloud 5.441s, correlation 7.633s, db 11.817s, handler 9.924s, platform 17.408s; remaining packages report no test files |
| Targeted reliability/security/restore tests | PASS: 8/8 (`SQLiteBackupRestore`, development bootstrap isolation/idempotence, collector expiry/replay/rotation, proxy/quota/audit/migration, ambiguous append/duplicate, consumer restart, poison/replay, unavailable storage offset retention) |
| `go vet ./...` | PASS |
| `go build -buildvcs=false -o sennet-backend.exe .` | PASS |
| `go test -race -count=1 ./...` | NOT EXECUTED: Windows host has no `gcc`; WSL reports no `bash`/Linux distribution |
| `python -m unittest -v` in `sdk/python` | PASS: 6/6 |
| `cargo test --locked` | PASS: 45 passed, 0 failed, 1 ignored (`config::tests::test_env_override`) |
| `cargo clippy --locked --all-targets` | PASS with 40 binary / 28 test warnings, largely existing dead-code and naming warnings |
| `cargo fmt --check` | FAIL: repository contains pre-existing rustfmt drift and two trailing-whitespace errors in `agent/src/k8s.rs`; changed Rust files were formatted directly |
| `cargo run --locked -- trace --count 1` | Expected explicit failure: packet-event tracing unsupported; no synthetic data |
| `npm run lint` | PASS |
| `npm run build` | PASS: 3,492 modules; 4m31s; largest uncompressed chunk 368.42 kB |
| `npm audit --json` before remediation | FAIL: 45 vulnerable nodes: 2 low, 9 moderate, 32 high, 2 critical |
| `npm audit fix` | PASS without `--force`; lockfile updated |
| `npm audit --json` after remediation | PASS: 0 vulnerabilities across 542 dependencies |
| `npm run test:browser` latest unchanged-suite run | FAIL: 6 passed, 3 failed, 1 skipped in 4.9m. Desktop filter/dashboard/topology cases failed; mobile counterparts passed. Earlier run was 5/4/1. No timeout was weakened. |
| Local load harness | PASS: 12,000 accepted/durable/stored; 1 rejected; 600 duplicate attempts; 0 duplicate rows; 0 lost |
| Docker Compose streaming smoke | NOT EXECUTED: Docker CLI installed, daemon pipe unavailable after Docker Desktop launch |
| Linux x86_64/aarch64, privileged eBPF, replica loss | NOT EXECUTED on this host |
| `git diff --check` | PASS; line-ending conversion warnings only |

The browser, formatting, race, Compose, and Linux rows are release failures or untested gates, not passes.
