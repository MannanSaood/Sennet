package cloud

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

type fixedCredentials struct{}

func (fixedCredentials) Retrieve(context.Context) (Credentials, error) {
	return Credentials{"AKID", "secret", "token"}, nil
}
func TestConfigContainsNoCredentialsAndDisablesUnsupported(t *testing.T) {
	c := CloudConfig{ID: "cost", Provider: ProviderAWS, AWS: &AWSConfig{Region: "us-east-1", AccountID: "123456789012", WorkloadIdentityID: "workload-1"}}
	raw, err := c.ToJSON()
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(strings.ToLower(raw), "secret") || strings.Contains(raw, "AKID") {
		t.Fatal("credentials serialized")
	}
	if (&CloudConfig{ID: "x", Provider: ProviderAzure}).Validate() == nil {
		t.Fatal("unsupported provider enabled")
	}
}
func TestAWSCostExplorerPaginationAndAccountScope(t *testing.T) {
	calls := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		if r.Header.Get("Authorization") == "" || r.Header.Get("X-Amz-Security-Token") != "token" {
			t.Error("request not signed")
		}
		var body map[string]any
		if json.NewDecoder(r.Body).Decode(&body) != nil {
			t.Error("bad body")
		}
		raw, _ := json.Marshal(body)
		if !strings.Contains(string(raw), "123456789012") {
			t.Error("missing account scope")
		}
		w.Header().Set("x-amzn-RequestId", "req")
		if calls == 1 {
			_, _ = w.Write([]byte(`{"ResultsByTime":[{"TimePeriod":{"Start":"2026-09-01","End":"2026-09-02"},"Estimated":false,"Groups":[{"Keys":["AmazonEC2"],"Metrics":{"UnblendedCost":{"Amount":"9007199254740993.0001","Unit":"USD"}}}]}],"NextPageToken":"next"}`))
		} else {
			if body["NextPageToken"] != "next" {
				t.Error("token missing")
			}
			_, _ = w.Write([]byte(`{"ResultsByTime":[]}`))
		}
	}))
	defer srv.Close()
	p, err := NewAWSProvider("cost", &AWSConfig{Region: "us-east-1", AccountID: "123456789012", WorkloadIdentityID: "workload-1"}, fixedCredentials{}, srv.Client())
	if err != nil {
		t.Fatal(err)
	}
	p.endpoint = srv.URL
	p.now = func() time.Time { return time.Date(2026, 9, 2, 0, 0, 0, 0, time.UTC) }
	page, err := p.FetchCostPage(context.Background(), FetchRequest{Start: p.now().AddDate(0, 0, -1), End: p.now()})
	if err != nil {
		t.Fatal(err)
	}
	if page.NextPageToken != "next" || page.Records[0].Amount != "9007199254740993.0001" || page.Records[0].ChargeKind != "observed" {
		t.Fatalf("bad page: %+v", page)
	}
	_, err = p.FetchCostPage(context.Background(), FetchRequest{Start: p.now().AddDate(0, 0, -1), End: p.now(), PageToken: page.NextPageToken})
	if err != nil {
		t.Fatal(err)
	}
	if calls != 2 {
		t.Fatal(calls)
	}
}
