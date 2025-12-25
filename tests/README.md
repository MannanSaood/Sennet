# Sennet Test Suite

This directory contains all tests organized by phase. Run tests progressively as you complete each phase.

## Test Structure

```
tests/
├── phase1/
│   ├── 1.1_proto/           # Proto generation tests
│   ├── 1.2_backend/         # Go backend tests
│   └── 1.3_agent/           # Rust agent tests
├── phase2/
│   ├── 2.1_ebpf/            # eBPF TC classifier tests
│   └── 2.2_interface/       # Interface discovery tests
├── phase3/
│   ├── 3.1_build/           # Cross-compilation tests
│   └── 3.2_installer/       # Self-update tests
├── phase4/
│   ├── 4.1_dashboard/       # Grafana dashboard tests
│   └── 4.2_docs/            # Documentation tests
└── integration/             # End-to-end tests
```

## Running Tests

### Quick Verification (PowerShell)
```powershell
# Run all phase 1.1 tests
.\tests\run_tests.ps1 -Phase 1.1

# Run all tests for a phase
.\tests\run_tests.ps1 -Phase 1

# Run all tests
.\tests\run_tests.ps1 -All
```

### Individual Test Commands
```powershell
# Phase 1.1: Proto
npx buf lint
npx buf build

# Phase 1.2: Go Backend
cd backend && go test ./...

# Phase 1.3: Rust Agent
cd agent && cargo test
```

## Test Status

| Phase | Subphase | Test File | Status |
|-------|----------|-----------|--------|
| 1.1 | Proto Generation | `phase1/1.1_proto/test_proto.ps1` | ✅ Ready |
| 1.2 | Go Backend | `phase1/1.2_backend/*_test.go` | 📝 Scaffolded |
| 1.3 | Rust Agent | `phase1/1.3_agent/tests/*` | 📝 Scaffolded |
| 2.1 | eBPF TC | `phase2/2.1_ebpf/test_ebpf.sh` | 📝 Scaffolded |
| 2.2 | Interface | `phase2/2.2_interface/test_interface.rs` | 📝 Scaffolded |
| 3.1 | Build | `phase3/3.1_build/test_artifacts.ps1` | 📝 Scaffolded |
| 3.2 | Installer | `phase3/3.2_installer/test_install.sh` | 📝 Scaffolded |
| 4.1 | Dashboard | `phase4/4.1_dashboard/test_dashboard.ps1` | 📝 Scaffolded |
| 4.2 | Docs | `phase4/4.2_docs/test_docs.ps1` | 📝 Scaffolded |
