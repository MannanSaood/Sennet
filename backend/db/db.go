// Package db provides SQLite database operations for Sennet
package db

import (
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"fmt"
	"strings"
	"time"

	_ "modernc.org/sqlite"
)

// DB wraps the SQLite database connection
type DB struct {
	conn *sql.DB
}

// User represents a user in the database (linked to Firebase Auth)
type User struct {
	ID          string
	FirebaseUID string
	Email       string
	Name        string
	Role        string // "admin", "user"
	CreatedAt   time.Time
}

// Agent represents a registered agent in the database
type Agent struct {
	ID       string
	LastSeen time.Time
	Version  string
	OwnerID  *string // Owner user ID for multi-tenancy
}

// APIKey represents an API key in the database
type APIKey struct {
	Key       string
	Name      string
	CreatedAt time.Time
	ExpiresAt *time.Time // nil means never expires
	LastUsed  *time.Time // nil means never used
	UserID    *string    // Owner user ID
}

// New creates a new database connection and initializes schema
func New(path string) (*DB, error) {
	conn, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	// Enable WAL mode for better concurrency
	_, err = conn.Exec("PRAGMA journal_mode=WAL")
	if err != nil {
		conn.Close()
		return nil, fmt.Errorf("failed to set WAL mode: %w", err)
	}

	db := &DB{conn: conn}
	if err := db.migrate(); err != nil {
		conn.Close()
		return nil, fmt.Errorf("failed to migrate database: %w", err)
	}

	return db, nil
}

// migrate creates the database schema
func (db *DB) migrate() error {
	schema := `
	-- Users table (linked to Firebase Auth)
	CREATE TABLE IF NOT EXISTS users (
		id TEXT PRIMARY KEY,
		firebase_uid TEXT UNIQUE,
		email TEXT UNIQUE NOT NULL,
		name TEXT,
		role TEXT DEFAULT 'user',
		created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
	);

	CREATE INDEX IF NOT EXISTS idx_users_firebase ON users(firebase_uid);
	CREATE INDEX IF NOT EXISTS idx_users_email ON users(email);

	CREATE TABLE IF NOT EXISTS agents (
		id TEXT PRIMARY KEY,
		last_seen TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
		version TEXT NOT NULL DEFAULT '',
		owner_id TEXT REFERENCES users(id)
	);

	CREATE TABLE IF NOT EXISTS api_keys (
		key TEXT PRIMARY KEY,
		name TEXT NOT NULL,
		created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
		expires_at TIMESTAMP,
		last_used TIMESTAMP,
		user_id TEXT REFERENCES users(id)
	);

	CREATE INDEX IF NOT EXISTS idx_agents_last_seen ON agents(last_seen);

	CREATE TABLE IF NOT EXISTS cloud_configs (
		id TEXT PRIMARY KEY,
		provider TEXT NOT NULL,
		config_json TEXT NOT NULL,
		created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
	);

	CREATE TABLE IF NOT EXISTS egress_costs (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		provider TEXT NOT NULL,
		date TEXT NOT NULL,
		service TEXT,
		region TEXT,
		cost_usd REAL NOT NULL,
		bytes_out INTEGER,
		created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
		UNIQUE(provider, date, service, region)
	);

	CREATE TABLE IF NOT EXISTS cost_attributions (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		date TEXT NOT NULL,
		entity_type TEXT NOT NULL,
		entity_name TEXT NOT NULL,
		cost_usd REAL NOT NULL,
		bytes INTEGER,
		provider TEXT,
		region TEXT,
		created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
	);

	CREATE TABLE IF NOT EXISTS recommendations (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		type TEXT NOT NULL,
		description TEXT NOT NULL,
		estimated_savings_usd REAL,
		status TEXT DEFAULT 'open',
		created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
	);

	CREATE INDEX IF NOT EXISTS idx_egress_costs_date ON egress_costs(date);
	CREATE INDEX IF NOT EXISTS idx_cost_attributions_date ON cost_attributions(date);
	`

	_, err := db.conn.Exec(schema)
	return err
}

