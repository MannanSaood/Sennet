package platform

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	_ "github.com/jackc/pgx/v5/stdlib"
	_ "modernc.org/sqlite"
	"strconv"
	"strings"
	"time"
)

const (
	CredentialSession     = "human_session"
	CredentialIntegration = "integration_key"
	CredentialEnrollment  = "collector_enrollment"
	CredentialCollector   = "collector"
)

type Principal struct {
	OrganizationID string `json:"organization_id"`
	WorkspaceID    string `json:"workspace_id"`
	Tenant         string `json:"tenant"` // Compatibility alias; always WorkspaceID.
	Subject        string `json:"subject"`
	SubjectType    string `json:"subject_type"`
	CredentialID   string `json:"credential_id"`
	CredentialType string `json:"credential_type"`
	Role           string `json:"role"`
}
type Organization struct {
	ID      string `json:"id"`
	Name    string `json:"name"`
	Status  string `json:"status"`
	Created int64  `json:"created"`
}
type Workspace struct {
	ID             string `json:"id"`
	OrganizationID string `json:"organization_id"`
	Name           string `json:"name"`
	Status         string `json:"status"`
	Created        int64  `json:"created"`
}
type Membership struct {
	ID             string `json:"id"`
	OrganizationID string `json:"organization_id"`
	HumanID        string `json:"human_id"`
	Status         string `json:"status"`
	Created        int64  `json:"created"`
}
type RoleAssignment struct {
	ID             string `json:"id"`
	OrganizationID string `json:"organization_id"`
	WorkspaceID    string `json:"workspace_id"`
	SubjectType    string `json:"subject_type"`
	SubjectID      string `json:"subject_id"`
	Role           string `json:"role"`
	Created        int64  `json:"created"`
}
type Key struct {
	ID      string `json:"id"`
	Name    string `json:"name"`
	Role    string `json:"role"`
	Type    string `json:"type"`
	Prefix  string `json:"prefix"`
	Created int64  `json:"created"`
	Expires int64  `json:"expires"`
	Revoked bool   `json:"revoked"`
}
type WorkloadIdentity struct {
	ID             string `json:"id"`
	OrganizationID string `json:"organization_id"`
	WorkspaceID    string `json:"workspace_id"`
	Name           string `json:"name"`
	Status         string `json:"status"`
	Created        int64  `json:"created"`
}
type AuditEvent struct {
	ID             string         `json:"id"`
	OrganizationID string         `json:"organization_id"`
	WorkspaceID    string         `json:"workspace_id"`
	SubjectType    string         `json:"subject_type"`
	SubjectID      string         `json:"subject_id"`
	Action         string         `json:"action"`
	TargetType     string         `json:"target_type"`
	TargetID       string         `json:"target_id"`
	Time           int64          `json:"time_ms"`
	Details        map[string]any `json:"details,omitempty"`
}
type AuditPage struct {
	Events []AuditEvent `json:"events"`
	Cursor string       `json:"cursor,omitempty"`
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
	s := &Store{db: d, postgres: pg}
	d.SetMaxOpenConns(16)
	d.SetConnMaxLifetime(5 * time.Minute)
	if !pg {
		d.SetMaxOpenConns(1)
		if _, err = d.Exec("PRAGMA journal_mode=WAL; PRAGMA busy_timeout=5000; PRAGMA synchronous=FULL; PRAGMA foreign_keys=ON;"); err != nil {
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
		`CREATE TABLE IF NOT EXISTS platform_organizations (id TEXT PRIMARY KEY,name TEXT NOT NULL,status TEXT NOT NULL,created BIGINT NOT NULL)`,
		`CREATE TABLE IF NOT EXISTS platform_workspaces (id TEXT PRIMARY KEY,organization_id TEXT NOT NULL,name TEXT NOT NULL,status TEXT NOT NULL,created BIGINT NOT NULL,UNIQUE(organization_id,name),FOREIGN KEY(organization_id) REFERENCES platform_organizations(id))`,
		`CREATE INDEX IF NOT EXISTS platform_workspaces_org ON platform_workspaces(organization_id,created)`,
		`CREATE TABLE IF NOT EXISTS platform_subjects (id TEXT PRIMARY KEY,type TEXT NOT NULL,issuer TEXT NOT NULL,external_id TEXT NOT NULL,display_name TEXT NOT NULL,status TEXT NOT NULL,created BIGINT NOT NULL,UNIQUE(type,issuer,external_id))`,
		`CREATE TABLE IF NOT EXISTS platform_memberships (id TEXT PRIMARY KEY,organization_id TEXT NOT NULL,human_id TEXT NOT NULL,status TEXT NOT NULL,created BIGINT NOT NULL,UNIQUE(organization_id,human_id),FOREIGN KEY(organization_id) REFERENCES platform_organizations(id),FOREIGN KEY(human_id) REFERENCES platform_subjects(id))`,
		`CREATE TABLE IF NOT EXISTS platform_role_assignments (id TEXT PRIMARY KEY,organization_id TEXT NOT NULL,workspace_id TEXT NOT NULL,subject_type TEXT NOT NULL,subject_id TEXT NOT NULL,role TEXT NOT NULL,created BIGINT NOT NULL,UNIQUE(organization_id,workspace_id,subject_type,subject_id),FOREIGN KEY(organization_id) REFERENCES platform_organizations(id),FOREIGN KEY(workspace_id) REFERENCES platform_workspaces(id))`,
		`CREATE INDEX IF NOT EXISTS platform_roles_subject ON platform_role_assignments(subject_type,subject_id,workspace_id)`,
		`CREATE TABLE IF NOT EXISTS platform_credentials (id TEXT PRIMARY KEY,type TEXT NOT NULL,hash TEXT UNIQUE NOT NULL,prefix TEXT NOT NULL,organization_id TEXT NOT NULL,workspace_id TEXT NOT NULL,subject_id TEXT NOT NULL,name TEXT NOT NULL,created BIGINT NOT NULL,expires BIGINT NOT NULL,revoked INTEGER NOT NULL DEFAULT 0,rotated_from TEXT NOT NULL DEFAULT '')`,
		`CREATE INDEX IF NOT EXISTS platform_credentials_scope ON platform_credentials(organization_id,workspace_id,type,created)`,
		`CREATE TABLE IF NOT EXISTS platform_enrollments (id TEXT PRIMARY KEY,organization_id TEXT NOT NULL,workspace_id TEXT NOT NULL,name TEXT NOT NULL,created BIGINT NOT NULL,expires BIGINT NOT NULL,used BIGINT NOT NULL DEFAULT 0,revoked INTEGER NOT NULL DEFAULT 0)`,
		`CREATE TABLE IF NOT EXISTS platform_workloads (id TEXT PRIMARY KEY,organization_id TEXT NOT NULL,workspace_id TEXT NOT NULL,name TEXT NOT NULL,status TEXT NOT NULL,created BIGINT NOT NULL)`,
		`CREATE INDEX IF NOT EXISTS platform_workloads_scope ON platform_workloads(organization_id,workspace_id,created)`,
		// V1 telemetry tables keep tenant as the workspace partition key.
		`CREATE TABLE IF NOT EXISTS platform_keys (id TEXT PRIMARY KEY,hash TEXT UNIQUE NOT NULL,tenant TEXT NOT NULL,name TEXT NOT NULL,role TEXT NOT NULL,prefix TEXT NOT NULL,created BIGINT NOT NULL,expires BIGINT NOT NULL,revoked INTEGER NOT NULL DEFAULT 0)`,
		`CREATE TABLE IF NOT EXISTS platform_agents (tenant TEXT NOT NULL,id TEXT NOT NULL,version TEXT NOT NULL,seen BIGINT NOT NULL,payload TEXT NOT NULL,PRIMARY KEY(tenant,id))`,
		`CREATE TABLE IF NOT EXISTS platform_events (tenant TEXT NOT NULL,id TEXT NOT NULL,time_ms BIGINT NOT NULL,signal TEXT NOT NULL,service TEXT NOT NULL,trace_id TEXT NOT NULL,payload TEXT NOT NULL,PRIMARY KEY(tenant,id))`,
		`CREATE INDEX IF NOT EXISTS platform_events_query ON platform_events(tenant,time_ms,id)`,
		`CREATE TABLE IF NOT EXISTS platform_resources (tenant TEXT NOT NULL,kind TEXT NOT NULL,id TEXT NOT NULL,payload TEXT NOT NULL,updated BIGINT NOT NULL,PRIMARY KEY(tenant,kind,id))`,
		`CREATE TABLE IF NOT EXISTS platform_audit (id TEXT PRIMARY KEY,tenant TEXT NOT NULL,subject TEXT NOT NULL,action TEXT NOT NULL,target TEXT NOT NULL,time_ms BIGINT NOT NULL)`,
		`CREATE TABLE IF NOT EXISTS platform_audit_events (id TEXT PRIMARY KEY,organization_id TEXT NOT NULL,workspace_id TEXT NOT NULL,subject_type TEXT NOT NULL,subject_id TEXT NOT NULL,action TEXT NOT NULL,target_type TEXT NOT NULL,target_id TEXT NOT NULL,time_ms BIGINT NOT NULL,details TEXT NOT NULL)`,
		`CREATE INDEX IF NOT EXISTS platform_audit_export ON platform_audit_events(organization_id,workspace_id,time_ms,id)`,
		`CREATE TABLE IF NOT EXISTS platform_replay_nonces (scope TEXT NOT NULL,nonce TEXT NOT NULL,expires BIGINT NOT NULL,PRIMARY KEY(scope,nonce))`,
		`CREATE INDEX IF NOT EXISTS platform_replay_expiry ON platform_replay_nonces(expires)`,
		`CREATE TABLE IF NOT EXISTS platform_quota_windows (quota_key TEXT NOT NULL,window_start BIGINT NOT NULL,used BIGINT NOT NULL,expires BIGINT NOT NULL,PRIMARY KEY(quota_key,window_start))`,
		`CREATE TABLE IF NOT EXISTS platform_quarantine (id TEXT PRIMARY KEY,source_table TEXT NOT NULL,legacy_key TEXT NOT NULL,reason TEXT NOT NULL,payload TEXT NOT NULL,quarantined BIGINT NOT NULL)`,
		`CREATE TABLE IF NOT EXISTS platform_monitor_state (tenant TEXT NOT NULL,monitor_id TEXT NOT NULL,state TEXT NOT NULL,observed DOUBLE PRECISION NOT NULL,window_end BIGINT NOT NULL,evaluation_key TEXT NOT NULL,updated BIGINT NOT NULL,PRIMARY KEY(tenant,monitor_id))`,
		`CREATE TABLE IF NOT EXISTS platform_monitor_evaluations (tenant TEXT NOT NULL,monitor_id TEXT NOT NULL,evaluation_key TEXT NOT NULL,state TEXT NOT NULL,observed DOUBLE PRECISION NOT NULL,window_end BIGINT NOT NULL,created BIGINT NOT NULL,PRIMARY KEY(tenant,monitor_id,evaluation_key))`,
		`CREATE TABLE IF NOT EXISTS platform_notification_outbox (id TEXT PRIMARY KEY,tenant TEXT NOT NULL,monitor_id TEXT NOT NULL,evaluation_key TEXT NOT NULL,transition TEXT NOT NULL,payload TEXT NOT NULL,created BIGINT NOT NULL,delivered BIGINT NOT NULL DEFAULT 0,UNIQUE(tenant,monitor_id,evaluation_key,transition))`,
		`CREATE INDEX IF NOT EXISTS platform_monitor_due ON platform_resources(kind,updated,tenant)`,
		`CREATE INDEX IF NOT EXISTS platform_outbox_pending ON platform_notification_outbox(delivered,created)`,
		`INSERT INTO platform_schema(version) VALUES(3) ON CONFLICT(version) DO NOTHING`,
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
	if !pg {
		for _, q := range []string{`CREATE TRIGGER IF NOT EXISTS platform_audit_events_no_update BEFORE UPDATE ON platform_audit_events BEGIN SELECT RAISE(ABORT,'audit events are immutable'); END`, `CREATE TRIGGER IF NOT EXISTS platform_audit_events_no_delete BEFORE DELETE ON platform_audit_events BEGIN SELECT RAISE(ABORT,'audit events are immutable'); END`} {
			if _, err = d.ExecContext(ctx, q); err != nil {
				d.Close()
				return nil, err
			}
		}
	} else {
		immutability := []string{
			`CREATE OR REPLACE FUNCTION sennet_immutable_audit() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN RAISE EXCEPTION 'audit events are immutable'; END $$`,
			`DO $$ BEGIN IF NOT EXISTS (SELECT 1 FROM pg_trigger WHERE tgname='platform_audit_events_no_update') THEN CREATE TRIGGER platform_audit_events_no_update BEFORE UPDATE ON platform_audit_events FOR EACH ROW EXECUTE FUNCTION sennet_immutable_audit(); END IF; IF NOT EXISTS (SELECT 1 FROM pg_trigger WHERE tgname='platform_audit_events_no_delete') THEN CREATE TRIGGER platform_audit_events_no_delete BEFORE DELETE ON platform_audit_events FOR EACH ROW EXECUTE FUNCTION sennet_immutable_audit(); END IF; END $$`,
		}
		for _, q := range immutability {
			if _, err = d.ExecContext(ctx, q); err != nil {
				d.Close()
				return nil, err
			}
		}
	}
	return s, nil
}
func (s *Store) Close() error { return s.db.Close() }
func (s *Store) q(q string) string {
	if !s.postgres {
		return q
	}
	n := 0
	return replaceParams(q, &n)
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
func newSecret(prefix string) string { return prefix + randomID() + randomID() }
func digest(v string) string         { h := sha256.Sum256([]byte(v)); return hex.EncodeToString(h[:]) }
func validRole(v string) bool {
	switch v {
	case "owner", "admin", "editor", "viewer", "reader", "ingest":
		return true
	}
	return false
}
func validName(v string, max int) bool { v = strings.TrimSpace(v); return v != "" && len(v) <= max }

func (s *Store) Seed(ctx context.Context, token, workspace string) error {
	if len(token) < 24 || !strings.HasPrefix(token, "sk_") || !validName(workspace, 128) {
		return errors.New("bootstrap key must start sk_, contain at least 24 characters, and have a workspace")
	}
	now := time.Now().UnixMilli()
	org := "org_" + digest("bootstrap:" + workspace)[:24]
	subject := "int_" + digest(token)[:24]
	credential := "cred_" + digest(token)[:24]
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	items := []struct {
		q string
		a []any
	}{
		{`INSERT INTO platform_organizations(id,name,status,created) VALUES(?,?,?,?) ON CONFLICT(id) DO NOTHING`, []any{org, workspace, "active", now}},
		{`INSERT INTO platform_workspaces(id,organization_id,name,status,created) VALUES(?,?,?,?,?) ON CONFLICT(id) DO NOTHING`, []any{workspace, org, workspace, "active", now}},
		{`INSERT INTO platform_subjects(id,type,issuer,external_id,display_name,status,created) VALUES(?,?,?,?,?,?,?) ON CONFLICT(id) DO NOTHING`, []any{subject, "integration", "local", subject, "Bootstrap", "active", now}},
		{`INSERT INTO platform_role_assignments(id,organization_id,workspace_id,subject_type,subject_id,role,created) VALUES(?,?,?,?,?,?,?) ON CONFLICT(organization_id,workspace_id,subject_type,subject_id) DO NOTHING`, []any{randomID(), org, workspace, "integration", subject, "admin", now}},
		{`INSERT INTO platform_credentials(id,type,hash,prefix,organization_id,workspace_id,subject_id,name,created,expires,revoked,rotated_from) VALUES(?,?,?,?,?,?,?,?,?,?,?,?) ON CONFLICT(hash) DO NOTHING`, []any{credential, CredentialIntegration, digest(token), token[:8], org, workspace, subject, "Bootstrap", now, int64(0), 0, ""}},
	}
	for _, v := range items {
		if _, err = tx.ExecContext(ctx, s.q(v.q), v.a...); err != nil {
			return err
		}
	}
	return tx.Commit()
}

func (s *Store) Authenticate(ctx context.Context, token string) (Principal, error) {
	var p Principal
	var expires int64
	var revoked int
	err := s.db.QueryRowContext(ctx, s.q(`SELECT c.id,c.type,c.organization_id,c.workspace_id,c.subject_id,c.expires,c.revoked FROM platform_credentials c JOIN platform_organizations o ON o.id=c.organization_id AND o.status='active' JOIN platform_workspaces w ON w.id=c.workspace_id AND w.organization_id=c.organization_id AND w.status='active' WHERE c.hash=?`), digest(token)).Scan(&p.CredentialID, &p.CredentialType, &p.OrganizationID, &p.WorkspaceID, &p.Subject, &expires, &revoked)
	if err != nil || revoked != 0 || (expires != 0 && expires <= time.Now().UnixMilli()) {
		return Principal{}, sql.ErrNoRows
	}
	p.Tenant = p.WorkspaceID
	if p.CredentialType == CredentialEnrollment {
		p.SubjectType = "enrollment"
		p.Role = "enroll"
		return p, nil
	}
	if p.CredentialType == CredentialSession {
		p.SubjectType = "human"
	} else if p.CredentialType == CredentialCollector {
		p.SubjectType = "workload"
	} else {
		p.SubjectType = "integration"
	}
	if p.SubjectType == "human" {
		var status string
		if err = s.db.QueryRowContext(ctx, s.q(`SELECT m.status FROM platform_memberships m JOIN platform_subjects s ON s.id=m.human_id AND s.type='human' AND s.status='active' WHERE m.organization_id=? AND m.human_id=?`), p.OrganizationID, p.Subject).Scan(&status); err != nil || status != "active" {
			return Principal{}, sql.ErrNoRows
		}
	} else if p.SubjectType == "integration" {
		var status string
		if err = s.db.QueryRowContext(ctx, s.q(`SELECT status FROM platform_subjects WHERE id=? AND type='integration'`), p.Subject).Scan(&status); err != nil || status != "active" {
			return Principal{}, sql.ErrNoRows
		}
	} else if p.SubjectType == "workload" {
		var status string
		if err = s.db.QueryRowContext(ctx, s.q(`SELECT status FROM platform_workloads WHERE id=? AND organization_id=? AND workspace_id=?`), p.Subject, p.OrganizationID, p.WorkspaceID).Scan(&status); err != nil || status != "active" {
			return Principal{}, sql.ErrNoRows
		}
	}
	err = s.db.QueryRowContext(ctx, s.q(`SELECT role FROM platform_role_assignments WHERE organization_id=? AND workspace_id=? AND subject_type=? AND subject_id=?`), p.OrganizationID, p.WorkspaceID, p.SubjectType, p.Subject).Scan(&p.Role)
	if err != nil || !validRole(p.Role) {
		return Principal{}, sql.ErrNoRows
	}
	return p, nil
}

// PrincipalForHuman never provisions identities or roles; an exact active mapping is required.
func (s *Store) PrincipalForHuman(ctx context.Context, issuer, externalID, workspace string) (Principal, error) {
	p := Principal{WorkspaceID: workspace, Tenant: workspace, SubjectType: "human", CredentialType: CredentialSession}
	err := s.db.QueryRowContext(ctx, s.q(`SELECT s.id,w.organization_id,r.role FROM platform_subjects s JOIN platform_workspaces w ON w.id=? JOIN platform_organizations o ON o.id=w.organization_id AND o.status='active' JOIN platform_memberships m ON m.organization_id=w.organization_id AND m.human_id=s.id AND m.status='active' JOIN platform_role_assignments r ON r.organization_id=w.organization_id AND r.workspace_id=w.id AND r.subject_type='human' AND r.subject_id=s.id WHERE s.type='human' AND s.issuer=? AND s.external_id=? AND s.status='active' AND w.status='active'`), workspace, issuer, externalID).Scan(&p.Subject, &p.OrganizationID, &p.Role)
	if err != nil || !validRole(p.Role) {
		return Principal{}, sql.ErrNoRows
	}
	p.CredentialID = issuer + ":" + externalID
	return p, nil
}

func (s *Store) CreateHumanSession(ctx context.Context, p Principal, expires int64) (Key, string, error) {
	now := time.Now().UnixMilli()
	if p.SubjectType != "human" || expires <= now || expires > now+int64((12*time.Hour)/time.Millisecond) {
		return Key{}, "", errors.New("human session expiry must be within 12 hours")
	}
	token := newSecret("ses_")
	k := Key{ID: "cred_" + randomID(), Name: "Human session", Role: p.Role, Type: CredentialSession, Prefix: token[:8], Created: now, Expires: expires}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return k, "", err
	}
	defer tx.Rollback()
	if _, err = tx.ExecContext(ctx, s.q(`INSERT INTO platform_credentials(id,type,hash,prefix,organization_id,workspace_id,subject_id,name,created,expires,revoked,rotated_from) VALUES(?,?,?,?,?,?,?,?,?,?,0,'')`), k.ID, k.Type, digest(token), k.Prefix, p.OrganizationID, p.WorkspaceID, p.Subject, k.Name, k.Created, k.Expires); err != nil {
		return k, "", err
	}
	if err = s.auditTx(ctx, tx, p, "session.create", "human_session", k.ID, map[string]any{"expires": expires}); err != nil {
		return k, "", err
	}
	if err = tx.Commit(); err != nil {
		return k, "", err
	}
	return k, token, nil
}

func (s *Store) Keys(ctx context.Context, p Principal) ([]Key, error) {
	rows, err := s.db.QueryContext(ctx, s.q(`SELECT c.id,c.name,r.role,c.type,c.prefix,c.created,c.expires,c.revoked FROM platform_credentials c JOIN platform_role_assignments r ON r.organization_id=c.organization_id AND r.workspace_id=c.workspace_id AND r.subject_id=c.subject_id WHERE c.organization_id=? AND c.workspace_id=? AND c.type=? ORDER BY c.created DESC LIMIT 1000`), p.OrganizationID, p.WorkspaceID, CredentialIntegration)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Key{}
	for rows.Next() {
		var k Key
		var revoked int
		if err = rows.Scan(&k.ID, &k.Name, &k.Role, &k.Type, &k.Prefix, &k.Created, &k.Expires, &revoked); err != nil {
			return nil, err
		}
		k.Revoked = revoked != 0
		out = append(out, k)
	}
	return out, rows.Err()
}
func (s *Store) CreateKey(ctx context.Context, p Principal, name, role string, expires int64) (Key, string, error) {
	if !validName(name, 100) || !validRole(role) || role == "owner" || expires <= time.Now().UnixMilli() {
		return Key{}, "", errors.New("valid name, delegated role and future expiry required")
	}
	token := newSecret("sk_")
	now := time.Now().UnixMilli()
	subject := "int_" + randomID()
	id := "cred_" + randomID()
	k := Key{ID: id, Name: name, Role: role, Type: CredentialIntegration, Prefix: token[:8], Created: now, Expires: expires}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return k, "", err
	}
	defer tx.Rollback()
	if _, err = tx.ExecContext(ctx, s.q(`INSERT INTO platform_subjects(id,type,issuer,external_id,display_name,status,created) VALUES(?,?,?,?,?,?,?)`), subject, "integration", "local", subject, name, "active", now); err != nil {
		return k, "", err
	}
	if _, err = tx.ExecContext(ctx, s.q(`INSERT INTO platform_role_assignments(id,organization_id,workspace_id,subject_type,subject_id,role,created) VALUES(?,?,?,?,?,?,?)`), randomID(), p.OrganizationID, p.WorkspaceID, "integration", subject, role, now); err != nil {
		return k, "", err
	}
	if _, err = tx.ExecContext(ctx, s.q(`INSERT INTO platform_credentials(id,type,hash,prefix,organization_id,workspace_id,subject_id,name,created,expires,revoked,rotated_from) VALUES(?,?,?,?,?,?,?,?,?,?,0,'')`), id, CredentialIntegration, digest(token), token[:8], p.OrganizationID, p.WorkspaceID, subject, name, now, expires); err != nil {
		return k, "", err
	}
	if err = s.auditTx(ctx, tx, p, "credential.create", "integration_key", id, map[string]any{"role": role, "expires": expires}); err != nil {
		return k, "", err
	}
	if err = tx.Commit(); err != nil {
		return k, "", err
	}
	return k, token, nil
}
func (s *Store) Revoke(ctx context.Context, p Principal, id string) error {
	return s.revokeCredential(ctx, p, id, CredentialIntegration)
}
func (s *Store) revokeCredential(ctx context.Context, p Principal, id, kind string) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	q := `UPDATE platform_credentials SET revoked=1 WHERE organization_id=? AND workspace_id=? AND id=? AND revoked=0`
	args := []any{p.OrganizationID, p.WorkspaceID, id}
	if kind != "" {
		q += ` AND type=?`
		args = append(args, kind)
	}
	r, err := tx.ExecContext(ctx, s.q(q), args...)
	if err != nil {
		return err
	}
	n, _ := r.RowsAffected()
	if n == 0 {
		return sql.ErrNoRows
	}
	if err = s.auditTx(ctx, tx, p, "credential.revoke", kind, id, nil); err != nil {
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
	_, err := s.db.ExecContext(ctx, s.q(`INSERT INTO platform_agents(tenant,id,version,seen,payload) VALUES(?,?,?,?,?) ON CONFLICT(tenant,id) DO UPDATE SET version=excluded.version,seen=excluded.seen,payload=excluded.payload`), p.WorkspaceID, a.ID, a.Version, a.Seen, string(b))
	return err
}
func (s *Store) Agents(ctx context.Context, p Principal) ([]Agent, error) {
	rows, err := s.db.QueryContext(ctx, s.q(`SELECT payload FROM platform_agents WHERE tenant=? ORDER BY seen DESC LIMIT 1000`), p.WorkspaceID)
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
	rows, err := s.db.QueryContext(ctx, s.q(`SELECT payload FROM platform_resources WHERE tenant=? AND kind=? ORDER BY updated DESC LIMIT 1000`), p.WorkspaceID, kind)
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
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err = tx.ExecContext(ctx, s.q(`INSERT INTO platform_resources(tenant,kind,id,payload,updated) VALUES(?,?,?,?,?) ON CONFLICT(tenant,kind,id) DO UPDATE SET payload=excluded.payload,updated=excluded.updated`), p.WorkspaceID, kind, id, string(b), time.Now().UnixMilli()); err != nil {
		return err
	}
	if err = s.auditTx(ctx, tx, p, "resource.write", kind, id, nil); err != nil {
		return err
	}
	return tx.Commit()
}
func (s *Store) DeleteResource(ctx context.Context, p Principal, kind, id string) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	r, err := tx.ExecContext(ctx, s.q(`DELETE FROM platform_resources WHERE tenant=? AND kind=? AND id=?`), p.WorkspaceID, kind, id)
	if err != nil {
		return err
	}
	n, _ := r.RowsAffected()
	if n == 0 {
		return sql.ErrNoRows
	}
	if err = s.auditTx(ctx, tx, p, "resource.delete", kind, id, nil); err != nil {
		return err
	}
	return tx.Commit()
}

