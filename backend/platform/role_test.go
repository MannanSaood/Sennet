package platform

import (
	"context"
	"errors"
	"github.com/segmentio/kafka-go"
	"net/http/httptest"
	"testing"
	"time"
)

type blockingIngest struct {
	entered chan struct{}
	release chan struct{}
}

func (b *blockingIngest) Write(ctx context.Context, _ []Event) error {
	close(b.entered)
	select {
	case <-b.release:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

type fakeDeadAdmin struct{}

func (fakeDeadAdmin) List(context.Context, int) ([]DeadLetter, error) {
	return []DeadLetter{{ID: "dead-1", Reason: "invalid_event"}}, nil
}
func (fakeDeadAdmin) Replay(context.Context, string, *Event) (DeadLetter, error) {
	return DeadLetter{}, errors.New("not used")
}

func TestProcessRoleRoutingAndAuthenticatedMetrics(t *testing.T) {
	s, _, key, _ := fixture(t)
	metrics := &DataPlaneMetrics{}
	api := NewAPI(s, nil, s)
	api.Resolve = testResolver(key, "")
	api.Role = "gateway"
	api.Metrics = metrics
	api.InternalToken = "operator-secret"
	if w := call(api.Handler(), "GET", "/api/events", key, nil); w.Code != 404 {
		t.Fatalf("gateway served query endpoint: %d", w.Code)
	}
	r := httptest.NewRequest("GET", "/internal/metrics", nil)
	w := httptest.NewRecorder()
	api.Handler().ServeHTTP(w, r)
	if w.Code != 401 {
		t.Fatalf("unauthenticated metrics returned %d", w.Code)
	}
	r = httptest.NewRequest("GET", "/internal/metrics", nil)
	r.Header.Set("Authorization", "Bearer operator-secret")
	w = httptest.NewRecorder()
	api.Handler().ServeHTTP(w, r)
	if w.Code != 200 {
		t.Fatalf("authenticated metrics returned %d", w.Code)
	}
	query := NewAPI(s, s, nil)
	query.Resolve = testResolver(key, "")
	query.Role = "query-control"
	query.DeadLetters = fakeDeadAdmin{}
	if w = call(query.Handler(), "GET", "/api/dead-letter", key, nil); w.Code != 200 {
		t.Fatalf("admin dead-letter inspection returned %d: %s", w.Code, w.Body.String())
	}
}

func TestBoundedIngestorRejectsSaturation(t *testing.T) {
	next := &blockingIngest{entered: make(chan struct{}), release: make(chan struct{})}
	metrics := &DataPlaneMetrics{}
	bounded := NewBoundedIngestor(next, 1, metrics)
	done := make(chan error, 1)
	go func() { done <- bounded.Write(context.Background(), []Event{sample("one")}) }()
	<-next.entered
	if err := bounded.Write(context.Background(), []Event{sample("two")}); !errors.Is(err, ErrIngestSaturated) {
		t.Fatalf("expected saturation, got %v", err)
	}
	close(next.release)
	if err := <-done; err != nil {
		t.Fatal(err)
	}
	if metrics.QueueSaturated.Load() != 1 {
		t.Fatal("saturation metric not incremented")
	}
}

func TestKafkaProducerUsesAllAcknowledgementsAndByteBound(t *testing.T) {
	p := NewKafkaProducer(KafkaConfig{Brokers: []string{"127.0.0.1:1"}, Topic: "events", BatchSize: 7, BatchBytes: 16, BatchTimeout: time.Millisecond}, nil)
	defer p.Close()
	w, ok := p.Writer.(*kafka.Writer)
	if !ok || w.RequiredAcks != kafka.RequireAll || w.Async || w.BatchSize != 7 || w.BatchBytes != 16 {
		t.Fatal("producer durability or batching configuration changed")
	}
	if err := p.Write(context.Background(), []Event{sample("record-too-large")}); !errors.Is(err, ErrProducerBatchTooLarge) {
		t.Fatalf("expected producer byte rejection, got %v", err)
	}
}
