package handler

import (
	"encoding/json"
	"net/http"

	"github.com/sennet/sennet/backend/db"
)

type KeyHandler struct {
	database *db.DB
}

func NewKeyHandler(database *db.DB) *KeyHandler {
	return &KeyHandler{
		database: database,
	}
}

// HandleGetKeys lists all API keys
func (h *KeyHandler) HandleGetKeys(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	keys, err := h.database.ListAPIKeys()
	if err != nil {
		http.Error(w, "Failed to list keys", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(keys)
}

// HandleCreateKey creates a new API key
func (h *KeyHandler) HandleCreateKey(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		Name string `json:"name"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	if req.Name == "" {
		http.Error(w, "Name is required", http.StatusBadRequest)
		return
	}

	key, err := h.database.CreateAPIKey(req.Name)
	if err != nil {
		http.Error(w, "Failed to create key", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{
		"key":  key,
		"name": req.Name,
	})
}

// HandleDeleteKey deletes an API key (actually just marks it or deletes - specific logic depends on DB implementation)
// Note: The current DB implementation doesn't have a DeleteAPIKey method, but we can add one or use Execute directly.
// For now, let's implement a direct SQL delete for expediency, or better, add it to DB.
// Let's assume we'll just return not implemented until we add it to DB, or I'll implement it here via DB access.
// Actually, looking at db.go, there is no DeleteAPIKey. Let's add it there first??
// No, I can't edit `db.go` and `handler` in one turn efficiently without context.
// I will start with List and Create, as those are the most critical.
// I'll skip Delete for this exact step to keep it atomic, or I can try to use raw Exec if I had access.
// Let's stick to List and Create for now.
