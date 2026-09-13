package platform

import (
	"context"
	"math"
	"path/filepath"
	"testing"
	"testing/quick"
	"time"
)

func analyticalFixture(t *testing.T) (*Store, int64) {
	t.Helper()
	s, err := Open(filepath.Join(t.TempDir(), "query.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { s.Close() })
	return s, time.Now().UnixMilli() / 60000 * 60000
}
func metricEvent(id, tenant string, at int64, value float64) Event {
	return Event{ID: id, Tenant: tenant, Time: at, Signal: "metric", Service: "api", Name: "requests_total", Value: value, Status: "ok", Attributes: map[string]string{}}
}
func request(from, to int64, op string) AnalyticalRequest {
	return AnalyticalRequest{Version: "v1", Signal: "metric", Name: "requests_total", From: from, To: to, Operation: op, StepMS: 60000, Budget: QueryBudget{MaxRows: 1000, MaxBytes: 1 << 20, MaxCPUMS: 1000, TimeoutMS: 1000}}
}

func TestCounterResetLateDuplicateAndExactAggregates(t *testing.T) {
	s, base := analyticalFixture(t)
	events := []Event{metricEvent("b", "a", base+20000, 120), metricEvent("a", "a", base+10000, 100), metricEvent("c", "a", base+30000, 5), metricEvent("d", "a", base+40000, 15), metricEvent("d", "a", base+40000, 999)}
	if err := s.Write(context.Background(), events); err != nil {
		t.Fatal(err)
	}
	r := request(base, base+60000, "rate")
	out, err := s.AnalyticalQuery(context.Background(), "a", r)
	if err != nil {
		t.Fatal(err)
	}
	if len(out.Points) != 1 || math.Abs(out.Points[0].Value-35.0/60) > 1e-12 || out.Points[0].Count != 4 {
		t.Fatalf("reset/late/duplicate result: %+v", out)
	}
	r.Operation = "histogram"
	out, err = s.AnalyticalQuery(context.Background(), "a", r)
	if err != nil {
		t.Fatal(err)
	}
	p := out.Points[0]
	if p.P50 != 15 || p.P95 != 120 || p.P99 != 120 {
		t.Fatalf("quantiles not exact: %+v", p)
	}
}

func TestCounterRateProperty(t *testing.T) {
	property := func(samples []uint16) bool {
		if len(samples) == 0 {
			return true
		}
		q := request(1, 60001, "rate")
		rows := make([]analyticalRow, 0, len(samples))
		for i, v := range samples {
			rows = append(rows, analyticalRow{event: metricEvent(string(rune(i+1)), "a", int64(i+1), float64(v))})
		}
		points := aggregateRows(rows, q)
		return len(points) == 1 && points[0].Count == int64(len(samples)) && !math.IsNaN(points[0].Value) && !math.IsInf(points[0].Value, 0) && points[0].Value >= 0
	}
	if err := quick.Check(property, &quick.Config{MaxCount: 500}); err != nil {
		t.Fatal(err)
	}
}

func TestEmptyWindowTenantSkewPartialAndCancellation(t *testing.T) {
	s, base := analyticalFixture(t)
	r := request(base, base+60000, "count")
	out, err := s.AnalyticalQuery(context.Background(), "missing", r)
	if err != nil || len(out.Points) != 0 || out.Execution.Partial {
		t.Fatalf("empty window: %+v %v", out, err)
	}
	events := []Event{}
	for i := 0; i < 30; i++ {
		tenant := "large"
		if i < 2 {
			tenant = "small"
		}
		events = append(events, metricEvent(randomID(), tenant, base+int64(i)*1000, float64(i)))
	}
	if err = s.Write(context.Background(), events); err != nil {
		t.Fatal(err)
	}
	r.Budget.MaxRows = 5
	out, err = s.AnalyticalQuery(context.Background(), "large", r)
	if err != nil || !out.Execution.Partial || out.Execution.Reasons[0] != "row_budget" {
		t.Fatalf("tenant budget: %+v %v", out, err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err = s.AnalyticalQuery(ctx, "large", r)
	if err == nil {
		t.Fatal("cancelled query completed")
	}
}

func TestMonitorReplayRestartAndSLOBurn(t *testing.T) {
	path := filepath.Join(t.TempDir(), "monitor.db")
	s, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	end := time.Now().UnixMilli() / 30000 * 30000
	events := []Event{}
	for i := 0; i < 100; i++ {
		e := metricEvent(randomID(), "a", end-int64(i)*1000, float64(i))
		e.Signal = "trace"
		if i < 20 {
			e.Status = "error"
		}
		events = append(events, e)
	}
	if err = s.Write(context.Background(), events); err != nil {
		t.Fatal(err)
	}
	e := &MonitorEvaluator{Store: s, Telemetry: s, Now: func() time.Time { return time.UnixMilli(end + 1000) }}
	m := Monitor{ID: "m1", Version: "v1", Name: "errors", Signal: "trace", Service: "api", Threshold: 10}
	first, changed, err := e.Evaluate(context.Background(), "a", m, end)
	if err != nil || !changed || first.State != "firing" {
		t.Fatalf("first: %+v %v %v", first, changed, err)
	}
	_, changed, err = e.Evaluate(context.Background(), "a", m, end)
	if err != nil || changed {
		t.Fatalf("replay transitioned: %v %v", changed, err)
	}
	pending, err := s.Pending(context.Background(), 10)
	if err != nil || len(pending) != 1 {
		t.Fatalf("outbox: %d %v", len(pending), err)
	}
	s.Close()
	s, err = Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	e.Store = s
	e.Telemetry = s
	_, changed, err = e.Evaluate(context.Background(), "a", m, end)
	if err != nil || changed {
		t.Fatalf("restart replay transitioned: %v %v", changed, err)
	}
	pending, err = s.Pending(context.Background(), 10)
	if err != nil || len(pending) != 1 {
		t.Fatalf("restart outbox: %d %v", len(pending), err)
	}
	slo := Monitor{ID: "slo", Version: "v1", Name: "availability", Signal: "trace", Service: "api", SLO: &SLODefinition{Target: .99}}
	result, changed, err := e.Evaluate(context.Background(), "a", slo, end)
	if err != nil || !changed || result.State != "firing" {
		t.Fatalf("SLO burn: %+v %v %v", result, changed, err)
	}
}

func TestFinanceFloatingPointAggregationRejected(t *testing.T) {
	s, base := analyticalFixture(t)
	r := request(base, base+60000, "sum")
	r.Signal = "finance"
	if _, err := s.AnalyticalQuery(context.Background(), "a", r); err == nil {
		t.Fatal("finance sum accepted")
	}
}

func BenchmarkAnalyticalTenThousandRows(b *testing.B) {
	s, err := Open(filepath.Join(b.TempDir(), "bench.db"))
	if err != nil {
		b.Fatal(err)
	}
	defer s.Close()
	base := time.Now().UnixMilli() / 60000 * 60000
	events := make([]Event, 0, 1000)
	for batch := 0; batch < 10; batch++ {
		events = events[:0]
		for i := 0; i < 1000; i++ {
			n := batch*1000 + i
			events = append(events, metricEvent(randomID(), "bench", base+int64(n%60)*1000, float64(n)))
		}
		if err = s.Write(context.Background(), events); err != nil {
			b.Fatal(err)
		}
	}
	q := request(base, base+60000, "p95")
	q.Budget.MaxRows = 10000
	q.Budget.MaxBytes = 16 << 20
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err = s.AnalyticalQuery(context.Background(), "bench", q); err != nil {
			b.Fatal(err)
		}
	}
}
