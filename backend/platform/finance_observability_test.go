package platform

import (
	"context"
	"github.com/sennet/sennet/backend/cloud"
	"path/filepath"
	"strconv"
	"testing"
	"testing/quick"
	"time"
)

func financeFixture(id, account, state string, sequence, correction uint64, eventTime, receiveTime int64) Event {
	return Event{ID: id, Time: eventTime, Signal: "finance", Service: "payments", Name: "payment." + state, Attributes: map[string]string{"finance.schema": FinanceSchemaV1, "source_id": "gateway", "transaction_id": "tx", "sequence": strconv.FormatUint(sequence, 10), "receive_time_ms": strconv.FormatInt(receiveTime, 10), "correction_version": strconv.FormatUint(correction, 10), "provider": "processor", "account": account, "state": state, "currency": "USD", "amount_minor": "90071992547409930001"}}
}

func TestExactMinorUnitsProperty(t *testing.T) {
	err := quick.Check(func(n uint64) bool {
		if n == 0 {
			n = 1
		}
		e := financeFixture("e", "a", "authorized", 1, 0, 1, 2)
		e.Attributes["amount_minor"] = strconv.FormatUint(n, 10)
		f, err := parseFinanceEvent(e)
		return err == nil && f.Amount == strconv.FormatUint(n, 10)
	}, nil)
	if err != nil {
		t.Fatal(err)
	}
}

func TestLifecycleAnomaliesAndReplayAreDeterministic(t *testing.T) {
	now := time.Now().UnixMilli()
	events := []Event{financeFixture("settle", "a", "settled", 3, 0, now-20*60*1000, now), financeFixture("auth", "a", "authorized", 1, 0, now-1000, now+1000), financeFixture("dup", "a", "authorized", 1, 0, now-1000, now+2000)}
	tls, wms, err := reconcileFinance(events)
	if err != nil {
		t.Fatal(err)
	}
	kinds := map[string]bool{}
	for _, c := range tls[0].Conclusions {
		kinds[c.Kind] = true
	}
	for _, k := range []string{"duplicate", "sequence_gap", "out_of_order", "invalid_transition", "stale_event"} {
		if !kinds[k] {
			t.Fatalf("missing %s: %+v", k, tls[0].Conclusions)
		}
	}
	s, err := Open(filepath.Join(t.TempDir(), "finance.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	for i := 0; i < 2; i++ {
		if err = s.persistFinance(context.Background(), "workspace", tls, wms); err != nil {
			t.Fatal(err)
		}
	}
	var conclusions int
	if err = s.db.QueryRow(`SELECT COUNT(*) FROM platform_finance_conclusions WHERE tenant='workspace'`).Scan(&conclusions); err != nil {
		t.Fatal(err)
	}
	if conclusions != len(tls[0].Conclusions) {
		t.Fatalf("replay duplicated conclusions: %d", conclusions)
	}
}

type fixtureCostProvider struct {
	account string
	calls   int
}

func (p *fixtureCostProvider) Name() cloud.ProviderType             { return cloud.ProviderAWS }
func (p *fixtureCostProvider) AccountID() string                    { return p.account }
func (p *fixtureCostProvider) TestConnection(context.Context) error { return nil }
func (p *fixtureCostProvider) FetchCostPage(_ context.Context, r cloud.FetchRequest) (cloud.CostPage, error) {
	p.calls++
	token := ""
	if r.PageToken == "" {
		token = "page-2"
	}
	return cloud.CostPage{NextPageToken: token, RequestID: "request", Records: []cloud.CostResult{{SourceID: "source", Provider: cloud.ProviderAWS, AccountID: p.account, WorkloadIdentityID: "workload", PeriodStart: "2026-09-01", PeriodEnd: "2026-09-02", Service: "AmazonEC2", Amount: "9007199254740993.0001", Currency: "USD", ChargeKind: "observed"}}}, nil
}
func TestCloudCostsAreReplaySafeAndAccountScoped(t *testing.T) {
	s, err := Open(filepath.Join(t.TempDir(), "cost.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	start := time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC)
	for _, account := range []string{"111111111111", "222222222222"} {
		p := &fixtureCostProvider{account: account}
		for i := 0; i < 2; i++ {
			if _, err = s.SyncCloudCosts(context.Background(), "workspace", "source", p, start, start.AddDate(0, 0, 1)); err != nil {
				t.Fatal(err)
			}
		}
	}
	var count int
	if err = s.db.QueryRow(`SELECT COUNT(*) FROM platform_cloud_costs WHERE tenant='workspace'`).Scan(&count); err != nil {
		t.Fatal(err)
	}
	if count != 2 {
		t.Fatalf("accounts overwrote or replay duplicated rows: %d", count)
	}
	var amount string
	if err = s.db.QueryRow(`SELECT amount FROM platform_cloud_costs LIMIT 1`).Scan(&amount); err != nil || amount != "9007199254740993.0001" {
		t.Fatalf("exact amount changed: %q %v", amount, err)
	}
}
