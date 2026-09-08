# Metrics

OTLP gauge, sum and explicit histogram observations are accepted. Integer exact values and histogram boundaries/counts are preserved as attributes. Network snapshots include cumulative byte/packet counters; calculate rates from differences with reset handling, not from the raw counter value.

The TUI shows byte-per-second rates using measured elapsed time. Web event charts count records within the displayed page. Partial query results and missing data are explicitly labeled. A zero counter is not evidence that a failed collector is healthy.
