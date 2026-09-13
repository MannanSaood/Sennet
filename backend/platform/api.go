package platform

import (
	"bytes"
	"compress/gzip"
	"context"
	"crypto/hmac"
	"encoding/json"
	"errors"
	"io"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"
)

type Resolver func(context.Context, string, string) (Principal, error)
type API struct {
	Store         *Store
	Telemetry     Telemetry
	Ingest        Ingestor
	Resolve       Resolver
	Origins       []string
	Mode          string
	Role          string
	Brokers       []string
	Ready         func(context.Context) error
	Metrics       *DataPlaneMetrics
	InternalToken string
	DeadLetters   DeadLetterAdmin
	queries       chan struct{}
	limiter       *localLimiter
	Quota         Quota
	Replay        ReplayStore
	Proxies       TrustedProxies
}
type contextKey struct{}

func principal(r *http.Request) Principal {
	p, _ := r.Context().Value(contextKey{}).(Principal)
	return p
}
func NewAPI(s *Store, t Telemetry, in Ingestor) *API {
	return &API{Store: s, Telemetry: t, Ingest: in, queries: make(chan struct{}, 16), limiter: &localLimiter{entries: map[string]localBucket{}}, Quota: NewSQLQuota(s), Replay: NewSQLReplayStore(s)}
}
func respond(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}
func problem(w http.ResponseWriter, status int, msg string) {
	respond(w, status, map[string]string{"error": msg})
}
func decode(r *http.Request, v any) error {
	d := json.NewDecoder(r.Body)
	d.DisallowUnknownFields()
	if err := d.Decode(v); err != nil {
		return err
	}
	var extra any
	if err := d.Decode(&extra); err != io.EOF {
		return errors.New("one JSON object required")
	}
	return nil
}
func (a *API) Handler() http.Handler { return http.HandlerFunc(a.serve) }
func (a *API) serve(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Header().Set("X-Frame-Options", "DENY")
	w.Header().Set("Referrer-Policy", "no-referrer")
	origin := r.Header.Get("Origin")
	if origin != "" {
		allowed := false
		for _, o := range a.Origins {
			if origin == o {
				allowed = true
			}
		}
		if !allowed {
			problem(w, 403, "origin not allowed")
			return
		}
		w.Header().Set("Access-Control-Allow-Origin", origin)
		w.Header().Set("Vary", "Origin")
		w.Header().Set("Access-Control-Allow-Headers", "Authorization, Content-Type, X-Sennet-Workspace, X-Sennet-Signature-Version, X-Sennet-Signature, X-Sennet-Timestamp, X-Sennet-Nonce")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, DELETE, OPTIONS")
	}
	if r.Method == "OPTIONS" {
		w.WriteHeader(204)
		return
	}
	if r.URL.Path == "/live" {
		respond(w, 200, map[string]string{"status": "live"})
		return
	}
	if r.URL.Path == "/health" || r.URL.Path == "/ready" {
		ctx, cancel := context.WithTimeout(r.Context(), 3*time.Second)
		defer cancel()
		var err error
		if a.Ready != nil {
			err = a.Ready(ctx)
		} else {
			err = a.Store.Ping(ctx)
			if err == nil {
				err = a.Telemetry.Ping(ctx)
			}
			if err == nil && len(a.Brokers) > 0 {
				err = brokerReachable(ctx, a.Brokers[0])
			}
		}
		if err != nil {
			problem(w, 503, "dependency unavailable")
			return
		}
		respond(w, 200, map[string]string{"status": "ready", "mode": a.Mode, "role": a.Role})
		return
	}
	if r.URL.Path == "/internal/metrics" {
		parts := strings.Fields(r.Header.Get("Authorization"))
		if a.InternalToken == "" || len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") || !hmac.Equal([]byte(parts[1]), []byte(a.InternalToken)) {
			problem(w, 401, "operator bearer credential required")
			return
		}
		w.Header().Set("Content-Type", "text/plain; version=0.0.4")
		if a.Metrics != nil {
			_, _ = io.WriteString(w, a.Metrics.Prometheus(a.Role))
		}
		return
	}
	ip := a.Proxies.ClientIP(r)
	if !a.limiter.allow("ip:"+ip, 100, 200) {
		w.Header().Set("Retry-After", "1")
		problem(w, 429, "source request budget exhausted")
		return
	}
	parts := strings.Fields(r.Header.Get("Authorization"))
	if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") {
		problem(w, 401, "bearer credential required")
		return
	}
	token := parts[1]
	p, err := a.Store.Authenticate(r.Context(), token)
	if err != nil && a.Resolve != nil && !strings.HasPrefix(token, "sk_") && !strings.HasPrefix(token, "ses_") && !strings.HasPrefix(token, "col_") && !strings.HasPrefix(token, "enr_") {
		p, err = a.Resolve(r.Context(), token, r.Header.Get("X-Sennet-Workspace"))
	}
	if err != nil || p.OrganizationID == "" || p.WorkspaceID == "" {
		problem(w, 401, "invalid or expired credential")
		return
	}
	op, known := routeOperation(r.Method, r.URL.Path)
	if !known {
		problem(w, 404, "endpoint not available")
		return
	}
	if !Allowed(p, op) {
		problem(w, 403, "operation not permitted")
		return
	}
	quotaKey := ScopedKey(p, "quota", "requests")
	allowed, quotaErr := a.Quota.Allow(r.Context(), quotaKey, 6000, time.Minute, 1)
	if quotaErr != nil {
		problem(w, 503, "quota service unavailable")
		return
	}
	if !allowed {
		w.Header().Set("Retry-After", "1")
		problem(w, 429, "workspace request budget exhausted")
		return
	}
	requestCtx, requestCancel := context.WithTimeout(r.Context(), 20*time.Second)
	defer requestCancel()
	r = r.WithContext(context.WithValue(requestCtx, contextKey{}, p))
	if r.Method == "POST" || r.Method == "PUT" {
		reader := io.Reader(http.MaxBytesReader(w, r.Body, 4<<20))
		if r.Header.Get("Content-Encoding") == "gzip" {
			gz, e := gzip.NewReader(reader)
			if e != nil {
				problem(w, 400, "invalid gzip")
				return
			}
			defer gz.Close()
			reader = gz
		} else if r.Header.Get("Content-Encoding") != "" {
			problem(w, 415, "unsupported content encoding")
			return
		}
		b, e := io.ReadAll(io.LimitReader(reader, (4<<20)+1))
		if e != nil || len(b) > 4<<20 {
			problem(w, 413, "body exceeds 4 MiB")
			return
		}
		if e := verifySignature(r.Context(), a.Replay, p, r, token, b); e != nil {
			problem(w, 401, e.Error())
			return
		}
		r.Body = io.NopCloser(bytes.NewReader(b))
	}
	path := r.URL.Path
	allowsIngest := a.Role == "gateway" || a.Role == "all" || a.Role == ""
	allowsQuery := a.Role == "query-control" || a.Role == "all" || a.Role == ""
	if path == "/sentinel.v1.SentinelService/Heartbeat" {
		if !allowsIngest {
			problem(w, 404, "endpoint not served by this process role")
			return
		}
		a.heartbeat(w, r)
		return
	}
	if path == "/api/events" && r.Method == "POST" || strings.HasPrefix(path, "/v1/") {
		if !allowsIngest {
			problem(w, 404, "endpoint not served by this process role")
			return
		}
		if strings.HasPrefix(path, "/v1/") {
			a.otlp(w, r)
		} else {
			a.ingest(w, r)
		}
		return
	}
	if !allowsQuery {
		problem(w, 404, "endpoint not served by this process role")
		return
	}
	switch path {
	case "/api/session":
		if r.Method != "GET" {
			problem(w, 405, "GET required")
			return
		}
		respond(w, 200, p)
	case "/api/capabilities":
		if r.Method != "GET" {
			problem(w, 405, "GET required")
			return
		}
		respond(w, 200, map[string]any{"mode": a.Mode, "signals": signals, "retention_days": 30, "max_query_rows": 1000, "cloud_integrations": false, "team_management": false, "billing": false})
	case "/api/agents":
		if r.Method != "GET" {
			problem(w, 405, "GET required")
			return
		}
		agents, err := a.Store.Agents(r.Context(), p)
		if err != nil {
			problem(w, 503, "inventory unavailable")
			return
		}
		respond(w, 200, agents)
	case "/api/finance/reconciliation", "/api/finance/timelines", "/api/finance/queue", "/api/finance/latency", "/api/finance/provider-errors", "/api/finance/dependency-impact":
		a.finance(w, r)
	case "/api/summary":
		a.summary(w, r)
	case "/api/query":
		a.analyticalQuery(w, r)
	case "/api/topology":
		a.topology(w, r)
	case "/api/trace", "/api/correlations":
		a.traceInvestigation(w, r)
	case "/api/agent-runs":
		a.agentRun(w, r)
	case "/api/pipeline-health":
		a.pipelineHealth(w, r)
	case "/api/notification-status":
		a.notificationStatus(w, r)
	case "/api/dashboard-versions":
		a.dashboardVersions(w, r)
	case "/api/events":
		a.query(w, r)
	case "/api/stats":
		a.stats(w, r)
	case "/api/alerts":
		a.monitors(w, r)
	case "/api/dead-letter":
		a.deadLetter(w, r)
	case "/api/keys", "/api/keys/create":
		a.keys(w, r)
	case "/api/dashboards", "/api/preferences":
		a.resources(w, r, strings.TrimPrefix(path, "/api/"))
	case "/api/human-sessions", "/api/organizations", "/api/workspaces", "/api/memberships", "/api/role-assignments", "/api/collector-enrollments", "/api/collectors/enroll", "/api/collector-credentials/rotate", "/api/collector-credentials", "/api/workload-identities", "/api/audit-events":
		a.control(w, r)
	default:
		problem(w, 404, "endpoint not available")
	}
}
func (a *API) ingest(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		problem(w, 405, "POST required")
		return
	}
	var req struct {
		Events []Event `json:"events"`
	}
	if err := decode(r, &req); err != nil {
		if a.Metrics != nil {
			a.Metrics.RejectedEvents.Add(1)
		}
		problem(w, 400, "invalid batch JSON")
		return
	}
	if err := validateEvents(principal(r).Tenant, req.Events); err != nil {
		if a.Metrics != nil {
			a.Metrics.RejectedEvents.Add(uint64(len(req.Events)))
		}
		problem(w, 400, err.Error())
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 15*time.Second)
	defer cancel()
	if err := a.Ingest.Write(ctx, req.Events); err != nil {
		if a.Metrics != nil {
			a.Metrics.RejectedEvents.Add(uint64(len(req.Events)))
		}
		log.Print("durable ingest unavailable")
		if errors.Is(err, ErrIngestSaturated) {
			w.Header().Set("Retry-After", "1")
			problem(w, 429, "ingest queue saturated; retry with the same event IDs")
			return
		}
		if errors.Is(err, ErrProducerBatchTooLarge) {
			problem(w, 413, "batch exceeds configured Kafka append bound; split the batch")
			return
		}
		problem(w, 503, "durable ingest unavailable; retry with the same event IDs")
		return
	}
	if a.Metrics != nil {
		a.Metrics.AcceptedEvents.Add(uint64(len(req.Events)))
		a.Metrics.GatewayAccepted.Add(uint64(len(req.Events)))
	}
	respond(w, 202, map[string]any{"accepted": len(req.Events), "durable": true})
}

