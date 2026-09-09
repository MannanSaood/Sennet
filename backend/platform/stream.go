package platform

import (
	"context"
	"crypto/sha256"
	"crypto/tls"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/segmentio/kafka-go"
	"github.com/segmentio/kafka-go/sasl/plain"
	"net"
	"strings"
	"time"
)

var (
	ErrIngestSaturated       = errors.New("ingest queue saturated")
	ErrProducerBatchTooLarge = errors.New("producer batch too large")
)

type Ingestor interface {
	Write(context.Context, []Event) error
}
type Pinger interface{ Ping(context.Context) error }

type KafkaConfig struct {
	Brokers                      []string
	Topic, Group, User, Password string
	TLS                          bool
	BatchSize                    int
	BatchBytes                   int64
	BatchTimeout, WriteTimeout   time.Duration
}

func (c KafkaConfig) dialerAndTransport() (*kafka.Dialer, *kafka.Transport) {
	dialer := &kafka.Dialer{Timeout: 10 * time.Second}
	transport := &kafka.Transport{}
	if c.TLS {
		tlsConfig := &tls.Config{MinVersion: tls.VersionTLS12}
		dialer.TLS, transport.TLS = tlsConfig, tlsConfig
	}
	if c.User != "" {
		mechanism := plain.Mechanism{Username: c.User, Password: c.Password}
		dialer.SASLMechanism, transport.SASL = mechanism, mechanism
	}
	return dialer, transport
}

type messageWriter interface {
	WriteMessages(context.Context, ...kafka.Message) error
	Close() error
}

// KafkaProducer acknowledges only after Kafka confirms an acks=all append.
// Kafka replication and min.insync.replicas remain broker/topic responsibilities.
type KafkaProducer struct {
	Writer   messageWriter
	Brokers  []string
	Topic    string
	MaxBytes int64
	Metrics  *DataPlaneMetrics
}

func NewKafkaProducer(c KafkaConfig, metrics *DataPlaneMetrics) *KafkaProducer {
	_, transport := c.dialerAndTransport()
	if c.BatchSize <= 0 {
		c.BatchSize = 500
	}
	if c.BatchBytes <= 0 {
		c.BatchBytes = 1 << 20
	}
	if c.BatchTimeout <= 0 {
		c.BatchTimeout = 10 * time.Millisecond
	}
	if c.WriteTimeout <= 0 {
		c.WriteTimeout = 10 * time.Second
	}
	w := &kafka.Writer{Addr: kafka.TCP(c.Brokers...), Topic: c.Topic, Balancer: &kafka.Hash{}, RequiredAcks: kafka.RequireAll, Async: false, MaxAttempts: 3, WriteTimeout: c.WriteTimeout, BatchSize: c.BatchSize, BatchBytes: c.BatchBytes, BatchTimeout: c.BatchTimeout, Transport: transport}
	return &KafkaProducer{Writer: w, Brokers: append([]string(nil), c.Brokers...), Topic: c.Topic, MaxBytes: c.BatchBytes, Metrics: metrics}
}

func partitionKey(e Event) string {
	if e.Signal == "finance" {
		return strings.Join([]string{e.Tenant, e.Attributes["source_id"], e.Attributes["account_id"], e.Attributes["entity_id"]}, ":")
	}
	if e.TraceID != "" {
		return e.Tenant + ":trace:" + e.TraceID
	}
	return e.Tenant + ":service:" + e.Service
}

func (k *KafkaProducer) Write(ctx context.Context, events []Event) error {
	messages := make([]kafka.Message, 0, len(events))
	var total int64
	for _, e := range events {
		b, err := json.Marshal(e)
		if err != nil {
			return err
		}
		key := partitionKey(e)
		total += int64(len(b) + len(key))
		if total > k.MaxBytes {
			return fmt.Errorf("%w: exceeds %d bytes", ErrProducerBatchTooLarge, k.MaxBytes)
		}
		messages = append(messages, kafka.Message{Key: []byte(key), Value: b})
	}
	if err := k.Writer.WriteMessages(ctx, messages...); err != nil {
		if k.Metrics != nil {
			k.Metrics.ProducerErrors.Add(1)
		}
		return err
	}
	return nil
}

func (k *KafkaProducer) WriteRaw(ctx context.Context, key, value []byte) error {
	if int64(len(key)+len(value)) > k.MaxBytes {
		return fmt.Errorf("record exceeds %d bytes", k.MaxBytes)
	}
	return k.Writer.WriteMessages(ctx, kafka.Message{Key: key, Value: value})
}
func (k *KafkaProducer) Ping(ctx context.Context) error {
	if len(k.Brokers) == 0 {
		return errors.New("no Kafka brokers configured")
	}
	return brokerReachable(ctx, k.Brokers[0])
}
func (k *KafkaProducer) Close() error { return k.Writer.Close() }

