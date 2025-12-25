# Architecture

This document describes the internal architecture of Sennet.

## Components

### Agent (Rust + eBPF)

```
┌─────────────────────────────────────────────────────────┐
│                      Sennet Agent                        │
├──────────────────┬──────────────────┬───────────────────┤
│   Config Loader  │  Identity Mgr    │   Heartbeat Loop  │
├──────────────────┴──────────────────┴───────────────────┤
│                    eBPF Manager                          │
├──────────────────┬──────────────────┬───────────────────┤
│   TC Ingress     │   TC Egress      │   RingBuf Events  │
└──────────────────┴──────────────────┴───────────────────┘
                           │
                           ▼
              ┌─────────────────────────┐
              │      Linux Kernel       │
              │    (TC Hook Points)     │
              └─────────────────────────┘
```

### Control Plane (Go)

```
┌─────────────────────────────────────────────────────────┐
│                   Control Plane                          │
├──────────────────┬──────────────────┬───────────────────┤
│   ConnectRPC     │   Auth Middleware │    Metrics       │
├──────────────────┴──────────────────┴───────────────────┤
│                    SQLite Database                       │
└─────────────────────────────────────────────────────────┘
```

## Data Flow

1. **eBPF Programs** capture packets at TC hooks
2. **PerCpuArray** maps store packet counters
3. **RingBuf** sends events (anomalies, large packets)
4. **Agent** reads maps every 10s and sends to control plane
5. **Control Plane** stores in SQLite and exposes `/metrics`
6. **Prometheus** scrapes `/metrics`
7. **Grafana** visualizes the data

## eBPF Maps

| Map | Type | Purpose |
|-----|------|---------|
| `COUNTERS` | `PerCpuArray<PacketCounters>` | RX/TX packet/byte counts |
| `EVENTS` | `RingBuf` | Anomaly events, large packets |

## API Endpoints

| Endpoint | Method | Auth | Description |
|----------|--------|------|-------------|
| `/health` | GET | No | Health check |
| `/metrics` | GET | No | Prometheus metrics |
| `/sentinel.v1.SentinelService/Heartbeat` | POST | Yes | Agent heartbeat |
