package platform

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"errors"
	"github.com/segmentio/kafka-go"
	"github.com/segmentio/kafka-go/sasl/plain"
	"log"
	"net"
	"time"
)

type Ingestor interface {
	Write(context.Context, []Event) error
}
type KafkaLog struct {
	Writer *kafka.Writer
	Reader *kafka.Reader
}

func NewKafka(brokers []string, topic, user, password string, secure bool) *KafkaLog {
	dialer := &kafka.Dialer{Timeout: 10 * time.Second}
	transport := &kafka.Transport{}
	if secure {
		dialer.TLS = &tls.Config{MinVersion: tls.VersionTLS12}
		transport.TLS = dialer.TLS
	}
	if user != "" {
		mechanism := plain.Mechanism{Username: user, Password: password}
		dialer.SASLMechanism = mechanism
		transport.SASL = mechanism
	}
	return &KafkaLog{Writer: &kafka.Writer{Addr: kafka.TCP(brokers...), Topic: topic, Balancer: &kafka.Hash{}, RequiredAcks: kafka.RequireAll, Async: false, MaxAttempts: 3, WriteTimeout: 10 * time.Second, BatchBytes: 1 << 20, Transport: transport}, Reader: kafka.NewReader(kafka.ReaderConfig{Brokers: brokers, Topic: topic, GroupID: "sennet-analytics-v1", Dialer: dialer, MinBytes: 1, MaxBytes: 8 << 20, CommitInterval: 0, StartOffset: kafka.FirstOffset})}
}
func (k *KafkaLog) Write(ctx context.Context, events []Event) error {
	messages := make([]kafka.Message, 0, len(events))
	for _, e := range events {
		b, err := json.Marshal(e)
		if err != nil {
			return err
		}
		key := e.Tenant + ":" + e.TraceID
		if e.TraceID == "" {
			key = e.Tenant + ":" + e.Service
		}
		messages = append(messages, kafka.Message{Key: []byte(key), Value: b})
	}
	return k.Writer.WriteMessages(ctx, messages...)
}
func (k *KafkaLog) Run(ctx context.Context, sink Telemetry) {
	for ctx.Err() == nil {
		messages := make([]kafka.Message, 0, 500)
		events := make([]Event, 0, 500)
		// Fetch under a short batch deadline; commit only after the full batch
		// reaches analytics. A crash after write is safely replayed by ID.
		batchCtx, cancel := context.WithTimeout(ctx, time.Second)
		for len(messages) < 500 {
			m, err := k.Reader.FetchMessage(batchCtx)
			if err != nil {
				break
			}
			var e Event
			if err = json.Unmarshal(m.Value, &e); err != nil || e.Tenant == "" {
				cancel()
				log.Printf("invalid stream record blocks partition %d offset %d; operator repair required", m.Partition, m.Offset)
				for ctx.Err() == nil {
					pause(ctx)
				}
				return
			}
			messages = append(messages, m)
			events = append(events, e)
		}
		cancel()
		if len(events) == 0 {
			pause(ctx)
			continue
		}
		for ctx.Err() == nil {
			writeCtx, cancel := context.WithTimeout(ctx, 20*time.Second)
			err := sink.Write(writeCtx, events)
			cancel()
			if err == nil {
				break
			}
			log.Print("analytics unavailable; preserving uncommitted offsets")
			pause(ctx)
		}
		if ctx.Err() != nil {
			return
		}
		for ctx.Err() == nil {
			if err := k.Reader.CommitMessages(ctx, messages...); err == nil {
				break
			}
			pause(ctx)
		}
	}
}
func pause(ctx context.Context) {
	select {
	case <-ctx.Done():
	case <-time.After(time.Second):
	}
}
func (k *KafkaLog) Close() error { return errors.Join(k.Reader.Close(), k.Writer.Close()) }
func brokerReachable(ctx context.Context, broker string) error {
	c, err := (&net.Dialer{Timeout: 2 * time.Second}).DialContext(ctx, "tcp", broker)
	if err == nil {
		c.Close()
	}
	return err
}
