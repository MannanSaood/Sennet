package platform

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
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
type Key struct {
	ID      string `json:"id"`
	Name    string `json:"name"`
	Role    string `json:"role"`
	Prefix  string `json:"prefix"`
	Created int64  `json:"created"`
	Expires int64  `json:"expires"`
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
		`CREATE TABLE IF NOT EXISTS platform_keys (id TEXT PRIMARY KEY, hash TEXT UNIQUE NOT NULL, tenant TEXT NOT NULL, name TEXT NOT NULL, role TEXT NOT NULL, prefix TEXT NOT NULL, created BIGINT NOT NULL, expires BIGINT NOT NULL, revoked INTEGER NOT NULL DEFAULT 0)`,
		`CREATE INDEX IF NOT EXISTS platform_keys_tenant ON platform_keys(tenant,created)`,
		`CREATE TABLE IF NOT EXISTS platform_agents (tenant TEXT NOT NULL, id TEXT NOT NULL, version TEXT NOT NULL, seen BIGINT NOT NULL, payload TEXT NOT NULL, PRIMARY KEY(tenant,id))`,
		`CREATE TABLE IF NOT EXISTS platform_events (tenant TEXT NOT NULL, id TEXT NOT NULL, time_ms BIGINT NOT NULL, signal TEXT NOT NULL, service TEXT NOT NULL, trace_id TEXT NOT NULL, payload TEXT NOT NULL, PRIMARY KEY(tenant,id))`,
		`CREATE INDEX IF NOT EXISTS platform_events_query ON platform_events(tenant,time_ms,id)`,
		`CREATE TABLE IF NOT EXISTS platform_resources (tenant TEXT NOT NULL, kind TEXT NOT NULL, id TEXT NOT NULL, payload TEXT NOT NULL, updated BIGINT NOT NULL, PRIMARY KEY(tenant,kind,id))`,
		`CREATE TABLE IF NOT EXISTS platform_audit (id TEXT PRIMARY KEY, tenant TEXT NOT NULL, subject TEXT NOT NULL, action TEXT NOT NULL, target TEXT NOT NULL, time_ms BIGINT NOT NULL)`,
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
func (s *Store) Seed(ctx context.Context, token, tenant string) error {
	if len(token) < 24 || !strings.HasPrefix(token, "sk_") || tenant == "" {
		return errors.New("bootstrap key must start sk_, contain at least 24 characters, and have a tenant")
	}
	_, err := s.db.ExecContext(ctx, s.q(`INSERT INTO platform_keys(id,hash,tenant,name,role,prefix,created,expires) VALUES(?,?,?,?,?,?,?,?) ON CONFLICT(hash) DO NOTHING`), randomID(), digest(token), tenant, "Bootstrap", "admin", token[:8], time.Now().UnixMilli(), int64(0))
	return err
}
func (s *Store) Authenticate(ctx context.Context, token string) (Principal, error) {
	var p Principal
	var id string
	err := s.db.QueryRowContext(ctx, s.q(`SELECT tenant,id,role FROM platform_keys WHERE hash=? AND revoked=0 AND (expires=0 OR expires>?)`), digest(token), time.Now().UnixMilli()).Scan(&p.Tenant, &id, &p.Role)
	p.Subject = "key:" + id
	return p, err
}
func (s *Store) Keys(ctx context.Context, p Principal) ([]Key, error) {
	rows, err := s.db.QueryContext(ctx, s.q(`SELECT id,name,role,prefix,created,expires FROM platform_keys WHERE tenant=? AND revoked=0 ORDER BY created DESC LIMIT 1000`), p.Tenant)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Key{}
	for rows.Next() {
		var k Key
		if err = rows.Scan(&k.ID, &k.Name, &k.Role, &k.Prefix, &k.Created, &k.Expires); err != nil {
			return nil, err
		}
		out = append(out, k)
	}
	return out, rows.Err()
}
func (s *Store) CreateKey(ctx context.Context, p Principal, name, role string, expires int64) (Key, string, error) {
	if p.Role != "admin" || len(strings.TrimSpace(name)) == 0 || len(name) > 100 || !validRole(role) || expires <= time.Now().UnixMilli() {
		return Key{}, "", errors.New("admin role, name, valid scope and future expiry required")
	}
	token := "sk_" + randomID() + randomID()
	k := Key{randomID(), name, role, token[:8], time.Now().UnixMilli(), expires}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return k, "", err
	}
	defer tx.Rollback()
	_, err = tx.ExecContext(ctx, s.q(`INSERT INTO platform_keys(id,hash,tenant,name,role,prefix,created,expires) VALUES(?,?,?,?,?,?,?,?)`), k.ID, digest(token), p.Tenant, k.Name, k.Role, k.Prefix, k.Created, k.Expires)
	if err != nil {
		return k, "", err
	}
	_, err = tx.ExecContext(ctx, s.q(`INSERT INTO platform_audit(id,tenant,subject,action,target,time_ms) VALUES(?,?,?,?,?,?)`), randomID(), p.Tenant, p.Subject, "key.create", k.ID, k.Created)
	if err != nil {
		return k, "", err
	}
	return k, token, tx.Commit()
}
func (s *Store) Revoke(ctx context.Context, p Principal, id string) error {
	if p.Role != "admin" {
		return errors.New("admin required")
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	r, err := tx.ExecContext(ctx, s.q(`UPDATE platform_keys SET revoked=1 WHERE tenant=? AND id=? AND revoked=0`), p.Tenant, id)
	if err != nil {
		return err
	}
	n, _ := r.RowsAffected()
	if n == 0 {
		return sql.ErrNoRows
	}
	_, err = tx.ExecContext(ctx, s.q(`INSERT INTO platform_audit(id,tenant,subject,action,target,time_ms) VALUES(?,?,?,?,?,?)`), randomID(), p.Tenant, p.Subject, "key.revoke", id, time.Now().UnixMilli())
	if err != nil {
		return err
	}
	return tx.Commit()
}

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
