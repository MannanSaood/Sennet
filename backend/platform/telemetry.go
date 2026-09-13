package platform

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// IDs and exact quantities remain strings on the wire. Tenant is assigned at
// authentication and is never accepted from an untrusted event payload.
type Event struct {
	ID         string            `json:"id"`
	Tenant     string            `json:"tenant,omitempty"`
	Time       int64             `json:"time"`
	Signal     string            `json:"signal"`
	Service    string            `json:"service"`
	Name       string            `json:"name"`
	TraceID    string            `json:"trace_id,omitempty"`
	SpanID     string            `json:"span_id,omitempty"`
	ParentID   string            `json:"parent_id,omitempty"`
	Duration   float64           `json:"duration_ms"`
	Status     string            `json:"status"`
	Value      float64           `json:"value"`
	Attributes map[string]string `json:"attributes"`
}
type Query struct {
	From    int64
	To      int64
	Signal  string
	Service string
	Trace   string
	Search  string
	Limit   int
	Cursor  string
}
type Page struct {
	Events    []Event `json:"events"`
	Cursor    string  `json:"next_cursor"`
	Timestamp int64   `json:"timestamp"`
	Partial   bool    `json:"partial"`
}
type Telemetry interface {
	Write(context.Context, []Event) error
	Query(context.Context, string, Query) (Page, error)
	Ping(context.Context) error
}

var signals = map[string]bool{"trace": true, "log": true, "metric": true, "flow": true, "agent": true, "finance": true}
var money = regexp.MustCompile(`^-?[0-9]{1,24}(\.[0-9]{1,12})?$`)

func validateEvents(tenant string, events []Event) error {
	if len(events) == 0 || len(events) > 1000 {
		return errors.New("batch requires 1–1000 events")
	}
	for i := range events {
		e := &events[i]
		e.Tenant = tenant
		if err := validateEvent(e, true); err != nil {
			return err
		}
	}
	return nil
}

func validateEvent(e *Event, enforceIngressTime bool) error {
	if e.ID == "" {
		return errors.New("stable event id required for replay")
	}
	invalidTime := e.Time <= 0
	if enforceIngressTime {
		now := time.Now().UnixMilli()
		invalidTime = e.Time < now-int64(30*24*time.Hour/time.Millisecond) || e.Time > now+300000
	}
	if len(e.ID) > 160 || !signals[e.Signal] || len(e.Service) == 0 || len(e.Service) > 200 || len(e.Name) > 500 || e.Duration < 0 || math.IsNaN(e.Duration) || math.IsInf(e.Duration, 0) || math.IsNaN(e.Value) || math.IsInf(e.Value, 0) || invalidTime {
		return errors.New("invalid event fields or timestamp outside 30-day retention")
	}
	if len(e.TraceID) > 128 || len(e.SpanID) > 128 || len(e.ParentID) > 128 || len(e.Attributes) > 64 {
		return errors.New("event cardinality limits exceeded")
	}
	clean := map[string]string{}
	for k, v := range e.Attributes {
		if len(k) > 128 || len(v) > 4096 {
			return errors.New("attribute exceeds size limit")
		}
		lower := strings.ToLower(k)
		if strings.Contains(lower, "password") || strings.Contains(lower, "secret") || strings.Contains(lower, "authorization") || lower == "prompt" || lower == "completion" || strings.Contains(lower, "prompt.content") || strings.Contains(lower, "completion.content") {
			continue
		}
		clean[k] = v
	}
	e.Attributes = clean
	if body := e.Attributes["content.body"]; body != "" && (e.Attributes["content.captured"] != "true" || e.Attributes["content.retention_class"] == "") {
		return errors.New("content capture requires explicit capture and retention policy")
	}
	if e.Signal == "agent" && e.Attributes["sennet.agent.convention"] != "" {
		if e.Attributes["sennet.agent.convention"] != "sennet.agent.v1" || e.Attributes["agent.run.id"] == "" {
			return errors.New("agent events require sennet.agent.v1 and agent.run.id")
		}
		for _, key := range []string{"gen_ai.usage.input_tokens", "gen_ai.usage.output_tokens", "gen_ai.usage.cached_tokens"} {
			if raw := e.Attributes[key]; raw != "" {
				if n, err := strconv.ParseUint(raw, 10, 64); err != nil || strconv.FormatUint(n, 10) != raw {
					return errors.New("agent token counts must be exact unsigned integers")
				}
			}
		}
		if e.Attributes["gen_ai.cost.estimated"] != "" && !money.MatchString(e.Attributes["gen_ai.cost.estimated"]) || e.Attributes["gen_ai.cost.observed"] != "" && !money.MatchString(e.Attributes["gen_ai.cost.observed"]) {
			return errors.New("agent costs must be exact decimal strings")
		}
	}
	if e.Signal == "finance" {
		if e.Attributes["transaction_id"] == "" || e.Attributes["state"] == "" || len(e.Attributes["currency"]) != 3 || !money.MatchString(e.Attributes["amount"]) {
			return errors.New("finance requires transaction_id, state, ISO currency and exact decimal amount")
		}
	}
	return nil
}
func (s *Store) Write(ctx context.Context, events []Event) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	for _, e := range events {
		b, err := json.Marshal(e)
		if err != nil {
			return err
		}
		_, err = tx.ExecContext(ctx, s.q(`INSERT INTO platform_events(tenant,id,time_ms,signal,service,trace_id,payload) VALUES(?,?,?,?,?,?,?) ON CONFLICT(tenant,id) DO NOTHING`), e.Tenant, e.ID, e.Time, e.Signal, e.Service, e.TraceID, string(b))
		if err != nil {
			return err
		}
	}
	return tx.Commit()
}
func (s *Store) Ping(ctx context.Context) error { return s.db.PingContext(ctx) }

