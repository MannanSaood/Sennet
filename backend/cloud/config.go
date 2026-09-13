package cloud

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"
)

type ProviderType string

const (
	ProviderAWS   ProviderType = "aws"
	ProviderAzure ProviderType = "azure"
	ProviderGCP   ProviderType = "gcp"
)

// CloudConfig contains safe metadata only. Runtime credentials are supplied by
// a CredentialProvider and are never serialized into database JSON.
type CloudConfig struct {
	ID       string       `json:"id"`
	Provider ProviderType `json:"provider"`
	AWS      *AWSConfig   `json:"aws,omitempty"`
	Azure    *AzureConfig `json:"-"`
	GCP      *GCPConfig   `json:"-"`
}
type AWSConfig struct {
	Region             string `json:"region"`
	AccountID          string `json:"account_id"`
	WorkloadIdentityID string `json:"workload_identity_id"`
	RoleARN            string `json:"role_arn,omitempty"`
	AccessKeyID        string `json:"-"`
	SecretAccessKey    string `json:"-"`
	ExternalID         string `json:"-"`
	FlowLogsBucket     string `json:"flow_logs_bucket,omitempty"`
}
type AzureConfig struct {
	TenantID       string `json:"-"`
	ClientID       string `json:"-"`
	ClientSecret   string `json:"-"`
	SubscriptionID string `json:"-"`
}
type GCPConfig struct {
	ProjectID          string `json:"project_id"`
	ServiceAccountJSON string `json:"-"`
	ServiceAccountFile string `json:"-"`
}

func (c *CloudConfig) Validate() error {
	if c == nil || strings.TrimSpace(c.ID) == "" {
		return errors.New("provider configuration id is required")
	}
	switch c.Provider {
	case ProviderAWS:
		if c.AWS == nil {
			return errors.New("AWS config required for provider 'aws'")
		}
		return c.AWS.Validate()
	case ProviderAzure, ProviderGCP:
		return fmt.Errorf("provider %q is disabled: no truthful cost ingestion implementation", c.Provider)
	default:
		return fmt.Errorf("unsupported provider: %s", c.Provider)
	}
}
func (c *AWSConfig) Validate() error {
	if c == nil || strings.TrimSpace(c.Region) == "" {
		return errors.New("AWS region is required")
	}
	if len(c.AccountID) != 12 {
		return errors.New("AWS 12-digit account_id is required")
	}
	for _, r := range c.AccountID {
		if r < '0' || r > '9' {
			return errors.New("AWS account_id must contain 12 digits")
		}
	}
	if strings.TrimSpace(c.WorkloadIdentityID) == "" {
		return errors.New("workload_identity_id is required")
	}
	return nil
}
func (c *CloudConfig) ToJSON() (string, error) {
	if err := c.Validate(); err != nil {
		return "", err
	}
	b, err := json.Marshal(c)
	return string(b), err
}
func CloudConfigFromJSON(data string) (*CloudConfig, error) {
	var c CloudConfig
	if err := json.Unmarshal([]byte(data), &c); err != nil {
		return nil, err
	}
	if err := c.Validate(); err != nil {
		return nil, err
	}
	return &c, nil
}