// Close closes the database connection
func (db *DB) Close() error {
	return db.conn.Close()
}

// Ping checks database connectivity
func (db *DB) Ping() error {
	return db.conn.Ping()
}

// CreateOrUpdateAgent creates or updates an agent record
func (db *DB) CreateOrUpdateAgent(agentID, version string) error {
	query := `
	INSERT INTO agents (id, last_seen, version)
	VALUES (?, CURRENT_TIMESTAMP, ?)
	ON CONFLICT(id) DO UPDATE SET
		last_seen = CURRENT_TIMESTAMP,
		version = excluded.version
	`
	_, err := db.conn.Exec(query, agentID, version)
	return err
}

// GetAgent retrieves an agent by ID
func (db *DB) GetAgent(agentID string) (*Agent, error) {
	query := `SELECT id, last_seen, version FROM agents WHERE id = ?`
	row := db.conn.QueryRow(query, agentID)

	agent := &Agent{}
	err := row.Scan(&agent.ID, &agent.LastSeen, &agent.Version)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return agent, nil
}

// CreateAPIKey generates and stores a new API key
func (db *DB) CreateAPIKey(name string) (string, error) {
	// Generate random key: sk_<32 hex chars>
	bytes := make([]byte, 16)
	if _, err := rand.Read(bytes); err != nil {
		return "", fmt.Errorf("failed to generate random key: %w", err)
	}
	key := "sk_" + hex.EncodeToString(bytes)

	query := `INSERT INTO api_keys (key, name, created_at) VALUES (?, ?, CURRENT_TIMESTAMP)`
	_, err := db.conn.Exec(query, key, name)
	if err != nil {
		return "", err
	}

	return key, nil
}

// EnsureAPIKey ensures a specific API key exists (for seeding from environment)
func (db *DB) EnsureAPIKey(key, name string) error {
	query := `INSERT OR IGNORE INTO api_keys (key, name, created_at) VALUES (?, ?, CURRENT_TIMESTAMP)`
	_, err := db.conn.Exec(query, key, name)
	return err
}

