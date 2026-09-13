package correlation

import (
	"context"
	"errors"
	"github.com/sennet/sennet/backend/cloud"
	"github.com/sennet/sennet/backend/db"
)

// Engine remains only so old, unmounted HTTP handlers compile. Financial cost
// ingestion moved to platform.Store where workspace/account keys and exact text
// amounts are enforced. This legacy path must never claim success.
type Engine struct {
	database *db.DB
	registry *cloud.Registry
}

func NewEngine(d *db.DB, r *cloud.Registry) *Engine { return &Engine{d, r} }

type CostSummary struct {
	TotalCostUSD string            `json:"total_cost_usd"`
	ByProvider   map[string]string `json:"by_provider"`
	ByService    map[string]string `json:"by_service"`
	ByRegion     map[string]string `json:"by_region"`
	Period       string            `json:"period"`
}

func (e *Engine) SyncCosts(context.Context, int) error {
	return errors.New("legacy unscoped float cost synchronization is disabled; use workspace-scoped platform cost ingestion")
}
func (e *Engine) GetCostSummary(string, string) (*CostSummary, error) {
	return nil, errors.New("legacy float cost summary is disabled")
}
func (e *Engine) AttributeCosts(string) error {
	return errors.New("legacy cost attribution is disabled")
}
