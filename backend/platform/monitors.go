package platform

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"net/http"
	"time"
)

type SLODefinition struct {
	Target           float64 `json:"target"`
	WindowDays       int     `json:"window_days"`
	FastShortMinutes int     `json:"fast_short_minutes"`
	FastLongMinutes  int     `json:"fast_long_minutes"`
	FastBurnRate     float64 `json:"fast_burn_rate"`
	SlowShortMinutes int     `json:"slow_short_minutes"`
	SlowLongMinutes  int     `json:"slow_long_minutes"`
	SlowBurnRate     float64 `json:"slow_burn_rate"`
}

type Monitor struct {
	ID        string         `json:"id"`
	Version   string         `json:"version"`
	Name      string         `json:"name"`
	Signal    string         `json:"signal"`
	Service   string         `json:"service"`
	Threshold int            `json:"threshold,omitempty"`
	SLO       *SLODefinition `json:"slo,omitempty"`
	State     string         `json:"state"`
	Observed  float64        `json:"observed"`
	Checked   int64          `json:"checked_at"`
	Partial   bool           `json:"partial"`
}

type Notification struct {
	ID, Tenant, MonitorID, EvaluationKey, Transition string
	Payload                                          []byte
	Created                                          int64
}

// NotificationOutbox is a durability boundary, not a provider. Dispatchers mark
// records delivered only after an external provider acknowledges them.
type NotificationOutbox interface {
	Pending(context.Context, int) ([]Notification, error)
	MarkDelivered(context.Context, string, int64) error
}

