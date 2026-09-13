package cloud

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"sort"
	"strings"
	"sync"
	"time"
)

type CostResult struct {
	SourceID           string       `json:"source_id"`
	Provider           ProviderType `json:"provider"`
	AccountID          string       `json:"account_id"`
	WorkloadIdentityID string       `json:"workload_identity_id"`
	PeriodStart        string       `json:"period_start"`
	PeriodEnd          string       `json:"period_end"`
	Service            string       `json:"service"`
	Region             string       `json:"region,omitempty"`
	Amount             string       `json:"amount"`
	Currency           string       `json:"currency"`
	ChargeKind         string       `json:"charge_kind"`
}
type FetchRequest struct {
	Start, End time.Time
	PageToken  string
}
type CostPage struct {
	Records       []CostResult
	NextPageToken string
	RequestID     string
}
type Provider interface {
	Name() ProviderType
	AccountID() string
	FetchCostPage(context.Context, FetchRequest) (CostPage, error)
	TestConnection(context.Context) error
}
type Registry struct {
	mu        sync.RWMutex
	providers map[string]Provider
}

func NewRegistry() *Registry { return &Registry{providers: map[string]Provider{}} }
func (r *Registry) Register(id string, p Provider) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.providers[id] = p
}
func (r *Registry) Get(id string) (Provider, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	p, ok := r.providers[id]
	return p, ok
}
func (r *Registry) List() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make([]string, 0, len(r.providers))
	for k := range r.providers {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}
func (r *Registry) Remove(id string) { r.mu.Lock(); defer r.mu.Unlock(); delete(r.providers, id) }
func CreateProvider(c *CloudConfig) (Provider, error) {
	if err := c.Validate(); err != nil {
		return nil, err
	}
	return NewAWSProvider(c.ID, c.AWS, EnvCredentials{}, http.DefaultClient)
}

type Credentials struct{ AccessKeyID, SecretAccessKey, SessionToken string }
type CredentialProvider interface {
	Retrieve(context.Context) (Credentials, error)
}
type EnvCredentials struct{}

func (EnvCredentials) Retrieve(context.Context) (Credentials, error) {
	c := Credentials{os.Getenv("AWS_ACCESS_KEY_ID"), os.Getenv("AWS_SECRET_ACCESS_KEY"), os.Getenv("AWS_SESSION_TOKEN")}
	if c.AccessKeyID == "" || c.SecretAccessKey == "" {
		return Credentials{}, errors.New("AWS workload credentials unavailable")
	}
	return c, nil
}

type AWSProvider struct {
	id          string
	config      AWSConfig
	credentials CredentialProvider
	client      *http.Client
	endpoint    string
	now         func() time.Time
}

func NewAWSProvider(id string, c *AWSConfig, cp CredentialProvider, client *http.Client) (*AWSProvider, error) {
	if err := c.Validate(); err != nil {
		return nil, err
	}
	if cp == nil {
		return nil, errors.New("credential provider required")
	}
	if client == nil {
		client = http.DefaultClient
	}
	return &AWSProvider{id: id, config: *c, credentials: cp, client: client, endpoint: "https://ce.us-east-1.amazonaws.com", now: time.Now}, nil
}
func (p *AWSProvider) Name() ProviderType { return ProviderAWS }
func (p *AWSProvider) AccountID() string  { return p.config.AccountID }

type ceResponse struct {
	ResultsByTime []struct {
		TimePeriod struct {
			Start string `json:"Start"`
			End   string `json:"End"`
		} `json:"TimePeriod"`
		Estimated bool `json:"Estimated"`
		Groups    []struct {
			Keys    []string `json:"Keys"`
			Metrics map[string]struct {
				Amount string `json:"Amount"`
				Unit   string `json:"Unit"`
			} `json:"Metrics"`
		} `json:"Groups"`
	} `json:"ResultsByTime"`
	NextPageToken string `json:"NextPageToken"`
}

