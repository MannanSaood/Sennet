package platform

import (
	"net/http"
	"sort"
	"strconv"
	"strings"
)

type AgentNode struct {
	Event
	Kind          string   `json:"kind"`
	Links         []string `json:"links,omitempty"`
	MissingParent bool     `json:"missing_parent"`
	Critical      bool     `json:"critical"`
}
type AgentFinding struct {
	Kind  string `json:"kind"`
	Key   string `json:"key"`
	Count int    `json:"count"`
}
type TokenCost struct {
	SpanID         string `json:"span_id"`
	Input          string `json:"input_tokens"`
	Output         string `json:"output_tokens"`
	Cached         string `json:"cached_tokens"`
	Estimated      string `json:"estimated_cost,omitempty"`
	Observed       string `json:"observed_cost,omitempty"`
	Currency       string `json:"currency,omitempty"`
	PricingVersion string `json:"pricing_version,omitempty"`
}
type EvaluationComparison struct {
	Evaluator string `json:"evaluator"`
	Version   string `json:"version"`
	Score     string `json:"score"`
	Outcome   string `json:"outcome"`
	SpanID    string `json:"span_id"`
}
type AgentRunResponse struct {
	Convention             string                 `json:"convention"`
	RunID                  string                 `json:"run_id"`
	Nodes                  []AgentNode            `json:"nodes"`
	CriticalPath           []string               `json:"critical_path"`
	Findings               []AgentFinding         `json:"findings"`
	TokenCostWaterfall     []TokenCost            `json:"token_cost_waterfall"`
	Evaluations            []EvaluationComparison `json:"evaluations"`
	QueueWaitMS            string                 `json:"queue_wait_ms"`
	InfrastructureTraceIDs []string               `json:"infrastructure_trace_ids"`
	Partial                bool                   `json:"partial"`
	Scanned                int                    `json:"scanned"`
}

func (a *API) agentRun(w http.ResponseWriter, r *http.Request) {
	if r.Method != "GET" {
		problem(w, 405, "GET required")
		return
	}
	run := r.URL.Query().Get("run_id")
	if run == "" || len(run) > 160 {
		problem(w, 400, "bounded run_id required")
		return
	}
	from, to := parseWindow(r)
	events, partial, err := boundedEvents(r.Context(), a.Telemetry, principal(r).WorkspaceID, Query{From: from, To: to, Signal: "agent"}, 10000)
	if err != nil {
		problem(w, 503, "agent investigation unavailable")
		return
	}
	filtered := make([]Event, 0)
	for _, e := range events {
		if e.Attributes["agent.run.id"] == run {
			filtered = append(filtered, e)
		}
	}
	sort.Slice(filtered, func(i, j int) bool {
		if filtered[i].Time == filtered[j].Time {
			return filtered[i].ID < filtered[j].ID
		}
		return filtered[i].Time < filtered[j].Time
	})
	byID := map[string]Event{}
	children := map[string][]string{}
	attempts := map[string]int{}
	tools := map[string]int{}
	findings := []AgentFinding{}
	nodes := []AgentNode{}
	waterfall := []TokenCost{}
	evals := []EvaluationComparison{}
	infraSet := map[string]bool{}
	var queue uint64
	for _, e := range filtered {
		if e.SpanID != "" {
			byID[e.SpanID] = e
		}
		if e.ParentID != "" {
			children[e.ParentID] = append(children[e.ParentID], e.SpanID)
		}
		task := e.Attributes["agent.task.id"]
		if task != "" {
			attempts[task]++
		}
		if e.Attributes["agent.event.kind"] == "tool" && e.Status == "error" {
			tools[e.Attributes["gen_ai.tool.name"]]++
		}
		q, _ := strconv.ParseUint(e.Attributes["agent.queue.wait_ms"], 10, 64)
		queue += q
		if t := e.Attributes["infra.trace_id"]; t != "" {
			infraSet[t] = true
		}
	}
	// Longest causal chain by cumulative span duration. Recursive lookup makes
	// reconstruction independent of arrival/event-time order, including late spans.
	end := map[string]int64{}
	path := map[string][]string{}
	visiting := map[string]bool{}
	var buildPath func(string) (int64, []string)
	buildPath = func(id string) (int64, []string) {
		if p, ok := path[id]; ok {
			return end[id], p
		}
		e, ok := byID[id]
		if !ok || visiting[id] {
			return 0, nil
		}
		visiting[id] = true
		parentDuration, parentPath := buildPath(e.ParentID)
		delete(visiting, id)
		duration := parentDuration + int64(e.Duration*1000)
		p := append(append([]string{}, parentPath...), id)
		end[id], path[id] = duration, p
		return duration, p
	}
	for id := range byID {
		buildPath(id)
	}
	critical := []string{}
	var bestEnd int64
	for id, v := range end {
		p := path[id]
		if v > bestEnd || (v == bestEnd && strings.Join(p, "/") < strings.Join(critical, "/")) {
			bestEnd = v
			critical = p
		}
	}
	criticalSet := map[string]bool{}
	for _, id := range critical {
		criticalSet[id] = true
	}
	for _, e := range filtered {
		kind := e.Attributes["agent.event.kind"]
		nodes = append(nodes, AgentNode{Event: e, Kind: kind, Links: links(e), MissingParent: e.ParentID != "" && byID[e.ParentID].SpanID == "", Critical: criticalSet[e.SpanID]})
		if e.Attributes["gen_ai.usage.input_tokens"] != "" || e.Attributes["gen_ai.cost.estimated"] != "" || e.Attributes["gen_ai.cost.observed"] != "" {
			waterfall = append(waterfall, TokenCost{e.SpanID, e.Attributes["gen_ai.usage.input_tokens"], e.Attributes["gen_ai.usage.output_tokens"], e.Attributes["gen_ai.usage.cached_tokens"], e.Attributes["gen_ai.cost.estimated"], e.Attributes["gen_ai.cost.observed"], e.Attributes["gen_ai.cost.currency"], e.Attributes["gen_ai.pricing.version"]})
		}
		if kind == "evaluation" {
			evals = append(evals, EvaluationComparison{e.Attributes["evaluation.evaluator"], e.Attributes["evaluation.evaluator.version"], e.Attributes["evaluation.score"], e.Attributes["agent.outcome"], e.SpanID})
		}
	}
	for task, n := range attempts {
		if n > 1 {
			findings = append(findings, AgentFinding{"retry", task, n})
		}
		if n > 5 {
			findings = append(findings, AgentFinding{"retry_storm", task, n})
		}
	}
	for tool, n := range tools {
		findings = append(findings, AgentFinding{"tool_failure", tool, n})
	}
	// A repeated task on its own ancestry is a semantic loop.
	for _, e := range filtered {
		for p := e.ParentID; p != ""; {
			pe, ok := byID[p]
			if !ok {
				break
			}
			if e.Attributes["agent.task.id"] != "" && pe.Attributes["agent.task.id"] == e.Attributes["agent.task.id"] {
				findings = append(findings, AgentFinding{"loop", e.Attributes["agent.task.id"], 1})
				break
			}
			p = pe.ParentID
		}
	}
	sort.Slice(findings, func(i, j int) bool { return findings[i].Kind+findings[i].Key < findings[j].Kind+findings[j].Key })
	infra := []string{}
	for id := range infraSet {
		infra = append(infra, id)
	}
	sort.Strings(infra)
	respond(w, 200, AgentRunResponse{"sennet.agent.v1", run, nodes, critical, findings, waterfall, evals, strconv.FormatUint(queue, 10), infra, partial, len(events)})
}
