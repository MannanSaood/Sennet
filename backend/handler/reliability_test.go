package handler_test

import (
	"connectrpc.com/connect"
	"context"
	sentinelv1 "github.com/sennet/sennet/gen/go/sentinel/v1"
	"testing"
)

func TestHeartbeatRejectsEmptyIdentity(t *testing.T) {
	h, _, cleanup := setupTestHandler(t, "1.0.0")
	defer cleanup()
	_, err := h.Heartbeat(context.Background(), connect.NewRequest(&sentinelv1.HeartbeatRequest{AgentId: "  "}))
	if connect.CodeOf(err) != connect.CodeInvalidArgument {
		t.Fatalf("expected invalid argument, got %v", err)
	}
}

func TestHeartbeatDoesNotAcknowledgeDatabaseFailure(t *testing.T) {
	h, database, cleanup := setupTestHandler(t, "1.0.0")
	defer cleanup()
	database.Close()
	_, err := h.Heartbeat(context.Background(), connect.NewRequest(&sentinelv1.HeartbeatRequest{AgentId: "agent-1"}))
	if connect.CodeOf(err) != connect.CodeUnavailable {
		t.Fatalf("expected unavailable, got %v", err)
	}
}
