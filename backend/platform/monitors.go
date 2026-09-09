package platform

import (
	"context"
	"encoding/json"
	"net/http"
	"time"
)

type Monitor struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	Signal    string `json:"signal"`
	Service   string `json:"service"`
	Threshold int    `json:"threshold"`
	State     string `json:"state"`
	Observed  int    `json:"observed"`
	Checked   int64  `json:"checked_at"`
	Partial   bool   `json:"partial"`
}

func (a *API) evaluate(ctx context.Context, p Principal, m Monitor) Monitor {
	now := time.Now().UnixMilli()
	m.Checked = now
	agg, ok := a.Telemetry.(Aggregator)
	if !ok {
		m.State = "unavailable"
		return m
	}
	summary, err := agg.Summary(ctx, p.Tenant, Query{From: now - 300000, To: now, Signal: m.Signal, Service: m.Service, Limit: 1000})
	if err != nil {
		m.State = "unavailable"
		return m
	}
	m.Observed = int(summary.Errors)
	m.Partial = false
	m.State = "healthy"
	if m.Observed >= m.Threshold {
		m.State = "firing"
	}
	return m
}

// Bounded periodic evaluation. No outbound notifications are sent by this service.
func (a *API) Run(ctx context.Context) {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			rows, err := a.Store.db.QueryContext(ctx, `SELECT r.tenant,w.organization_id,r.payload FROM platform_resources r JOIN platform_workspaces w ON w.id=r.tenant WHERE r.kind='alerts' ORDER BY r.updated ASC LIMIT 100`)
			if err != nil {
				continue
			}
			type item struct {
				p Principal
				m Monitor
			}
			items := []item{}
			for rows.Next() {
				var tenant, organization, raw string
				if rows.Scan(&tenant, &organization, &raw) != nil {
					continue
				}
				var m Monitor
				if json.Unmarshal([]byte(raw), &m) == nil {
					items = append(items, item{Principal{OrganizationID: organization, WorkspaceID: tenant, Tenant: tenant, Role: "admin", Subject: "monitor", SubjectType: "system"}, m})
				}
			}
			rows.Close()
			for _, item := range items {
				qctx, cancel := context.WithTimeout(ctx, 10*time.Second)
				m := a.evaluate(qctx, item.p, item.m)
				cancel()
				raw, _ := json.Marshal(m)
				_ = a.Store.PutResource(ctx, item.p, "alerts", m.ID, raw)
			}
			if !a.Store.postgres {
				_, _ = a.Store.db.ExecContext(ctx, `DELETE FROM platform_events WHERE time_ms < ?`, time.Now().Add(-30*24*time.Hour).UnixMilli())
			}
		}
	}
}
func (a *API) monitors(w http.ResponseWriter, r *http.Request) {
	p := principal(r)
	if r.Method == "POST" {
		var m Monitor
		if err := decode(r, &m); err != nil || m.Name == "" || len(m.Name) > 200 || m.Threshold < 1 || m.Threshold > 1000 || (m.Signal != "" && !signals[m.Signal]) || len(m.Service) > 200 {
			problem(w, 400, "valid monitor name, signal and threshold (1–1000) required")
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