func (p *AWSProvider) FetchCostPage(ctx context.Context, f FetchRequest) (CostPage, error) {
	if !f.Start.Before(f.End) {
		return CostPage{}, errors.New("start must precede end")
	}
	body := map[string]any{"TimePeriod": map[string]string{"Start": f.Start.UTC().Format("2006-01-02"), "End": f.End.UTC().Format("2006-01-02")}, "Granularity": "DAILY", "Metrics": []string{"UnblendedCost"}, "GroupBy": []map[string]string{{"Type": "DIMENSION", "Key": "SERVICE"}}, "Filter": map[string]any{"Dimensions": map[string]any{"Key": "LINKED_ACCOUNT", "Values": []string{p.config.AccountID}}}}
	if f.PageToken != "" {
		body["NextPageToken"] = f.PageToken
	}
	b, _ := json.Marshal(body)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, p.endpoint, bytes.NewReader(b))
	if err != nil {
		return CostPage{}, err
	}
	req.Header.Set("Content-Type", "application/x-amz-json-1.1")
	req.Header.Set("X-Amz-Target", "AWSInsightsIndexService.GetCostAndUsage")
	cred, err := p.credentials.Retrieve(ctx)
	if err != nil {
		return CostPage{}, err
	}
	signAWS(req, b, cred, "ce", "us-east-1", p.now().UTC())
	resp, err := p.client.Do(req)
	if err != nil {
		return CostPage{}, err
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(io.LimitReader(resp.Body, 4<<20))
	if err != nil {
		return CostPage{}, err
	}
	if resp.StatusCode/100 != 2 {
		return CostPage{}, fmt.Errorf("AWS Cost Explorer status %d: %s", resp.StatusCode, strings.TrimSpace(string(raw)))
	}
	var decoded ceResponse
	if err = json.Unmarshal(raw, &decoded); err != nil {
		return CostPage{}, err
	}
	out := CostPage{NextPageToken: decoded.NextPageToken, RequestID: resp.Header.Get("x-amzn-RequestId")}
	for _, day := range decoded.ResultsByTime {
		kind := "observed"
		if day.Estimated {
			kind = "estimate"
		}
		for _, g := range day.Groups {
			m, ok := g.Metrics["UnblendedCost"]
			if !ok || len(g.Keys) < 1 {
				continue
			}
			out.Records = append(out.Records, CostResult{p.id, ProviderAWS, p.config.AccountID, p.config.WorkloadIdentityID, day.TimePeriod.Start, day.TimePeriod.End, g.Keys[0], "", m.Amount, m.Unit, kind})
		}
	}
	return out, nil
}
func (p *AWSProvider) TestConnection(ctx context.Context) error {
	end := p.now().UTC()
	_, err := p.FetchCostPage(ctx, FetchRequest{Start: end.AddDate(0, 0, -1), End: end})
	return err
}
func signAWS(req *http.Request, payload []byte, c Credentials, service, region string, now time.Time) {
	date := now.Format("20060102")
	stamp := now.Format("20060102T150405Z")
	req.Header.Set("X-Amz-Date", stamp)
	if c.SessionToken != "" {
		req.Header.Set("X-Amz-Security-Token", c.SessionToken)
	}
	h := sha256.Sum256(payload)
	headers := "content-type:" + req.Header.Get("Content-Type") + "\n" + "host:" + req.URL.Host + "\n" + "x-amz-date:" + stamp + "\n" + "x-amz-target:" + req.Header.Get("X-Amz-Target") + "\n"
	signed := "content-type;host;x-amz-date;x-amz-target"
	canonical := req.Method + "\n" + req.URL.EscapedPath() + "\n" + req.URL.RawQuery + "\n" + headers + "\n" + signed + "\n" + hex.EncodeToString(h[:])
	ch := sha256.Sum256([]byte(canonical))
	scope := date + "/" + region + "/" + service + "/aws4_request"
	toSign := "AWS4-HMAC-SHA256\n" + stamp + "\n" + scope + "\n" + hex.EncodeToString(ch[:])
	key := hm(hm(hm(hm([]byte("AWS4"+c.SecretAccessKey), date), region), service), "aws4_request")
	sig := hex.EncodeToString(hm(key, toSign))
	req.Header.Set("Authorization", "AWS4-HMAC-SHA256 Credential="+c.AccessKeyID+"/"+scope+", SignedHeaders="+signed+", Signature="+sig)
}
func hm(key []byte, value string) []byte {
	h := hmac.New(sha256.New, key)
	_, _ = h.Write([]byte(value))
	return h.Sum(nil)
}
