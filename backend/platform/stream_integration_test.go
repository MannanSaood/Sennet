package platform

import (
	"context"
	"encoding/json"
	"errors"
	"github.com/segmentio/kafka-go"
	"os"
	"sync"
	"testing"
	"time"
)

type dedupSink struct {
	mu             sync.Mutex
	events         map[string]Event
	failAfterWrite int
	unavailable    bool
}

func (s *dedupSink) Write(_ context.Context, events []Event) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.unavailable {
		return errors.New("storage unavailable")
	}
	for _, e := range events {
		s.events[e.Tenant+":"+e.ID] = e
	}
	if s.failAfterWrite > 0 {
		s.failAfterWrite--
		return errors.New("ambiguous response after append")
	}
	return nil
}
func (s *dedupSink) Ping(context.Context) error {
	if s.unavailable {
		return errors.New("storage unavailable")
	}
	return nil
}
func (s *dedupSink) count() int { s.mu.Lock(); defer s.mu.Unlock(); return len(s.events) }

type memoryDLQ struct {
	mu    sync.Mutex
	items []DeadLetter
}

func (d *memoryDLQ) WriteDeadLetter(_ context.Context, item DeadLetter) error {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.items = append(d.items, item)
	return nil
}

type fakeWriter struct {
	mu       sync.Mutex
	messages []kafka.Message
}

func (w *fakeWriter) WriteMessages(_ context.Context, messages ...kafka.Message) error {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.messages = append(w.messages, messages...)
	return nil
}
func (w *fakeWriter) Close() error { return nil }

type blockingReader struct {
	message kafka.Message
	sent    bool
	commits int
}

func (r *blockingReader) FetchMessage(ctx context.Context) (kafka.Message, error) {
	if !r.sent {
		r.sent = true
		return r.message, nil
	}
	<-ctx.Done()
	return kafka.Message{}, ctx.Err()
}
func (r *blockingReader) CommitMessages(context.Context, ...kafka.Message) error {
	r.commits++
	return nil
}
func (r *blockingReader) Stats() kafka.ReaderStats { return kafka.ReaderStats{Lag: 1} }
func (r *blockingReader) Close() error             { return nil }

func streamMessage(t *testing.T, id string) kafka.Message {
	t.Helper()
	e := sample(id)
	e.Tenant = "tenant-a"
	b, err := json.Marshal(e)
	if err != nil {
		t.Fatal(err)
	}
	return kafka.Message{Topic: "sennet-events", Partition: 2, Offset: 9, Key: []byte(partitionKey(e)), Value: b}
}

func TestStorageRetryAfterAppendAndDuplicateDelivery(t *testing.T) {
	sink := &dedupSink{events: map[string]Event{}, failAfterWrite: 1}
	c := &StorageConsumer{Sink: sink, Topic: "sennet-events", MaxDeadLetter: 1 << 20}
	m := streamMessage(t, "immutable-1")
	if err := c.process(context.Background(), []kafka.Message{m}); err == nil {
		t.Fatal("expected ambiguous append failure")
	}
	if err := c.process(context.Background(), []kafka.Message{m, m}); err != nil {
		t.Fatal(err)
	}
	if sink.count() != 1 {
		t.Fatalf("duplicate delivery stored %d events", sink.count())
	}
}
func TestConsumerRestartReplaysUncommittedEvent(t *testing.T) {
	sink := &dedupSink{events: map[string]Event{}}
	m := streamMessage(t, "restart-1")
	if err := (&StorageConsumer{Sink: sink, Topic: "sennet-events", MaxDeadLetter: 1 << 20}).process(context.Background(), []kafka.Message{m}); err != nil {
		t.Fatal(err)
	}
	if err := (&StorageConsumer{Sink: sink, Topic: "sennet-events", MaxDeadLetter: 1 << 20}).process(context.Background(), []kafka.Message{m}); err != nil {
		t.Fatal(err)
	}
	if sink.count() != 1 {
		t.Fatal("restart duplicate was not idempotent")
	}
}
func TestPoisonRecordDeadLetterAndCorrectedReplay(t *testing.T) {
	sink := &dedupSink{events: map[string]Event{}}
	dlq := &memoryDLQ{}
	bad := sample("poison-1")
	bad.Tenant = "tenant-a"
	bad.Signal = "unknown"
	raw, _ := json.Marshal(bad)
	msg := kafka.Message{Partition: 1, Offset: 4, Key: []byte("key"), Value: raw}
	c := &StorageConsumer{Sink: sink, DeadLetters: dlq, Topic: "sennet-events", MaxDeadLetter: 1 << 20}
	if err := c.process(context.Background(), []kafka.Message{msg}); err != nil {
		t.Fatal(err)
	}
	if len(dlq.items) != 1 || dlq.items[0].Reason != "invalid_event" || sink.count() != 0 {
		t.Fatalf("unexpected poison handling: %+v", dlq.items)
	}
	corrected := bad
	corrected.Signal = "trace"
	writer := &fakeWriter{}
	admin := &KafkaDeadLetterAdmin{Source: &KafkaProducer{Writer: writer, MaxBytes: 1 << 20}}
	if _, err := admin.replay(context.Background(), dlq.items[0], &corrected); err != nil {
		t.Fatal(err)
	}
	if _, err := admin.replay(context.Background(), dlq.items[0], &corrected); err != nil {
		t.Fatal(err)
	}
	if len(writer.messages) != 2 {
		t.Fatal("repeat replay did not append")
	}
	if err := c.process(context.Background(), writer.messages); err != nil {
		t.Fatal(err)
	}
	if sink.count() != 1 {
		t.Fatal("repeat replay was not storage-idempotent")
	}
}
func TestUnavailableStorageRetainsOffset(t *testing.T) {
	reader := &blockingReader{message: streamMessage(t, "unavailable-1")}
	sink := &dedupSink{events: map[string]Event{}, unavailable: true}
	c := &StorageConsumer{Reader: reader, Sink: sink, Topic: "sennet-events", Brokers: []string{"unused"}, BatchSize: 1, BatchWait: 10 * time.Millisecond, WriteTimeout: 10 * time.Millisecond, RetryDelay: time.Millisecond, MaxDeadLetter: 1 << 20}
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()
	_ = c.Run(ctx)
	if reader.commits != 0 {
		t.Fatal("offset committed while storage unavailable")
	}
}
func TestFileArchiveIsIdempotent(t *testing.T) {
	f := &FileArchive{Root: t.TempDir()}
	m := streamMessage(t, "archive-1")
	var e Event
	_ = json.Unmarshal(m.Value, &e)
	r := ArchiveRecord{Topic: "sennet-events", Partition: m.Partition, Offset: m.Offset, Key: m.Key, Value: m.Value, Event: e}
	if err := f.Write(context.Background(), []ArchiveRecord{r, r}); err != nil {
		t.Fatal(err)
	}
	b, err := os.ReadFile(f.path(r))
	if err != nil || len(b) == 0 {
		t.Fatal("archive not written", err)
	}
}