func (s *Store) Pending(ctx context.Context, limit int) ([]Notification, error) {
	if limit < 1 || limit > 500 {
		return nil, errors.New("outbox limit must be 1-500")
	}
	rows, err := s.db.QueryContext(ctx, s.q(`SELECT id,tenant,monitor_id,evaluation_key,transition,payload,created FROM platform_notification_outbox WHERE delivered=0 ORDER BY created,id LIMIT ?`), limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Notification{}
	for rows.Next() {
		var n Notification
		if err = rows.Scan(&n.ID, &n.Tenant, &n.MonitorID, &n.EvaluationKey, &n.Transition, &n.Payload, &n.Created); err != nil {
			return nil, err
		}
		out = append(out, n)
	}
	return out, rows.Err()
}
func (s *Store) MarkDelivered(ctx context.Context, id string, at int64) error {
	r, err := s.db.ExecContext(ctx, s.q(`UPDATE platform_notification_outbox SET delivered=? WHERE id=? AND delivered=0`), at, id)
	if err != nil {
		return err
	}
	n, _ := r.RowsAffected()
	if n != 1 {
		return sql.ErrNoRows
	}
	return nil
}

type MonitorEvaluator struct {
	Store     *Store
	Telemetry Telemetry
	Metrics   *DataPlaneMetrics
	Now       func() time.Time
}

func defaultSLO(s *SLODefinition) {
	if s.WindowDays == 0 {
		s.WindowDays = 30
	}
	if s.FastShortMinutes == 0 {
		s.FastShortMinutes = 5
	}
	if s.FastLongMinutes == 0 {
		s.FastLongMinutes = 60
	}
	if s.FastBurnRate == 0 {
		s.FastBurnRate = 14.4
	}
	if s.SlowShortMinutes == 0 {
		s.SlowShortMinutes = 30
	}
	if s.SlowLongMinutes == 0 {
		s.SlowLongMinutes = 360
	}
	if s.SlowBurnRate == 0 {
		s.SlowBurnRate = 6
	}
}
func validMonitor(m *Monitor) bool {
	if m.Version == "" {
		m.Version = "v1"
	}
	if m.Version != "v1" || m.Name == "" || len(m.Name) > 200 || (m.Signal != "" && !signals[m.Signal]) || len(m.Service) > 200 {
		return false
	}
	if m.SLO == nil {
		return m.Threshold >= 1 && m.Threshold <= 1000000
	}
	defaultSLO(m.SLO)
	return m.SLO.Target > 0 && m.SLO.Target < 1 && m.SLO.WindowDays >= 1 && m.SLO.WindowDays <= 365 && m.SLO.FastShortMinutes < m.SLO.FastLongMinutes && m.SLO.SlowShortMinutes < m.SLO.SlowLongMinutes && m.SLO.SlowLongMinutes <= m.SLO.WindowDays*24*60
}
func (e *MonitorEvaluator) summary(ctx context.Context, tenant string, m Monitor, end int64, minutes int) (Summary, error) {
	a, ok := e.Telemetry.(Aggregator)
	if !ok {
		return Summary{}, errors.New("aggregation unavailable")
	}
	return a.Summary(ctx, tenant, Query{From: end - int64(minutes)*60000, To: end, Signal: m.Signal, Service: m.Service, Limit: 1000})
}
func burn(sum Summary, target float64) float64 {
	if sum.Events == 0 {
		return 0
	}
	return (float64(sum.Errors) / float64(sum.Events)) / (1 - target)
}
func (e *MonitorEvaluator) calculate(ctx context.Context, tenant string, m Monitor, end int64) (string, float64, error) {
	if m.SLO == nil {
		s, err := e.summary(ctx, tenant, m, end, 5)
		if err != nil {
			return "unavailable", 0, err
		}
		if s.Errors >= int64(m.Threshold) {
			return "firing", float64(s.Errors), nil
		}
		return "healthy", float64(s.Errors), nil
	}
	s := *m.SLO
	defaultSLO(&s)
	fs, err := e.summary(ctx, tenant, m, end, s.FastShortMinutes)
	if err != nil {
		return "unavailable", 0, err
	}
	fl, err := e.summary(ctx, tenant, m, end, s.FastLongMinutes)
	if err != nil {
		return "unavailable", 0, err
	}
	ss, err := e.summary(ctx, tenant, m, end, s.SlowShortMinutes)
	if err != nil {
		return "unavailable", 0, err
	}
	sl, err := e.summary(ctx, tenant, m, end, s.SlowLongMinutes)
	if err != nil {
		return "unavailable", 0, err
	}
	max := burn(fs, s.Target)
	for _, v := range []float64{burn(fl, s.Target), burn(ss, s.Target), burn(sl, s.Target)} {
		if v > max {
			max = v
		}
	}
	firing := (burn(fs, s.Target) >= s.FastBurnRate && burn(fl, s.Target) >= s.FastBurnRate) || (burn(ss, s.Target) >= s.SlowBurnRate && burn(sl, s.Target) >= s.SlowBurnRate)
	if firing {
		return "firing", max, nil
	}
	return "healthy", max, nil
}
func evaluationKey(tenant string, m Monitor, end int64) string {
	definition, _ := json.Marshal(struct {
		Tenant, ID, Version, Name, Signal, Service string
		Threshold                                  int
		SLO                                        *SLODefinition
		End                                        int64
	}{tenant, m.ID, m.Version, m.Name, m.Signal, m.Service, m.Threshold, m.SLO, end})
	sum := sha256.Sum256(definition)
	return hex.EncodeToString(sum[:])
}

func (e *MonitorEvaluator) Evaluate(ctx context.Context, tenant string, m Monitor, windowEnd int64) (Monitor, bool, error) {
	if !validMonitor(&m) {
		return m, false, errors.New("invalid monitor")
	}
	state, observed, err := e.calculate(ctx, tenant, m, windowEnd)
	if err != nil {
		if e.Metrics != nil {
			e.Metrics.MonitorErrors.Add(1)
		}
		return m, false, err
	}
	now := time.Now()
	if e.Now != nil {
		now = e.Now()
	}
	key := evaluationKey(tenant, m, windowEnd)
	tx, err := e.Store.db.BeginTx(ctx, nil)
	if err != nil {
		return m, false, err
	}
	defer tx.Rollback()
	r, err := tx.ExecContext(ctx, e.Store.q(`INSERT INTO platform_monitor_evaluations(tenant,monitor_id,evaluation_key,state,observed,window_end,created) VALUES(?,?,?,?,?,?,?) ON CONFLICT(tenant,monitor_id,evaluation_key) DO NOTHING`), tenant, m.ID, key, state, observed, windowEnd, now.UnixMilli())
	if err != nil {
		return m, false, err
	}
	inserted, _ := r.RowsAffected()
	if inserted == 0 {
		m.State = state
		m.Observed = observed
		m.Checked = windowEnd
		return m, false, nil
	}
	previous := "pending"
	_ = tx.QueryRowContext(ctx, e.Store.q(`SELECT state FROM platform_monitor_state WHERE tenant=? AND monitor_id=?`), tenant, m.ID).Scan(&previous)
	_, err = tx.ExecContext(ctx, e.Store.q(`INSERT INTO platform_monitor_state(tenant,monitor_id,state,observed,window_end,evaluation_key,updated) VALUES(?,?,?,?,?,?,?) ON CONFLICT(tenant,monitor_id) DO UPDATE SET state=excluded.state,observed=excluded.observed,window_end=excluded.window_end,evaluation_key=excluded.evaluation_key,updated=excluded.updated`), tenant, m.ID, state, observed, windowEnd, key, now.UnixMilli())
	if err != nil {
		return m, false, err
	}
	transitioned := previous != state
	if transitioned {
		payload, _ := json.Marshal(map[string]any{"monitor_id": m.ID, "from": previous, "to": state, "observed": observed, "window_end": windowEnd})
		_, err = tx.ExecContext(ctx, e.Store.q(`INSERT INTO platform_notification_outbox(id,tenant,monitor_id,evaluation_key,transition,payload,created) VALUES(?,?,?,?,?,?,?) ON CONFLICT(tenant,monitor_id,evaluation_key,transition) DO NOTHING`), randomID(), tenant, m.ID, key, previous+"->"+state, string(payload), now.UnixMilli())
		if err != nil {
			return m, false, err
		}
	}
	if err = tx.Commit(); err != nil {
		return m, false, err
	}
	m.State = state
	m.Observed = observed
	m.Checked = windowEnd
	if e.Metrics != nil {
		e.Metrics.MonitorEvaluations.Add(1)
		if transitioned {
			e.Metrics.MonitorTransitions.Add(1)
		}
		delay := now.UnixMilli() - windowEnd
		if delay > 0 {
			e.Metrics.MonitorDelayMillis.Store(uint64(delay))
		}
	}
	return m, transitioned, nil
}

// Run is the scheduler loop; request handlers only validate and persist definitions.
func (e *MonitorEvaluator) Run(ctx context.Context) {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			e.RunOnce(ctx)
		}
	}
}
func (e *MonitorEvaluator) RunOnce(ctx context.Context) {
	now := time.Now()
	if e.Now != nil {
		now = e.Now()
	}
	end := now.UnixMilli() / 30000 * 30000
	rows, err := e.Store.db.QueryContext(ctx, `SELECT r.tenant,r.payload FROM platform_resources r WHERE r.kind='alerts' ORDER BY r.updated LIMIT 100`)
	if err != nil {
		return
	}
	type item struct {
		tenant string
		m      Monitor
	}
	items := []item{}
	for rows.Next() {
		var it item
		var raw string
		if rows.Scan(&it.tenant, &raw) == nil && json.Unmarshal([]byte(raw), &it.m) == nil {
			items = append(items, it)
		}
	}
	rows.Close()
	for _, it := range items {
		qctx, cancel := context.WithTimeout(ctx, 10*time.Second)
		m, _, err := e.Evaluate(qctx, it.tenant, it.m, end)
		cancel()
		if err == nil {
			raw, _ := json.Marshal(m)
			_, _ = e.Store.db.ExecContext(ctx, e.Store.q(`UPDATE platform_resources SET payload=?,updated=? WHERE tenant=? AND kind='alerts' AND id=?`), string(raw), time.Now().UnixMilli(), it.tenant, m.ID)
		}
	}
}

func (a *API) Run(ctx context.Context) {
	(&MonitorEvaluator{Store: a.Store, Telemetry: a.Telemetry, Metrics: a.Metrics}).Run(ctx)
}
func (a *API) monitors(w http.ResponseWriter, r *http.Request) {
	p := principal(r)
	if r.Method == "POST" {
		var m Monitor
		if err := decode(r, &m); err != nil || !validMonitor(&m) {
			problem(w, 400, "valid versioned threshold or SLO monitor required")
			return
		}
		if m.ID == "" {
			m.ID = randomID()
		}
		if len(m.ID) > 128 {
			problem(w, 400, "invalid ID")
			return
		}
		m.State = "pending"
		m.Checked = 0
		b, _ := json.Marshal(m)
		if err := a.Store.PutResource(r.Context(), p, "alerts", m.ID, b); err != nil {
			problem(w, 503, "monitor store unavailable")
			return
		}
		respond(w, 201, m)
		return
	}
	a.resources(w, r, "alerts")
}
