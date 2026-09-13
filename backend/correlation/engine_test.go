package correlation

import (
	"context"
	"github.com/sennet/sennet/backend/cloud"
	"github.com/sennet/sennet/backend/db"
	"path/filepath"
	"testing"
)

func TestSyncCostsSurfacesProviderFailure(t *testing.T) {
	database, err := db.New(filepath.Join(t.TempDir(), "costs.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	registry := cloud.NewRegistry()
	provider, err := cloud.NewAWSProvider("aws", &cloud.AWSConfig{Region: "us-east-1", AccountID: "123456789012", WorkloadIdentityID: "test"}, cloud.EnvCredentials{}, nil)
	if err != nil {
		t.Fatal(err)
	}
	registry.Register("aws", provider)
	if err := NewEngine(database, registry).SyncCosts(context.Background(), 1); err == nil {
		t.Fatal("sync must not report success when provider fetch fails")
	}
}
