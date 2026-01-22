package handler

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/sennet/sennet/backend/cloud"
	"github.com/sennet/sennet/backend/correlation"
	"github.com/sennet/sennet/backend/db"
)

type CostHandler struct {
	database  *db.DB
	registry  *cloud.Registry
	engine    *correlation.Engine
	recEngine *correlation.RecommendationEngine
}

func NewCostHandler(database *db.DB, registry *cloud.Registry) *CostHandler {
	engine := correlation.NewEngine(database, registry)
	recEngine := correlation.NewRecommendationEngine(database)
	return &CostHandler{
		database:  database,
		registry:  registry,
		engine:    engine,
		recEngine: recEngine,
	}
}

type CloudConfigRequest struct {
	ID       string `json:"id"`
	Provider string `json:"provider"`
	AWS      struct {
		AccessKeyID     string `json:"access_key_id,omitempty"`
		SecretAccessKey string `json:"secret_access_key,omitempty"`
		RoleARN         string `json:"role_arn,omitempty"`
		Region          string `json:"region"`
	} `json:"aws,omitempty"`
	Azure struct {
		TenantID       string `json:"tenant_id"`
		ClientID       string `json:"client_id"`
		ClientSecret   string `json:"client_secret"`
		SubscriptionID string `json:"subscription_id"`
	} `json:"azure,omitempty"`
	GCP struct {
		ProjectID          string `json:"project_id"`
		ServiceAccountJSON string `json:"service_account_json,omitempty"`
	} `json:"gcp,omitempty"`
}

func (h *CostHandler) HandleGetCosts(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	startDate := r.URL.Query().Get("start")
	endDate := r.URL.Query().Get("end")

	if startDate == "" {
		startDate = time.Now().AddDate(0, 0, -30).Format("2006-01-02")
	}
	if endDate == "" {
		endDate = time.Now().Format("2006-01-02")
	}

	costs, err := h.database.GetEgressCosts(startDate, endDate)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(costs)
}

func (h *CostHandler) HandleGetCostsSummary(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	startDate := r.URL.Query().Get("start")
	endDate := r.URL.Query().Get("end")

	if startDate == "" {
		startDate = time.Now().AddDate(0, 0, -30).Format("2006-01-02")
	}
	if endDate == "" {
		endDate = time.Now().Format("2006-01-02")
	}

	summary, err := h.engine.GetCostSummary(startDate, endDate)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(summary)
}

func (h *CostHandler) HandleGetRecommendations(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	recs, err := h.database.GetRecommendations()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(recs)
}

func (h *CostHandler) HandleClouds(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		h.listClouds(w, r)
	case http.MethodPost:
		h.addCloud(w, r)
	case http.MethodDelete:
		h.deleteCloud(w, r)
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

func (h *CostHandler) listClouds(w http.ResponseWriter, r *http.Request) {
	configs, err := h.database.GetCloudConfigs()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	response := make([]map[string]interface{}, 0, len(configs))
	for _, c := range configs {
		response = append(response, map[string]interface{}{
			"id":         c.ID,
			"provider":   c.Provider,
			"created_at": c.CreatedAt,
		})
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

func (h *CostHandler) addCloud(w http.ResponseWriter, r *http.Request) {
	var req CloudConfigRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid JSON: "+err.Error(), http.StatusBadRequest)
		return
	}

	if req.ID == "" {
		http.Error(w, "id is required", http.StatusBadRequest)
		return
	}
	if req.Provider == "" {
		http.Error(w, "provider is required", http.StatusBadRequest)
		return
	}

	cloudConfig := &cloud.CloudConfig{
		ID:       req.ID,
		Provider: cloud.ProviderType(req.Provider),
	}

	switch req.Provider {
	case "aws":
		cloudConfig.AWS = &cloud.AWSConfig{
			AccessKeyID:     req.AWS.AccessKeyID,
			SecretAccessKey: req.AWS.SecretAccessKey,
			RoleARN:         req.AWS.RoleARN,
			Region:          req.AWS.Region,
		}
	case "azure":
		cloudConfig.Azure = &cloud.AzureConfig{
			TenantID:       req.Azure.TenantID,
			ClientID:       req.Azure.ClientID,
			ClientSecret:   req.Azure.ClientSecret,
			SubscriptionID: req.Azure.SubscriptionID,
		}
	case "gcp":
		cloudConfig.GCP = &cloud.GCPConfig{
			ProjectID:          req.GCP.ProjectID,
			ServiceAccountJSON: req.GCP.ServiceAccountJSON,
		}
	default:
		http.Error(w, "Unsupported provider: "+req.Provider, http.StatusBadRequest)
		return
	}

	if err := cloudConfig.Validate(); err != nil {
		http.Error(w, "Validation error: "+err.Error(), http.StatusBadRequest)
		return
	}

	configJSON, err := cloudConfig.ToJSON()
	if err != nil {
		http.Error(w, "Failed to serialize config", http.StatusInternalServerError)
		return
	}

	if err := h.database.SaveCloudConfig(req.ID, req.Provider, configJSON); err != nil {
		http.Error(w, "Failed to save config: "+err.Error(), http.StatusInternalServerError)
		return
	}

	provider, err := cloud.CreateProvider(cloudConfig)
	if err == nil {
		h.registry.Register(req.ID, provider)
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(map[string]string{
		"status": "created",
		"id":     req.ID,
	})
}

func (h *CostHandler) deleteCloud(w http.ResponseWriter, r *http.Request) {
	id := r.URL.Query().Get("id")
	if id == "" {
		http.Error(w, "id query parameter required", http.StatusBadRequest)
		return
	}

	if err := h.database.DeleteCloudConfig(id); err != nil {
		http.Error(w, "Failed to delete: "+err.Error(), http.StatusInternalServerError)
		return
	}

	h.registry.Remove(id)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{
		"status": "deleted",
		"id":     id,
	})
}

func (h *CostHandler) HandleSyncCosts(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	if err := h.engine.SyncCosts(r.Context(), 30); err != nil {
		http.Error(w, "Sync failed: "+err.Error(), http.StatusInternalServerError)
		return
	}

	startDate := time.Now().AddDate(0, 0, -30).Format("2006-01-02")
	endDate := time.Now().Format("2006-01-02")
	h.recEngine.GenerateRecommendations(startDate, endDate)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{
		"status": "synced",
	})
}
