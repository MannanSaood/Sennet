package handler

import (
	"encoding/json"
	"net/http"
	"sync"

	"github.com/sennet/sennet/backend/db"
)

type StatsHandler struct {
	database *db.DB
	mu       sync.RWMutex
	stats    *DashboardStats
}

func NewStatsHandler(database *db.DB) *StatsHandler {
	return &StatsHandler{
		database: database,
		stats:    &DashboardStats{},
	}
}

type DashboardStats struct {
	ActiveAgents  int    `json:"active_agents"`
	RxPackets     uint64 `json:"rx_packets"`
	TxPackets     uint64 `json:"tx_packets"`
	RxBytes       uint64 `json:"rx_bytes"`
	TxBytes       uint64 `json:"tx_bytes"`
	DropCount     uint64 `json:"drop_count"`
	UptimeSeconds uint64 `json:"uptime_seconds"`
	Timestamp     int64  `json:"timestamp"`
}

func (h *StatsHandler) HandleStats(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	h.mu.RLock()
	stats := *h.stats
	h.mu.RUnlock()

	activeCount, err := h.database.GetActiveAgentCount(5)
	if err == nil {
		stats.ActiveAgents = activeCount
	}

	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-cache")
	json.NewEncoder(w).Encode(stats)
}

func (h *StatsHandler) UpdateStats(rxPkts, txPkts, rxBytes, txBytes, drops, uptime uint64) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.stats.RxPackets += rxPkts
	h.stats.TxPackets += txPkts
	h.stats.RxBytes += rxBytes
	h.stats.TxBytes += txBytes
	h.stats.DropCount += drops
	if uptime > h.stats.UptimeSeconds {
		h.stats.UptimeSeconds = uptime
	}
}

func (h *StatsHandler) SetStats(rxPkts, txPkts, rxBytes, txBytes, drops, uptime uint64) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.stats.RxPackets = rxPkts
	h.stats.TxPackets = txPkts
	h.stats.RxBytes = rxBytes
	h.stats.TxBytes = txBytes
	h.stats.DropCount = drops
	h.stats.UptimeSeconds = uptime
}
