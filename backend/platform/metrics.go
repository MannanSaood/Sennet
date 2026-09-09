package platform

import (
	"fmt"
	"strings"
	"sync/atomic"
)

type DataPlaneMetrics struct {
	IngestCapacity, IngestInFlight                                           atomic.Int64
	AcceptedEvents, RejectedEvents, QueueSaturated, ProducerErrors           atomic.Uint64
	ConsumerLag                                                              atomic.Int64
	ConsumerRetries, ConsumerBatches, ConsumerEvents, DeadLettered, Replayed atomic.Uint64
}

func (m *DataPlaneMetrics) Prometheus(role string) string {
	values := []struct {
		name, help, kind string
		v                any
	}{
		{"sennet_ingest_capacity", "Configured concurrent durable append capacity.", "gauge", m.IngestCapacity.Load()},
		{"sennet_ingest_in_flight", "Durable appends currently in flight.", "gauge", m.IngestInFlight.Load()},
		{"sennet_ingest_events_accepted_total", "Events acknowledged after the durability boundary.", "counter", m.AcceptedEvents.Load()},
		{"sennet_ingest_events_rejected_total", "Events rejected before durable append.", "counter", m.RejectedEvents.Load()},
		{"sennet_ingest_queue_saturated_total", "Requests rejected because the gateway append queue was saturated.", "counter", m.QueueSaturated.Load()},
		{"sennet_kafka_producer_errors_total", "Kafka producer append errors.", "counter", m.ProducerErrors.Load()},
		{"sennet_storage_consumer_lag", "Kafka storage consumer lag reported by the active reader.", "gauge", m.ConsumerLag.Load()},
		{"sennet_storage_consumer_retries_total", "Storage, archive, dead-letter, and commit retries.", "counter", m.ConsumerRetries.Load()},
		{"sennet_storage_consumer_batches_total", "Storage consumer batches committed.", "counter", m.ConsumerBatches.Load()},
		{"sennet_storage_consumer_events_total", "Source records committed by the storage consumer.", "counter", m.ConsumerEvents.Load()},
		{"sennet_dead_letter_events_total", "Poison records durably dead-lettered.", "counter", m.DeadLettered.Load()},
		{"sennet_dead_letter_replays_total", "Dead-letter replay requests appended to the source topic.", "counter", m.Replayed.Load()},
	}
	var b strings.Builder
	for _, item := range values {
		fmt.Fprintf(&b, "# HELP %s %s\n# TYPE %s %s\n%s{role=%q} %v\n", item.name, item.help, item.name, item.kind, item.name, role, item.v)
	}
	return b.String()
}
