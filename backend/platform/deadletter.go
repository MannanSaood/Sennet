package platform

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/segmentio/kafka-go"
	"time"
)

type DeadLetterAdmin interface {
	List(context.Context, int) ([]DeadLetter, error)
	Replay(context.Context, string, *Event) (DeadLetter, error)
}

type KafkaDeadLetterAdmin struct {
	Config    KafkaConfig
	Source    *KafkaProducer
	Metrics   *DataPlaneMetrics
	ScanLimit int
}

func (a *KafkaDeadLetterAdmin) reader(partition int) *kafka.Reader {
	dialer, _ := a.Config.dialerAndTransport()
	return kafka.NewReader(kafka.ReaderConfig{Brokers: a.Config.Brokers, Topic: a.Config.Topic, Partition: partition, Dialer: dialer, MinBytes: 1, MaxBytes: 8 << 20, StartOffset: kafka.FirstOffset})
}

func (a *KafkaDeadLetterAdmin) scan(ctx context.Context, limit int, id string) ([]DeadLetter, error) {
	if limit < 1 || limit > 100000 {
		return nil, errors.New("dead-letter scan bound must be 1-100000")
	}
	out := make([]DeadLetter, 0, limit)
	if len(a.Config.Brokers) == 0 {
		return nil, errors.New("no Kafka brokers configured")
	}
	dialer, _ := a.Config.dialerAndTransport()
	partitions, err := dialer.LookupPartitions(ctx, "tcp", a.Config.Brokers[0], a.Config.Topic)
	if err != nil {
		return nil, err
	}
	if len(partitions) == 0 {
		return out, nil
	}
	perPartition := (limit + len(partitions) - 1) / len(partitions)
	scanned := 0
	for _, partition := range partitions {
		if scanned >= limit {
			break
		}
		r := a.reader(partition.ID)
		partitionScanned := 0
		for scanned < limit && partitionScanned < perPartition {
			readCtx, cancel := context.WithTimeout(ctx, 250*time.Millisecond)
			m, err := r.ReadMessage(readCtx)
			cancel()
			if err != nil {
				if errors.Is(err, context.DeadlineExceeded) || errors.Is(err, context.Canceled) {
					break
				}
				_ = r.Close()
				return nil, err
			}
			scanned++
			partitionScanned++
			var d DeadLetter
			if json.Unmarshal(m.Value, &d) != nil {
				continue
			}
			if id == "" || d.ID == id {
				out = append(out, d)
			}
			if id != "" && d.ID == id {
				_ = r.Close()
				return out, nil
			}
		}
		_ = r.Close()
	}
	return out, nil
}

func (a *KafkaDeadLetterAdmin) List(ctx context.Context, limit int) ([]DeadLetter, error) {
	if limit < 1 || limit > 1000 {
		return nil, errors.New("dead-letter limit must be 1-1000")
	}
	return a.scan(ctx, limit, "")
}

func (a *KafkaDeadLetterAdmin) Replay(ctx context.Context, id string, replacement *Event) (DeadLetter, error) {
	if id == "" {
		return DeadLetter{}, errors.New("dead-letter ID required")
	}
	limit := a.ScanLimit
	if limit <= 0 {
		limit = 10000
	}
	items, err := a.scan(ctx, limit, id)
	if err != nil {
		return DeadLetter{}, err
	}
	if len(items) == 0 {
		return DeadLetter{}, errors.New("dead-letter record not found within scan bound")
	}
	return a.replay(ctx, items[0], replacement)
}

func (a *KafkaDeadLetterAdmin) replay(ctx context.Context, d DeadLetter, replacement *Event) (DeadLetter, error) {
	if !d.Replayable || d.OriginalValue == "" {
		return DeadLetter{}, errors.New("dead-letter record is not replayable")
	}
	key, err := base64.StdEncoding.DecodeString(d.OriginalKey)
	if err != nil {
		return DeadLetter{}, fmt.Errorf("decode original key: %w", err)
	}
	value, err := base64.StdEncoding.DecodeString(d.OriginalValue)
	if err != nil {
		return DeadLetter{}, fmt.Errorf("decode original value: %w", err)
	}
	var original Event
	_ = json.Unmarshal(value, &original)
	if replacement != nil {
		corrected := *replacement
		if original.ID != "" && corrected.ID == "" {
			corrected.ID = original.ID
		}
		if original.Tenant != "" && corrected.Tenant == "" {
			corrected.Tenant = original.Tenant
		}
		if original.Time != 0 && corrected.Time == 0 {
			corrected.Time = original.Time
		}
		if original.ID != "" && corrected.ID != original.ID {
			return DeadLetter{}, errors.New("replacement must preserve the original event ID")
		}
		if original.Tenant != "" && corrected.Tenant != original.Tenant {
			return DeadLetter{}, errors.New("replacement must preserve the gateway-derived tenant")
		}
		if original.Time != 0 && corrected.Time != original.Time {
			return DeadLetter{}, errors.New("replacement must preserve the original event timestamp")
		}
		value, err = json.Marshal(corrected)
		if err != nil {
			return DeadLetter{}, err
		}
	}
	e, reason, detail := poisonReason(value)
	if reason != "" {
		return DeadLetter{}, fmt.Errorf("record remains invalid (%s): %s", reason, detail)
	}
	if e.ID == "" {
		return DeadLetter{}, errors.New("replay would violate immutable event ID contract")
	}
	if err = a.Source.WriteRaw(ctx, key, value); err != nil {
		return DeadLetter{}, err
	}
	if a.Metrics != nil {
		a.Metrics.Replayed.Add(1)
	}
	return d, nil
}
