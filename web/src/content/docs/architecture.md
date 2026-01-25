# Architecture

Sennet follows a modern, distributed architecture designed for scale and security.

## High-Level Overview

1.  **Agent (Data Plane)**: Runs on every node. It loads eBPF programs into the kernel to intercept and analyze network packets efficiently.
2.  **Control Plane**: A centralized service that ingests data from agents, aggregates metrics, and manages configuration.
3.  **Dashboard**: The user interface for visualizing data and managing the fleet.

## The Agent

The Sennet Agent is written in Rust. It has two main components:

-   **User-Space Daemon**: Manages communication with the Control Plane and handles configuration.
-   **Kernel-Space eBPF**: Safe, sandboxed bytecode running in the kernel's network datapath (TC hook).

### Why TC (Traffic Control)?

We use the TC hook rather than XDP (eXpress Data Path) for most metrics because it allows us to see packets *after* they have been processed by the generic networking stack (e.g., after GRO/GSO), but *before* they reach the application. This gives us better context (like SKB metadata) while still being extremely fast.

## Data Flow

1.  **Packet Arrival**: A network packet arrives at the NIC.
2.  **eBPF Hook**: Our eBPF program reads packet headers and updates in-kernel BPF Maps (counters/histograms).
3.  **Aggregation**: The user-space agent periodically polls these maps (every 1s).
4.  **Batching**: Metrics are batched and compressed.
5.  **Transmission**: Batches are sent to the Control Plane via gRPC (ConnectRPC).