func (a *API) deadLetter(w http.ResponseWriter, r *http.Request) {
	if a.DeadLetters == nil {
		problem(w, 404, "dead-letter administration is not configured")
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()
	switch r.Method {
	case "GET":
		limit := 100
		if raw := r.URL.Query().Get("limit"); raw != "" {
			n, err := strconv.Atoi(raw)
			if err != nil || n < 1 || n > 1000 {
				problem(w, 400, "limit must be 1-1000")
				return
			}
			limit = n
		}
		items, err := a.DeadLetters.List(ctx, limit)
		if err != nil {
			problem(w, 503, "dead-letter inspection unavailable")
			return
		}
		respond(w, 200, map[string]any{"items": items, "bounded": true})
	case "POST":
		var req struct {
			ID    string `json:"id"`
			Event *Event `json:"event,omitempty"`
		}
		if decode(r, &req) != nil {
			problem(w, 400, "valid dead-letter ID required")
			return
		}
		d, err := a.DeadLetters.Replay(ctx, req.ID, req.Event)
		if err != nil {
			problem(w, 409, err.Error())
			return
		}
		respond(w, 202, map[string]any{"id": d.ID, "event_reappended": true, "storage_idempotent": true})
	default:
		problem(w, 405, "GET or POST required")
	}
}
func (a *API) query(w http.ResponseWriter, r *http.Request) {
	if r.Method != "GET" {
		problem(w, 405, "GET required")
		return
	}
	q, err := parseQuery(r)
	if err != nil {
		problem(w, 400, err.Error())
		return
	}
	select {
	case a.queries <- struct{}{}:
		defer func() { <-a.queries }()
	default:
		problem(w, 429, "query capacity exhausted")
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 20*time.Second)
	defer cancel()
	page, err := a.Telemetry.Query(ctx, principal(r).Tenant, q)
	if err != nil {
		problem(w, 503, "query unavailable")
		return
	}
	respond(w, 200, page)
}
func (a *API) keys(w http.ResponseWriter, r *http.Request) {
	p := principal(r)
	switch r.Method {
	case "GET":
		keys, err := a.Store.Keys(r.Context(), p)
		if err != nil {
			problem(w, 503, "key store unavailable")
			return
		}
		respond(w, 200, keys)
	case "POST":
		var req struct {
			Name    string `json:"name"`
			Role    string `json:"role"`
			Expires int64  `json:"expires"`
		}
		if err := decode(r, &req); err != nil {
			problem(w, 400, "invalid key request")
			return
		}
		k, secret, err := a.Store.CreateKey(r.Context(), p, req.Name, req.Role, req.Expires)
		if err != nil {
			problem(w, 400, "valid name, role and future expiry required")
			return
		}
		respond(w, 201, map[string]any{"metadata": k, "key": secret})
	case "DELETE":
		if err := a.Store.Revoke(r.Context(), p, r.URL.Query().Get("id")); err != nil {
			problem(w, 404, "key not found")
			return
		}
		w.WriteHeader(204)
	default:
		problem(w, 405, "method not allowed")
	}
}
func (a *API) heartbeat(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		problem(w, 405, "POST required")
		return
	}
	var req struct {
		AgentID string                     `json:"agentId"`
		Version string                     `json:"currentVersion"`
		Metrics map[string]json.RawMessage `json:"metrics"`
	}
	if err := decode(r, &req); err != nil || strings.TrimSpace(req.AgentID) == "" || len(req.AgentID) > 128 || len(req.Version) > 64 {
		problem(w, 400, "invalid heartbeat")
		return
	}
	agent := Agent{ID: req.AgentID, Version: req.Version, Seen: time.Now().UnixMilli(), Collection: "unavailable", Metrics: map[string]string{}}
	if req.Metrics != nil {
		agent.Collection = "collecting"
	}
	for _, key := range []string{"rxPackets", "txPackets", "rxBytes", "txBytes", "dropCount", "uptimeSeconds"} {
		if b, ok := req.Metrics[key]; ok {
			v := strings.Trim(string(b), "\"")
			if _, err := strconv.ParseUint(v, 10, 64); err != nil {
				problem(w, 400, "invalid counter")
				return
			}
			agent.Metrics[key] = v
		}
	}
	p := principal(r)
	if err := a.Store.PutAgent(r.Context(), p, agent); err != nil {
		problem(w, 503, "inventory write unavailable")
		return
	}
	// Inventory acknowledgements do not duplicate the agent durable telemetry outbox.
	respond(w, 200, map[string]string{"command": "COMMAND_NOOP", "latestVersion": req.Version, "configHash": "local-config-v1"})
}
func (a *API) stats(w http.ResponseWriter, r *http.Request) {
	if r.Method != "GET" {
		problem(w, 405, "GET required")
		return
	}
	agents, err := a.Store.Agents(r.Context(), principal(r))
	if err != nil {
		problem(w, 503, "inventory unavailable")
		return
	}
	counts := map[string]uint64{}
	active := 0
	now := time.Now().UnixMilli()
	for _, ag := range agents {
		if now-ag.Seen > 300000 {
			continue
		}
		active++
		for k, v := range ag.Metrics {
			n, _ := strconv.ParseUint(v, 10, 64)
			if ^uint64(0)-counts[k] >= n {
				counts[k] += n
			}
		}
	}
	respond(w, 200, map[string]any{"active_agents": active, "counters": counts, "timestamp": now, "agents": agents})
}
func (a *API) resources(w http.ResponseWriter, r *http.Request, kind string) {
	p := principal(r)
	switch r.Method {
	case "GET":
		out, err := a.Store.Resources(r.Context(), p, kind)
		if err != nil {
			problem(w, 503, "settings unavailable")
			return
		}
		respond(w, 200, out)
	case "POST":
		var v map[string]any
		if err := decode(r, &v); err != nil {
			problem(w, 400, "invalid resource")
			return
		}
		id, _ := v["id"].(string)
		if id == "" {
			id = randomID()
			v["id"] = id
		}
		name, _ := v["name"].(string)
		if len(id) > 128 || len(name) == 0 || len(name) > 200 {
			problem(w, 400, "name required")
			return
		}
		b, err := json.Marshal(v)
		if err != nil || len(b) > 65536 {
			problem(w, 400, "resource too large")
			return
		}
		if err = a.Store.PutResource(r.Context(), p, kind, id, b); err != nil {
			problem(w, 503, "settings write unavailable")
			return
		}
		if kind == "dashboards" {
			v["dashboard_id"] = id
			v["version_id"] = randomID()
			v["created_at"] = time.Now().UnixMilli()
			snapshot, _ := json.Marshal(v)
			if err = a.Store.PutResource(r.Context(), p, "dashboard-version:"+id, v["version_id"].(string), snapshot); err != nil {
				problem(w, 503, "dashboard version write unavailable")
				return
			}
		}
		respond(w, 201, v)
	case "DELETE":
		if err := a.Store.DeleteResource(r.Context(), p, kind, r.URL.Query().Get("id")); err != nil {
			problem(w, 404, "resource not found")
			return
		}
		w.WriteHeader(204)
	default:
		problem(w, 405, "method not allowed")
	}
}
