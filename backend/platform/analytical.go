package platform

import (
	"context"
	"encoding/json"
	"errors"
	"math"
	"net/http"
	"sort"
	"time"
)

const QueryContractVersion = "v1"

type QueryBudget struct {
	MaxRows   int   `json:"max_rows"`
	MaxBytes  int64 `json:"max_bytes"`
	MaxCPUMS  int   `json:"max_cpu_ms"`
	TimeoutMS int   `json:"timeout_ms"`
}

type AnalyticalRequest struct {
	Version   string      `json:"version"`
	Signal    string      `json:"signal"`
	From      int64       `json:"from"`
	To        int64       `json:"to"`
	Service   string      `json:"service,omitempty"`
	Name      string      `json:"name,omitempty"`
	Operation string      `json:"operation"`
	GroupBy   []string    `json:"group_by,omitempty"`
	StepMS    int64       `json:"step_ms,omitempty"`
	Compare   bool        `json:"compare_previous,omitempty"`
	Exemplars bool        `json:"exemplars,omitempty"`
	Budget    QueryBudget `json:"budget"`
}

type QueryPoint struct {
	Time      int64             `json:"time"`
	Group     map[string]string `json:"group,omitempty"`
	Value     float64           `json:"value"`
	Count     int64             `json:"count"`
	P50       float64           `json:"p50,omitempty"`
	P95       float64           `json:"p95,omitempty"`
	P99       float64           `json:"p99,omitempty"`
	Exemplars []string          `json:"exemplars,omitempty"`
}

type QueryExecution struct {
	RowsScanned  int64    `json:"rows_scanned"`
	BytesScanned int64    `json:"bytes_scanned"`
	ElapsedMS    int64    `json:"elapsed_ms"`
	Partial      bool     `json:"partial"`
	Reasons      []string `json:"partial_reasons,omitempty"`
	StepMS       int64    `json:"step_ms"`
}

type AnalyticalResponse struct {
	Version    string         `json:"version"`
	Points     []QueryPoint   `json:"points"`
	Comparison []QueryPoint   `json:"comparison,omitempty"`
	Execution  QueryExecution `json:"execution"`
}

type AnalyticalQuerier interface {
	AnalyticalQuery(context.Context, string, AnalyticalRequest) (AnalyticalResponse, error)
}

func (q *AnalyticalRequest) validate() error {
	if q.Version != QueryContractVersion || !signals[q.Signal] || q.From <= 0 || q.To <= q.From || q.To-q.From > int64(30*24*time.Hour/time.Millisecond) {
		return errors.New("version, signal, and explicit bounded time range are required")
	}
	allowedOps := map[string]bool{"count": true, "sum": true, "avg": true, "min": true, "max": true, "rate": true, "histogram": true, "p50": true, "p95": true, "p99": true}
	if !allowedOps[q.Operation] {
		return errors.New("unsupported operation")
	}
	if q.Signal == "finance" && q.Operation != "count" {
		return errors.New("finance supports count only; exact values are not floating-point metrics")
	}
	if q.Signal == "log" && q.Operation != "count" {
		return errors.New("logs support count only")
	}
	if q.Operation == "rate" && (q.Signal != "metric" || q.Name == "") {
		return errors.New("rate requires one named metric counter")
	}
	if q.Budget.MaxRows < 1 || q.Budget.MaxRows > 100000 || q.Budget.MaxBytes < 1024 || q.Budget.MaxBytes > 64<<20 || q.Budget.MaxCPUMS < 1 || q.Budget.MaxCPUMS > 15000 || q.Budget.TimeoutMS < 1 || q.Budget.TimeoutMS > 15000 {
		return errors.New("row, byte, and timeout budgets are required and exceed safe bounds")
	}
	if len(q.GroupBy) > 2 || len(q.Service) > 200 || len(q.Name) > 500 {
		return errors.New("filter or grouping cardinality exceeds bounds")
	}
	for _, g := range q.GroupBy {
		if g != "service" && g != "signal" && g != "name" && g != "status" {
			return errors.New("group_by permits service, signal, name, or status")
		}
	}
	span := q.To - q.From
	if q.StepMS == 0 {
		q.StepMS = (span + 299) / 300
	}
	if q.StepMS < 1000 {
		q.StepMS = 1000
	}
	if (span+q.StepMS-1)/q.StepMS > 300 {
		return errors.New("step produces more than 300 points")
	}
	return nil
}

