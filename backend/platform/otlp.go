package platform

import (
	"bytes"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	logs "go.opentelemetry.io/proto/otlp/collector/logs/v1"
	metrics "go.opentelemetry.io/proto/otlp/collector/metrics/v1"
	traces "go.opentelemetry.io/proto/otlp/collector/trace/v1"
	common "go.opentelemetry.io/proto/otlp/common/v1"
	metric "go.opentelemetry.io/proto/otlp/metrics/v1"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"
)

func attrs(values []*common.KeyValue) map[string]string {
	out := map[string]string{}
	for _, kv := range values {
		if kv == nil || kv.Value == nil {
			continue
		}
		v := kv.Value
		switch v.Value.(type) {
		case *common.AnyValue_StringValue:
			out[kv.Key] = v.GetStringValue()
		case *common.AnyValue_IntValue:
			out[kv.Key] = strconv.FormatInt(v.GetIntValue(), 10)
		case *common.AnyValue_DoubleValue:
			out[kv.Key] = strconv.FormatFloat(v.GetDoubleValue(), 'g', -1, 64)
		case *common.AnyValue_BoolValue:
			out[kv.Key] = strconv.FormatBool(v.GetBoolValue())
		default:
			b, _ := protojson.Marshal(v)
			out[kv.Key] = string(b)
		}
	}
	return out
}
func merged(base map[string]string, extra []*common.KeyValue) map[string]string {
	out := map[string]string{}
	for k, v := range base {
		out[k] = v
	}
	for k, v := range attrs(extra) {
		out[k] = v
	}
	return out
}
func eventTime(ns uint64) int64 {
	if ns == 0 {
		return time.Now().UnixMilli()
	}
	return int64(ns / 1000000)
}
func canonicalJSON(b []byte) ([]byte, error) {
	var v any
	decoder := json.NewDecoder(bytes.NewReader(b))
	decoder.UseNumber()
	if err := decoder.Decode(&v); err != nil {
		return nil, err
	}
	if err := decoder.Decode(new(any)); err != io.EOF {
		return nil, fmt.Errorf("trailing JSON content")
	}
	var visit func(any) error
	visit = func(x any) error {
		switch n := x.(type) {
		case map[string]any:
			for k, child := range n {
				if k == "traceId" || k == "spanId" || k == "parentSpanId" {
					s, ok := child.(string)
					if !ok {
						return fmt.Errorf("invalid ID")
					}
					raw, err := hex.DecodeString(s)
					if err != nil {
						return err
					}
					n[k] = base64.StdEncoding.EncodeToString(raw)
				} else if err := visit(child); err != nil {
					return err
				}
			}
		case []any:
			for _, child := range n {
				if err := visit(child); err != nil {
					return err
				}
			}
		}
		return nil
	}
	if err := visit(v); err != nil {
		return nil, err
	}
	return json.Marshal(v)
}
func (a *API) otlp(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		problem(w, 405, "POST required")
		return
	}
	var msg proto.Message
	switch r.URL.Path {
	case "/v1/traces":
		msg = &traces.ExportTraceServiceRequest{}
	case "/v1/logs":
		msg = &logs.ExportLogsServiceRequest{}
	case "/v1/metrics":
		msg = &metrics.ExportMetricsServiceRequest{}
	default:
		problem(w, 404, "unknown signal")
		return
	}
	b, err := io.ReadAll(r.Body)
	if err != nil {
		problem(w, 400, "invalid payload")
		return
	}
	binary := strings.HasPrefix(r.Header.Get("Content-Type"), "application/x-protobuf")
	if binary {
		err = proto.Unmarshal(b, msg)
	} else if strings.HasPrefix(r.Header.Get("Content-Type"), "application/json") {
		b, err = canonicalJSON(b)
		if err == nil {
			err = protojson.Unmarshal(b, msg)
		}
	} else {
		problem(w, 415, "OTLP requires application/json or application/x-protobuf")
		return
	}
	if err != nil {
		problem(w, 400, "invalid OTLP payload")
		return
	}
	events := []Event{}
	switch req := msg.(type) {
	case *traces.ExportTraceServiceRequest:
		for _, rs := range req.ResourceSpans {
			base := attrs(rs.GetResource().GetAttributes())
			service := base["service.name"]
			if service == "" {
				service = "unknown_service"
			}
			for _, scope := range rs.ScopeSpans {
				for _, s := range scope.Spans {
					if len(s.TraceId) != 16 || len(s.SpanId) != 8 || s.EndTimeUnixNano < s.StartTimeUnixNano {
						problem(w, 400, "invalid span IDs or duration")
						return
					}
					attributes := merged(base, s.Attributes)
					status := "ok"
					if s.GetStatus().GetCode() == 2 {
						status = "error"
					}
					signal := "trace"
					if attributes["gen_ai.operation.name"] != "" || attributes["agent.id"] != "" {
						signal = "agent"
					}
					if len(s.Links) > 0 {
						links := []string{}
						for _, l := range s.Links {
							links = append(links, hex.EncodeToString(l.TraceId)+":"+hex.EncodeToString(l.SpanId))
						}
						attributes["span.links"] = strings.Join(links, ",")
					}
					events = append(events, Event{ID: hex.EncodeToString(s.TraceId) + ":" + hex.EncodeToString(s.SpanId), Time: eventTime(s.StartTimeUnixNano), Signal: signal, Service: service, Name: s.Name, TraceID: hex.EncodeToString(s.TraceId), SpanID: hex.EncodeToString(s.SpanId), ParentID: hex.EncodeToString(s.ParentSpanId), Duration: float64(s.EndTimeUnixNano-s.StartTimeUnixNano) / 1e6, Status: status, Attributes: attributes})
				}
			}
		}
	case *logs.ExportLogsServiceRequest:
		for _, rs := range req.ResourceLogs {
			base := attrs(rs.GetResource().GetAttributes())
			service := base["service.name"]
			if service == "" {
				service = "unknown_service"
			}
			for _, scope := range rs.ScopeLogs {
				for _, l := range scope.LogRecords {
					raw, _ := proto.Marshal(l)
					attributes := merged(base, l.Attributes)
					attributes["severity"] = l.SeverityText
					status := "ok"
					if l.SeverityNumber >= 17 {
						status = "error"
					}
					events = append(events, Event{ID: digest(service + string(raw)), Time: eventTime(l.TimeUnixNano), Signal: "log", Service: service, Name: l.GetBody().GetStringValue(), TraceID: hex.EncodeToString(l.TraceId), SpanID: hex.EncodeToString(l.SpanId), Status: status, Attributes: attributes})
				}
			}
		}
	case *metrics.ExportMetricsServiceRequest:
		for _, rs := range req.ResourceMetrics {
			base := attrs(rs.GetResource().GetAttributes())
			service := base["service.name"]
			if service == "" {
				service = "unknown_service"
			}
			for _, scope := range rs.ScopeMetrics {
				for _, m := range scope.Metrics {
					addPoint := func(p *metric.NumberDataPoint, kind string) {
						raw, _ := proto.Marshal(p)
						at := merged(base, p.Attributes)
						at["unit"] = m.Unit
						at["metric.kind"] = kind
						v := p.GetAsDouble()
						if _, ok := p.Value.(*metric.NumberDataPoint_AsInt); ok {
							v = float64(p.GetAsInt())
							at["exact_value"] = strconv.FormatInt(p.GetAsInt(), 10)
						}
						events = append(events, Event{ID: digest(service + m.Name + string(raw)), Time: eventTime(p.TimeUnixNano), Signal: "metric", Service: service, Name: m.Name, Value: v, Status: "ok", Attributes: at})
					}
					switch d := m.Data.(type) {
					case *metric.Metric_Gauge:
						for _, p := range d.Gauge.DataPoints {
							addPoint(p, "gauge")
						}
					case *metric.Metric_Sum:
						for _, p := range d.Sum.DataPoints {
							addPoint(p, "sum:"+d.Sum.AggregationTemporality.String())
						}
					case *metric.Metric_Histogram:
						for _, p := range d.Histogram.DataPoints {
							raw, _ := proto.Marshal(p)
							at := merged(base, p.Attributes)
							at["unit"] = m.Unit
							at["metric.kind"] = "histogram"
							at["count"] = strconv.FormatUint(p.Count, 10)
							b, _ := json.Marshal(p.BucketCounts)
							at["buckets"] = string(b)
							b, _ = json.Marshal(p.ExplicitBounds)
							at["bounds"] = string(b)
							events = append(events, Event{ID: digest(service + m.Name + string(raw)), Time: eventTime(p.TimeUnixNano), Signal: "metric", Service: service, Name: m.Name, Value: p.GetSum(), Status: "ok", Attributes: at})
						}
					default:
						problem(w, 400, "metric type unsupported; use gauge, sum or explicit histogram")
						return
					}
				}
			}
		}
	}
	if len(events) > 0 {
		if err = validateEvents(principal(r).Tenant, events); err != nil {
			if a.Metrics != nil {
				a.Metrics.RejectedEvents.Add(uint64(len(events)))
			}
			problem(w, 400, err.Error())
			return
		}
		if err = a.Ingest.Write(r.Context(), events); err != nil {
			if a.Metrics != nil {
				a.Metrics.RejectedEvents.Add(uint64(len(events)))
			}
			if errors.Is(err, ErrIngestSaturated) {
				w.Header().Set("Retry-After", "1")
				problem(w, 429, "ingest queue saturated; retry with the same event IDs")
				return
			}
			if errors.Is(err, ErrProducerBatchTooLarge) {
				problem(w, 413, "batch exceeds configured Kafka append bound; split the request")
				return
			}
			problem(w, 503, "durable ingest unavailable; retry with the same event IDs")
			return
		}
		if a.Metrics != nil {
			a.Metrics.AcceptedEvents.Add(uint64(len(events)))
		}
	}
	if binary {
		w.Header().Set("Content-Type", "application/x-protobuf")
		w.WriteHeader(200)
	} else {
		respond(w, 200, map[string]any{})
	}
}
