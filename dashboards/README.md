# Sennet Grafana Dashboards

This directory contains Grafana dashboard JSON files that can be imported into your Grafana instance.

## Overview Dashboard

**File:** `overview.json`

### Panels

| Panel | Type | Data Source | Description |
|-------|------|-------------|-------------|
| Packets/sec (Ingress) | Time series | eBPF Map | RX packets per second |
| Packets/sec (Egress) | Time series | eBPF Map | TX packets per second |
| Throughput | Time series | eBPF Map | Bytes per second (RX/TX) |
| Drop Rate | Time series | eBPF Map | Dropped packets per second |
| Anomalies | Stat | RingBuf | Anomaly events in last 5 minutes |
| Large Packets | Stat | RingBuf | Jumbo frame detections |
| Active Agents | Stat | Heartbeat | Currently connected agents |
| Anomaly Events Log | Table | RingBuf | Recent anomaly events |

## Import Instructions

### Grafana Cloud

1. Log into your Grafana Cloud instance
2. Navigate to **Dashboards** → **Import**
3. Click **Upload JSON file** and select `overview.json`
4. Select your Prometheus data source
5. Click **Import**

### Self-Hosted Grafana

```bash
# Using Grafana API
curl -X POST \
  -H "Authorization: Bearer YOUR_API_KEY" \
  -H "Content-Type: application/json" \
  -d @dashboards/overview.json \
  http://localhost:3000/api/dashboards/import
```

## Required Metrics

The dashboard expects these Prometheus metrics from the Sennet agent:

| Metric | Type | Description |
|--------|------|-------------|
| `sennet_rx_packets_total` | Counter | Total received packets |
| `sennet_tx_packets_total` | Counter | Total transmitted packets |
| `sennet_rx_bytes_total` | Counter | Total received bytes |
| `sennet_tx_bytes_total` | Counter | Total transmitted bytes |
| `sennet_drop_count_total` | Counter | Total dropped packets |
| `sennet_anomaly_events_total` | Counter | Anomaly events from RingBuf |
| `sennet_large_packet_events_total` | Counter | Large packet detections |

## Variables

The dashboard includes an `agent` template variable to filter by specific agent IDs.