type analyticalRow struct {
	event Event
	bytes int64
}
type aggregateCell struct {
	values              []float64
	first, last         float64
	firstTime, lastTime int64
	increase            float64
	count               int64
	exemplars           []string
}
type aggregateKey struct {
	bucket int64
	group  string
}

func numericValue(e Event) float64 {
	if e.Signal == "trace" {
		return e.Duration
	}
	return e.Value
}

func groupValues(e Event, fields []string) (map[string]string, string) {
	g := map[string]string{}
	for _, field := range fields {
		var v string
		switch field {
		case "service":
			v = e.Service
		case "signal":
			v = e.Signal
		case "name":
			v = e.Name
		case "status":
			v = e.Status
		}
		g[field] = v
	}
	b, _ := json.Marshal(g)
	return g, string(b)
}

func percentile(values []float64, p float64) float64 {
	if len(values) == 0 {
		return 0
	}
	v := append([]float64(nil), values...)
	sort.Float64s(v)
	i := int(math.Ceil(p*float64(len(v)))) - 1
	if i < 0 {
		i = 0
	}
	return v[i]
}

func (s *Store) analyticalWindow(ctx context.Context, tenant string, q AnalyticalRequest) ([]QueryPoint, QueryExecution, error) {
	started := time.Now()
	exec := QueryExecution{StepMS: q.StepMS, Reasons: []string{}}
	sqlq := `SELECT payload FROM platform_events WHERE tenant=? AND time_ms>=? AND time_ms<=? AND signal=?`
	args := []any{tenant, q.From, q.To, q.Signal}
	if q.Service != "" {
		sqlq += ` AND service=?`
		args = append(args, q.Service)
	}
	// Name is intentionally applied after decoding; it is still bounded by scan budgets.
	sqlq += ` ORDER BY time_ms,id LIMIT ?`
	args = append(args, q.Budget.MaxRows+1)
	rows, err := s.db.QueryContext(ctx, s.q(sqlq), args...)
	if err != nil {
		return nil, exec, err
	}
	defer rows.Close()
	data := []analyticalRow{}
	for rows.Next() {
		var raw string
		if err = rows.Scan(&raw); err != nil {
			return nil, exec, err
		}
		exec.RowsScanned++
		exec.BytesScanned += int64(len(raw))
		if exec.RowsScanned > int64(q.Budget.MaxRows) {
			exec.Partial = true
			exec.Reasons = append(exec.Reasons, "row_budget")
			break
		}
		if exec.BytesScanned > q.Budget.MaxBytes {
			exec.Partial = true
			exec.Reasons = append(exec.Reasons, "byte_budget")
			break
		}
		var e Event
		if err = json.Unmarshal([]byte(raw), &e); err != nil {
			return nil, exec, err
		}
		if q.Name != "" && e.Name != q.Name {
			continue
		}
		data = append(data, analyticalRow{e, int64(len(raw))})
	}
	if err = rows.Err(); err != nil {
		return nil, exec, err
	}
	exec.ElapsedMS = time.Since(started).Milliseconds()
	return aggregateRows(data, q), exec, nil
}

func aggregateRows(data []analyticalRow, q AnalyticalRequest) []QueryPoint {
	cells := map[aggregateKey]*aggregateCell{}
	groups := map[aggregateKey]map[string]string{}
	for _, r := range data {
		value := numericValue(r.event)
		bucket := q.From + ((r.event.Time-q.From)/q.StepMS)*q.StepMS
		g, key := groupValues(r.event, q.GroupBy)
		k := aggregateKey{bucket, key}
		c := cells[k]
		if c == nil {
			c = &aggregateCell{first: value, last: value, firstTime: r.event.Time, lastTime: r.event.Time}
			cells[k] = c
			groups[k] = g
		}
		c.count++
		c.values = append(c.values, value)
		if r.event.Time > c.lastTime {
			if value < c.last {
				c.increase += value
			} else {
				c.increase += value - c.last
			}
			c.last = value
			c.lastTime = r.event.Time
		}
		if q.Exemplars && len(c.exemplars) < 3 && (r.event.TraceID != "" || r.event.ID != "") {
			if r.event.TraceID != "" {
				c.exemplars = append(c.exemplars, r.event.TraceID)
			} else {
				c.exemplars = append(c.exemplars, r.event.ID)
			}
		}
	}
	points := make([]QueryPoint, 0, len(cells))
	for k, c := range cells {
		p := QueryPoint{Time: k.bucket, Group: groups[k], Count: c.count, Exemplars: c.exemplars}
		switch q.Operation {
		case "count":
			p.Value = float64(c.count)
		case "sum":
			for _, v := range c.values {
				p.Value += v
			}
		case "avg":
			for _, v := range c.values {
				p.Value += v
			}
			p.Value /= float64(c.count)
		case "min":
			p.Value = percentile(c.values, 0)
		case "max":
			p.Value = percentile(c.values, 1)
		case "rate":
			p.Value = c.increase / (float64(q.StepMS) / 1000)
		case "histogram":
			p.P50 = percentile(c.values, .50)
			p.P95 = percentile(c.values, .95)
			p.P99 = percentile(c.values, .99)
			p.Value = p.P50
		case "p50":
			p.Value = percentile(c.values, .50)
		case "p95":
			p.Value = percentile(c.values, .95)
		case "p99":
			p.Value = percentile(c.values, .99)
		}
		points = append(points, p)
	}
	sort.Slice(points, func(i, j int) bool {
		if points[i].Time == points[j].Time {
			return fmtGroup(points[i].Group) < fmtGroup(points[j].Group)
		}
		return points[i].Time < points[j].Time
	})
	return points
}