type BoundedIngestor struct {
	next    Ingestor
	slots   chan struct{}
	metrics *DataPlaneMetrics
}

func NewBoundedIngestor(next Ingestor, capacity int, metrics *DataPlaneMetrics) *BoundedIngestor {
	if capacity < 1 {
		capacity = 64
	}
	if metrics != nil {
		metrics.IngestCapacity.Store(int64(capacity))
	}
	return &BoundedIngestor{next: next, slots: make(chan struct{}, capacity), metrics: metrics}
}
func (b *BoundedIngestor) Write(ctx context.Context, events []Event) error {
	select {
	case b.slots <- struct{}{}:
		if b.metrics != nil {
			b.metrics.IngestInFlight.Store(int64(len(b.slots)))
		}
		defer func() {
			<-b.slots
			if b.metrics != nil {
				b.metrics.IngestInFlight.Store(int64(len(b.slots)))
			}
		}()
		return b.next.Write(ctx, events)
	default:
		if b.metrics != nil {
			b.metrics.QueueSaturated.Add(1)
		}
		return ErrIngestSaturated
	}
}

type DeadLetter struct {
	ID            string            `json:"id"`
	Reason        string            `json:"reason"`
	Detail        string            `json:"detail,omitempty"`
	SourceTopic   string            `json:"source_topic"`
	Partition     int               `json:"partition"`
	Offset        int64             `json:"offset"`
	OriginalKey   string            `json:"original_key"`
	OriginalValue string            `json:"original_value,omitempty"`
	Headers       map[string]string `json:"headers,omitempty"`
	CreatedAt     int64             `json:"created_at"`
	Replayable    bool              `json:"replayable"`
}

func deadLetterID(topic string, partition int, offset int64) string {
	sum := sha256.Sum256([]byte(fmt.Sprintf("%s/%d/%d", topic, partition, offset)))
	return hex.EncodeToString(sum[:16])
}

type DeadLetterWriter interface {
	WriteDeadLetter(context.Context, DeadLetter) error
}
type KafkaDeadLetterWriter struct{ Producer *KafkaProducer }

func (w KafkaDeadLetterWriter) WriteDeadLetter(ctx context.Context, d DeadLetter) error {
	b, err := json.Marshal(d)
	if err != nil {
		return err
	}
	return w.Producer.WriteRaw(ctx, []byte(d.ID), b)
}

type ArchiveRecord struct {
	Topic      string
	Partition  int
	Offset     int64
	Key, Value []byte
	Event      Event
}
type Archive interface {
	Write(context.Context, []ArchiveRecord) error
	Ping(context.Context) error
}
type messageReader interface {
	FetchMessage(context.Context) (kafka.Message, error)
	CommitMessages(context.Context, ...kafka.Message) error
	Stats() kafka.ReaderStats
	Close() error
}

type StorageConsumer struct {
	Reader                              messageReader
	Sink                                Ingestor
	Archive                             Archive
	DeadLetters                         DeadLetterWriter
	Topic                               string
	Brokers                             []string
	BatchSize                           int
	BatchWait, WriteTimeout, RetryDelay time.Duration
	MaxDeadLetter                       int
	Metrics                             *DataPlaneMetrics
}

func NewStorageConsumer(c KafkaConfig, sink Ingestor, archive Archive, deadLetters DeadLetterWriter, metrics *DataPlaneMetrics) *StorageConsumer {
	dialer, _ := c.dialerAndTransport()
	if c.Group == "" {
		c.Group = "sennet-storage-v1"
	}
	return &StorageConsumer{Reader: kafka.NewReader(kafka.ReaderConfig{Brokers: c.Brokers, Topic: c.Topic, GroupID: c.Group, Dialer: dialer, MinBytes: 1, MaxBytes: 8 << 20, CommitInterval: 0, StartOffset: kafka.FirstOffset}), Sink: sink, Archive: archive, DeadLetters: deadLetters, Topic: c.Topic, Brokers: append([]string(nil), c.Brokers...), BatchSize: 500, BatchWait: time.Second, WriteTimeout: 20 * time.Second, RetryDelay: time.Second, MaxDeadLetter: 1 << 20, Metrics: metrics}
}

