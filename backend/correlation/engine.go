package correlation

import (
	"context"
	"time"

	"github.com/sennet/sennet/backend/cloud"
	"github.com/sennet/sennet/backend/db"
)

type Engine struct {
	database *db.DB
	registry *cloud.Registry
}

func NewEngine(database *db.DB, registry *cloud.Registry) *Engine {
	return &Engine{
		database: database,
		registry: registry,
	}
}

type CostSummary struct {
	TotalCostUSD float64            `json:"total_cost_usd"`
	ByProvider   map[string]float64 `json:"by_provider"`
	ByService    map[string]float64 `json:"by_service"`
	ByRegion     map[string]float64 `json:"by_region"`
	Period       string             `json:"period"`
}

func (e *Engine) SyncCosts(ctx context.Context, days int) error {
	endDate := time.Now()
	startDate := endDate.AddDate(0, 0, -days)

	for _, id := range e.registry.List() {
		provider, ok := e.registry.Get(id)
		if !ok {
			continue
		}

		costs, err := provider.FetchCosts(ctx, startDate, endDate)
		if err != nil {
			continue
		}

		for _, cost := range costs {
			e.database.SaveEgressCost(
				string(provider.Name()),
				cost.Date.Format("2006-01-02"),
				cost.Service,
				cost.Region,
				cost.CostUSD,
				cost.BytesOut,
			)
		}
	}

	return nil
}

func (e *Engine) GetCostSummary(startDate, endDate string) (*CostSummary, error) {
	costs, err := e.database.GetEgressCosts(startDate, endDate)
	if err != nil {
		return nil, err
	}

	summary := &CostSummary{
		ByProvider: make(map[string]float64),
		ByService:  make(map[string]float64),
		ByRegion:   make(map[string]float64),
		Period:     startDate + " to " + endDate,
	}

	for _, cost := range costs {
		summary.TotalCostUSD += cost.CostUSD
		summary.ByProvider[cost.Provider] += cost.CostUSD
		if cost.Service != "" {
			summary.ByService[cost.Service] += cost.CostUSD
		}
		if cost.Region != "" {
			summary.ByRegion[cost.Region] += cost.CostUSD
		}
	}

	return summary, nil
}

func (e *Engine) AttributeCosts(date string) error {
	return nil
}