func (s *Store) Organizations(ctx context.Context, p Principal) ([]Organization, error) {
	rows, err := s.db.QueryContext(ctx, s.q(`SELECT id,name,status,created FROM platform_organizations WHERE id=?`), p.OrganizationID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Organization{}
	for rows.Next() {
		var v Organization
		if err = rows.Scan(&v.ID, &v.Name, &v.Status, &v.Created); err != nil {
			return nil, err
		}
		out = append(out, v)
	}
	return out, rows.Err()
}
func (s *Store) Workspaces(ctx context.Context, p Principal) ([]Workspace, error) {
	rows, err := s.db.QueryContext(ctx, s.q(`SELECT id,organization_id,name,status,created FROM platform_workspaces WHERE organization_id=? ORDER BY created`), p.OrganizationID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Workspace{}
	for rows.Next() {
		var v Workspace
		if err = rows.Scan(&v.ID, &v.OrganizationID, &v.Name, &v.Status, &v.Created); err != nil {
			return nil, err
		}
		out = append(out, v)
	}
	return out, rows.Err()
}
func (s *Store) CreateWorkspace(ctx context.Context, p Principal, name string) (Workspace, error) {
	if !validName(name, 100) {
		return Workspace{}, errors.New("name required")
	}
	v := Workspace{ID: "ws_" + randomID(), OrganizationID: p.OrganizationID, Name: name, Status: "active", Created: time.Now().UnixMilli()}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return v, err
	}
	defer tx.Rollback()
	if _, err = tx.ExecContext(ctx, s.q(`INSERT INTO platform_workspaces(id,organization_id,name,status,created) VALUES(?,?,?,?,?)`), v.ID, v.OrganizationID, v.Name, v.Status, v.Created); err != nil {
		return v, err
	}
	if err = s.auditTx(ctx, tx, p, "workspace.create", "workspace", v.ID, nil); err != nil {
		return v, err
	}
	return v, tx.Commit()
}

func (s *Store) Memberships(ctx context.Context, p Principal) ([]Membership, error) {
	rows, err := s.db.QueryContext(ctx, s.q(`SELECT id,organization_id,human_id,status,created FROM platform_memberships WHERE organization_id=? ORDER BY created`), p.OrganizationID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Membership{}
	for rows.Next() {
		var v Membership
		if err = rows.Scan(&v.ID, &v.OrganizationID, &v.HumanID, &v.Status, &v.Created); err != nil {
			return nil, err
		}
		out = append(out, v)
	}
	return out, rows.Err()
}
func (s *Store) CreateMembership(ctx context.Context, p Principal, issuer, externalID, display string) (Membership, error) {
	if !validName(issuer, 100) || !validName(externalID, 200) || !validName(display, 200) {
		return Membership{}, errors.New("identity fields required")
	}
	now := time.Now().UnixMilli()
	human := "human_" + randomID()
	m := Membership{ID: "member_" + randomID(), OrganizationID: p.OrganizationID, HumanID: human, Status: "active", Created: now}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return m, err
	}
	defer tx.Rollback()
	if _, err = tx.ExecContext(ctx, s.q(`INSERT INTO platform_subjects(id,type,issuer,external_id,display_name,status,created) VALUES(?,?,?,?,?,?,?)`), human, "human", issuer, externalID, display, "active", now); err != nil {
		return m, err
	}
	if _, err = tx.ExecContext(ctx, s.q(`INSERT INTO platform_memberships(id,organization_id,human_id,status,created) VALUES(?,?,?,?,?)`), m.ID, m.OrganizationID, m.HumanID, m.Status, m.Created); err != nil {
		return m, err
	}
	if err = s.auditTx(ctx, tx, p, "membership.create", "human", human, map[string]any{"issuer": issuer}); err != nil {
		return m, err
	}
	return m, tx.Commit()
}
func (s *Store) SetMembershipStatus(ctx context.Context, p Principal, id, status string) error {
	if status != "active" && status != "revoked" {
		return errors.New("invalid status")
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	r, err := tx.ExecContext(ctx, s.q(`UPDATE platform_memberships SET status=? WHERE organization_id=? AND id=?`), status, p.OrganizationID, id)
	if err != nil {
		return err
	}
	n, _ := r.RowsAffected()
	if n == 0 {
		return sql.ErrNoRows
	}
	if err = s.auditTx(ctx, tx, p, "membership."+status, "membership", id, nil); err != nil {
		return err
	}
	return tx.Commit()
}

func (s *Store) RoleAssignments(ctx context.Context, p Principal) ([]RoleAssignment, error) {
	rows, err := s.db.QueryContext(ctx, s.q(`SELECT id,organization_id,workspace_id,subject_type,subject_id,role,created FROM platform_role_assignments WHERE organization_id=? ORDER BY created`), p.OrganizationID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []RoleAssignment{}
	for rows.Next() {
		var v RoleAssignment
		if err = rows.Scan(&v.ID, &v.OrganizationID, &v.WorkspaceID, &v.SubjectType, &v.SubjectID, &v.Role, &v.Created); err != nil {
			return nil, err
		}
		out = append(out, v)
	}
	return out, rows.Err()
}
func (s *Store) AssignRole(ctx context.Context, p Principal, workspace, subjectType, subjectID, role string) (RoleAssignment, error) {
	if !validRole(role) || role == "owner" || !validName(subjectID, 128) || (subjectType != "human" && subjectType != "integration" && subjectType != "workload") {
		return RoleAssignment{}, errors.New("invalid assignment")
	}
	var org string
	if err := s.db.QueryRowContext(ctx, s.q(`SELECT organization_id FROM platform_workspaces WHERE id=? AND status='active'`), workspace).Scan(&org); err != nil || org != p.OrganizationID {
		return RoleAssignment{}, sql.ErrNoRows
	}
	if subjectType == "human" {
		var status string
		if err := s.db.QueryRowContext(ctx, s.q(`SELECT status FROM platform_memberships WHERE organization_id=? AND human_id=?`), p.OrganizationID, subjectID).Scan(&status); err != nil || status != "active" {
			return RoleAssignment{}, sql.ErrNoRows
		}
	}
	v := RoleAssignment{ID: "role_" + randomID(), OrganizationID: p.OrganizationID, WorkspaceID: workspace, SubjectType: subjectType, SubjectID: subjectID, Role: role, Created: time.Now().UnixMilli()}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return v, err
	}
	defer tx.Rollback()
	_, err = tx.ExecContext(ctx, s.q(`INSERT INTO platform_role_assignments(id,organization_id,workspace_id,subject_type,subject_id,role,created) VALUES(?,?,?,?,?,?,?) ON CONFLICT(organization_id,workspace_id,subject_type,subject_id) DO UPDATE SET role=excluded.role`), v.ID, v.OrganizationID, v.WorkspaceID, v.SubjectType, v.SubjectID, v.Role, v.Created)
	if err != nil {
		return v, err
	}
	if err = s.auditTx(ctx, tx, p, "role.assign", "role_assignment", v.ID, map[string]any{"workspace_id": workspace, "subject_type": subjectType, "subject_id": subjectID, "role": role}); err != nil {
		return v, err
	}
	return v, tx.Commit()
}

func (s *Store) CreateEnrollment(ctx context.Context, p Principal, name string, expires int64) (Key, string, error) {
	now := time.Now().UnixMilli()
	if !validName(name, 100) || expires <= now || expires > now+int64((15*time.Minute)/time.Millisecond) {
		return Key{}, "", errors.New("enrollment expiry must be within 15 minutes")
	}
	token := newSecret("enr_")
	id := "enr_" + randomID()
	k := Key{ID: id, Name: name, Role: "enroll", Type: CredentialEnrollment, Prefix: token[:8], Created: now, Expires: expires}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return k, "", err
	}
	defer tx.Rollback()
	if _, err = tx.ExecContext(ctx, s.q(`INSERT INTO platform_enrollments(id,organization_id,workspace_id,name,created,expires,used,revoked) VALUES(?,?,?,?,?,?,0,0)`), id, p.OrganizationID, p.WorkspaceID, name, now, expires); err != nil {
		return k, "", err
	}
	if _, err = tx.ExecContext(ctx, s.q(`INSERT INTO platform_credentials(id,type,hash,prefix,organization_id,workspace_id,subject_id,name,created,expires,revoked,rotated_from) VALUES(?,?,?,?,?,?,?,?,?,?,0,'')`), "cred_"+randomID(), CredentialEnrollment, digest(token), token[:8], p.OrganizationID, p.WorkspaceID, id, name, now, expires); err != nil {
		return k, "", err
	}
	if err = s.auditTx(ctx, tx, p, "collector.enrollment.create", "enrollment", id, map[string]any{"expires": expires}); err != nil {
		return k, "", err
	}
	if err = tx.Commit(); err != nil {
		return k, "", err
	}
	return k, token, nil
}
func (s *Store) EnrollCollector(ctx context.Context, p Principal, name string) (WorkloadIdentity, Key, string, error) {
	if p.CredentialType != CredentialEnrollment || !validName(name, 100) {
		return WorkloadIdentity{}, Key{}, "", errors.New("valid enrollment required")
	}
	now := time.Now().UnixMilli()
	w := WorkloadIdentity{ID: "workload_" + randomID(), OrganizationID: p.OrganizationID, WorkspaceID: p.WorkspaceID, Name: name, Status: "active", Created: now}
	token := newSecret("col_")
	k := Key{ID: "cred_" + randomID(), Name: name, Role: "ingest", Type: CredentialCollector, Prefix: token[:8], Created: now, Expires: now + int64((15*time.Minute)/time.Millisecond)}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return w, k, "", err
	}
	defer tx.Rollback()
	r, err := tx.ExecContext(ctx, s.q(`UPDATE platform_enrollments SET used=? WHERE id=? AND organization_id=? AND workspace_id=? AND used=0 AND revoked=0 AND expires>?`), now, p.Subject, p.OrganizationID, p.WorkspaceID, now)
	if err != nil {
		return w, k, "", err
	}
	n, _ := r.RowsAffected()
	if n == 0 {
		return w, k, "", errors.New("enrollment used, revoked, or expired")
	}
	if _, err = tx.ExecContext(ctx, s.q(`UPDATE platform_credentials SET revoked=1 WHERE id=?`), p.CredentialID); err != nil {
		return w, k, "", err
	}
	if _, err = tx.ExecContext(ctx, s.q(`INSERT INTO platform_workloads(id,organization_id,workspace_id,name,status,created) VALUES(?,?,?,?,?,?)`), w.ID, w.OrganizationID, w.WorkspaceID, w.Name, w.Status, w.Created); err != nil {
		return w, k, "", err
	}
	if _, err = tx.ExecContext(ctx, s.q(`INSERT INTO platform_role_assignments(id,organization_id,workspace_id,subject_type,subject_id,role,created) VALUES(?,?,?,?,?,?,?)`), randomID(), w.OrganizationID, w.WorkspaceID, "workload", w.ID, "ingest", now); err != nil {
		return w, k, "", err
	}
	if _, err = tx.ExecContext(ctx, s.q(`INSERT INTO platform_credentials(id,type,hash,prefix,organization_id,workspace_id,subject_id,name,created,expires,revoked,rotated_from) VALUES(?,?,?,?,?,?,?,?,?,?,0,'')`), k.ID, k.Type, digest(token), k.Prefix, w.OrganizationID, w.WorkspaceID, w.ID, k.Name, k.Created, k.Expires); err != nil {
		return w, k, "", err
	}
	actor := p
	actor.SubjectType = "enrollment"
	if err = s.auditTx(ctx, tx, actor, "collector.enroll", "workload", w.ID, map[string]any{"credential_id": k.ID, "expires": k.Expires}); err != nil {
		return w, k, "", err
	}
	if err = tx.Commit(); err != nil {
		return w, k, "", err
	}
	return w, k, token, nil
}
func (s *Store) RotateCollector(ctx context.Context, p Principal, credential string) (Key, string, error) {
	now := time.Now().UnixMilli()
	if p.CredentialType == CredentialCollector {
		credential = p.CredentialID
	}
	var subject, name string
	err := s.db.QueryRowContext(ctx, s.q(`SELECT subject_id,name FROM platform_credentials WHERE id=? AND organization_id=? AND workspace_id=? AND type=? AND revoked=0 AND expires>?`), credential, p.OrganizationID, p.WorkspaceID, CredentialCollector, now).Scan(&subject, &name)
	if err != nil {
		return Key{}, "", sql.ErrNoRows
	}
	token := newSecret("col_")
	k := Key{ID: "cred_" + randomID(), Name: name, Role: "ingest", Type: CredentialCollector, Prefix: token[:8], Created: now, Expires: now + int64((15*time.Minute)/time.Millisecond)}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return k, "", err
	}
	defer tx.Rollback()
	r, err := tx.ExecContext(ctx, s.q(`UPDATE platform_credentials SET revoked=1 WHERE id=? AND revoked=0`), credential)
	if err != nil {
		return k, "", err
	}
	n, _ := r.RowsAffected()
	if n == 0 {
		return k, "", sql.ErrNoRows
	}
	if _, err = tx.ExecContext(ctx, s.q(`INSERT INTO platform_credentials(id,type,hash,prefix,organization_id,workspace_id,subject_id,name,created,expires,revoked,rotated_from) VALUES(?,?,?,?,?,?,?,?,?,?,0,?)`), k.ID, k.Type, digest(token), k.Prefix, p.OrganizationID, p.WorkspaceID, subject, k.Name, k.Created, k.Expires, credential); err != nil {
		return k, "", err
	}
	if err = s.auditTx(ctx, tx, p, "collector.credential.rotate", "collector_credential", k.ID, map[string]any{"rotated_from": credential, "expires": k.Expires}); err != nil {
		return k, "", err
	}
	if err = tx.Commit(); err != nil {
		return k, "", err
	}
	return k, token, nil
}
func (s *Store) Workloads(ctx context.Context, p Principal) ([]WorkloadIdentity, error) {
	rows, err := s.db.QueryContext(ctx, s.q(`SELECT id,organization_id,workspace_id,name,status,created FROM platform_workloads WHERE organization_id=? AND workspace_id=? ORDER BY created`), p.OrganizationID, p.WorkspaceID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []WorkloadIdentity{}
	for rows.Next() {
		var v WorkloadIdentity
		if err = rows.Scan(&v.ID, &v.OrganizationID, &v.WorkspaceID, &v.Name, &v.Status, &v.Created); err != nil {
			return nil, err
		}
		out = append(out, v)
	}
	return out, rows.Err()
}
func (s *Store) CredentialKeys(ctx context.Context, p Principal, kind string) ([]Key, error) {
	query := `SELECT id,name,type,prefix,created,expires,revoked FROM platform_credentials WHERE organization_id=? AND workspace_id=? AND type=? ORDER BY created DESC LIMIT 1000`
	if kind == CredentialEnrollment {
		query = `SELECT e.id,e.name,c.type,c.prefix,e.created,e.expires,CASE WHEN e.revoked=1 OR e.used<>0 THEN 1 ELSE 0 END FROM platform_enrollments e JOIN platform_credentials c ON c.subject_id=e.id AND c.type=? WHERE e.organization_id=? AND e.workspace_id=? ORDER BY e.created DESC LIMIT 1000`
	}
	args := []any{p.OrganizationID, p.WorkspaceID, kind}
	if kind == CredentialEnrollment {
		args = []any{kind, p.OrganizationID, p.WorkspaceID}
	}
	rows, err := s.db.QueryContext(ctx, s.q(query), args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Key{}
	for rows.Next() {
		var k Key
		var revoked int
		if err = rows.Scan(&k.ID, &k.Name, &k.Type, &k.Prefix, &k.Created, &k.Expires, &revoked); err != nil {
			return nil, err
		}
		k.Revoked = revoked != 0
		if kind == CredentialCollector {
			k.Role = "ingest"
		} else if kind == CredentialEnrollment {
			k.Role = "enroll"
		}
		out = append(out, k)
	}
	return out, rows.Err()
}
func (s *Store) RevokeEnrollment(ctx context.Context, p Principal, id string) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	r, err := tx.ExecContext(ctx, s.q(`UPDATE platform_enrollments SET revoked=1 WHERE id=? AND organization_id=? AND workspace_id=? AND revoked=0 AND used=0`), id, p.OrganizationID, p.WorkspaceID)
	if err != nil {
		return err
	}
	n, _ := r.RowsAffected()
	if n == 0 {
		return sql.ErrNoRows
	}
	if _, err = tx.ExecContext(ctx, s.q(`UPDATE platform_credentials SET revoked=1 WHERE subject_id=? AND type=?`), id, CredentialEnrollment); err != nil {
		return err
	}
	if err = s.auditTx(ctx, tx, p, "collector.enrollment.revoke", "enrollment", id, nil); err != nil {
		return err
	}
	return tx.Commit()
}

func scrubAudit(v any) any {
	switch x := v.(type) {
	case map[string]any:
		out := map[string]any{}
		for k, v := range x {
			lower := strings.ToLower(k)
			if strings.Contains(lower, "secret") || strings.Contains(lower, "token") || strings.Contains(lower, "hash") || strings.Contains(lower, "password") || strings.Contains(lower, "authorization") || strings.Contains(lower, "key") {
				out[k] = "[REDACTED]"
			} else {
				out[k] = scrubAudit(v)
			}
		}
		return out
	case []any:
		out := make([]any, len(x))
		for i := range x {
			out[i] = scrubAudit(x[i])
		}
		return out
	case string:
		if len(x) > 512 {
			return x[:512]
		}
		return x
	default:
		return x
	}
}
func (s *Store) auditTx(ctx context.Context, tx *sql.Tx, p Principal, action, targetType, targetID string, details map[string]any) error {
	if details == nil {
		details = map[string]any{}
	}
	clean, _ := scrubAudit(details).(map[string]any)
	b, err := json.Marshal(clean)
	if err != nil {
		return err
	}
	_, err = tx.ExecContext(ctx, s.q(`INSERT INTO platform_audit_events(id,organization_id,workspace_id,subject_type,subject_id,action,target_type,target_id,time_ms,details) VALUES(?,?,?,?,?,?,?,?,?,?)`), randomID(), p.OrganizationID, p.WorkspaceID, p.SubjectType, p.Subject, action, targetType, targetID, time.Now().UnixMilli(), string(b))
	return err
}
func encodeAuditCursor(t int64, id string) string {
	return base64.RawURLEncoding.EncodeToString([]byte(strconv.FormatInt(t, 10) + ":" + id))
}
func decodeAuditCursor(v string) (int64, string, error) {
	if v == "" {
		return 0, "", nil
	}
	b, err := base64.RawURLEncoding.DecodeString(v)
	if err != nil {
		return 0, "", err
	}
	parts := strings.SplitN(string(b), ":", 2)
	if len(parts) != 2 {
		return 0, "", errors.New("invalid cursor")
	}
	n, err := strconv.ParseInt(parts[0], 10, 64)
	return n, parts[1], err
}
func (s *Store) AuditEvents(ctx context.Context, p Principal, limit int, cursor string) (AuditPage, error) {
	if limit == 0 {
		limit = 100
	}
	if limit < 1 || limit > 200 {
		return AuditPage{}, errors.New("limit must be 1-200")
	}
	t, id, err := decodeAuditCursor(cursor)
	if err != nil {
		return AuditPage{}, err
	}
	q := `SELECT id,organization_id,workspace_id,subject_type,subject_id,action,target_type,target_id,time_ms,details FROM platform_audit_events WHERE organization_id=? AND workspace_id=?`
	args := []any{p.OrganizationID, p.WorkspaceID}
	if cursor != "" {
		q += ` AND (time_ms<? OR (time_ms=? AND id<?))`
		args = append(args, t, t, id)
	}
	q += ` ORDER BY time_ms DESC,id DESC LIMIT ?`
	args = append(args, limit+1)
	rows, err := s.db.QueryContext(ctx, s.q(q), args...)
	if err != nil {
		return AuditPage{}, err
	}
	defer rows.Close()
	out := AuditPage{Events: []AuditEvent{}}
	for rows.Next() {
		var e AuditEvent
		var raw string
		if err = rows.Scan(&e.ID, &e.OrganizationID, &e.WorkspaceID, &e.SubjectType, &e.SubjectID, &e.Action, &e.TargetType, &e.TargetID, &e.Time, &raw); err != nil {
			return out, err
		}
		_ = json.Unmarshal([]byte(raw), &e.Details)
		out.Events = append(out.Events, e)
	}
	if err = rows.Err(); err != nil {
		return out, err
	}
	if len(out.Events) > limit {
		last := out.Events[limit-1]
		out.Events = out.Events[:limit]
		out.Cursor = encodeAuditCursor(last.Time, last.ID)
	}
	return out, nil
}
