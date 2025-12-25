# Contributing

Thank you for your interest in contributing to Sennet!

## Development Setup

### Prerequisites

- Rust (stable + nightly)
- Go 1.23+
- Linux with kernel 5.15+ (for eBPF)
- bpf-linker (`cargo install bpf-linker`)

### Clone and Build

```bash
git clone https://github.com/your-org/sennet.git
cd sennet

# Build backend
cd backend && go build . && cd ..

# Build agent (eBPF requires Linux)
cd agent && cargo build && cd ..
```

## Project Structure

```
sennet/
├── agent/           # Rust agent with eBPF
│   ├── src/         # Userspace code
│   └── sennet-ebpf/ # eBPF program
├── backend/         # Go control plane
├── proto/           # Protobuf definitions
├── gen/             # Generated code
├── dashboards/      # Grafana dashboards
├── docs/            # Documentation
└── tests/           # Test suite
```

## Running Tests

```bash
# All tests
./tests/run_tests.ps1 -All

# Specific phase
./tests/run_tests.ps1 -Phase 1.1

# Go tests
cd backend && go test ./...

# Rust tests
cd agent && cargo test
```

## Pull Request Process

1. Fork the repository
2. Create a feature branch
3. Make your changes
4. Run tests
5. Submit a pull request

## Code Style

- **Rust**: `cargo fmt` and `cargo clippy`
- **Go**: `gofmt` and `go vet`

## License

By contributing, you agree that your contributions will be licensed under the MIT License.
