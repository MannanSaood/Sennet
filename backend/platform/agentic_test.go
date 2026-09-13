package platform

import (
	"context"
	"encoding/json"
	"testing"
	"time"
)

func agentEvent(id, span, parent, kind, task string, at int64, duration float64) Event {
	a := map[string]string{"sennet.agent.convention": "sennet.agent.v1", "agent.event.kind": kind, "agent.run.id": "run-1"}
	if task != "" {
		a["agent.task.id"] = task
	}
	return Event{ID: id, Time: at, Signal: "agent", Service: "agents", Name: "agent." + kind, TraceID: "trace", SpanID: span, ParentID: parent, Duration: duration, Status: "ok", Attributes: a}
}
func TestAgentRunDeterministicIsolationMissingSpanAndProvenance(t *testing.T) {
	s, h, a, b := fixture(t)
	now := time.Now().UnixMilli()
	events := []Event{agentEvent("1", "root", "", "run", "", now, 1000), agentEvent("2", "branch-a", "root", "task", "research", now+10, 500), agentEvent("3", "retry", "branch-a", "retry", "research", now+100, 700), agentEvent("4", "missing", "absent", "task", "late", now+900, 20)}
	events[2].Attributes["gen_ai.usage.input_tokens"] = "9007199254740993"
	events[2].Attributes["gen_ai.usage.output_tokens"] = "11"
	events[2].Attributes["gen_ai.cost.estimated"] = "0.12"
	events[2].Attributes["gen_ai.cost.observed"] = "0.13"
	events[2].Attributes["gen_ai.pricing.version"] = "price-v7"
	if err := validateEvents("a", events); err != nil {
		t.Fatal(err)
	}
	if err := s.Write(context.Background(), events); err != nil {
		t.Fatal(err)
	}
	path := "/api/agent-runs?run_id=run-1&from=" + strconv64(now-1) + "&to=" + strconv64(now+2000)
	w := call(h, "GET", path, a, nil)
	if w.Code != 200 {
		t.Fatal(w.Body.String())
	}
	var got AgentRunResponse
	if err := json.Unmarshal(w.Body.Bytes(), &got); err != nil {
		t.Fatal(err)
	}
	if len(got.Nodes) != 4 || !got.Nodes[3].MissingParent || len(got.CriticalPath) != 3 || got.CriticalPath[2] != "retry" {
		t.Fatalf("bad DAG: %+v", got)
	}
	if len(got.TokenCostWaterfall) != 1 || got.TokenCostWaterfall[0].Input != "9007199254740993" || got.TokenCostWaterfall[0].PricingVersion != "price-v7" || got.TokenCostWaterfall[0].Estimated == got.TokenCostWaterfall[0].Observed {
		t.Fatalf("lost provenance: %+v", got.TokenCostWaterfall)
	}
	if w = call(h, "GET", path, b, nil); w.Code != 200 {
		t.Fatal(w.Code)
	}
	if err := json.Unmarshal(w.Body.Bytes(), &got); err != nil || len(got.Nodes) != 0 {
		t.Fatal("cross-tenant run disclosure")
	}
	// Idempotent retry preserves one copy and therefore the same DAG.
	if err := s.Write(context.Background(), events); err != nil {
		t.Fatal(err)
	}
	w = call(h, "GET", path, a, nil)
	var again AgentRunResponse
	_ = json.Unmarshal(w.Body.Bytes(), &again)
	if len(again.Nodes) != 4 || len(again.CriticalPath) != 3 {
		t.Fatal("retry changed DAG")
	}
}

func TestAgentTokenAndConventionValidation(t *testing.T) {
	e := agentEvent("x", "s", "", "model", "", time.Now().UnixMilli(), 1)
	e.Attributes["gen_ai.usage.input_tokens"] = "1.5"
	if validateEvent(&e, true) == nil {
		t.Fatal("fractional tokens accepted")
	}
	e.Attributes["gen_ai.usage.input_tokens"] = "12"
	e.Attributes["sennet.agent.convention"] = "future"
	if validateEvent(&e, true) == nil {
		t.Fatal("unknown convention accepted")
	}
	e.Attributes["sennet.agent.convention"] = "sennet.agent.v1"
	e.Attributes["content.body"] = "raw"
	e.Attributes["content.captured"] = "false"
	if validateEvent(&e, true) == nil {
		t.Fatal("content without explicit retention accepted")
	}
}
