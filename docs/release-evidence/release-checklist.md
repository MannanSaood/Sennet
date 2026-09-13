# Release checklist — 2026-09-14

- [x] Required workstream order verified in Git history.
- [x] Workstream commits reviewed for disabled tests, fabricated data, tenant scope, and lossy paths.
- [x] Development authentication contract reconciled and isolation-tested.
- [x] Synthetic packet trace path removed from the active surface.
- [x] Backend unit tests, vet, and binary build executed.
- [x] Python SDK tests executed.
- [x] Agent unit tests (45 passed) and clippy executed; one test remains explicitly ignored and warnings remain.
- [x] Dependency audit remediated to zero known npm advisories.
- [x] Reproducible local load harness added and executed.
- [x] SQLite backup restoration executed.
- [ ] Browser suite clean: latest run was 6 passed, 3 failed, 1 skipped.
- [x] Web production build confirmed with host permission; largest output chunk was 368.42 kB uncompressed.
- [ ] Go race test: Windows lacks a C compiler and WSL has no Linux distribution.
- [ ] Linux x86_64/aarch64 and privileged eBPF attach tests.
- [ ] Compose streaming smoke: Docker daemon unavailable.
- [ ] Real broker, ClickHouse, archive, disk-full, replica-loss, and restore chaos matrix.
- [ ] Production object archive and restore procedure.
- [ ] Finance source completeness, prohibited-data filter, immutable audit/retention/legal hold, and external owner approvals.
- [ ] Deployment resource/cost measurement.

Production release is **NO-GO**. Local backend/SDK evaluation is **GO**. Distributed evaluation deployment remains **NO-GO pending Compose evidence**.
