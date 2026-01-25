# Metrics & Data

Sennet collects a wide range of metrics relative to network performance and health.

## Standard Metrics

| Metric Name | Type | Description |
| :--- | :--- | :--- |
| `net.bytes.rx` | Counter | Total bytes received |
| `net.bytes.tx` | Counter | Total bytes transmitted |
| `net.packets.rx` | Counter | Total packets received |
| `net.packets.tx` | Counter | Total packets transmitted |
| `net.drops` | Counter | Packets dropped by the kernel or NIC |

## Flow Metrics

Flow metrics are enriched with metadata:

-   **Source IP / Port**
-   **Dest IP / Port**
-   **Protocol** (TCP, UDP, ICMP)
-   **PID** (Process ID)
-   **Comm** (Command name, e.g., `nginx`)
-   **Pod Name** (if in K8s)
-   **Namespace** (if in K8s)

> [!NOTE]
> Flow metrics are aggregated by default to reduce cardinality. You can enable high-cardinality mode in settings if needed.

## Retransmissions

We track TCP retransmissions directly from the kernel stack. A high retransmission rate usually indicates network congestion or faulty hardware.
