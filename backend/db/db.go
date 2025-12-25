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

// Agent represents a registered agent in the database
type Agent struct {
	ID       string
	LastSeen time.Time
	Version  string
}

// APIKey represents an API key in the database
type APIKey struct {
	Key       string
	Name      string
	CreatedAt time.Time
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
	CREATE TABLE IF NOT EXISTS agents (
		id TEXT PRIMARY KEY,
		last_seen TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
		version TEXT NOT NULL DEFAULT ''
	);

	CREATE TABLE IF NOT EXISTS api_keys (
		key TEXT PRIMARY KEY,
		name TEXT NOT NULL,
		created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
	);

	CREATE INDEX IF NOT EXISTS idx_agents_last_seen ON agents(last_seen);
	`

	_, err := db.conn.Exec(schema)
	return err
}

// Close closes the database connection
func (db *DB) Close() error {
	return db.conn.Close()
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

// ListAPIKeys returns all API keys
func (db *DB) ListAPIKeys() ([]APIKey, error) {
	query := `SELECT key, name, created_at FROM api_keys ORDER BY created_at DESC`
	rows, err := db.conn.Query(query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var keys []APIKey
	for rows.Next() {
		var k APIKey
		if err := rows.Scan(&k.Key, &k.Name, &k.CreatedAt); err != nil {
			return nil, err
		}
		keys = append(keys, k)
	}
	return keys, rows.Err()
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
