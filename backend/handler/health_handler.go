package handler

import (
	"encoding/json"
	"net/http"
	"runtime"
	"time"

	"github.com/sennet/sennet/backend/db"
)

type HealthHandler struct {
	database  *db.DB
	startTime time.Time
	version   string
}

func NewHealthHandler(database *db.DB, version string) *HealthHandler {
	return &HealthHandler{
		database:  database,
		startTime: time.Now(),
		version:   version,
	}
}

type HealthResponse struct {
	Status    string            `json:"status"`
	Version   string            `json:"version"`
	Uptime    string            `json:"uptime"`
	Checks    map[string]string `json:"checks,omitempty"`
	Timestamp string            `json:"timestamp"`
}

func (h *HealthHandler) HandleHealth(w http.ResponseWriter, r *http.Request) {
	response := HealthResponse{
		Status:    "ok",
		Version:   h.version,
		Uptime:    time.Since(h.startTime).Round(time.Second).String(),
		Timestamp: time.Now().UTC().Format(time.RFC3339),
		Checks:    make(map[string]string),
	}

	if err := h.database.Ping(); err != nil {
		response.Status = "degraded"
		response.Checks["database"] = "error: " + err.Error()
	} else {
		response.Checks["database"] = "ok"
	}

	w.Header().Set("Content-Type", "application/json")
	if response.Status != "ok" {
		w.WriteHeader(http.StatusServiceUnavailable)
	}
	json.NewEncoder(w).Encode(response)
}

func (h *HealthHandler) HandleReady(w http.ResponseWriter, r *http.Request) {
	if err := h.database.Ping(); err != nil {
		http.Error(w, "not ready", http.StatusServiceUnavailable)
		return
	}
	w.WriteHeader(http.StatusOK)
	w.Write([]byte("ready"))
}

func (h *HealthHandler) HandleLive(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
	w.Write([]byte("live"))
}

type RuntimeInfo struct {
	GoVersion    string `json:"go_version"`
	NumGoroutine int    `json:"goroutines"`
	NumCPU       int    `json:"cpus"`
	GOOS         string `json:"os"`
	GOARCH       string `json:"arch"`
}

func (h *HealthHandler) HandleDebug(w http.ResponseWriter, r *http.Request) {
	info := RuntimeInfo{
		GoVersion:    runtime.Version(),
		NumGoroutine: runtime.NumGoroutine(),
		NumCPU:       runtime.NumCPU(),
		GOOS:         runtime.GOOS,
		GOARCH:       runtime.GOARCH,
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(info)
}
