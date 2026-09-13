package platform

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"math/big"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"
)

const FinanceSchemaV1 = "sennet.finance.payment.v1"
const FinanceRuleV1 = "payment-auth-capture-set-1"

type FinanceConclusion struct {
	Key      string   `json:"key"`
	Kind     string   `json:"kind"`
	Severity string   `json:"severity"`
	EventIDs []string `json:"event_ids"`
	Detail   string   `json:"detail"`
}
type FinanceTimeline struct {
	SourceID      string              `json:"source_id"`
	AccountID     string              `json:"account_id"`
	TransactionID string              `json:"transaction_id"`
	Currency      string              `json:"currency"`
	Amount        string              `json:"amount"`
	State         string              `json:"state"`
	LastSequence  uint64              `json:"last_sequence"`
	Events        []Event             `json:"events"`
	Conclusions   []FinanceConclusion `json:"conclusions"`
}
type FinanceWatermark struct {
	SourceID     string `json:"source_id"`
	AccountID    string `json:"account_id"`
	LastSequence uint64 `json:"last_sequence"`
	EventTime    int64  `json:"event_time_ms"`
	ReceiveTime  int64  `json:"receive_time_ms"`
	GapCount     int    `json:"gap_count"`
}
type financeEvent struct {
	Event                                                 Event
	Source, Account, Transaction, State, Currency, Amount string
	Sequence, Correction                                  uint64
	Receive                                               int64
}

