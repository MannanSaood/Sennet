// Package metrics provides Prometheus metrics for the Sennet backend
package metrics

import (
	"net/http"
	"sync"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

var (
	// Agent metrics - updated on heartbeat
	RxPackets = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Namespace: "sennet",
			Name:      "rx_packets_total",
			Help:      "Total received packets reported by agent",
		},
		[]string{"agent_id"},
	)

	TxPackets = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Namespace: "sennet",
			Name:      "tx_packets_total",
			Help:      "Total transmitted packets reported by agent",
		},
		[]string{"agent_id"},
	)

	RxBytes = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Namespace: "sennet",
			Name:      "rx_bytes_total",
			Help:      "Total received bytes reported by agent",
		},
		[]string{"agent_id"},
	)

	TxBytes = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Namespace: "sennet",
			Name:      "tx_bytes_total",
			Help:      "Total transmitted bytes reported by agent",
		},
		[]string{"agent_id"},
	)

	DropCount = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Namespace: "sennet",
			Name:      "drop_count_total",
			Help:      "Total dropped packets reported by agent",
		},
		[]string{"agent_id"},
	)

	UptimeSeconds = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Namespace: "sennet",
			Name:      "uptime_seconds",
			Help:      "Agent uptime in seconds",
		},
		[]string{"agent_id"},
	)

	// Event counters from RingBuf
	AnomalyEvents = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: "sennet",
			Name:      "anomaly_events_total",
			Help:      "Total anomaly events detected by eBPF",
		},
		[]string{"agent_id"},
	)

	LargePacketEvents = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: "sennet",
			Name:      "large_packet_events_total",
			Help:      "Total large packet events detected by eBPF",
		},
		[]string{"agent_id"},
	)

	// Backend metrics
	HeartbeatTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: "sennet",
			Name:      "heartbeat_total",
			Help:      "Total heartbeat requests received",
		},
		[]string{"agent_id"},
	)

	ActiveAgents = prometheus.NewGauge(
		prometheus.GaugeOpts{
			Namespace: "sennet",
			Name:      "active_agents",
			Help:      "Number of agents that sent heartbeat in last 5 minutes",
		},
	)

	initOnce sync.Once
)

// Init registers all metrics with Prometheus
func Init() {
	initOnce.Do(func() {
		prometheus.MustRegister(
			RxPackets,
			TxPackets,
			RxBytes,
			TxBytes,
			DropCount,
			UptimeSeconds,
			AnomalyEvents,
			LargePacketEvents,
			HeartbeatTotal,
			ActiveAgents,
		)
	})
}

// Handler returns the Prometheus HTTP handler
func Handler() http.Handler {
	return promhttp.Handler()
}

// UpdateAgentMetrics updates all metrics for an agent
func UpdateAgentMetrics(agentID string, rxPkts, txPkts, rxBytes, txBytes, drops, uptime uint64) {
	RxPackets.WithLabelValues(agentID).Set(float64(rxPkts))
	TxPackets.WithLabelValues(agentID).Set(float64(txPkts))
	RxBytes.WithLabelValues(agentID).Set(float64(rxBytes))
	TxBytes.WithLabelValues(agentID).Set(float64(txBytes))
	DropCount.WithLabelValues(agentID).Set(float64(drops))
	UptimeSeconds.WithLabelValues(agentID).Set(float64(uptime))
	HeartbeatTotal.WithLabelValues(agentID).Inc()
}

// RecordAnomalyEvent increments the anomaly counter for an agent
func RecordAnomalyEvent(agentID string) {
	AnomalyEvents.WithLabelValues(agentID).Inc()
}

// RecordLargePacketEvent increments the large packet counter for an agent
func RecordLargePacketEvent(agentID string) {
	LargePacketEvents.WithLabelValues(agentID).Inc()
}

// SetActiveAgents sets the number of active agents
func SetActiveAgents(count int) {
	ActiveAgents.Set(float64(count))
}