type cursor struct {
	Time int64  `json:"t"`
	ID   string `json:"i"`
}

func decodeCursor(raw string) (cursor, error) {
	var c cursor
	if raw == "" {
		return c, nil
	}
	b, err := base64.RawURLEncoding.DecodeString(raw)
	if err != nil {
		return c, err
	}
	err = json.Unmarshal(b, &c)
	return c, err
}
func makePage(events []Event, limit int) Page {
	p := Page{Events: events, Timestamp: time.Now().UnixMilli()}
	if len(events) > limit {
		p.Events = events[:limit]
		last := p.Events[limit-1]
		b, _ := json.Marshal(cursor{last.Time, last.ID})
		p.Cursor = base64.RawURLEncoding.EncodeToString(b)
	}
	return p
}
func (s *Store) Query(ctx context.Context, tenant string, q Query) (Page, error) {
	sqlq := `SELECT payload FROM platform_events WHERE tenant=? AND time_ms>=? AND time_ms<=?`
	args := []any{tenant, q.From, q.To}
	for _, filter := range []struct{ col, v string }{{"signal", q.Signal}, {"service", q.Service}, {"trace_id", q.Trace}} {
		if filter.v != "" {
			sqlq += " AND " + filter.col + "=?"
			args = append(args, filter.v)
		}
	}
	if q.Search != "" {
		sqlq += " AND LOWER(payload) LIKE ? ESCAPE '!'"
		v := strings.NewReplacer("!", "!!", "%", "!%", "_", "!_").Replace(strings.ToLower(q.Search))
		args = append(args, "%"+v+"%")
	}
	if q.Cursor != "" {
		c, err := decodeCursor(q.Cursor)
		if err != nil {
			return Page{}, err
		}
		sqlq += " AND (time_ms<? OR (time_ms=? AND id<?))"
		args = append(args, c.Time, c.Time, c.ID)
	}
	sqlq += " ORDER BY time_ms DESC,id DESC LIMIT ?"
	args = append(args, q.Limit+1)
	rows, err := s.db.QueryContext(ctx, s.q(sqlq), args...)
	if err != nil {
		return Page{}, err
	}
	defer rows.Close()
	events := []Event{}
	for rows.Next() {
		var b string
		var e Event
		if err = rows.Scan(&b); err != nil {
			return Page{}, err
		}
		if err = json.Unmarshal([]byte(b), &e); err != nil {
			return Page{}, err
		}
		e.Tenant = ""
		events = append(events, e)
	}
	return makePage(events, q.Limit), rows.Err()
}
func parseQuery(r *http.Request) (Query, error) {
	v := r.URL.Query()
	now := time.Now().UnixMilli()
	q := Query{From: now - 3600000, To: now, Signal: v.Get("signal"), Service: v.Get("service"), Trace: v.Get("trace_id"), Search: v.Get("search"), Limit: 200, Cursor: v.Get("cursor")}
	for name, dst := range map[string]*int64{"from": &q.From, "to": &q.To} {
		if v.Get(name) != "" {
			n, err := strconv.ParseInt(v.Get(name), 10, 64)
			if err != nil {
				return q, errors.New("invalid time")
			}
			*dst = n
		}
	}
	if v.Get("limit") != "" {
		n, err := strconv.Atoi(v.Get("limit"))
		if err != nil {
			return q, err
		}
		q.Limit = n
	}
	if q.Limit < 1 || q.Limit > 1000 || q.To < q.From || q.To-q.From > int64(30*24*time.Hour/time.Millisecond) || len(q.Search) > 200 || len(q.Service) > 200 || len(q.Trace) > 128 || len(q.Cursor) > 512 || (q.Signal != "" && !signals[q.Signal]) {
		return q, errors.New("query exceeds time, filter or row limits")
	}
	if _, err := decodeCursor(q.Cursor); err != nil {
		return q, errors.New("invalid cursor")
	}
	return q, nil
}