// ValidateAPIKey checks if an API key exists and is valid
func (db *DB) ValidateAPIKey(key string) (bool, error) {
	// Basic format check
	if !strings.HasPrefix(key, "sk_") {
		return false, nil
	}

	query := `SELECT 1 FROM api_keys WHERE key = ?`
	row := db.conn.QueryRow(query, key)

	var exists int
	err := row.Scan(&exists)
	if err == sql.ErrNoRows {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return true, nil
}

// APIKeyExists checks if an API key exists (for signature verification)
func (db *DB) APIKeyExists(key string) (bool, error) {
	query := `SELECT 1 FROM api_keys WHERE key = ? AND (expires_at IS NULL OR expires_at > datetime('now'))`
	row := db.conn.QueryRow(query, key)

	var exists int
	err := row.Scan(&exists)
	if err == sql.ErrNoRows {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return true, nil
}

// UpdateAPIKeyLastUsed updates the last_used timestamp for an API key
func (db *DB) UpdateAPIKeyLastUsed(key string) error {
	query := `UPDATE api_keys SET last_used = CURRENT_TIMESTAMP WHERE key = ?`
	_, err := db.conn.Exec(query, key)
	return err
}

// RotateAPIKey creates a new API key and marks the old one as expiring in 24 hours
// Returns the new API key
func (db *DB) RotateAPIKey(oldKey string) (string, error) {
	// Get the name of the old key
	var name string
	err := db.conn.QueryRow(`SELECT name FROM api_keys WHERE key = ?`, oldKey).Scan(&name)
	if err != nil {
		return "", fmt.Errorf("old key not found: %w", err)
	}

	// Mark old key to expire in 24 hours (grace period for agent updates)
	_, err = db.conn.Exec(
		`UPDATE api_keys SET expires_at = datetime('now', '+1 day') WHERE key = ?`,
		oldKey,
	)
	if err != nil {
		return "", fmt.Errorf("failed to set expiration on old key: %w", err)
	}

	// Create new key with same name (appending "-rotated")
	newKey, err := db.CreateAPIKey(name + "-rotated")
	if err != nil {
		return "", fmt.Errorf("failed to create new key: %w", err)
	}

	return newKey, nil
}

// DeleteExpiredAPIKeys removes API keys that have passed their expiration
func (db *DB) DeleteExpiredAPIKeys() (int64, error) {
	result, err := db.conn.Exec(`DELETE FROM api_keys WHERE expires_at IS NOT NULL AND expires_at < datetime('now')`)
	if err != nil {
		return 0, err
	}
	return result.RowsAffected()
}

// ListAPIKeys returns all API keys
func (db *DB) ListAPIKeys() ([]APIKey, error) {
	query := `SELECT key, name, created_at, expires_at, last_used FROM api_keys ORDER BY created_at DESC`
	rows, err := db.conn.Query(query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var keys []APIKey
	for rows.Next() {
		var k APIKey
		if err := rows.Scan(&k.Key, &k.Name, &k.CreatedAt, &k.ExpiresAt, &k.LastUsed); err != nil {
			return nil, err
		}
		keys = append(keys, k)
	}
	return keys, rows.Err()
}

// ========== User Management ==========

// CreateUser creates a new user from Firebase Auth data
func (db *DB) CreateUser(firebaseUID, email, name, role string) (*User, error) {
	id := generateUserID()
	query := `INSERT INTO users (id, firebase_uid, email, name, role) VALUES (?, ?, ?, ?, ?)`
	_, err := db.conn.Exec(query, id, firebaseUID, email, name, role)
	if err != nil {
		return nil, err
	}
	return &User{
		ID:          id,
		FirebaseUID: firebaseUID,
		Email:       email,
		Name:        name,
		Role:        role,
	}, nil
}

// generateUserID creates a new user ID
func generateUserID() string {
	bytes := make([]byte, 8)
	rand.Read(bytes)
	return "usr_" + hex.EncodeToString(bytes)
}

// GetUserByFirebaseUID retrieves a user by their Firebase UID
func (db *DB) GetUserByFirebaseUID(firebaseUID string) (*User, error) {
	query := `SELECT id, firebase_uid, email, name, role, created_at FROM users WHERE firebase_uid = ?`
	row := db.conn.QueryRow(query, firebaseUID)

	user := &User{}
	err := row.Scan(&user.ID, &user.FirebaseUID, &user.Email, &user.Name, &user.Role, &user.CreatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return user, nil
}

// GetUserByEmail retrieves a user by their email
func (db *DB) GetUserByEmail(email string) (*User, error) {
	query := `SELECT id, firebase_uid, email, name, role, created_at FROM users WHERE email = ?`
	row := db.conn.QueryRow(query, email)

	user := &User{}
	err := row.Scan(&user.ID, &user.FirebaseUID, &user.Email, &user.Name, &user.Role, &user.CreatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return user, nil
}

// GetOrCreateUser gets a user by Firebase UID or creates one if not exists
func (db *DB) GetOrCreateUser(firebaseUID, email, name string) (*User, error) {
	user, err := db.GetUserByFirebaseUID(firebaseUID)
	if err != nil {
		return nil, err
	}
	if user != nil {
		return user, nil
	}
	return db.CreateUser(firebaseUID, email, name, "user")
}

// GetAgentsByOwner returns agents owned by a specific user
func (db *DB) GetAgentsByOwner(ownerID string) ([]Agent, error) {
	query := `SELECT id, last_seen, version, owner_id FROM agents WHERE owner_id = ?`
	rows, err := db.conn.Query(query, ownerID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var agents []Agent
	for rows.Next() {
		var a Agent
		if err := rows.Scan(&a.ID, &a.LastSeen, &a.Version, &a.OwnerID); err != nil {
			return nil, err
		}
		agents = append(agents, a)
	}
	return agents, rows.Err()
}

// SetAgentOwner assigns an agent to a user
func (db *DB) SetAgentOwner(agentID, ownerID string) error {
	query := `UPDATE agents SET owner_id = ? WHERE id = ?`
	_, err := db.conn.Exec(query, ownerID, agentID)
	return err
}

// GetAgentCount returns the total number of registered agents
func (db *DB) GetAgentCount() (int, error) {
	var count int
	err := db.conn.QueryRow(`SELECT COUNT(*) FROM agents`).Scan(&count)
	return count, err
}

// GetActiveAgentCount returns agents seen in the last N minutes
func (db *DB) GetActiveAgentCount(minutes int) (int, error) {
	query := `SELECT COUNT(*) FROM agents WHERE last_seen > datetime('now', ?)`
	var count int
	err := db.conn.QueryRow(query, fmt.Sprintf("-%d minutes", minutes)).Scan(&count)
	return count, err
}

// CloudConfig represents a cloud provider configuration
type CloudConfig struct {
	ID         string
	Provider   string
	ConfigJSON string
	CreatedAt  time.Time
}

// EgressCost represents a daily egress cost aggregate
type EgressCost struct {
	ID        int64
	Provider  string
	Date      string
	Service   string
	Region    string
	CostUSD   float64
	BytesOut  int64
	CreatedAt time.Time
}

// CostAttribution represents cost attributed to an entity
type CostAttribution struct {
	ID         int64
	Date       string
	EntityType string
	EntityName string
	CostUSD    float64
	Bytes      int64
	Provider   string
	Region     string
	CreatedAt  time.Time
}

// Recommendation represents an optimization recommendation
type Recommendation struct {
	ID                  int64
	Type                string
	Description         string
	EstimatedSavingsUSD float64
	Status              string
	CreatedAt           time.Time
}

// SaveCloudConfig stores a cloud provider configuration
func (db *DB) SaveCloudConfig(id, provider, configJSON string) error {
	query := `
	INSERT INTO cloud_configs (id, provider, config_json, created_at)
	VALUES (?, ?, ?, CURRENT_TIMESTAMP)
	ON CONFLICT(id) DO UPDATE SET
		provider = excluded.provider,
		config_json = excluded.config_json
	`
	_, err := db.conn.Exec(query, id, provider, configJSON)
	return err
}

// GetCloudConfigs returns all cloud configurations
func (db *DB) GetCloudConfigs() ([]CloudConfig, error) {
	query := `SELECT id, provider, config_json, created_at FROM cloud_configs ORDER BY created_at DESC`
	rows, err := db.conn.Query(query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var configs []CloudConfig
	for rows.Next() {
		var c CloudConfig
		if err := rows.Scan(&c.ID, &c.Provider, &c.ConfigJSON, &c.CreatedAt); err != nil {
			return nil, err
		}
		configs = append(configs, c)
	}
	return configs, rows.Err()
}

// GetCloudConfig returns a specific cloud configuration by ID
func (db *DB) GetCloudConfig(id string) (*CloudConfig, error) {
	query := `SELECT id, provider, config_json, created_at FROM cloud_configs WHERE id = ?`
	row := db.conn.QueryRow(query, id)

	var c CloudConfig
	err := row.Scan(&c.ID, &c.Provider, &c.ConfigJSON, &c.CreatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &c, nil
}

// DeleteCloudConfig removes a cloud configuration
func (db *DB) DeleteCloudConfig(id string) error {
	_, err := db.conn.Exec(`DELETE FROM cloud_configs WHERE id = ?`, id)
	return err
}

// SaveEgressCost stores or updates a daily egress cost
func (db *DB) SaveEgressCost(provider, date, service, region string, costUSD float64, bytesOut int64) error {
	query := `
	INSERT INTO egress_costs (provider, date, service, region, cost_usd, bytes_out, created_at)
	VALUES (?, ?, ?, ?, ?, ?, CURRENT_TIMESTAMP)
	ON CONFLICT(provider, date, service, region) DO UPDATE SET
		cost_usd = excluded.cost_usd,
		bytes_out = excluded.bytes_out
	`
	_, err := db.conn.Exec(query, provider, date, service, region, costUSD, bytesOut)
	return err
}

// GetEgressCosts returns egress costs for a date range
func (db *DB) GetEgressCosts(startDate, endDate string) ([]EgressCost, error) {
	query := `
	SELECT id, provider, date, service, region, cost_usd, bytes_out, created_at
	FROM egress_costs
	WHERE date >= ? AND date <= ?
	ORDER BY date DESC, provider, service
	`
	rows, err := db.conn.Query(query, startDate, endDate)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var costs []EgressCost
	for rows.Next() {
		var c EgressCost
		if err := rows.Scan(&c.ID, &c.Provider, &c.Date, &c.Service, &c.Region, &c.CostUSD, &c.BytesOut, &c.CreatedAt); err != nil {
			return nil, err
		}
		costs = append(costs, c)
	}
	return costs, rows.Err()
}

// GetEgressCostsSummary returns aggregated costs by provider and service
func (db *DB) GetEgressCostsSummary(startDate, endDate string) (map[string]float64, error) {
	query := `
	SELECT provider || ':' || COALESCE(service, 'unknown'), SUM(cost_usd)
	FROM egress_costs
	WHERE date >= ? AND date <= ?
	GROUP BY provider, service
	`
	rows, err := db.conn.Query(query, startDate, endDate)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	summary := make(map[string]float64)
	for rows.Next() {
		var key string
		var total float64
		if err := rows.Scan(&key, &total); err != nil {
			return nil, err
		}
		summary[key] = total
	}
	return summary, rows.Err()
}

// SaveCostAttribution stores a cost attribution record
func (db *DB) SaveCostAttribution(date, entityType, entityName string, costUSD float64, bytes int64, provider, region string) error {
	query := `
	INSERT INTO cost_attributions (date, entity_type, entity_name, cost_usd, bytes, provider, region, created_at)
	VALUES (?, ?, ?, ?, ?, ?, ?, CURRENT_TIMESTAMP)
	`
	_, err := db.conn.Exec(query, date, entityType, entityName, costUSD, bytes, provider, region)
	return err
}

// GetCostAttributions returns attributions for a date range
func (db *DB) GetCostAttributions(startDate, endDate string) ([]CostAttribution, error) {
	query := `
	SELECT id, date, entity_type, entity_name, cost_usd, bytes, provider, region, created_at
	FROM cost_attributions
	WHERE date >= ? AND date <= ?
	ORDER BY cost_usd DESC
	`
	rows, err := db.conn.Query(query, startDate, endDate)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var attrs []CostAttribution
	for rows.Next() {
		var a CostAttribution
		if err := rows.Scan(&a.ID, &a.Date, &a.EntityType, &a.EntityName, &a.CostUSD, &a.Bytes, &a.Provider, &a.Region, &a.CreatedAt); err != nil {
			return nil, err
		}
		attrs = append(attrs, a)
	}
	return attrs, rows.Err()
}

// SaveRecommendation stores an optimization recommendation
func (db *DB) SaveRecommendation(recType, description string, estimatedSavingsUSD float64) error {
	query := `
	INSERT INTO recommendations (type, description, estimated_savings_usd, status, created_at)
	VALUES (?, ?, ?, 'open', CURRENT_TIMESTAMP)
	`
	_, err := db.conn.Exec(query, recType, description, estimatedSavingsUSD)
	return err
}

// GetRecommendations returns all open recommendations
func (db *DB) GetRecommendations() ([]Recommendation, error) {
	query := `
	SELECT id, type, description, estimated_savings_usd, status, created_at
	FROM recommendations
	WHERE status = 'open'
	ORDER BY estimated_savings_usd DESC
	`
	rows, err := db.conn.Query(query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var recs []Recommendation
	for rows.Next() {
		var r Recommendation
		if err := rows.Scan(&r.ID, &r.Type, &r.Description, &r.EstimatedSavingsUSD, &r.Status, &r.CreatedAt); err != nil {
			return nil, err
		}
		recs = append(recs, r)
	}
	return recs, rows.Err()
}

// UpdateRecommendationStatus updates the status of a recommendation
func (db *DB) UpdateRecommendationStatus(id int64, status string) error {
	_, err := db.conn.Exec(`UPDATE recommendations SET status = ? WHERE id = ?`, status, id)
	return err
}
