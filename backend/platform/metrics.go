package platform

import (
	"fmt"
	"strings"
	"sync/atomic"
)

type DataPlaneMetrics struct {
	IngestCapacity, IngestInFlight                                            atomic.Int64
	AcceptedEvents, RejectedEvents, QueueSaturated, ProducerErrors            atomic.Uint64
	ConsumerLag                                                               atomic.Int64
	ConsumerRetries, ConsumerBatches, ConsumerEvents, DeadLettered, Replayed  atomic.Uint64
	QueryInFlight, QueryCapacity                                              atomic.Int64
	QueryRequests, QueryRejected, QueryPartial, QueryCancelled                atomic.Uint64
	QueryRejectValidation, QueryRejectAdmission, QueryRejectUnavailable       atomic.Uint64
	QueryRows, QueryBytes, QueryCPUNanos, QueryLatencyNanos                   atomic.Uint64
	StorageLatencyNanos, StorageWrites, QueueAgeMillis, GatewayAccepted       atomic.Uint64
	MonitorEvaluations, MonitorTransitions, MonitorErrors, MonitorDelayMillis atomic.Uint64
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
		{"sennet_gateway_acceptance_total", "Events accepted at the gateway durability boundary.", "counter", m.GatewayAccepted.Load()},
		{"sennet_storage_write_latency_seconds_total", "Cumulative analytics storage write latency.", "counter", float64(m.StorageLatencyNanos.Load()) / 1e9},
		{"sennet_storage_queue_age_milliseconds", "Age of the oldest record in the last storage batch.", "gauge", m.QueueAgeMillis.Load()},
		{"sennet_query_capacity", "Configured concurrent analytical query capacity.", "gauge", m.QueryCapacity.Load()},
		{"sennet_query_in_flight", "Analytical queries currently executing.", "gauge", m.QueryInFlight.Load()},
		{"sennet_query_requests_total", "Admitted analytical query requests.", "counter", m.QueryRequests.Load()},
		{"sennet_query_rejected_total", "Analytical queries rejected by validation or admission.", "counter", m.QueryRejected.Load()},
		{"sennet_query_partial_total", "Analytical queries returning bounded partial results.", "counter", m.QueryPartial.Load()},
		{"sennet_query_cancelled_total", "Analytical queries cancelled or timed out.", "counter", m.QueryCancelled.Load()},
		{"sennet_query_rows_scanned_total", "Rows scanned by analytical queries.", "counter", m.QueryRows.Load()},
		{"sennet_query_bytes_scanned_total", "Estimated bytes scanned by analytical queries.", "counter", m.QueryBytes.Load()},
		{"sennet_query_cpu_seconds_total", "Cumulative analytical query CPU time.", "counter", float64(m.QueryCPUNanos.Load()) / 1e9},
		{"sennet_query_latency_seconds_total", "Cumulative analytical query wall time.", "counter", float64(m.QueryLatencyNanos.Load()) / 1e9},
		{"sennet_monitor_evaluations_total", "Durably recorded monitor evaluations.", "counter", m.MonitorEvaluations.Load()},
		{"sennet_monitor_transitions_total", "Unique monitor state transitions enqueued.", "counter", m.MonitorTransitions.Load()},
		{"sennet_monitor_errors_total", "Monitor evaluation failures.", "counter", m.MonitorErrors.Load()},
		{"sennet_monitor_delay_milliseconds", "Delay between scheduled window end and evaluation.", "gauge", m.MonitorDelayMillis.Load()},
	}
	var b strings.Builder
	for _, item := range values {
		fmt.Fprintf(&b, "# HELP %s %s\n# TYPE %s %s\n%s{role=%q} %v\n", item.name, item.help, item.name, item.kind, item.name, role, item.v)
	}
	fmt.Fprintf(&b, "# HELP sennet_query_rejections_by_reason_total Analytical query rejections by stable reason.\n# TYPE sennet_query_rejections_by_reason_total counter\n")
	for _, item := range []struct {
		reason string
		value  uint64
	}{{"validation", m.QueryRejectValidation.Load()}, {"admission", m.QueryRejectAdmission.Load()}, {"unavailable", m.QueryRejectUnavailable.Load()}} {
		fmt.Fprintf(&b, "sennet_query_rejections_by_reason_total{role=%q,reason=%q} %d\n", role, item.reason, item.value)
	}
	return b.String()
}