// ClickHouse uses server-side typed parameters, bounded output, deadline and FINAL
// to collapse replayed IDs in ReplacingMergeTree without waiting for merges.
type ClickHouse struct {
	URL      string
	User     string
	Password string
	Client   *http.Client
	InitMode string
}

func (c *ClickHouse) request(ctx context.Context, query string, params url.Values, body []byte) ([]byte, error) {
	u, err := url.Parse(c.URL)
	if err != nil {
		return nil, err
	}
	v := u.Query()
	v.Set("query", query)
	v.Set("max_execution_time", "15")
	v.Set("max_result_rows", "10001")
	v.Set("max_result_bytes", "16777216")
	v.Set("result_overflow_mode", "throw")
	for k, values := range params {
		v[k] = values
	}
	u.RawQuery = v.Encode()
	req, err := http.NewRequestWithContext(ctx, "POST", u.String(), bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.SetBasicAuth(c.User, c.Password)
	resp, err := c.Client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	b, err := io.ReadAll(io.LimitReader(resp.Body, 17<<20))
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("analytics request failed (%d)", resp.StatusCode)
	}
	return b, nil
}
func (c *ClickHouse) Init(ctx context.Context) error {
	if c.InitMode == "none" {
		return nil
	}
	if c.InitMode != "" && c.InitMode != "local" {
		return errors.New("ClickHouse init mode must be local or none")
	}
	_, err := c.request(ctx, `CREATE TABLE IF NOT EXISTS sennet_events (tenant String,id String,time_ms Int64,signal LowCardinality(String),service LowCardinality(String),trace_id String,payload String) ENGINE=ReplacingMergeTree ORDER BY (tenant,time_ms,id) TTL toDateTime(time_ms/1000) + INTERVAL 30 DAY`, nil, nil)
	return err
}
func (c *ClickHouse) Ping(ctx context.Context) error {
	_, err := c.request(ctx, "SELECT 1", nil, nil)
	return err
}
func (c *ClickHouse) Write(ctx context.Context, events []Event) error {
	var b bytes.Buffer
	enc := json.NewEncoder(&b)
	for _, e := range events {
		payload, err := json.Marshal(e)
		if err != nil {
			return err
		}
		if err = enc.Encode(map[string]any{"tenant": e.Tenant, "id": e.ID, "time_ms": e.Time, "signal": e.Signal, "service": e.Service, "trace_id": e.TraceID, "payload": string(payload)}); err != nil {
			return err
		}
	}
	_, err := c.request(ctx, "INSERT INTO sennet_events FORMAT JSONEachRow", nil, b.Bytes())
	return err
}
func (c *ClickHouse) Query(ctx context.Context, tenant string, q Query) (Page, error) {
	query := `SELECT payload FROM sennet_events FINAL WHERE tenant={tenant:String} AND time_ms>={from:Int64} AND time_ms<={to:Int64}`
	p := url.Values{"param_tenant": {tenant}, "param_from": {strconv.FormatInt(q.From, 10)}, "param_to": {strconv.FormatInt(q.To, 10)}}
	for _, f := range []struct{ col, v string }{{"signal", q.Signal}, {"service", q.Service}, {"trace_id", q.Trace}} {
		if f.v != "" {
			query += " AND " + f.col + "={" + f.col + ":String}"
			p.Set("param_"+f.col, f.v)
		}
	}
	if q.Search != "" {
		query += " AND positionCaseInsensitive(payload,{search:String})>0"
		p.Set("param_search", q.Search)
	}
	if q.Cursor != "" {
		cur, err := decodeCursor(q.Cursor)
		if err != nil {
			return Page{}, err
		}
		query += " AND (time_ms<{cursor_time:Int64} OR (time_ms={cursor_time:Int64} AND id<{cursor_id:String}))"
		p.Set("param_cursor_time", strconv.FormatInt(cur.Time, 10))
		p.Set("param_cursor_id", cur.ID)
	}
	query += " ORDER BY time_ms DESC,id DESC LIMIT " + strconv.Itoa(q.Limit+1) + " FORMAT JSONEachRow"
	b, err := c.request(ctx, query, p, nil)
	if err != nil {
		return Page{}, err
	}
	dec := json.NewDecoder(bytes.NewReader(b))
	events := []Event{}
	for {
		var row struct {
			Payload string `json:"payload"`
		}
		err = dec.Decode(&row)
		if err == io.EOF {
			break
		}
		if err != nil {
			return Page{}, err
		}
		var e Event
		if err = json.Unmarshal([]byte(row.Payload), &e); err != nil {
			return Page{}, err
		}
		e.Tenant = ""
		events = append(events, e)
	}
	return makePage(events, q.Limit), nil
}
