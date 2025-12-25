# Sennet eBPF Build Instructions

## Prerequisites (Ubuntu 22.04 / WSL2)

```bash
# Install Rust (if not already installed)
curl --proto '=https' --tlsv1.2 -sSf https://sh.rustup.rs | sh

# Add nightly toolchain (required for eBPF)
rustup install nightly
rustup component add rust-src --toolchain nightly

# Install bpf-linker
cargo install bpf-linker

# Install build dependencies
sudo apt update
sudo apt install -y clang llvm libelf-dev linux-headers-$(uname -r)
```

## Building the eBPF Program

```bash
cd /mnt/c/Users/DELL/Documents/Sennet/agent

# Build the eBPF program (must use nightly + bpfel target)
cd sennet-ebpf
cargo +nightly build --target bpfel-unknown-none -Z build-std=core --release

# Build the userspace agent
cd ..
cargo build --release
```

## Running (requires root)

```bash
export SENNET_API_KEY="sk_00d846bdef445997241e33ebdea3ebdea"
export SENNET_SERVER_URL="https://sennet.onrender.com"

# Run with sudo (required for eBPF)
sudo -E ./target/release/sennet
```

## Verify eBPF is Loaded

```bash
# Check TC filters
tc filter show dev eth0 ingress
tc filter show dev eth0 egress

# Check BPF programs
sudo bpftool prog list

# Check BPF maps
sudo bpftool map list
```

## Troubleshooting

### "Operation not permitted"
- Run with sudo
- Check kernel version: `uname -r` (need 5.15+)

### "BTF not found"
- Install linux-headers: `sudo apt install linux-headers-$(uname -r)`
- Check BTF: `ls /sys/kernel/btf/vmlinux`

### WSL2 Specific
WSL2 may not have full eBPF support. For best results:
- Use a native Linux VM or machine
- Or use the CI/CD pipeline which builds on GitHub runners
