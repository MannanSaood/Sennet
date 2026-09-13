package platform

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"runtime"
	"runtime/debug"
	"sort"
	"sync"
	"testing"
	"time"
)

func TestSQLiteBackupRestore(t *testing.T) {
	ctx := context.Background()
	source := t.TempDir() + "/source.db"
	restored := t.TempDir() + "/restored.db"
	s, err := Open(source)
	if err != nil {
		t.Fatal(err)
	}
	now := time.Now().UnixMilli()
	event := Event{ID: "restore-proof", Tenant: "tenant-restore", Time: now, Signal: "log", Service: "backup", Name: "restore"}
	if err = s.Write(ctx, []Event{event}); err != nil {
		t.Fatal(err)
	}
	if err = s.Close(); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(source)
	if err != nil {
		t.Fatal(err)
	}
	if err = os.WriteFile(restored, data, 0o600); err != nil {
		t.Fatal(err)
	}
	r, err := Open(restored)
	if err != nil {
		t.Fatal(err)
	}
	defer r.Close()
	page, err := r.Query(ctx, "tenant-restore", Query{From: now - 1, To: now + 1, Limit: 10})
	if err != nil || len(page.Events) != 1 || page.Events[0].ID != event.ID {
		t.Fatalf("restored event mismatch: page=%+v err=%v", page, err)
	}
}

// TestReleaseLoadHarness is opt-in because it is a measurement harness, not a
// unit test. Run with SENNET_RELEASE_LOAD=1 go test -run TestReleaseLoadHarness
// -count=1 -v ./platform. It exercises the local SQLite durability boundary;
// distributed Kafka/ClickHouse capacity must be measured separately.
func TestReleaseLoadHarness(t *testing.T) {
	if os.Getenv("SENNET_RELEASE_LOAD") != "1" {
		t.Skip("set SENNET_RELEASE_LOAD=1 to execute the release load harness")
	}
	const total, duplicateCount, tenants, queryWorkers = 12000, 600, 8, 8
	ctx := context.Background()
	s, err := Open(t.TempDir() + "/release-load.db")
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()

	var before, after runtime.MemStats
	runtime.ReadMemStats(&before)
	now := time.Now().UnixMilli()
	events := make([]Event, 0, total)
	for i := 0; i < total; i++ {
		tenant := fmt.Sprintf("tenant-%02d", 1+(i%tenants))
		if i < total*70/100 {
			tenant = "tenant-00"
		} // explicit tenant skew
		padding := 128
		switch i % 10 {
		case 0:
			padding = 2048
		case 1, 2:
			padding = 512
		}
		events = append(events, Event{ID: fmt.Sprintf("event-%06d", i), Tenant: tenant,
			Time: now - int64(i%3600000), Signal: []string{"trace", "log", "metric", "flow"}[i%4],
			Service: fmt.Sprintf("service-%04d", i%1200), Name: "release-load",
			TraceID: fmt.Sprintf("trace-%06d", i/4), Value: float64(i % 1000),
			Attributes: map[string]string{"cardinality.key": fmt.Sprintf("value-%06d", i), "padding": string(make([]byte, padding))}})
	}

	batchLatencies := []time.Duration{}
	started := time.Now()
	for start := 0; start < len(events); start += 200 {
		end := start + 200
		if end > len(events) {
			end = len(events)
		}
		batch := events[start:end]
		for i := range batch {
			if err := validateEvent(&batch[i], true); err != nil {
				t.Fatal(err)
			}
		}
		at := time.Now()
		if err := s.Write(ctx, batch); err != nil {
			t.Fatal(err)
		}
		batchLatencies = append(batchLatencies, time.Since(at))
	}
	ingestElapsed := time.Since(started)
	dupes := append([]Event(nil), events[:duplicateCount]...)
	if err := s.Write(ctx, dupes); err != nil {
		t.Fatal(err)
	}
	rejected := 0
	bad := []Event{{ID: "bad", Time: now, Signal: "unsupported", Service: "svc"}}
	if validateEvents("tenant-00", bad) != nil {
		rejected++
	}

	queryLatencies := make([]time.Duration, 0, queryWorkers*10)
	var qmu sync.Mutex
	var wg sync.WaitGroup
	for worker := 0; worker < queryWorkers; worker++ {
		wg.Add(1)
		go func(worker int) {
			defer wg.Done()
			for i := 0; i < 10; i++ {
				at := time.Now()
				_, qerr := s.Query(ctx, fmt.Sprintf("tenant-%02d", worker%tenants), Query{From: now - 3600000, To: now + 1, Limit: 1000})
				if qerr != nil {
					t.Error(qerr)
					return
				}
				qmu.Lock()
				queryLatencies = append(queryLatencies, time.Since(at))
				qmu.Unlock()
			}
		}(worker)
	}
	wg.Wait()

	stored := 0
	for tenant := 0; tenant <= tenants; tenant++ {
		cursor := ""
		for {
			p, qerr := s.Query(ctx, fmt.Sprintf("tenant-%02d", tenant), Query{From: now - 3600000, To: now + 1, Limit: 1000, Cursor: cursor})
			if qerr != nil {
				t.Fatal(qerr)
			}
			stored += len(p.Events)
			if p.Cursor == "" {
				break
			}
			cursor = p.Cursor
		}
	}
	runtime.ReadMemStats(&after)
	accepted := total
	lost := accepted - stored
	if lost != 0 {
		t.Fatalf("accepted=%d stored=%d lost=%d", accepted, stored, lost)
	}
	version := "unknown"
	if bi, ok := debug.ReadBuildInfo(); ok {
		version = bi.GoVersion
	}
	report := map[string]any{
		"boundary": "local SQLite WAL, synchronous=FULL", "go": version, "os": runtime.GOOS, "arch": runtime.GOARCH,
		"payload_distribution_bytes": map[string]int{"70_percent": 128, "20_percent": 512, "10_percent": 2048},
		"cardinality_values":         total, "tenant_skew_percent": 70, "concurrent_queries": queryWorkers,
		"accepted": accepted, "durable": accepted, "stored": stored, "rejected": rejected,
		"duplicate_attempts": duplicateCount, "duplicate_rows": 0, "lost": lost,
		"sustained_ingest_events_per_second": float64(total) / ingestElapsed.Seconds(),
		"ingest_batch_p95_ms":                durationPercentile(batchLatencies, 95), "ingest_batch_p99_ms": durationPercentile(batchLatencies, 99),
		"query_p95_ms": durationPercentile(queryLatencies, 95), "query_p99_ms": durationPercentile(queryLatencies, 99),
		"elapsed_ms": ingestElapsed.Milliseconds(), "heap_alloc_delta_bytes": int64(after.TotalAlloc - before.TotalAlloc),
	}
	b, _ := json.Marshal(report)
	t.Log(string(b))
}

func durationPercentile(values []time.Duration, p int) float64 {
	ordered := append([]time.Duration(nil), values...)
	sort.Slice(ordered, func(i, j int) bool { return ordered[i] < ordered[j] })
	if len(ordered) == 0 {
		return 0
	}
	index := (len(ordered)*p+99)/100 - 1
	if index < 0 {
		index = 0
	}
	return float64(ordered[index].Microseconds()) / 1000
}
