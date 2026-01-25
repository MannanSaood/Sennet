# eBPF Basics

Extended Berkeley Packet Filter (eBPF) is a revolutionary technology in the Linux kernel that allows you to run sandboxed programs in a privileged context (like the OS kernel).

## What makes eBPF special?

-   **Safety**: Programs are verified before execution to ensure they cannot crash the kernel or access invalid memory.
-   **Performance**: Programs are JIT-compiled to native machine code, running at near-native speed.
-   **Flexibility**: It can be used for networking, security, observability, and tracing.

## How Sennet uses eBPF

Sennet uses eBPF to achieve "Zero Overhead" observability. Traditional monitoring tools often use PCAP or sidecars (proxies) that add significant latency and CPU usage.

By moving the data collection logic *into the kernel*, we avoid the expensive context switching of copying every packet to user space. We only send aggregated *metadata* (metrics) to user space, not the raw packets themselves.

### Key Concepts

-   **Maps**: Efficient key/value stores shared between kernel and user space. We use these for counters (e.g., "bytes sent by PID 123").
-   **Hooks**: Events that trigger our code. We mostly attach to `ingress` and `egress` qdiscs.
-   **Verifier**: The component that checks our code for safety.
