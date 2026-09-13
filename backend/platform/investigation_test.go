package platform

import (
	"encoding/json"
	"fmt"
	"math"
	"testing"
	"time"
)

func TestFormulaHistogramAndInvestigationContracts(t *testing.T) {
	s, h, key, _ := fixture(t)
	now := time.Now().UnixMilli()
	events := []Event{
		{ID: "root", Time: now - 1000, Signal: "trace", Service: "checkout", Name: "root", TraceID: "trace-1", SpanID: "root-span", Duration: 100, Status: "ok", Attributes: map[string]string{}},
		{ID: "child", Time: now - 900, Signal: "trace", Service: "payments", Name: "child", TraceID: "trace-1", SpanID: "child-span", ParentID: "root-span", Duration: 40, Status: "error", Attributes: map[string]string{"span.link": "async-2"}},
		{ID: "log", Time: now - 850, Signal: "log", Service: "payments", Name: "failed", TraceID: "trace-1", Status: "error", Attributes: map[string]string{}},
	}
	for i := range events {
		events[i].Tenant = "a"
	}
	if err := s.Write(t.Context(), events); err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{"/api/topology?from=" + strconv64(now-2000) + "&to=" + strconv64(now), "/api/trace?trace_id=trace-1&from=" + strconv64(now-2000) + "&to=" + strconv64(now), "/api/correlations?trace_id=trace-1&from=" + strconv64(now-2000) + "&to=" + strconv64(now), "/api/pipeline-health", "/api/notification-status"} {
		if w := call(h, "GET", path, key, nil); w.Code != 200 {
			t.Fatalf("%s: %d %s", path, w.Code, w.Body.String())
		}
	}
	w := call(h, "POST", "/api/dashboards", key, map[string]any{"id": "dash-1", "name": "Evidence", "panels": []any{}})
	if w.Code != 201 {
		t.Fatal(w.Code, w.Body.String())
	}
	w = call(h, "GET", "/api/dashboard-versions?id=dash-1", key, nil)
	var versions []json.RawMessage
	if json.Unmarshal(w.Body.Bytes(), &versions) != nil || len(versions) != 1 {
		t.Fatalf("versions: %s", w.Body.String())
	}
}

func TestServerFormulaAndHistogramBuckets(t *testing.T) {
	s, base := analyticalFixture(t)
	if err := s.Write(t.Context(), []Event{metricEvent("a", "a", base+1000, 10), metricEvent("b", "a", base+2000, 20)}); err != nil {
		t.Fatal(err)
	}
	q := request(base, base+60000, "histogram")
	q.Formula = "A*2"
	out, err := s.AnalyticalQuery(t.Context(), "a", q)
	if err != nil {
		t.Fatal(err)
	}
	if len(out.Points) != 1 || math.Abs(out.Points[0].Value-20) > 1e-9 || len(out.Points[0].Histogram) == 0 {
		t.Fatalf("formula/histogram: %+v", out)
	}
	q.Formula = "A/0"
	if _, err = s.AnalyticalQuery(t.Context(), "a", q); err == nil {
		t.Fatal("division by zero accepted")
	}
}

func strconv64(value int64) string { return fmt.Sprint(value) }
