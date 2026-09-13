package platform

import (
	"context"
	"encoding/base64"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"
)

type TopologyEdge struct {
	Source      string  `json:"source"`
	Destination string  `json:"destination"`
	Calls       int64   `json:"calls"`
	Errors      int64   `json:"errors"`
	DurationMS  float64 `json:"duration_ms"`
}
type TopologyResponse struct {
	Edges         []TopologyEdge `json:"edges"`
	Clusters      map[string]int `json:"clusters"`
	NextCursor    string         `json:"next_cursor,omitempty"`
	Scanned       int            `json:"scanned"`
	Partial       bool           `json:"partial"`
	PartialReason string         `json:"partial_reason,omitempty"`
}
type TraceSpan struct {
	Event
	Links         []string `json:"links,omitempty"`
	MissingParent bool     `json:"missing_parent"`
	Late          bool     `json:"late"`
	Critical      bool     `json:"critical"`
	FanOut        int      `json:"fan_out"`
	FanIn         int      `json:"fan_in"`
}
type TraceResponse struct {
	TraceID   string      `json:"trace_id"`
	Spans     []TraceSpan `json:"spans"`
	Logs      []Event     `json:"logs"`
	Metrics   []Event     `json:"metrics"`
	Partial   bool        `json:"partial"`
	ClockSkew bool        `json:"clock_skew_detected"`
}

func boundedEvents(ctx context.Context, telemetry Telemetry, tenant string, q Query, max int) ([]Event, bool, error) {
	out := []Event{}
	q.Limit = 1000
	for len(out) < max {
		page, err := telemetry.Query(ctx, tenant, q)
		if err != nil {
			return nil, false, err
		}
		out = append(out, page.Events...)
		if page.Cursor == "" {
			return out, page.Partial, nil
		}
		q.Cursor = page.Cursor
	}
	return out[:max], true, nil
}

func parseWindow(r *http.Request) (int64, int64) {
	to, _ := strconv.ParseInt(r.URL.Query().Get("to"), 10, 64)
	from, _ := strconv.ParseInt(r.URL.Query().Get("from"), 10, 64)
	if to == 0 {
		to = time.Now().UnixMilli()
	}
	if from == 0 {
		from = to - int64(time.Hour/time.Millisecond)
	}
	return from, to
}

func (a *API) topology(w http.ResponseWriter, r *http.Request) {
	if r.Method != "GET" {
		problem(w, 405, "GET required")
		return
	}
	from, to := parseWindow(r)
	events, partial, err := boundedEvents(r.Context(), a.Telemetry, principal(r).WorkspaceID, Query{From: from, To: to, Signal: "trace", Service: r.URL.Query().Get("service"), Search: r.URL.Query().Get("search")}, 100000)
	if err != nil {
		problem(w, 503, "topology unavailable")
		return
	}
	spans := map[string]Event{}
	for _, e := range events {
		if e.SpanID != "" {
			spans[e.TraceID+":"+e.SpanID] = e
		}
	}
	edges := map[string]*TopologyEdge{}
	clusters := map[string]int{}
	for _, e := range events {
		p := spans[e.TraceID+":"+e.ParentID]
		source := p.Service
		if source == "" {
			source = e.Attributes["source.service"]
		}
		dest := e.Attributes["destination.service"]
		if dest == "" {
			dest = e.Service
		}
		if source == "" || source == dest {
			continue
		}
		key := source + "\x00" + dest
		edge := edges[key]
		if edge == nil {
			edge = &TopologyEdge{Source: source, Destination: dest}
			edges[key] = edge
		}
		edge.Calls++
		edge.DurationMS += e.Duration
		if e.Status == "error" {
			edge.Errors++
		}
	}
	all := make([]TopologyEdge, 0, len(edges))
	for _, e := range edges {
		all = append(all, *e)
		for _, s := range []string{e.Source, e.Destination} {
			parts := strings.FieldsFunc(s, func(r rune) bool { return r == '.' || r == '/' || r == '-' })
			cluster := s
			if len(parts) > 0 {
				cluster = parts[0]
			}
			clusters[cluster]++
		}
	}
	sort.Slice(all, func(i, j int) bool {
		if all[i].Calls == all[j].Calls {
			return all[i].Source+all[i].Destination < all[j].Source+all[j].Destination
		}
		return all[i].Calls > all[j].Calls
	})
	offset := 0
	if raw := r.URL.Query().Get("cursor"); raw != "" {
		b, decodeErr := base64.RawURLEncoding.DecodeString(raw)
		parsed, parseErr := strconv.Atoi(string(b))
		if decodeErr != nil || parseErr != nil || parsed < 0 || parsed > len(all) {
			problem(w, 400, "invalid topology cursor")
			return
		}
		offset = parsed
	}
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	if limit < 1 || limit > 1000 {
		limit = 500
	}
	end := offset + limit
	if end > len(all) {
		end = len(all)
	}
	next := ""
	if end < len(all) {
		next = base64.RawURLEncoding.EncodeToString([]byte(strconv.Itoa(end)))
	}
	respond(w, 200, TopologyResponse{Edges: all[offset:end], Clusters: clusters, NextCursor: next, Scanned: len(events), Partial: partial, PartialReason: map[bool]string{true: "scan_budget"}[partial]})
}

