package cloud

import (
	"context"
	"fmt"
	"sync"
	"time"
)

type CostResult struct {
	Date     time.Time
	Service  string
	Region   string
	CostUSD  float64
	BytesOut int64
}

type FlowLogEntry struct {
	Timestamp time.Time
	SrcIP     string
	DstIP     string
	SrcPort   int
	DstPort   int
	Bytes     int64
	Packets   int64
	Action    string
	Protocol  int
}

type Provider interface {
	Name() ProviderType
	FetchCosts(ctx context.Context, startDate, endDate time.Time) ([]CostResult, error)
	FetchFlowLogs(ctx context.Context, startDate, endDate time.Time) ([]FlowLogEntry, error)
	TestConnection(ctx context.Context) error
}

type Registry struct {
	mu        sync.RWMutex
	providers map[string]Provider
}

func NewRegistry() *Registry {
	return &Registry{
		providers: make(map[string]Provider),
	}
}

func (r *Registry) Register(id string, provider Provider) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.providers[id] = provider
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
	ids := make([]string, 0, len(r.providers))
	for id := range r.providers {
		ids = append(ids, id)
	}
	return ids
}

func (r *Registry) Remove(id string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.providers, id)
}

func CreateProvider(config *CloudConfig) (Provider, error) {
	switch config.Provider {
	case ProviderAWS:
		return NewAWSProvider(config.ID, config.AWS)
	case ProviderAzure:
		return NewAzureProvider(config.ID, config.Azure)
	case ProviderGCP:
		return NewGCPProvider(config.ID, config.GCP)
	default:
		return nil, fmt.Errorf("unsupported provider: %s", config.Provider)
	}
}

type AWSProvider struct {
	id     string
	config *AWSConfig
}

func NewAWSProvider(id string, config *AWSConfig) (*AWSProvider, error) {
	if config == nil {
		return nil, fmt.Errorf("AWS config is nil")
	}
	return &AWSProvider{id: id, config: config}, nil
}

func (p *AWSProvider) Name() ProviderType {
	return ProviderAWS
}

func (p *AWSProvider) FetchCosts(ctx context.Context, startDate, endDate time.Time) ([]CostResult, error) {
	return nil, fmt.Errorf("AWS Cost Explorer not implemented - requires aws-sdk-go-v2")
}

func (p *AWSProvider) FetchFlowLogs(ctx context.Context, startDate, endDate time.Time) ([]FlowLogEntry, error) {
	return nil, fmt.Errorf("AWS Flow Logs not implemented - requires aws-sdk-go-v2")
}

func (p *AWSProvider) TestConnection(ctx context.Context) error {
	return nil
}

type AzureProvider struct {
	id     string
	config *AzureConfig
}

func NewAzureProvider(id string, config *AzureConfig) (*AzureProvider, error) {
	if config == nil {
		return nil, fmt.Errorf("Azure config is nil")
	}
	return &AzureProvider{id: id, config: config}, nil
}

func (p *AzureProvider) Name() ProviderType {
	return ProviderAzure
}

func (p *AzureProvider) FetchCosts(ctx context.Context, startDate, endDate time.Time) ([]CostResult, error) {
	return nil, fmt.Errorf("Azure Cost Management not implemented - requires azure-sdk-for-go")
}

func (p *AzureProvider) FetchFlowLogs(ctx context.Context, startDate, endDate time.Time) ([]FlowLogEntry, error) {
	return nil, fmt.Errorf("Azure NSG Flow Logs not implemented")
}

func (p *AzureProvider) TestConnection(ctx context.Context) error {
	return nil
}

type GCPProvider struct {
	id     string
	config *GCPConfig
}

func NewGCPProvider(id string, config *GCPConfig) (*GCPProvider, error) {
	if config == nil {
		return nil, fmt.Errorf("GCP config is nil")
	}
	return &GCPProvider{id: id, config: config}, nil
}

func (p *GCPProvider) Name() ProviderType {
	return ProviderGCP
}

func (p *GCPProvider) FetchCosts(ctx context.Context, startDate, endDate time.Time) ([]CostResult, error) {
	return nil, fmt.Errorf("GCP Billing API not implemented - requires google-cloud-go")
}

func (p *GCPProvider) FetchFlowLogs(ctx context.Context, startDate, endDate time.Time) ([]FlowLogEntry, error) {
	return nil, fmt.Errorf("GCP VPC Flow Logs not implemented")
}

func (p *GCPProvider) TestConnection(ctx context.Context) error {
	return nil
}
