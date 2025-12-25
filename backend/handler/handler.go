// Package handler implements the SentinelService RPC handlers
package handler

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"log"

	"connectrpc.com/connect"
	"github.com/sennet/sennet/backend/db"
	sentinelv1 "github.com/sennet/sennet/gen/go/sentinel/v1"
)

// SentinelHandler implements the SentinelService
type SentinelHandler struct {
	db            *db.DB
	latestVersion string
	configHash    string
}

// NewSentinelHandler creates a new handler with the given database and version
func NewSentinelHandler(database *db.DB, latestVersion string) *SentinelHandler {
	// Generate a simple config hash (in production, this would be based on actual config)
	hash := sha256.Sum256([]byte(latestVersion))
	configHash := hex.EncodeToString(hash[:8])

	return &SentinelHandler{
		db:            database,
		latestVersion: latestVersion,
		configHash:    configHash,
	}
}

// Heartbeat handles agent heartbeat requests
func (h *SentinelHandler) Heartbeat(
	ctx context.Context,
	req *connect.Request[sentinelv1.HeartbeatRequest],
) (*connect.Response[sentinelv1.HeartbeatResponse], error) {
	agentID := req.Msg.AgentId
	currentVersion := req.Msg.CurrentVersion
	metrics := req.Msg.Metrics

	// Log the heartbeat
	log.Printf("Heartbeat from agent %s (v%s)", agentID, currentVersion)
	if metrics != nil {
		log.Printf("  Metrics: rx=%d tx=%d drops=%d uptime=%ds",
			metrics.RxPackets, metrics.TxPackets, metrics.DropCount, metrics.UptimeSeconds)
	}

	// Update agent in database
	if err := h.db.CreateOrUpdateAgent(agentID, currentVersion); err != nil {
		log.Printf("Failed to update agent %s: %v", agentID, err)
		// Continue anyway - don't fail the heartbeat
	}

	// Determine command based on version comparison
	command := h.determineCommand(currentVersion)

	response := &sentinelv1.HeartbeatResponse{
		Command:       command,
		LatestVersion: h.latestVersion,
		ConfigHash:    h.configHash,
	}

	return connect.NewResponse(response), nil
}

// determineCommand compares versions and decides what command to send
func (h *SentinelHandler) determineCommand(currentVersion string) sentinelv1.Command {
	if currentVersion == "" {
		return sentinelv1.Command_COMMAND_NOOP
	}

	// Simple version comparison
	if needsUpgrade(currentVersion, h.latestVersion) {
		log.Printf("Agent version %s < %s, issuing UPGRADE command", currentVersion, h.latestVersion)
		return sentinelv1.Command_COMMAND_UPGRADE
	}

	return sentinelv1.Command_COMMAND_NOOP
}

// needsUpgrade compares semver strings and returns true if current < latest
func needsUpgrade(current, latest string) bool {
	// Parse versions (simple implementation)
	currParts := parseVersion(current)
	latestParts := parseVersion(latest)

	for i := 0; i < 3; i++ {
		currVal := 0
		latestVal := 0
		if i < len(currParts) {
			currVal = currParts[i]
		}
		if i < len(latestParts) {
			latestVal = latestParts[i]
		}

		if currVal < latestVal {
			return true
		}
		if currVal > latestVal {
			return false
		}
	}
	return false // Equal versions
}

// parseVersion parses "1.2.3" into []int{1, 2, 3}
func parseVersion(v string) []int {
	parts := make([]int, 0, 3)
	current := 0
	for _, c := range v {
		if c >= '0' && c <= '9' {
			current = current*10 + int(c-'0')
		} else if c == '.' {
			parts = append(parts, current)
			current = 0
		}
	}
	parts = append(parts, current)
	return parts
}

// SetLatestVersion updates the advertised latest version
func (h *SentinelHandler) SetLatestVersion(version string) {
	h.latestVersion = version
	hash := sha256.Sum256([]byte(version))
	h.configHash = hex.EncodeToString(hash[:8])
}