func links(e Event) []string {
	out := []string{}
	for k, v := range e.Attributes {
		if strings.Contains(strings.ToLower(k), "link") {
			for _, x := range strings.Split(v, ",") {
				if x = strings.TrimSpace(x); x != "" {
					out = append(out, x)
				}
			}
		}
	}
	return out
}
func (a *API) traceInvestigation(w http.ResponseWriter, r *http.Request) {
	if r.Method != "GET" {
		problem(w, 405, "GET required")
		return
	}
	id := r.URL.Query().Get("trace_id")
	if id == "" || len(id) > 128 {
		problem(w, 400, "bounded trace_id required")
		return
	}
	from, to := parseWindow(r)
	events, partial, err := boundedEvents(r.Context(), a.Telemetry, principal(r).WorkspaceID, Query{From: from, To: to, Trace: id}, 10000)
	if err != nil {
		problem(w, 503, "trace unavailable")
		return
	}
	spans := []TraceSpan{}
	logs := []Event{}
	metrics := []Event{}
	ids := map[string]bool{}
	children := map[string]int{}
	for _, e := range events {
		if e.SpanID != "" {
			ids[e.SpanID] = true
			children[e.ParentID]++
		}
		if e.Signal == "log" {
			logs = append(logs, e)
		}
		if e.Signal == "metric" {
			metrics = append(metrics, e)
		}
	}
	sort.Slice(events, func(i, j int) bool { return events[i].Time < events[j].Time })
	critical := map[string]bool{}
	var tip Event
	for _, e := range events {
		if e.SpanID != "" && e.Time+int64(e.Duration) > tip.Time+int64(tip.Duration) {
			tip = e
		}
	}
	for tip.SpanID != "" {
		critical[tip.SpanID] = true
		parent := ""
		for _, e := range events {
			if e.SpanID == tip.ParentID {
				tip = e
				parent = e.SpanID
				break
			}
		}
		if parent == "" {
			break
		}
	}
	clockSkew := false
	for _, e := range events {
		if e.SpanID == "" {
			continue
		}
		missing := e.ParentID != "" && !ids[e.ParentID]
		late := e.Attributes["telemetry.late"] == "true" || e.Attributes["span.late"] == "true"
		for _, p := range events {
			if p.SpanID == e.ParentID && e.Time < p.Time {
				clockSkew = true
			}
		}
		spans = append(spans, TraceSpan{Event: e, Links: links(e), MissingParent: missing, Late: late, Critical: critical[e.SpanID], FanOut: children[e.SpanID], FanIn: children[e.ParentID]})
	}
	respond(w, 200, TraceResponse{TraceID: id, Spans: spans, Logs: logs, Metrics: metrics, Partial: partial, ClockSkew: clockSkew})
}

func (a *API) pipelineHealth(w http.ResponseWriter, r *http.Request) {
	if r.Method != "GET" {
		problem(w, 405, "GET required")
		return
	}
	m := a.Metrics
	if m == nil {
		respond(w, 200, map[string]any{"configured": false, "stages": []any{}})
		return
	}
	storageState := "idle"
	if m.StorageWrites.Load() > 0 {
		storageState = "healthy"
	}
	stages := []map[string]any{{"name": "collector admission", "state": state(m.QueueSaturated.Load() == 0), "accepted": m.AcceptedEvents.Load(), "rejected": m.RejectedEvents.Load()}, {"name": "gateway append", "state": state(m.ProducerErrors.Load() == 0), "errors": m.ProducerErrors.Load(), "in_flight": m.IngestInFlight.Load()}, {"name": "storage queue", "state": state(m.ConsumerLag.Load() == 0 && m.QueueAgeMillis.Load() < 30000), "lag": m.ConsumerLag.Load(), "queue_age_ms": m.QueueAgeMillis.Load()}, {"name": "analytics storage", "state": storageState, "writes": m.StorageWrites.Load(), "latency_ms_total": m.StorageLatencyNanos.Load() / 1e6}}
	respond(w, 200, map[string]any{"configured": true, "scope": "deployment", "timestamp": time.Now().UnixMilli(), "stages": stages})
}
func state(ok bool) string {
	if ok {
		return "healthy"
	}
	return "degraded"
}

func (a *API) notificationStatus(w http.ResponseWriter, r *http.Request) {
	if r.Method != "GET" {
		problem(w, 405, "GET required")
		return
	}
	var pending, delivered int64
	err := a.Store.db.QueryRowContext(r.Context(), a.Store.q(`SELECT COUNT(*) FROM platform_notification_outbox WHERE tenant=? AND delivered=0`), principal(r).WorkspaceID).Scan(&pending)
	if err == nil {
		err = a.Store.db.QueryRowContext(r.Context(), a.Store.q(`SELECT COUNT(*) FROM platform_notification_outbox WHERE tenant=? AND delivered>0`), principal(r).WorkspaceID).Scan(&delivered)
	}
	if err != nil {
		problem(w, 503, "notification status unavailable")
		return
	}
	respond(w, 200, map[string]any{"provider_configured": false, "pending": pending, "delivered": delivered, "delivery_boundary": "outbox acknowledgement"})
}

func (a *API) dashboardVersions(w http.ResponseWriter, r *http.Request) {
	id := r.URL.Query().Get("id")
	if id == "" || len(id) > 128 {
		problem(w, 400, "dashboard id required")
		return
	}
	if r.Method != "GET" {
		problem(w, 405, "GET required")
		return
	}
	versions, err := a.Store.Resources(r.Context(), principal(r), "dashboard-version:"+id)
	if err != nil {
		problem(w, 503, "dashboard versions unavailable")
		return
	}
	respond(w, 200, versions)
}
