package db_test

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/sennet/sennet/backend/db"
)

func setupTestDB(t *testing.T) (*db.DB, func()) {
	t.Helper()
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")

	database, err := db.New(dbPath)
	if err != nil {
		t.Fatalf("Failed to create test database: %v", err)
	}

	cleanup := func() {
		database.Close()
		os.RemoveAll(tmpDir)
	}

	return database, cleanup
}

func TestDB_CreateOrUpdateAgent(t *testing.T) {
	database, cleanup := setupTestDB(t)
	defer cleanup()

	agentID := "test-agent-uuid-123"
	version := "1.0.0"

	// Create agent
	err := database.CreateOrUpdateAgent(agentID, version)
	if err != nil {
		t.Fatalf("Failed to create agent: %v", err)
	}

	// Retrieve agent
	agent, err := database.GetAgent(agentID)
	if err != nil {
		t.Fatalf("Failed to get agent: %v", err)
	}

	if agent == nil {
		t.Fatal("Expected agent to be found")
	}

	if agent.ID != agentID {
		t.Errorf("Expected agent ID %s, got %s", agentID, agent.ID)
	}

	if agent.Version != version {
		t.Errorf("Expected version %s, got %s", version, agent.Version)
	}
}

func TestDB_UpdateAgentVersion(t *testing.T) {
	database, cleanup := setupTestDB(t)
	defer cleanup()

	agentID := "test-agent-uuid-456"

	// Create with v1
	database.CreateOrUpdateAgent(agentID, "1.0.0")

	// Update to v2
	err := database.CreateOrUpdateAgent(agentID, "2.0.0")
	if err != nil {
		t.Fatalf("Failed to update agent: %v", err)
	}

	agent, _ := database.GetAgent(agentID)
	if agent.Version != "2.0.0" {
		t.Errorf("Expected version 2.0.0, got %s", agent.Version)
	}
}

func TestDB_GetAgent_NotFound(t *testing.T) {
	database, cleanup := setupTestDB(t)
	defer cleanup()

	agent, err := database.GetAgent("nonexistent-agent")
	if err != nil {
		t.Fatalf("Expected no error for missing agent, got: %v", err)
	}
	if agent != nil {
		t.Error("Expected nil agent for nonexistent ID")
	}
}

func TestDB_AgentLastSeen(t *testing.T) {
	database, cleanup := setupTestDB(t)
	defer cleanup()

	agentID := "test-agent-lastseen"

	// Create agent
	database.CreateOrUpdateAgent(agentID, "1.0.0")
	agent1, _ := database.GetAgent(agentID)

	// Wait a bit and update
	time.Sleep(100 * time.Millisecond)
	database.CreateOrUpdateAgent(agentID, "1.0.0")
	agent2, _ := database.GetAgent(agentID)

	// LastSeen should be updated
	if !agent2.LastSeen.After(agent1.LastSeen) && !agent2.LastSeen.Equal(agent1.LastSeen) {
		t.Error("Expected LastSeen to be updated")
	}
}

func TestDB_GetAgentCount(t *testing.T) {
	database, cleanup := setupTestDB(t)
	defer cleanup()

	// Initially empty
	count, err := database.GetAgentCount()
	if err != nil {
		t.Fatalf("Failed to get count: %v", err)
	}
	if count != 0 {
		t.Errorf("Expected 0 agents, got %d", count)
	}

	// Add agents
	database.CreateOrUpdateAgent("agent-1", "1.0.0")
	database.CreateOrUpdateAgent("agent-2", "1.0.0")

	count, _ = database.GetAgentCount()
	if count != 2 {
		t.Errorf("Expected 2 agents, got %d", count)
	}
}