func fmtGroup(g map[string]string) string { b, _ := json.Marshal(g); return string(b) }

func (s *Store) AnalyticalQuery(ctx context.Context, tenant string, q AnalyticalRequest) (AnalyticalResponse, error) {
	if err := q.validate(); err != nil {
		return AnalyticalResponse{}, err
	}
	deadline := q.Budget.TimeoutMS
	if q.Budget.MaxCPUMS < deadline {
		deadline = q.Budget.MaxCPUMS
	}
	ctx, cancel := context.WithTimeout(ctx, time.Duration(deadline)*time.Millisecond)
	defer cancel()
	window := q
	if q.Compare {
		window.Budget.MaxRows = (q.Budget.MaxRows + 1) / 2
		window.Budget.MaxBytes = (q.Budget.MaxBytes + 1) / 2
	}
	points, meta, err := s.analyticalWindow(ctx, tenant, window)
	if err != nil {
		return AnalyticalResponse{}, err
	}
	out := AnalyticalResponse{Version: QueryContractVersion, Points: points, Execution: meta}
	if q.Compare {
		span := q.To - q.From
		prior := window
		prior.Compare = false
		prior.From -= span
		prior.To -= span
		var comparisonMeta QueryExecution
		out.Comparison, comparisonMeta, err = s.analyticalWindow(ctx, tenant, prior)
		out.Execution.RowsScanned += comparisonMeta.RowsScanned
		out.Execution.BytesScanned += comparisonMeta.BytesScanned
		out.Execution.ElapsedMS += comparisonMeta.ElapsedMS
		out.Execution.Partial = out.Execution.Partial || comparisonMeta.Partial
		out.Execution.Reasons = append(out.Execution.Reasons, comparisonMeta.Reasons...)
	}
	return out, err
}

