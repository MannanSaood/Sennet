package platform

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"
)

func fixture(t *testing.T) (*Store, http.Handler, string, string) {
	t.Helper()
	s, err := Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { s.Close() })
	a := "sk_aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
	b := "sk_bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"
	if err = s.Seed(context.Background(), a, "a"); err != nil {
		t.Fatal(err)
	}
	if err = s.Seed(context.Background(), b, "b"); err != nil {
		t.Fatal(err)
	}
	api := NewAPI(s, s, s)
	return s, api.Handler(), a, b
}
func call(h http.Handler, method, path, key string, body any) *httptest.ResponseRecorder {
	var buf bytes.Buffer
	if body != nil {
		_ = json.NewEncoder(&buf).Encode(body)
	}
	r := httptest.NewRequest(method, path, &buf)
	r.Header.Set("Authorization", "Bearer "+key)
	r.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)
	return w
}
func sample(id string) Event {
	return Event{ID: id, Time: time.Now().UnixMilli(), Signal: "trace", Service: "orders", Name: "POST /orders", Status: "ok", Attributes: map[string]string{"secret": "redact-me"}}
}
func TestIsolationPersistenceAndReplay(t *testing.T) {
	s, h, a, b := fixture(t)
	e := sample("event-1")
	e.Tenant = "b"
	for i := 0; i < 2; i++ {
		w := call(h, "POST", "/api/events", a, map[string]any{"events": []Event{e}})
		if w.Code != 202 {
			t.Fatal(w.Code, w.Body.String())
		}
	}
	w := call(h, "GET", "/api/events", a, nil)
	var p Page
	if err := json.Unmarshal(w.Body.Bytes(), &p); err != nil {
		t.Fatal(err)
	}
	if len(p.Events) != 1 || p.Events[0].Tenant != "" || p.Events[0].Attributes["secret"] != "" {
		t.Fatal("deduplication/redaction failed", w.Body.String())
	}
	w = call(h, "GET", "/api/events", b, nil)
	if err := json.Unmarshal(w.Body.Bytes(), &p); err != nil {
		t.Fatal(err)
	}
	if len(p.Events) != 0 {
		t.Fatal("cross-tenant disclosure")
	}
	// The durable SQL path survives a separate DB connection (not process memory).
	var count int
	if err := s.db.QueryRow(`SELECT COUNT(*) FROM platform_events WHERE tenant='a'`).Scan(&count); err != nil || count != 1 {
		t.Fatal(count, err)
	}
}
func TestKeyLifecycleScopesAndNoSecretListing(t *testing.T) {
	s, h, a, b := fixture(t)
	w := call(h, "POST", "/api/keys", a, map[string]any{"name": "collector", "role": "ingest", "expires": time.Now().Add(time.Hour).UnixMilli()})
	if w.Code != 201 {
		t.Fatal(w.Body.String())
	}
	var created struct {
		Metadata Key    `json:"metadata"`
		Key      string `json:"key"`
	}
	_ = json.Unmarshal(w.Body.Bytes(), &created)
	for _, token := range []string{a, b} {
		w = call(h, "GET", "/api/keys", token, nil)
		if strings.Contains(w.Body.String(), created.Key) || strings.Contains(w.Body.String(), a) {
			t.Fatal("raw credential disclosed")
		}
	}
	if w = call(h, "GET", "/api/events", created.Key, nil); w.Code != 403 {
		t.Fatal("ingest can read")
	}
	if w = call(h, "DELETE", "/api/keys?id="+created.Metadata.ID, b, nil); w.Code != 404 {
		t.Fatal("cross-tenant revoke")
	}
	if w = call(h, "DELETE", "/api/keys?id="+created.Metadata.ID, a, nil); w.Code != 204 {
		t.Fatal(w.Body.String())
	}
	if w = call(h, "POST", "/api/events", created.Key, map[string]any{"events": []Event{sample("2")}}); w.Code != 401 {
		t.Fatal("revoked key accepted")
	}
	var stored string
	_ = s.db.QueryRow(`SELECT hash FROM platform_credentials WHERE id=?`, created.Metadata.ID).Scan(&stored)
	if stored == created.Key || len(stored) != 64 {
		t.Fatal("credential not hashed")
	}
}
func TestHeartbeatAndResourceIsolation(t *testing.T) {
	_, h, a, b := fixture(t)
	w := call(h, "POST", "/sentinel.v1.SentinelService/Heartbeat", a, map[string]any{"agentId": "node-1", "currentVersion": "1.0.0", "metrics": map[string]string{"rxBytes": "18446744073709551615"}})
	if w.Code != 200 {
		t.Fatal(w.Body.String())
	}
	w = call(h, "GET", "/api/agents", b, nil)
	if w.Body.String() != "[]\n" {
		t.Fatal(w.Body.String())
	}
	w = call(h, "POST", "/api/dashboards", a, map[string]string{"name": "view", "id": "same"})
	if w.Code != 201 {
		t.Fatal(w.Body.String())
	}
	w = call(h, "GET", "/api/dashboards", b, nil)
	if w.Body.String() != "[]\n" {
		t.Fatal(w.Body.String())
	}
}
func TestInvalidBatchAtomicAndQueryBounds(t *testing.T) {
	_, h, a, _ := fixture(t)
	good := sample("valid")
	bad := sample("invalid")
	bad.Signal = "unknown"
	w := call(h, "POST", "/api/events", a, map[string]any{"events": []Event{good, bad}})
	if w.Code != 400 {
		t.Fatal(w.Code)
	}
	w = call(h, "GET", "/api/events", a, nil)
	var p Page
	_ = json.Unmarshal(w.Body.Bytes(), &p)
	if len(p.Events) != 0 {
		t.Fatal("partially accepted invalid batch")
	}
	for _, path := range []string{"/api/events?limit=100000", "/api/events?cursor=garbage", "/api/events?from=0&to=999999999999999"} {
		if w = call(h, "GET", path, a, nil); w.Code != 400 {
			t.Fatal(path, w.Code)
		}
	}
}
func TestOTLPJSONHexIDs(t *testing.T) {
	_, h, a, _ := fixture(t)
	now := time.Now().UnixNano()
	body := map[string]any{"resourceSpans": []any{map[string]any{"resource": map[string]any{"attributes": []any{map[string]any{"key": "service.name", "value": map[string]string{"stringValue": "agent-service"}}}}, "scopeSpans": []any{map[string]any{"spans": []any{map[string]any{"traceId": "11111111111111111111111111111111", "spanId": "2222222222222222", "name": "tool.call", "startTimeUnixNano": now, "endTimeUnixNano": now + 1000000}}}}}}}
	w := call(h, "POST", "/v1/traces", a, body)
	if w.Code != 200 {
		t.Fatal(w.Code, w.Body.String())
	}
	w = call(h, "GET", "/api/events?trace_id=11111111111111111111111111111111", a, nil)
	var p Page
	_ = json.Unmarshal(w.Body.Bytes(), &p)
	if len(p.Events) != 1 || p.Events[0].Duration != 1 {
		t.Fatal(w.Body.String())
	}
}
func TestOversizedBodyAndPartialSignature(t *testing.T) {
	_, h, a, _ := fixture(t)
	r := httptest.NewRequest("POST", "/api/events", strings.NewReader(strings.Repeat("a", (4<<20)+1)))
	r.Header.Set("Authorization", "Bearer "+a)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)
	if w.Code != 413 {
		t.Fatal(w.Code)
	}
	r = httptest.NewRequest("POST", "/api/events", strings.NewReader(`{}`))
	r.Header.Set("Authorization", "Bearer "+a)
	r.Header.Set("X-Sennet-Signature", "bad")
	w = httptest.NewRecorder()
	h.ServeHTTP(w, r)
	if w.Code != 401 {
		t.Fatal(w.Code)
	}
}
func TestClickHouseUsesTenantParameters(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("param_tenant") != "tenant' OR 1=1" {
			t.Error("tenant parameter missing")
		}
		if strings.Contains(r.URL.Query().Get("query"), "OR 1=1") {
			t.Error("interpolation")
		}
		_, _ = io.WriteString(w, "")
	}))
	defer srv.Close()
	c := &ClickHouse{URL: srv.URL, Client: srv.Client()}
	_, err := c.Query(context.Background(), "tenant' OR 1=1", Query{From: 1, To: 2, Limit: 10})
	if err != nil {
		t.Fatal(err)
	}
}

