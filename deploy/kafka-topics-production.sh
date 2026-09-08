#!/bin/bash
# Production-oriented defaults require at least three live brokers. Validate
# capacity, retention, and failure behavior for the actual cluster before use.
set -euo pipefail
: "${KAFKA_BOOTSTRAP_SERVERS:?set KAFKA_BOOTSTRAP_SERVERS}"
bin="${KAFKA_TOPICS_BIN:-kafka-topics.sh}"
common=(--bootstrap-server "$KAFKA_BOOTSTRAP_SERVERS" --create --if-not-exists --replication-factor 3)
"$bin" "${common[@]}" --topic "${SENNET_KAFKA_TOPIC:-sennet-events}" --partitions "${SENNET_KAFKA_PARTITIONS:-48}" --config min.insync.replicas=2 --config retention.ms="${SENNET_KAFKA_RETENTION_MS:-604800000}"
"$bin" "${common[@]}" --topic "${SENNET_KAFKA_DLQ_TOPIC:-sennet-events-dead-letter}" --partitions "${SENNET_KAFKA_PARTITIONS:-48}" --config min.insync.replicas=2 --config retention.ms="${SENNET_KAFKA_DLQ_RETENTION_MS:-604800000}" --config cleanup.policy=delete
