package platform

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"fmt"
	_ "github.com/jackc/pgx/v5/stdlib"
	_ "modernc.org/sqlite"
	"strings"
	"time"
)

type Principal struct {
	Tenant  string `json:"tenant"`
	Subject string `json:"subject"`
	Role    string `json:"role"`
}
type Store struct {
	db       *sql.DB
	postgres bool
}

func Open(dsn string) (*Store, error) {
	driver := "sqlite"
	pg := strings.HasPrefix(dsn, "postgres://") || strings.HasPrefix(dsn, "postgresql://")
	if pg {
		driver = "pgx"
	}
	d, err := sql.Open(driver, dsn)
	if err != nil {
		return nil, err
	}
	s := &Store{d, pg}
	d.SetMaxOpenConns(16)
	d.SetConnMaxLifetime(5 * time.Minute)
	if !pg {
		d.SetMaxOpenConns(1)
		if _, err = d.Exec("PRAGMA journal_mode=WAL; PRAGMA busy_timeout=5000; PRAGMA synchronous=FULL;"); err != nil {
			d.Close()
			return nil, err
		}
	}
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	if err = d.PingContext(ctx); err != nil {
		d.Close()
		return nil, err
	}
	schema := []string{
		`CREATE TABLE IF NOT EXISTS platform_schema (version INTEGER PRIMARY KEY)`,
		`CREATE TABLE IF NOT EXISTS platform_agents (tenant TEXT NOT NULL, id TEXT NOT NULL, version TEXT NOT NULL, seen BIGINT NOT NULL, payload TEXT NOT NULL, PRIMARY KEY(tenant,id))`,
		`CREATE TABLE IF NOT EXISTS platform_events (tenant TEXT NOT NULL, id TEXT NOT NULL, time_ms BIGINT NOT NULL, signal TEXT NOT NULL, service TEXT NOT NULL, trace_id TEXT NOT NULL, payload TEXT NOT NULL, PRIMARY KEY(tenant,id))`,
		`CREATE INDEX IF NOT EXISTS platform_events_query ON platform_events(tenant,time_ms,id)`,
		`CREATE TABLE IF NOT EXISTS platform_resources (tenant TEXT NOT NULL, kind TEXT NOT NULL, id TEXT NOT NULL, payload TEXT NOT NULL, updated BIGINT NOT NULL, PRIMARY KEY(tenant,kind,id))`,
		`INSERT INTO platform_schema(version) VALUES(1) ON CONFLICT(version) DO NOTHING`,
	}
	tx, err := d.BeginTx(ctx, nil)
	if err != nil {
		d.Close()
		return nil, err
	}
	defer tx.Rollback()
	for _, q := range schema {
		if _, err = tx.ExecContext(ctx, q); err != nil {
			d.Close()
			return nil, err
		}
	}
	if err = tx.Commit(); err != nil {
		d.Close()
		return nil, err
	}
	return s, nil
}
func (s *Store) Close() error { return s.db.Close() }
func (s *Store) q(q string) string {
	if !s.postgres {
		return q
	}
	n := 0
	return strings.Map(func(r rune) rune { return r }, replaceParams(q, &n))
}
func replaceParams(q string, n *int) string {
	var b strings.Builder
	for _, c := range q {
		if c == '?' {
			*n++
			fmt.Fprintf(&b, "$%d", *n)
		} else {
			b.WriteRune(c)
		}
	}
	return b.String()
}
func randomID() string {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		panic(err)
	}
	return hex.EncodeToString(b)
}
func digest(v string) string  { h := sha256.Sum256([]byte(v)); return hex.EncodeToString(h[:]) }
func validRole(v string) bool { return v == "admin" || v == "reader" || v == "ingest" }

type Agent struct {
	ID         string            `json:"id"`
	Version    string            `json:"version"`
	Seen       int64             `json:"seen"`
	Metrics    map[string]string `json:"metrics"`
	Collection string            `json:"collection"`
}

func (s *Store) PutAgent(ctx context.Context, p Principal, a Agent) error {
	b, _ := json.Marshal(a)
	_, err := s.db.ExecContext(ctx, s.q(`INSERT INTO platform_agents(tenant,id,version,seen,payload) VALUES(?,?,?,?,?) ON CONFLICT(tenant,id) DO UPDATE SET version=excluded.version,seen=excluded.seen,payload=excluded.payload`), p.Tenant, a.ID, a.Version, a.Seen, string(b))
	return err
}
func (s *Store) Agents(ctx context.Context, p Principal) ([]Agent, error) {
	rows, err := s.db.QueryContext(ctx, s.q(`SELECT payload FROM platform_agents WHERE tenant=? ORDER BY seen DESC LIMIT 1000`), p.Tenant)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Agent{}
	for rows.Next() {
		var b string
		var a Agent
		if err = rows.Scan(&b); err != nil {
			return nil, err
		}
		if err = json.Unmarshal([]byte(b), &a); err != nil {
			return nil, err
		}
		out = append(out, a)
	}
	return out, rows.Err()
}
func (s *Store) Resources(ctx context.Context, p Principal, kind string) ([]json.RawMessage, error) {
	rows, err := s.db.QueryContext(ctx, s.q(`SELECT payload FROM platform_resources WHERE tenant=? AND kind=? ORDER BY updated DESC LIMIT 1000`), p.Tenant, kind)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []json.RawMessage{}
	for rows.Next() {
		var b string
		if err = rows.Scan(&b); err != nil {
			return nil, err
		}
		out = append(out, json.RawMessage(b))
	}
	return out, rows.Err()
}
func (s *Store) PutResource(ctx context.Context, p Principal, kind, id string, b []byte) error {
	_, err := s.db.ExecContext(ctx, s.q(`INSERT INTO platform_resources(tenant,kind,id,payload,updated) VALUES(?,?,?,?,?) ON CONFLICT(tenant,kind,id) DO UPDATE SET payload=excluded.payload,updated=excluded.updated`), p.Tenant, kind, id, string(b), time.Now().UnixMilli())
	return err
}
func (s *Store) DeleteResource(ctx context.Context, p Principal, kind, id string) error {
	r, err := s.db.ExecContext(ctx, s.q(`DELETE FROM platform_resources WHERE tenant=? AND kind=? AND id=?`), p.Tenant, kind, id)
	if err != nil {
		return err
	}
	n, _ := r.RowsAffected()
	if n == 0 {
		return sql.ErrNoRows
	}
	return nil
}