func TestWindowAggregationBeyondPageAndTenant(t *testing.T) {
	s, _, _, _ := fixture(t)
	ctx := context.Background()
	events := []Event{}
	for i := 0; i < 250; i++ {
		e := sample(strconv.Itoa(i))
		e.Tenant = "a"
		e.Duration = float64(i + 1)
		if i%10 == 0 {
			e.Status = "error"
		}
		events = append(events, e)
	}
	other := sample("foreign")
	other.Tenant = "b"
	other.Status = "error"
	events = append(events, other)
	if err := s.Write(ctx, events); err != nil {
		t.Fatal(err)
	}
	sum, err := s.Summary(ctx, "a", Query{From: time.Now().Add(-time.Hour).UnixMilli(), To: time.Now().Add(time.Minute).UnixMilli(), Limit: 10})
	if err != nil {
		t.Fatal(err)
	}
	if sum.Events != 250 || sum.Errors != 25 || sum.Services != 1 || sum.P95 != 238 {
		t.Fatalf("incorrect full-window aggregation: %+v", sum)
	}
}
func TestFinanceExactAmountAndSequenceGap(t *testing.T) {
	_, h, a, _ := fixture(t)
	one := sample("f1")
	one.Signal = "finance"
	one.Attributes = map[string]string{"transaction_id": "tx", "state": "created", "currency": "USD", "amount": "9007199254740993.0001", "sequence": "1"}
	two := one
	two.ID = "f2"
	two.Time++
	two.Attributes = map[string]string{"transaction_id": "tx", "state": "settled", "currency": "USD", "amount": "9007199254740993.0001", "sequence": "3"}
	if w := call(h, "POST", "/api/events", a, map[string]any{"events": []Event{one, two}}); w.Code != 202 {
		t.Fatal(w.Body.String())
	}
	w := call(h, "GET", "/api/finance/reconciliation?to="+strconv.FormatInt(time.Now().Add(time.Minute).UnixMilli(), 10), a, nil)
	if w.Code != 200 || !strings.Contains(w.Body.String(), "9007199254740993.0001") || !strings.Contains(w.Body.String(), "sequence gap") {
		t.Fatal(w.Code, w.Body.String())
	}
}
