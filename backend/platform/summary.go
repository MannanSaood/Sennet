package platform

import (
	"context"
	"encoding/json"
	"net/http"
	"net/url"
	"strconv"
	"strings"
)

type Summary struct {
	Events    int64          `json:"events"`
	Errors    int64          `json:"errors"`
	Services  int64          `json:"services"`
	P95       float64        `json:"p95_ms"`
	Buckets   []Bucket       `json:"buckets"`
	ByService []ServiceCount `json:"by_service"`
}
type Bucket struct {
	Time   int64 `json:"time"`
	Events int64 `json:"events"`
	Errors int64 `json:"errors"`
}
type ServiceCount struct {
	Name  string `json:"name"`
	Count int64  `json:"count"`
}
type Aggregator interface {
	Summary(context.Context, string, Query) (Summary, error)
}

func (s *Store) summaryFilter(tenant string, q Query) (string, []any) {
	where := `tenant=? AND time_ms>=? AND time_ms<=?`
	args := []any{tenant, q.From, q.To}
	for _, f := range []struct{ col, v string }{{"signal", q.Signal}, {"service", q.Service}, {"trace_id", q.Trace}} {
		if f.v != "" {
			where += " AND " + f.col + "=?"
			args = append(args, f.v)
		}
	}
	if q.Search != "" {
		where += " AND LOWER(payload) LIKE ? ESCAPE '!'"
		v := strings.NewReplacer("!", "!!", "%", "!%", "_", "!_").Replace(strings.ToLower(q.Search))
		args = append(args, "%"+v+"%")
	}
	return where, args
}
func (s *Store) Summary(ctx context.Context, tenant string, q Query) (Summary, error) {
	out := Summary{Buckets: []Bucket{}, ByService: []ServiceCount{}}
	where, args := s.summaryFilter(tenant, q)
	status := `json_extract(payload,'$.status')`
	duration := `CAST(json_extract(payload,'$.duration_ms') AS REAL)`
	if s.postgres {
		status = `(payload::jsonb->>'status')`
		duration = `CAST(payload::jsonb->>'duration_ms' AS DOUBLE PRECISION)`
	}
	err := s.db.QueryRowContext(ctx, s.q(`SELECT COUNT(*),COALESCE(SUM(CASE WHEN `+status+`='error' THEN 1 ELSE 0 END),0),COUNT(DISTINCT service) FROM platform_events WHERE `+where), args...).Scan(&out.Events, &out.Errors, &out.Services)
	if err != nil {
		return out, err
	}
	var spans int64
	err = s.db.QueryRowContext(ctx, s.q(`SELECT COUNT(*) FROM platform_events WHERE `+where+` AND `+duration+`>0`), args...).Scan(&spans)
	if err != nil {
		return out, err
	}
	if spans > 0 {
		offset := (spans*95+99)/100 - 1
		err = s.db.QueryRowContext(ctx, s.q(`SELECT `+duration+` FROM platform_events WHERE `+where+` AND `+duration+`>0 ORDER BY `+duration+` LIMIT 1 OFFSET `+strconv.FormatInt(offset, 10)), args...).Scan(&out.P95)
		if err != nil {
			return out, err
		}
	}
	step := (q.To - q.From) / 60
	if step < 1000 {
		step = 1000
	}
	bucket := `(time_ms/` + strconv.FormatInt(step, 10) + `)*` + strconv.FormatInt(step, 10)
	rows, err := s.db.QueryContext(ctx, s.q(`SELECT `+bucket+`,COUNT(*),SUM(CASE WHEN `+status+`='error' THEN 1 ELSE 0 END) FROM platform_events WHERE `+where+` GROUP BY 1 ORDER BY 1 LIMIT 62`), args...)
	if err != nil {
		return out, err
	}
	for rows.Next() {
		var b Bucket
		if err = rows.Scan(&b.Time, &b.Events, &b.Errors); err != nil {
			rows.Close()
			return out, err
		}
		out.Buckets = append(out.Buckets, b)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return out, err
	}
	rows, err = s.db.QueryContext(ctx, s.q(`SELECT service,COUNT(*) FROM platform_events WHERE `+where+` GROUP BY service ORDER BY 2 DESC LIMIT 20`), args...)
	if err != nil {
		return out, err
	}
	defer rows.Close()
	for rows.Next() {
		var c ServiceCount
		if err = rows.Scan(&c.Name, &c.Count); err != nil {
			return out, err
		}
		out.ByService = append(out.ByService, c)
	}
	return out, rows.Err()
}
func (c *ClickHouse) Summary(ctx context.Context, tenant string, q Query) (Summary, error) {
	out := Summary{Buckets: []Bucket{}, ByService: []ServiceCount{}}
	where := `tenant={tenant:String} AND time_ms>={from:Int64} AND time_ms<={to:Int64}`
	p := url.Values{"param_tenant": {tenant}, "param_from": {strconv.FormatInt(q.From, 10)}, "param_to": {strconv.FormatInt(q.To, 10)}}
	for _, f := range []struct{ col, v string }{{"signal", q.Signal}, {"service", q.Service}, {"trace_id", q.Trace}} {
		if f.v != "" {
			where += " AND " + f.col + "={" + f.col + ":String}"
			p.Set("param_"+f.col, f.v)
		}
	}
	if q.Search != "" {
		where += " AND positionCaseInsensitive(payload,{search:String})>0"
		p.Set("param_search", q.Search)
	}
	b, err := c.request(ctx, `SELECT count() events,countIf(JSONExtractString(payload,'status')='error') errors,uniqExact(service) services,if(countIf(JSONExtractFloat(payload,'duration_ms')>0)=0,0,quantileTDigestIf(0.95)(JSONExtractFloat(payload,'duration_ms'),JSONExtractFloat(payload,'duration_ms')>0)) p95_ms FROM sennet_events FINAL WHERE `+where+` FORMAT JSONEachRow`, p, nil)
	if err != nil {
		return out, err
	}
	// ClickHouse quotes 64-bit integers by default; ask for numeric counts below JS
	// safety bounds for aggregate display. Raw counter/monetary values stay strings.
	var raw struct {
		Events   json.Number `json:"events"`
		Errors   json.Number `json:"errors"`
		Services json.Number `json:"services"`
		P95      float64     `json:"p95_ms"`
	}
	if err = json.Unmarshal(b, &raw); err != nil {
		return out, err
	}
	out.Events, _ = raw.Events.Int64()
	out.Errors, _ = raw.Errors.Int64()
	out.Services, _ = raw.Services.Int64()
	out.P95 = raw.P95
	p.Set("output_format_json_quote_64bit_integers", "0")
	step := (q.To - q.From) / 60
	if step < 1000 {
		step = 1000
	}
	bucket := "intDiv(time_ms," + strconv.FormatInt(step, 10) + ")*" + strconv.FormatInt(step, 10)
	b, err = c.request(ctx, `SELECT `+bucket+` time,count() events,countIf(JSONExtractString(payload,'status')='error') errors FROM sennet_events FINAL WHERE `+where+` GROUP BY time ORDER BY time LIMIT 62 FORMAT JSONEachRow`, p, nil)
	if err != nil {
		return out, err
	}
	dec := json.NewDecoder(strings.NewReader(string(b)))
	for dec.More() {
		var item Bucket
		if err = dec.Decode(&item); err != nil {
			return out, err
		}
		out.Buckets = append(out.Buckets, item)
	}
	b, err = c.request(ctx, `SELECT service name,count() count FROM sennet_events FINAL WHERE `+where+` GROUP BY service ORDER BY count DESC LIMIT 20 FORMAT JSONEachRow`, p, nil)
	if err != nil {
		return out, err
	}
	dec = json.NewDecoder(strings.NewReader(string(b)))
	for dec.More() {
		var item ServiceCount
		if err = dec.Decode(&item); err != nil {
			return out, err
		}
		out.ByService = append(out.ByService, item)
	}
	return out, nil
}
func (a *API) summary(w http.ResponseWriter, r *http.Request) {
	if r.Method != "GET" {
		problem(w, 405, "GET required")
		return
	}
	q, err := parseQuery(r)
	if err != nil {
		problem(w, 400, err.Error())
		return
	}
	agg, ok := a.Telemetry.(Aggregator)
	if !ok {
		problem(w, 501, "aggregation unavailable")
		return
	}
	select {
	case a.queries <- struct{}{}:
		defer func() { <-a.queries }()
	default:
		problem(w, 429, "query capacity exhausted")
		return
	}
	out, err := agg.Summary(r.Context(), principal(r).Tenant, q)
	if err != nil {
		problem(w, 503, "summary unavailable")
		return
	}
	respond(w, 200, out)
}
