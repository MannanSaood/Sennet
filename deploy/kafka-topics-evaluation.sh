#!/bin/bash
set -euo pipefail
bin=/opt/kafka/bin/kafka-topics.sh
common=(--bootstrap-server kafka:9092 --create --if-not-exists --replication-factor 1)
"$bin" "${common[@]}" --topic sennet-events --partitions 12 --config min.insync.replicas=1 --config retention.ms=259200000
"$bin" "${common[@]}" --topic sennet-events-dead-letter --partitions 12 --config min.insync.replicas=1 --config retention.ms=604800000 --config cleanup.policy=delete
