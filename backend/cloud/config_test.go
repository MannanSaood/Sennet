package cloud

import (
	"testing"
)

func TestAWSConfig_Validate(t *testing.T) {
	tests := []struct {
		name    string
		config  *AWSConfig
		wantErr bool
	}{
		{
			name:    "empty config should error",
			config:  &AWSConfig{},
			wantErr: true,
		},
		{
			name: "valid static credentials",
			config: &AWSConfig{
				AccessKeyID:     "AKIAIOSFODNN7EXAMPLE",
				SecretAccessKey: "wJalrXUtnFEMI/K7MDENG/bPxRfiCYEXAMPLEKEY",
				Region:          "us-east-1",
			},
			wantErr: false,
		},
		{
			name: "valid role ARN",
			config: &AWSConfig{
				RoleARN: "arn:aws:iam::123456789012:role/CostExplorerRole",
				Region:  "us-east-1",
			},
			wantErr: false,
		},
		{
			name: "missing region",
			config: &AWSConfig{
				AccessKeyID:     "AKIAIOSFODNN7EXAMPLE",
				SecretAccessKey: "wJalrXUtnFEMI/K7MDENG/bPxRfiCYEXAMPLEKEY",
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.config.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestAzureConfig_Validate(t *testing.T) {
	tests := []struct {
		name    string
		config  *AzureConfig
		wantErr bool
	}{
		{
			name:    "empty config should error",
			config:  &AzureConfig{},
			wantErr: true,
		},
		{
			name: "valid config",
			config: &AzureConfig{
				TenantID:       "12345678-1234-1234-1234-123456789012",
				ClientID:       "abcdefgh-abcd-abcd-abcd-abcdefghijkl",
				ClientSecret:   "secret123",
				SubscriptionID: "sub-12345678",
			},
			wantErr: false,
		},
		{
			name: "missing tenant ID",
			config: &AzureConfig{
				ClientID:       "abcdefgh-abcd-abcd-abcd-abcdefghijkl",
				ClientSecret:   "secret123",
				SubscriptionID: "sub-12345678",
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.config.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestGCPConfig_Validate(t *testing.T) {
	tests := []struct {
		name    string
		config  *GCPConfig
		wantErr bool
	}{
		{
			name:    "empty config should error",
			config:  &GCPConfig{},
			wantErr: true,
		},
		{
			name: "valid with JSON credentials",
			config: &GCPConfig{
				ProjectID:          "my-project-123",
				ServiceAccountJSON: `{"type":"service_account","project_id":"my-project"}`,
			},
			wantErr: false,
		},
		{
			name: "valid with file path",
			config: &GCPConfig{
				ProjectID:          "my-project-123",
				ServiceAccountFile: "/path/to/credentials.json",
			},
			wantErr: false,
		},
		{
			name: "missing credentials",
			config: &GCPConfig{
				ProjectID: "my-project-123",
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.config.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestCloudConfig_ToJSON(t *testing.T) {
	config := &CloudConfig{
		ID:       "test-config",
		Provider: ProviderAWS,
		AWS: &AWSConfig{
			Region:  "us-east-1",
			RoleARN: "arn:aws:iam::123456789012:role/CostExplorerRole",
		},
	}

	jsonStr, err := config.ToJSON()
	if err != nil {
		t.Fatalf("ToJSON() error = %v", err)
	}

	parsed, err := CloudConfigFromJSON(jsonStr)
	if err != nil {
		t.Fatalf("CloudConfigFromJSON() error = %v", err)
	}

	if parsed.ID != config.ID {
		t.Errorf("ID mismatch: got %s, want %s", parsed.ID, config.ID)
	}
	if parsed.Provider != config.Provider {
		t.Errorf("Provider mismatch: got %s, want %s", parsed.Provider, config.Provider)
	}
	if parsed.AWS.Region != config.AWS.Region {
		t.Errorf("Region mismatch: got %s, want %s", parsed.AWS.Region, config.AWS.Region)
	}
}

func TestRegistry(t *testing.T) {
	registry := NewRegistry()

	config := &CloudConfig{
		ID:       "test-aws",
		Provider: ProviderAWS,
		AWS: &AWSConfig{
			Region:  "us-east-1",
			RoleARN: "arn:aws:iam::123456789012:role/TestRole",
		},
	}

	provider, err := CreateProvider(config)
	if err != nil {
		t.Fatalf("CreateProvider() error = %v", err)
	}

	registry.Register("test-aws", provider)

	if len(registry.List()) != 1 {
		t.Errorf("Expected 1 provider, got %d", len(registry.List()))
	}

	p, ok := registry.Get("test-aws")
	if !ok {
		t.Error("Expected to find provider 'test-aws'")
	}
	if p.Name() != ProviderAWS {
		t.Errorf("Expected provider name %s, got %s", ProviderAWS, p.Name())
	}

	registry.Remove("test-aws")
	if len(registry.List()) != 0 {
		t.Errorf("Expected 0 providers after removal, got %d", len(registry.List()))
	}
}