func parseFinanceEvent(e Event) (financeEvent, error) {
	a := e.Attributes
	for _, k := range []string{"source_id", "transaction_id", "sequence", "receive_time_ms", "correction_version", "provider", "account", "state", "currency"} {
		if strings.TrimSpace(a[k]) == "" {
			return financeEvent{}, errors.New("finance " + k + " required")
		}
	}
	if a["finance.schema"] != FinanceSchemaV1 {
		return financeEvent{}, errors.New("finance.schema must be " + FinanceSchemaV1)
	}
	seq, err := strconv.ParseUint(a["sequence"], 10, 64)
	if err != nil || seq == 0 {
		return financeEvent{}, errors.New("finance sequence must be positive")
	}
	cor, err := strconv.ParseUint(a["correction_version"], 10, 64)
	if err != nil {
		return financeEvent{}, errors.New("finance correction_version invalid")
	}
	recv, err := strconv.ParseInt(a["receive_time_ms"], 10, 64)
	if err != nil || recv <= 0 {
		return financeEvent{}, errors.New("finance receive_time_ms invalid")
	}
	amount := a["amount"]
	if minor := a["amount_minor"]; minor != "" {
		if _, ok := new(big.Int).SetString(minor, 10); !ok {
			return financeEvent{}, errors.New("amount_minor must be an integer")
		}
		amount = minor
	}
	if amount == "" || !money.MatchString(amount) {
		return financeEvent{}, errors.New("finance exact amount required")
	}
	return financeEvent{e, a["source_id"], a["account"], a["transaction_id"], a["state"], a["currency"], amount, seq, cor, recv}, nil
}
func reconcileFinance(events []Event) ([]FinanceTimeline, []FinanceWatermark, error) {
	groups := map[string][]financeEvent{}
	water := map[string]*FinanceWatermark{}
	for _, e := range events {
		f, err := parseFinanceEvent(e)
		if err != nil {
			return nil, nil, err
		}
		groups[f.Source+"\x00"+f.Account+"\x00"+f.Transaction] = append(groups[f.Source+"\x00"+f.Account+"\x00"+f.Transaction], f)
		wk := f.Source + "\x00" + f.Account
		if water[wk] == nil {
			water[wk] = &FinanceWatermark{SourceID: f.Source, AccountID: f.Account}
		}
		w := water[wk]
		if f.Sequence > w.LastSequence {
			w.LastSequence = f.Sequence
		}
		if e.Time > w.EventTime {
			w.EventTime = e.Time
		}
		if f.Receive > w.ReceiveTime {
			w.ReceiveTime = f.Receive
		}
	}
	out := make([]FinanceTimeline, 0, len(groups))
	for _, all := range groups {
		sort.SliceStable(all, func(i, j int) bool {
			if all[i].Sequence == all[j].Sequence {
				return all[i].Correction < all[j].Correction
			}
			return all[i].Sequence < all[j].Sequence
		})
		latest := map[uint64]financeEvent{}
		dups := map[uint64][]string{}
		received := append([]financeEvent(nil), all...)
		sort.SliceStable(received, func(i, j int) bool { return received[i].Receive < received[j].Receive })
		for _, f := range all {
			if old, ok := latest[f.Sequence]; ok {
				if old.Correction == f.Correction {
					dups[f.Sequence] = append(dups[f.Sequence], old.Event.ID, f.Event.ID)
				}
				if f.Correction >= old.Correction {
					latest[f.Sequence] = f
				}
			} else {
				latest[f.Sequence] = f
			}
		}
		seqs := make([]uint64, 0, len(latest))
		for n := range latest {
			seqs = append(seqs, n)
		}
		sort.Slice(seqs, func(i, j int) bool { return seqs[i] < seqs[j] })
		first := latest[seqs[0]]
		tl := FinanceTimeline{SourceID: first.Source, AccountID: first.Account, TransactionID: first.Transaction, Currency: first.Currency, Amount: first.Amount, State: "unknown", LastSequence: seqs[len(seqs)-1], Events: []Event{}, Conclusions: []FinanceConclusion{}}
		add := func(kind, severity, detail string, ids ...string) {
			sort.Strings(ids)
			h := sha256.Sum256([]byte(first.Source + "|" + first.Account + "|" + first.Transaction + "|" + FinanceRuleV1 + "|" + kind + "|" + strings.Join(ids, ",")))
			tl.Conclusions = append(tl.Conclusions, FinanceConclusion{hex.EncodeToString(h[:]), kind, severity, ids, detail})
		}
		states := []string{}
		for i, n := range seqs {
			f := latest[n]
			tl.Events = append(tl.Events, f.Event)
			states = append(states, f.State)
			if f.Currency != tl.Currency || f.Amount != tl.Amount {
				add("value_changed", "error", "currency or amount changed", f.Event.ID)
			}
			if f.Correction > 0 {
				add("correction", "info", "higher correction version selected", f.Event.ID)
			}
			if f.Receive-f.Event.Time > 15*60*1000 {
				add("stale_event", "warning", "receive time exceeds event time by 15 minutes", f.Event.ID)
			}
			if i > 0 && n > seqs[i-1]+1 {
				add("sequence_gap", "error", "source sequence is incomplete", latest[seqs[i-1]].Event.ID, f.Event.ID)
				water[f.Source+"\x00"+f.Account].GapCount++
			}
		}
		for _, ids := range dups {
			add("duplicate", "warning", "same sequence and correction observed repeatedly", ids...)
		}
		for i := 1; i < len(received); i++ {
			if received[i].Sequence < received[i-1].Sequence {
				add("out_of_order", "warning", "receive order differs from source sequence", received[i-1].Event.ID, received[i].Event.ID)
			}
		}
		valid := map[string]string{"authorized": "captured", "captured": "settled"}
		tl.State = states[len(states)-1]
		if states[0] == "settled" {
			add("unmatched_settlement", "error", "settlement has no observed authorization and capture", tl.Events[0].ID)
		}
		for i := 1; i < len(states); i++ {
			if valid[states[i-1]] != states[i] {
				add("invalid_transition", "error", "expected authorization to capture to settlement", tl.Events[i-1].ID, tl.Events[i].ID)
			}
		}
		out = append(out, tl)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].TransactionID < out[j].TransactionID })
	ws := make([]FinanceWatermark, 0, len(water))
	for _, w := range water {
		ws = append(ws, *w)
	}
	sort.Slice(ws, func(i, j int) bool {
		if ws[i].SourceID == ws[j].SourceID {
			return ws[i].AccountID < ws[j].AccountID
		}
		return ws[i].SourceID < ws[j].SourceID
	})
	return out, ws, nil
}
func (s *Store) persistFinance(ctx context.Context, tenant string, tls []FinanceTimeline, wms []FinanceWatermark) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	now := time.Now().UnixMilli()
	for _, tl := range tls {
		ew, rw := int64(0), int64(0)
		for _, e := range tl.Events {
			if e.Time > ew {
				ew = e.Time
			}
			n, _ := strconv.ParseInt(e.Attributes["receive_time_ms"], 10, 64)
			if n > rw {
				rw = n
			}
		}
		_, err = tx.ExecContext(ctx, s.q(`INSERT INTO platform_finance_reconciliation(tenant,source_id,account_id,transaction_id,rule_version,state,currency,amount,last_sequence,event_watermark,receive_watermark,updated) VALUES(?,?,?,?,?,?,?,?,?,?,?,?) ON CONFLICT(tenant,source_id,account_id,transaction_id,rule_version) DO UPDATE SET state=excluded.state,currency=excluded.currency,amount=excluded.amount,last_sequence=excluded.last_sequence,event_watermark=excluded.event_watermark,receive_watermark=excluded.receive_watermark,updated=excluded.updated`), tenant, tl.SourceID, tl.AccountID, tl.TransactionID, FinanceRuleV1, tl.State, tl.Currency, tl.Amount, tl.LastSequence, ew, rw, now)
		if err != nil {
			return err
		}
		for _, c := range tl.Conclusions {
			ids, _ := json.Marshal(c.EventIDs)
			_, err = tx.ExecContext(ctx, s.q(`INSERT INTO platform_finance_conclusions(tenant,conclusion_key,source_id,account_id,transaction_id,rule_version,kind,severity,event_ids,detail,created) VALUES(?,?,?,?,?,?,?,?,?,?,?) ON CONFLICT(tenant,conclusion_key) DO NOTHING`), tenant, c.Key, tl.SourceID, tl.AccountID, tl.TransactionID, FinanceRuleV1, c.Kind, c.Severity, string(ids), c.Detail, now)
			if err != nil {
				return err
			}
		}
	}
	for _, v := range wms {
		_, err = tx.ExecContext(ctx, s.q(`INSERT INTO platform_finance_sources(tenant,source_id,account_id,last_sequence,event_watermark,receive_watermark,gap_count,updated) VALUES(?,?,?,?,?,?,?,?) ON CONFLICT(tenant,source_id,account_id) DO UPDATE SET last_sequence=excluded.last_sequence,event_watermark=excluded.event_watermark,receive_watermark=excluded.receive_watermark,gap_count=excluded.gap_count,updated=excluded.updated`), tenant, v.SourceID, v.AccountID, v.LastSequence, v.EventTime, v.ReceiveTime, v.GapCount, now)
		if err != nil {
			return err
		}
	}
	return tx.Commit()
}
func (a *API) finance(w http.ResponseWriter, r *http.Request) {
	if r.Method != "GET" {
		problem(w, 405, "GET required")
		return
	}
	q, err := parseQuery(r)
	if err != nil {
		problem(w, 400, err.Error())
		return
	}
	q.Signal = "finance"
	q.Limit = 1000
	events, partial, err := boundedEvents(r.Context(), a.Telemetry, principal(r).Tenant, q, 1000)
	if err != nil {
		problem(w, 503, "financial events unavailable")
		return
	}
	tls, wms, err := reconcileFinance(events)
	if err != nil {
		problem(w, 422, err.Error())
		return
	}
	if err = a.Store.persistFinance(r.Context(), principal(r).Tenant, tls, wms); err != nil {
		problem(w, 503, "reconciliation state unavailable")
		return
	}
	view := strings.TrimPrefix(r.URL.Path, "/api/finance/")
	response := map[string]any{"schema_version": FinanceSchemaV1, "rule_version": FinanceRuleV1, "partial": partial, "bounded": true, "scope": "operational observability; not a ledger, accounting conclusion, or regulatory compliance assessment"}
	switch view {
	case "timelines", "reconciliation":
		response["timelines"] = tls
		response["watermarks"] = wms
	case "queue":
		items := []FinanceTimeline{}
		for _, tl := range tls {
			if len(tl.Conclusions) > 0 {
				items = append(items, tl)
			}
		}
		response["items"] = items
	case "latency":
		response["distributions"] = financeLatency(tls)
	case "provider-errors":
		counts := map[string]int{}
		for _, tl := range tls {
			for _, e := range tl.Events {
				if e.Attributes["provider_error"] != "" {
					counts[e.Attributes["provider"]]++
				}
			}
		}
		response["providers"] = counts
	case "dependency-impact":
		impact := map[string]int{}
		for _, tl := range tls {
			if len(tl.Conclusions) > 0 {
				for _, e := range tl.Events {
					if d := e.Attributes["dependency"]; d != "" {
						impact[d]++
					}
				}
			}
		}
		response["dependencies"] = impact
	default:
		problem(w, 404, "finance view unavailable")
		return
	}
	respond(w, 200, response)
}
func financeLatency(tls []FinanceTimeline) map[string]map[string]int64 {
	values := map[string][]int64{}
	for _, tl := range tls {
		for i := 1; i < len(tl.Events); i++ {
			k := tl.Events[i-1].Attributes["state"] + "_to_" + tl.Events[i].Attributes["state"]
			values[k] = append(values[k], tl.Events[i].Time-tl.Events[i-1].Time)
		}
	}
	out := map[string]map[string]int64{}
	for k, v := range values {
		sort.Slice(v, func(i, j int) bool { return v[i] < v[j] })
		out[k] = map[string]int64{"count": int64(len(v)), "p50_ms": v[(len(v)-1)/2], "p95_ms": v[(len(v)-1)*95/100]}
	}
	return out
}
