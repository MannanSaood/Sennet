package platform

import (
	"math/big"
	"net/http"
	"sort"
	"strconv"
	"time"
)

type Transaction struct {
	ID       string   `json:"id"`
	Currency string   `json:"currency"`
	Amount   string   `json:"amount"`
	State    string   `json:"state"`
	Events   int      `json:"events"`
	Issues   []string `json:"issues"`
	Last     int64    `json:"last"`
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
	page, err := a.Telemetry.Query(r.Context(), principal(r).Tenant, q)
	if err != nil {
		problem(w, 503, "financial events unavailable")
		return
	}
	groups := map[string][]Event{}
	for _, e := range page.Events {
		key := e.Attributes["source"] + ":" + e.Attributes["transaction_id"]
		groups[key] = append(groups[key], e)
	}
	out := []Transaction{}
	totals := map[string]*big.Rat{}
	for key, events := range groups {
		sort.SliceStable(events, func(i, j int) bool { return events[i].Time < events[j].Time })
		last := events[len(events)-1]
		tx := Transaction{ID: key, Currency: last.Attributes["currency"], Amount: last.Attributes["amount"], State: last.Attributes["state"], Events: len(events), Issues: []string{}, Last: last.Time}
		sequences := map[uint64]bool{}
		numbers := []uint64{}
		for _, e := range events {
			if e.Attributes["currency"] != tx.Currency {
				tx.Issues = append(tx.Issues, "currency changed")
			}
			seq := e.Attributes["sequence"]
			if seq != "" {
				n, err := strconv.ParseUint(seq, 10, 64)
				if err != nil {
					tx.Issues = append(tx.Issues, "invalid sequence")
					continue
				}
				if sequences[n] {
					tx.Issues = append(tx.Issues, "repeated sequence; inspect correction metadata")
				}
				sequences[n] = true
				numbers = append(numbers, n)
			}
		}
		sort.Slice(numbers, func(i, j int) bool { return numbers[i] < numbers[j] })
		for i := 1; i < len(numbers); i++ {
			if numbers[i] > numbers[i-1]+1 {
				tx.Issues = append(tx.Issues, "sequence gap")
			}
		}
		if tx.State == "settled" || tx.State == "completed" {
			amount, ok := new(big.Rat).SetString(tx.Amount)
			if ok {
				if totals[tx.Currency] == nil {
					totals[tx.Currency] = new(big.Rat)
				}
				totals[tx.Currency].Add(totals[tx.Currency], amount)
			}
		}
		out = append(out, tx)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Last > out[j].Last })
	amounts := map[string]string{}
	for currency, amount := range totals {
		amounts[currency] = amount.FloatString(12)
	}
	respond(w, 200, map[string]any{"transactions": out, "settled_totals": amounts, "partial": page.Cursor != "", "timestamp": time.Now().UnixMilli(), "scope": "latest 1000 events in requested window; not a ledger"})
}