func (c *ClickHouse) analyticalWindow(ctx context.Context, tenant string, q AnalyticalRequest) ([]QueryPoint, QueryExecution, error) {
	started := time.Now()
	meta := QueryExecution{StepMS: q.StepMS, Reasons: []string{}}
	limit := q.Budget.MaxRows
	if limit > 10000 {
		limit = 10000
	}
	page, err := c.Query(ctx, tenant, Query{From: q.From, To: q.To, Signal: q.Signal, Service: q.Service, Limit: limit})
	if err != nil {
		return nil, meta, err
	}
	data := make([]analyticalRow, 0, len(page.Events))
	for _, e := range page.Events {
		raw, _ := json.Marshal(e)
		meta.RowsScanned++
		meta.BytesScanned += int64(len(raw))
		if meta.BytesScanned > q.Budget.MaxBytes {
			meta.Partial = true
			meta.Reasons = append(meta.Reasons, "byte_budget")
			break
		}
		if q.Name != "" && e.Name != q.Name {
			continue
		}
		data = append(data, analyticalRow{event: e, bytes: int64(len(raw))})
	}
	if page.Cursor != "" {
		meta.Partial = true
		meta.Reasons = append(meta.Reasons, "row_budget")
	}
	meta.ElapsedMS = time.Since(started).Milliseconds()
	return aggregateRows(data, q), meta, nil
}
func (c *ClickHouse) AnalyticalQuery(ctx context.Context, tenant string, q AnalyticalRequest) (AnalyticalResponse, error) {
	if err := q.validate(); err != nil {
		return AnalyticalResponse{}, err
	}
	deadline := q.Budget.TimeoutMS
	if q.Budget.MaxCPUMS < deadline {
		deadline = q.Budget.MaxCPUMS
	}
	ctx, cancel := context.WithTimeout(ctx, time.Duration(deadline)*time.Millisecond)
	defer cancel()
	window := q
	if q.Compare {
		window.Budget.MaxRows = (q.Budget.MaxRows + 1) / 2
		window.Budget.MaxBytes = (q.Budget.MaxBytes + 1) / 2
	}
	points, meta, err := c.analyticalWindow(ctx, tenant, window)
	if err != nil {
		return AnalyticalResponse{}, err
	}
	out := AnalyticalResponse{Version: QueryContractVersion, Points: points, Execution: meta}
	if q.Compare {
		span := q.To - q.From
		prior := window
		prior.From -= span
		prior.To -= span
		prior.Compare = false
		var comparisonMeta QueryExecution
		out.Comparison, comparisonMeta, err = c.analyticalWindow(ctx, tenant, prior)
		out.Execution.RowsScanned += comparisonMeta.RowsScanned
		out.Execution.BytesScanned += comparisonMeta.BytesScanned
		out.Execution.ElapsedMS += comparisonMeta.ElapsedMS
		out.Execution.Partial = out.Execution.Partial || comparisonMeta.Partial
		out.Execution.Reasons = append(out.Execution.Reasons, comparisonMeta.Reasons...)
	}
	return out, err
}

func (a *API) analyticalQuery(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		problem(w, 405, "POST required")
		return
	}
	var q AnalyticalRequest
	if err := decode(r, &q); err != nil {
		if a.Metrics != nil {
			a.Metrics.QueryRejected.Add(1)
			a.Metrics.QueryRejectValidation.Add(1)
		}
		problem(w, 400, "invalid query contract")
		return
	}
	if err := q.validate(); err != nil {
		if a.Metrics != nil {
			a.Metrics.QueryRejected.Add(1)
			a.Metrics.QueryRejectValidation.Add(1)
		}
		problem(w, 400, err.Error())
		return
	}
	select {
	case a.queries <- struct{}{}:
		defer func() { <-a.queries }()
	default:
		if a.Metrics != nil {
			a.Metrics.QueryRejected.Add(1)
			a.Metrics.QueryRejectAdmission.Add(1)
		}
		problem(w, 429, "query concurrency capacity exhausted")
		return
	}
	querier, ok := a.Telemetry.(AnalyticalQuerier)
	if !ok {
		if a.Metrics != nil {
			a.Metrics.QueryRejected.Add(1)
			a.Metrics.QueryRejectUnavailable.Add(1)
		}
		problem(w, 501, "analytical query unavailable")
		return
	}
	if a.Metrics != nil {
		a.Metrics.QueryCapacity.Store(int64(cap(a.queries)))
		a.Metrics.QueryInFlight.Add(1)
		a.Metrics.QueryRequests.Add(1)
		defer a.Metrics.QueryInFlight.Add(-1)
	}
	deadline := q.Budget.TimeoutMS
	if q.Budget.MaxCPUMS < deadline {
		deadline = q.Budget.MaxCPUMS
	}
	ctx, cancel := context.WithTimeout(r.Context(), time.Duration(deadline)*time.Millisecond)
	defer cancel()
	started := time.Now()
	out, err := querier.AnalyticalQuery(ctx, principal(r).Tenant, q)
	if a.Metrics != nil {
		a.Metrics.QueryLatencyNanos.Add(uint64(time.Since(started)))
		if err == nil {
			a.Metrics.QueryRows.Add(uint64(out.Execution.RowsScanned))
			a.Metrics.QueryBytes.Add(uint64(out.Execution.BytesScanned))
			a.Metrics.QueryCPUNanos.Add(uint64(time.Duration(out.Execution.ElapsedMS) * time.Millisecond))
			if out.Execution.Partial {
				a.Metrics.QueryPartial.Add(1)
			}
		}
	}
	if err != nil {
		if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
			if a.Metrics != nil {
				a.Metrics.QueryCancelled.Add(1)
			}
			problem(w, 408, "query cancelled or timed out")
			return
		}
		problem(w, 503, "query unavailable")
		return
	}
	respond(w, 200, out)
}