func poisonReason(raw []byte) (Event, string, string) {
	var e Event
	if err := json.Unmarshal(raw, &e); err != nil {
		return e, "invalid_json", "record is not a valid event JSON object"
	}
	if e.Tenant == "" {
		return e, "missing_tenant", "trusted tenant is absent"
	}
	if err := validateEvent(&e, false); err != nil {
		return e, "invalid_event", err.Error()
	}
	return e, "", ""
}
func (c *StorageConsumer) process(ctx context.Context, messages []kafka.Message) error {
	events := make([]Event, 0, len(messages))
	archive := make([]ArchiveRecord, 0, len(messages))
	poison := make([]DeadLetter, 0)
	for _, m := range messages {
		e, reason, detail := poisonReason(m.Value)
		if reason != "" {
			d := DeadLetter{ID: deadLetterID(c.Topic, m.Partition, m.Offset), Reason: reason, Detail: detail, SourceTopic: c.Topic, Partition: m.Partition, Offset: m.Offset, OriginalKey: base64.StdEncoding.EncodeToString(m.Key), CreatedAt: time.Now().UnixMilli(), Replayable: len(m.Value) <= c.MaxDeadLetter}
			if d.Replayable {
				d.OriginalValue = base64.StdEncoding.EncodeToString(m.Value)
			} else {
				d.Reason, d.Detail = "record_too_large", "original value omitted by dead-letter size bound"
			}
			poison = append(poison, d)
			continue
		}
		events = append(events, e)
		archive = append(archive, ArchiveRecord{Topic: c.Topic, Partition: m.Partition, Offset: m.Offset, Key: append([]byte(nil), m.Key...), Value: append([]byte(nil), m.Value...), Event: e})
	}
	if len(archive) > 0 && c.Archive != nil {
		if err := c.Archive.Write(ctx, archive); err != nil {
			return fmt.Errorf("archive write: %w", err)
		}
	}
	if len(events) > 0 {
		if err := c.Sink.Write(ctx, events); err != nil {
			return fmt.Errorf("analytics write: %w", err)
		}
	}
	for _, d := range poison {
		if c.DeadLetters == nil {
			return errors.New("dead-letter writer is required for poison records")
		}
		if err := c.DeadLetters.WriteDeadLetter(ctx, d); err != nil {
			return fmt.Errorf("dead-letter write: %w", err)
		}
		if c.Metrics != nil {
			c.Metrics.DeadLettered.Add(1)
		}
	}
	return nil
}
func (c *StorageConsumer) Run(ctx context.Context) error {
	if c.BatchSize < 1 {
		c.BatchSize = 500
	}
	for ctx.Err() == nil {
		batchCtx, cancel := context.WithTimeout(ctx, c.BatchWait)
		messages := make([]kafka.Message, 0, c.BatchSize)
		for len(messages) < c.BatchSize {
			m, err := c.Reader.FetchMessage(batchCtx)
			if err != nil {
				break
			}
			messages = append(messages, m)
		}
		cancel()
		if len(messages) == 0 {
			continue
		}
		if c.Metrics != nil {
			c.Metrics.ConsumerLag.Store(c.Reader.Stats().Lag)
		}
		for ctx.Err() == nil {
			writeCtx, writeCancel := context.WithTimeout(ctx, c.WriteTimeout)
			err := c.process(writeCtx, messages)
			writeCancel()
			if err == nil {
				break
			}
			if c.Metrics != nil {
				c.Metrics.ConsumerRetries.Add(1)
			}
			if !pause(ctx, c.RetryDelay) {
				return ctx.Err()
			}
		}
		if ctx.Err() != nil {
			return ctx.Err()
		}
		for ctx.Err() == nil {
			if err := c.Reader.CommitMessages(ctx, messages...); err == nil {
				break
			}
			if c.Metrics != nil {
				c.Metrics.ConsumerRetries.Add(1)
			}
			if !pause(ctx, c.RetryDelay) {
				return ctx.Err()
			}
		}
		if c.Metrics != nil {
			stats := c.Reader.Stats()
			c.Metrics.ConsumerLag.Store(stats.Lag)
			c.Metrics.ConsumerBatches.Add(1)
			c.Metrics.ConsumerEvents.Add(uint64(len(messages)))
		}
	}
	return ctx.Err()
}
func (c *StorageConsumer) Ping(ctx context.Context) error {
	if len(c.Brokers) == 0 {
		return errors.New("no Kafka brokers configured")
	}
	if err := brokerReachable(ctx, c.Brokers[0]); err != nil {
		return err
	}
	if p, ok := c.Sink.(Pinger); ok {
		if err := p.Ping(ctx); err != nil {
			return err
		}
	}
	if c.Archive != nil {
		return c.Archive.Ping(ctx)
	}
	return nil
}
func (c *StorageConsumer) Close() error { return c.Reader.Close() }
func pause(ctx context.Context, delay time.Duration) bool {
	if delay <= 0 {
		delay = time.Second
	}
	t := time.NewTimer(delay)
	defer t.Stop()
	select {
	case <-ctx.Done():
		return false
	case <-t.C:
		return true
	}
}
func brokerReachable(ctx context.Context, broker string) error {
	c, err := (&net.Dialer{Timeout: 2 * time.Second}).DialContext(ctx, "tcp", broker)
	if err == nil {
		_ = c.Close()
	}
	return err
}
