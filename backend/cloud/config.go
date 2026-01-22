package cloud

import (
	"encoding/json"
	"fmt"
)

type ProviderType string

const (
	ProviderAWS   ProviderType = "aws"
	ProviderAzure ProviderType = "azure"
	ProviderGCP   ProviderType = "gcp"
)

type CloudConfig struct {
	ID       string       `json:"id"`
	Provider ProviderType `json:"provider"`
	AWS      *AWSConfig   `json:"aws,omitempty"`
	Azure    *AzureConfig `json:"azure,omitempty"`
	GCP      *GCPConfig   `json:"gcp,omitempty"`
}

type AWSConfig struct {
	AccessKeyID     string `json:"access_key_id,omitempty"`
	SecretAccessKey string `json:"secret_access_key,omitempty"`
	RoleARN         string `json:"role_arn,omitempty"`
	ExternalID      string `json:"external_id,omitempty"`
	Region          string `json:"region"`
	FlowLogsBucket  string `json:"flow_logs_bucket,omitempty"`
}

type AzureConfig struct {
	TenantID       string `json:"tenant_id"`
	ClientID       string `json:"client_id"`
	ClientSecret   string `json:"client_secret"`
	SubscriptionID string `json:"subscription_id"`
}

type GCPConfig struct {
	ProjectID          string `json:"project_id"`
	ServiceAccountJSON string `json:"service_account_json,omitempty"`
	ServiceAccountFile string `json:"service_account_file,omitempty"`
}

type EgressCost struct {
	ID        int64   `json:"id"`
	Provider  string  `json:"provider"`
	Date      string  `json:"date"`
	Service   string  `json:"service"`
	Region    string  `json:"region"`
	CostUSD   float64 `json:"cost_usd"`
	BytesOut  int64   `json:"bytes_out"`
	CreatedAt string  `json:"created_at"`
}

type CostAttribution struct {
	ID         int64   `json:"id"`
	Date       string  `json:"date"`
	EntityType string  `json:"entity_type"`
	EntityName string  `json:"entity_name"`
	CostUSD    float64 `json:"cost_usd"`
	Bytes      int64   `json:"bytes"`
	Provider   string  `json:"provider"`
	Region     string  `json:"region"`
	CreatedAt  string  `json:"created_at"`
}

type Recommendation struct {
	ID                  int64   `json:"id"`
	Type                string  `json:"type"`
	Description         string  `json:"description"`
	EstimatedSavingsUSD float64 `json:"estimated_savings_usd"`
	Status              string  `json:"status"`
	CreatedAt           string  `json:"created_at"`
}

func (c *CloudConfig) Validate() error {
	switch c.Provider {
	case ProviderAWS:
		if c.AWS == nil {
			return fmt.Errorf("AWS config required for provider 'aws'")
		}
		return c.AWS.Validate()
	case ProviderAzure:
		if c.Azure == nil {
			return fmt.Errorf("Azure config required for provider 'azure'")
		}
		return c.Azure.Validate()
	case ProviderGCP:
		if c.GCP == nil {
			return fmt.Errorf("GCP config required for provider 'gcp'")
		}
		return c.GCP.Validate()
	default:
		return fmt.Errorf("unsupported provider: %s", c.Provider)
	}
}

func (c *AWSConfig) Validate() error {
	if c.Region == "" {
		return fmt.Errorf("AWS region is required")
	}
	hasStaticCreds := c.AccessKeyID != "" && c.SecretAccessKey != ""
	hasRoleARN := c.RoleARN != ""
	if !hasStaticCreds && !hasRoleARN {
		return fmt.Errorf("AWS requires either access keys or role ARN")
	}
	return nil
}

func (c *AzureConfig) Validate() error {
	if c.TenantID == "" {
		return fmt.Errorf("Azure tenant_id is required")
	}
	if c.ClientID == "" {
		return fmt.Errorf("Azure client_id is required")
	}
	if c.ClientSecret == "" {
		return fmt.Errorf("Azure client_secret is required")
	}
	if c.SubscriptionID == "" {
		return fmt.Errorf("Azure subscription_id is required")
	}
	return nil
}

func (c *GCPConfig) Validate() error {
	if c.ProjectID == "" {
		return fmt.Errorf("GCP project_id is required")
	}
	if c.ServiceAccountJSON == "" && c.ServiceAccountFile == "" {
		return fmt.Errorf("GCP requires service_account_json or service_account_file")
	}
	return nil
}

func (c *CloudConfig) ToJSON() (string, error) {
	data, err := json.Marshal(c)
	if err != nil {
		return "", err
	}
	return string(data), nil
}

func CloudConfigFromJSON(data string) (*CloudConfig, error) {
	var config CloudConfig
	if err := json.Unmarshal([]byte(data), &config); err != nil {
		return nil, err
	}
	return &config, nil
}
