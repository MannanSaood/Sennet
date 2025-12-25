package handler_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"connectrpc.com/connect"
	"github.com/sennet/sennet/backend/db"
	"github.com/sennet/sennet/backend/handler"
	sentinelv1 "github.com/sennet/sennet/gen/go/sentinel/v1"
)

func setupTestHandler(t *testing.T, latestVersion string) (*handler.SentinelHandler, *db.DB, func()) {
	t.Helper()
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")

	database, err := db.New(dbPath)
	if err != nil {
		t.Fatalf("Failed to create test database: %v", err)
	}

	h := handler.NewSentinelHandler(database, latestVersion)

	cleanup := func() {
		database.Close()
		os.RemoveAll(tmpDir)
	}

	return h, database, cleanup
}

func TestHeartbeat_Success(t *testing.T) {
	h, _, cleanup := setupTestHandler(t, "1.0.0")
	defer cleanup()

	ctx := context.Background()
	req := connect.NewRequest(&sentinelv1.HeartbeatRequest{
		AgentId:        "test-agent-uuid",
		CurrentVersion: "1.0.0",
		Metrics: &sentinelv1.MetricsSummary{
			RxPackets:     1000,
			RxBytes:       1024000,
			TxPackets:     500,
			TxBytes:       512000,
			DropCount:     0,
			UptimeSeconds: 3600,
		},
	})

	resp, err := h.Heartbeat(ctx, req)
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	if resp.Msg.Command != sentinelv1.Command_COMMAND_NOOP {
		t.Errorf("Expected NOOP command, got: %v", resp.Msg.Command)
	}

	if resp.Msg.LatestVersion != "1.0.0" {
		t.Errorf("Expected latest version 1.0.0, got: %v", resp.Msg.LatestVersion)
	}
}

func TestHeartbeat_UpgradeNeeded(t *testing.T) {
	h, _, cleanup := setupTestHandler(t, "2.0.0")
	defer cleanup()

	ctx := context.Background()
	req := connect.NewRequest(&sentinelv1.HeartbeatRequest{
		AgentId:        "test-agent-uuid",
		CurrentVersion: "1.0.0", // Older than 2.0.0
	})

	resp, err := h.Heartbeat(ctx, req)
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	if resp.Msg.Command != sentinelv1.Command_COMMAND_UPGRADE {
		t.Errorf("Expected UPGRADE command, got: %v", resp.Msg.Command)
	}

	if resp.Msg.LatestVersion != "2.0.0" {
		t.Errorf("Expected latest version 2.0.0, got: %v", resp.Msg.LatestVersion)
	}
}

func TestHeartbeat_SameVersion(t *testing.T) {
	h, _, cleanup := setupTestHandler(t, "1.5.0")
	defer cleanup()

	ctx := context.Background()
	req := connect.NewRequest(&sentinelv1.HeartbeatRequest{
		AgentId:        "test-agent-uuid",
		CurrentVersion: "1.5.0", // Same as server
	})

	resp, err := h.Heartbeat(ctx, req)
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	if resp.Msg.Command != sentinelv1.Command_COMMAND_NOOP {
		t.Errorf("Expected NOOP for same version, got: %v", resp.Msg.Command)
	}
}

func TestHeartbeat_NewerVersion(t *testing.T) {
	h, _, cleanup := setupTestHandler(t, "1.0.0")
	defer cleanup()

	ctx := context.Background()
	req := connect.NewRequest(&sentinelv1.HeartbeatRequest{
		AgentId:        "test-agent-uuid",
		CurrentVersion: "2.0.0", // Newer than server (edge case)
	})

	resp, err := h.Heartbeat(ctx, req)
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	// Should be NOOP - agent is newer, no downgrade
	if resp.Msg.Command != sentinelv1.Command_COMMAND_NOOP {
		t.Errorf("Expected NOOP for newer agent, got: %v", resp.Msg.Command)
	}
}

func TestHeartbeat_AgentPersisted(t *testing.T) {
	h, database, cleanup := setupTestHandler(t, "1.0.0")
	defer cleanup()

	ctx := context.Background()
	agentID := "persisted-agent-uuid"

	req := connect.NewRequest(&sentinelv1.HeartbeatRequest{
		AgentId:        agentID,
		CurrentVersion: "1.0.0",
	})

	_, err := h.Heartbeat(ctx, req)
	if err != nil {
		t.Fatalf("Heartbeat failed: %v", err)
	}

	// Verify agent was saved to database
	agent, err := database.GetAgent(agentID)
	if err != nil {
		t.Fatalf("Failed to get agent: %v", err)
	}

	if agent == nil {
		t.Fatal("Expected agent to be persisted")
	}

	if agent.ID != agentID {
		t.Errorf("Expected agent ID %s, got %s", agentID, agent.ID)
	}

	if agent.Version != "1.0.0" {
		t.Errorf("Expected version 1.0.0, got %s", agent.Version)
	}
}

func TestHeartbeat_ConfigHash(t *testing.T) {
	h, _, cleanup := setupTestHandler(t, "1.0.0")
	defer cleanup()

	ctx := context.Background()
	req := connect.NewRequest(&sentinelv1.HeartbeatRequest{
		AgentId:        "test-agent",
		CurrentVersion: "1.0.0",
	})

	resp, _ := h.Heartbeat(ctx, req)

	if resp.Msg.ConfigHash == "" {
		t.Error("Expected non-empty config hash")
	}

	// Config hash should be consistent for same version
	resp2, _ := h.Heartbeat(ctx, req)
	if resp.Msg.ConfigHash != resp2.Msg.ConfigHash {
		t.Error("Expected consistent config hash")
	}
}

func TestHeartbeat_EmptyVersion(t *testing.T) {
	h, _, cleanup := setupTestHandler(t, "1.0.0")
	defer cleanup()

	ctx := context.Background()
	req := connect.NewRequest(&sentinelv1.HeartbeatRequest{
		AgentId:        "test-agent",
		CurrentVersion: "", // Empty version
	})

	resp, err := h.Heartbeat(ctx, req)
	if err != nil {
		t.Fatalf("Expected no error for empty version, got: %v", err)
	}

	// Should be NOOP, not crash
	if resp.Msg.Command != sentinelv1.Command_COMMAND_NOOP {
		t.Errorf("Expected NOOP for empty version, got: %v", resp.Msg.Command)
	}
}

func TestHeartbeat_MinorVersionUpgrade(t *testing.T) {
	h, _, cleanup := setupTestHandler(t, "1.2.0")
	defer cleanup()

	ctx := context.Background()
	req := connect.NewRequest(&sentinelv1.HeartbeatRequest{
		AgentId:        "test-agent",
		CurrentVersion: "1.1.0",
	})

	resp, _ := h.Heartbeat(ctx, req)

	if resp.Msg.Command != sentinelv1.Command_COMMAND_UPGRADE {
		t.Errorf("Expected UPGRADE for minor version bump, got: %v", resp.Msg.Command)
	}
}

func TestHeartbeat_PatchVersionUpgrade(t *testing.T) {
	h, _, cleanup := setupTestHandler(t, "1.0.5")
	defer cleanup()

	ctx := context.Background()
	req := connect.NewRequest(&sentinelv1.HeartbeatRequest{
		AgentId:        "test-agent",
		CurrentVersion: "1.0.3",
	})

	resp, _ := h.Heartbeat(ctx, req)

	if resp.Msg.Command != sentinelv1.Command_COMMAND_UPGRADE {
		t.Errorf("Expected UPGRADE for patch version bump, got: %v", resp.Msg.Command)
	}
}
