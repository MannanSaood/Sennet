package platform

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"github.com/sennet/sennet/backend/cloud"
	"math/big"
	"time"
)

type CostSyncResult struct {
	Records    int    `json:"records"`
	Pages      int    `json:"pages"`
	Checkpoint string `json:"checkpoint"`
}

func (s *Store) SyncCloudCosts(ctx context.Context, tenant, sourceID string, p cloud.Provider, start, end time.Time) (CostSyncResult, error) {
	if tenant == "" || sourceID == "" || p == nil {
		return CostSyncResult{}, errors.New("workspace, source and provider required")
	}
	token := ""
	result := CostSyncResult{}
	seen := map[string]bool{}
	for {
		if seen[token] {
			return result, errors.New("provider pagination token cycle")
		}
		seen[token] = true
		page, err := p.FetchCostPage(ctx, cloud.FetchRequest{Start: start, End: end, PageToken: token})
		if err != nil {
			return result, err
		}
		tx, err := s.db.BeginTx(ctx, nil)
		if err != nil {
			return result, err
		}
		now := time.Now().UnixMilli()
		for _, r := range page.Records {
			if r.SourceID != sourceID || r.AccountID != p.AccountID() {
				tx.Rollback()
				return result, errors.New("provider returned mismatched source or account")
			}
			if _, ok := new(big.Rat).SetString(r.Amount); !ok {
				tx.Rollback()
				return result, errors.New("provider returned invalid exact amount")
			}
			if r.ChargeKind != "observed" && r.ChargeKind != "estimate" {
				tx.Rollback()
				return result, errors.New("invalid charge kind")
			}
			_, err = tx.ExecContext(ctx, s.q(`INSERT INTO platform_cloud_costs(tenant,provider,source_id,account_id,period_start,period_end,service,region,amount,currency,charge_kind,workload_identity_id,provider_request_id,ingested) VALUES(?,?,?,?,?,?,?,?,?,?,?,?,?,?) ON CONFLICT(tenant,provider,source_id,account_id,period_start,service,region,charge_kind) DO UPDATE SET period_end=excluded.period_end,amount=excluded.amount,currency=excluded.currency,workload_identity_id=excluded.workload_identity_id,provider_request_id=excluded.provider_request_id,ingested=excluded.ingested`), tenant, string(r.Provider), r.SourceID, r.AccountID, r.PeriodStart, r.PeriodEnd, r.Service, r.Region, r.Amount, r.Currency, r.ChargeKind, r.WorkloadIdentityID, page.RequestID, now)
			if err != nil {
				tx.Rollback()
				return result, err
			}
			result.Records++
		}
		_, err = tx.ExecContext(ctx, s.q(`INSERT INTO platform_cloud_checkpoints(tenant,provider,source_id,account_id,period_start,period_end,page_token,status,provider_request_id,updated) VALUES(?,?,?,?,?,?,?,?,?,?) ON CONFLICT(tenant,provider,source_id,account_id) DO UPDATE SET period_start=excluded.period_start,period_end=excluded.period_end,page_token=excluded.page_token,status=excluded.status,provider_request_id=excluded.provider_request_id,updated=excluded.updated`), tenant, string(p.Name()), sourceID, p.AccountID(), start.UTC().Format("2006-01-02"), end.UTC().Format("2006-01-02"), page.NextPageToken, "page_committed", page.RequestID, now)
		if err != nil {
			tx.Rollback()
			return result, err
		}
		if err = tx.Commit(); err != nil {
			return result, err
		}
		result.Pages++
		token = page.NextPageToken
		result.Checkpoint = token
		if token == "" {
			break
		}
	}
	return result, nil
}

type AllocationRecord struct {
	Provider, AccountID, PeriodStart, Currency, Amount, RecordKind, RuleID, RuleVersion, Target string
	SourceKeys                                                                                  []string
}

func (s *Store) SaveAllocation(ctx context.Context, tenant string, a AllocationRecord) error {
	if _, ok := new(big.Rat).SetString(a.Amount); !ok {
		return errors.New("allocation amount must be exact")
	}
	switch a.RecordKind {
	case "observed_charge", "estimate", "unallocated_total", "allocation":
	default:
		return errors.New("invalid allocation record kind")
	}
	raw, _ := json.Marshal(a.SourceKeys)
	h := sha256.Sum256([]byte(tenant + "|" + a.Provider + "|" + a.AccountID + "|" + a.PeriodStart + "|" + a.RecordKind + "|" + a.RuleID + "|" + a.RuleVersion + "|" + a.Target + "|" + string(raw)))
	_, err := s.db.ExecContext(ctx, s.q(`INSERT INTO platform_cost_allocations(tenant,allocation_key,provider,account_id,period_start,currency,amount,record_kind,rule_id,rule_version,target,source_keys,created) VALUES(?,?,?,?,?,?,?,?,?,?,?,?,?) ON CONFLICT(tenant,allocation_key) DO NOTHING`), tenant, hex.EncodeToString(h[:]), a.Provider, a.AccountID, a.PeriodStart, a.Currency, a.Amount, a.RecordKind, a.RuleID, a.RuleVersion, a.Target, string(raw), time.Now().UnixMilli())
	return err
}
