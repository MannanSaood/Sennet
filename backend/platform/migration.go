package platform

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"time"
)

// OwnershipMapping is deliberately explicit. There is no wildcard or global owner.
type OwnershipMapping struct {
	LegacyTenant     string `json:"legacy_tenant"`
	OrganizationID   string `json:"organization_id"`
	OrganizationName string `json:"organization_name"`
	WorkspaceID      string `json:"workspace_id"`
	WorkspaceName    string `json:"workspace_name"`
}
type MigrationReport struct {
	Mapped               int `json:"mapped"`
	Quarantined          int `json:"quarantined"`
	CredentialsConverted int `json:"credentials_converted"`
	AuditConverted       int `json:"audit_converted"`
}
type ownership struct{ org, workspace string }

// MigrateLegacy is an offline control-plane migration. Existing v2 workspaces count
// as ownership; every other legacy record is removed from active tables and quarantined.
func (s *Store) MigrateLegacy(ctx context.Context, mappings []OwnershipMapping) (MigrationReport, error) {
	report := MigrationReport{}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return report, err
	}
	defer tx.Rollback()
	owned := map[string]ownership{}
	rows, err := tx.QueryContext(ctx, `SELECT id,organization_id FROM platform_workspaces`)
	if err != nil {
		return report, err
	}
	for rows.Next() {
		var workspace, org string
		if err = rows.Scan(&workspace, &org); err != nil {
			rows.Close()
			return report, err
		}
		owned[workspace] = ownership{org, workspace}
	}
	rows.Close()
	now := time.Now().UnixMilli()
	for _, m := range mappings {
		if !validName(m.LegacyTenant, 128) || !validName(m.OrganizationID, 128) || !validName(m.OrganizationName, 200) || !validName(m.WorkspaceID, 128) || !validName(m.WorkspaceName, 200) {
			return report, errors.New("every ownership mapping field is required")
		}
		if prior, ok := owned[m.LegacyTenant]; ok && (prior.org != m.OrganizationID || prior.workspace != m.WorkspaceID) {
			return report, fmt.Errorf("conflicting ownership for %s", m.LegacyTenant)
		}
		if _, err = tx.ExecContext(ctx, s.q(`INSERT INTO platform_organizations(id,name,status,created) VALUES(?,?,?,?) ON CONFLICT(id) DO NOTHING`), m.OrganizationID, m.OrganizationName, "active", now); err != nil {
			return report, err
		}
		if _, err = tx.ExecContext(ctx, s.q(`INSERT INTO platform_workspaces(id,organization_id,name,status,created) VALUES(?,?,?,?,?) ON CONFLICT(id) DO NOTHING`), m.WorkspaceID, m.OrganizationID, m.WorkspaceName, "active", now); err != nil {
			return report, err
		}
		var actualOrganization string
		if err = tx.QueryRowContext(ctx, s.q(`SELECT organization_id FROM platform_workspaces WHERE id=?`), m.WorkspaceID).Scan(&actualOrganization); err != nil {
			return report, err
		}
		if actualOrganization != m.OrganizationID {
			return report, fmt.Errorf("workspace %s already belongs to another organization", m.WorkspaceID)
		}
		owned[m.LegacyTenant] = ownership{m.OrganizationID, m.WorkspaceID}
	}
	for _, table := range []string{"platform_agents", "platform_events", "platform_resources"} {
		rows, err = tx.QueryContext(ctx, `SELECT DISTINCT tenant FROM `+table)
		if err != nil {
			return report, err
		}
		tenants := []string{}
		for rows.Next() {
			var tenant string
			if err = rows.Scan(&tenant); err != nil {
				rows.Close()
				return report, err
			}
			tenants = append(tenants, tenant)
		}
		rows.Close()
		for _, tenant := range tenants {
			owner, ok := owned[tenant]
			if ok {
				if owner.workspace != tenant {
					r, e := tx.ExecContext(ctx, s.q(`UPDATE `+table+` SET tenant=? WHERE tenant=?`), owner.workspace, tenant)
					if e != nil {
						return report, e
					}
					n, _ := r.RowsAffected()
					report.Mapped += int(n)
				}
				continue
			}
			n, e := s.quarantineTenant(ctx, tx, table, tenant)
			if e != nil {
				return report, e
			}
			report.Quarantined += n
		}
	}
	if err = s.migrateLegacyKeys(ctx, tx, owned, &report); err != nil {
		return report, err
	}
	if err = s.migrateLegacyAudit(ctx, tx, owned, &report); err != nil {
		return report, err
	}
	if err = tx.Commit(); err != nil {
		return report, err
	}
	return report, nil
}

func (s *Store) quarantineTenant(ctx context.Context, tx *sql.Tx, table, tenant string) (int, error) {
	rows, err := tx.QueryContext(ctx, s.q(`SELECT * FROM `+table+` WHERE tenant=?`), tenant)
	if err != nil {
		return 0, err
	}
	cols, err := rows.Columns()
	if err != nil {
		rows.Close()
		return 0, err
	}
	count := 0
	for rows.Next() {
		values := make([]any, len(cols))
		ptrs := make([]any, len(cols))
		for i := range values {
			ptrs[i] = &values[i]
		}
		if err = rows.Scan(ptrs...); err != nil {
			rows.Close()
			return count, err
		}
		payload := map[string]any{}
		key := tenant
		for i, col := range cols {
			v := values[i]
			if b, ok := v.([]byte); ok {
				v = string(b)
			}
			payload[col] = v
			if col == "id" {
				key = fmt.Sprint(v)
			}
		}
		if table == "platform_keys" {
			delete(payload, "hash")
		}
		raw, _ := json.Marshal(payload)
		if _, err = tx.ExecContext(ctx, s.q(`INSERT INTO platform_quarantine(id,source_table,legacy_key,reason,payload,quarantined) VALUES(?,?,?,?,?,?)`), randomID(), table, key, "missing explicit organization/workspace ownership", string(raw), time.Now().UnixMilli()); err != nil {
			rows.Close()
			return count, err
		}
		count++
	}
	if err = rows.Err(); err != nil {
		rows.Close()
		return count, err
	}
	rows.Close()
	r, err := tx.ExecContext(ctx, s.q(`DELETE FROM `+table+` WHERE tenant=?`), tenant)
	if err != nil {
		return count, err
	}
	n, _ := r.RowsAffected()
	if int(n) != count {
		return count, errors.New("quarantine count changed during migration")
	}
	return count, nil
}

func (s *Store) migrateLegacyKeys(ctx context.Context, tx *sql.Tx, owned map[string]ownership, report *MigrationReport) error {
	rows, err := tx.QueryContext(ctx, `SELECT id,hash,tenant,name,role,prefix,created,expires,revoked FROM platform_keys`)
	if err != nil {
		return err
	}
	defer rows.Close()
	type row struct {
		id, hash, tenant, name, role, prefix string
		created, expires                     int64
		revoked                              int
	}
	items := []row{}
	for rows.Next() {
		var v row
		if err = rows.Scan(&v.id, &v.hash, &v.tenant, &v.name, &v.role, &v.prefix, &v.created, &v.expires, &v.revoked); err != nil {
			return err
		}
		items = append(items, v)
	}
	if err = rows.Err(); err != nil {
		return err
	}
	rows.Close()
	for _, v := range items {
		owner, ok := owned[v.tenant]
		if !ok || !validRole(v.role) {
			payload, _ := json.Marshal(map[string]any{"id": v.id, "tenant": v.tenant, "name": v.name, "role": v.role, "prefix": v.prefix, "created": v.created, "expires": v.expires, "revoked": v.revoked})
			if _, err = tx.ExecContext(ctx, s.q(`INSERT INTO platform_quarantine(id,source_table,legacy_key,reason,payload,quarantined) VALUES(?,?,?,?,?,?)`), randomID(), "platform_keys", v.id, "missing explicit ownership or invalid legacy role", string(payload), time.Now().UnixMilli()); err != nil {
				return err
			}
			report.Quarantined++
		} else {
			subject := "legacy_integration_" + v.id
			if _, err = tx.ExecContext(ctx, s.q(`INSERT INTO platform_subjects(id,type,issuer,external_id,display_name,status,created) VALUES(?,?,?,?,?,?,?) ON CONFLICT(id) DO NOTHING`), subject, "integration", "legacy", v.id, v.name, "active", v.created); err != nil {
				return err
			}
			if _, err = tx.ExecContext(ctx, s.q(`INSERT INTO platform_role_assignments(id,organization_id,workspace_id,subject_type,subject_id,role,created) VALUES(?,?,?,?,?,?,?) ON CONFLICT(organization_id,workspace_id,subject_type,subject_id) DO NOTHING`), randomID(), owner.org, owner.workspace, "integration", subject, v.role, v.created); err != nil {
				return err
			}
			if _, err = tx.ExecContext(ctx, s.q(`INSERT INTO platform_credentials(id,type,hash,prefix,organization_id,workspace_id,subject_id,name,created,expires,revoked,rotated_from) VALUES(?,?,?,?,?,?,?,?,?,?,?,'') ON CONFLICT(hash) DO NOTHING`), "legacy_"+v.id, CredentialIntegration, v.hash, v.prefix, owner.org, owner.workspace, subject, v.name, v.created, v.expires, v.revoked); err != nil {
				return err
			}
			report.CredentialsConverted++
		}
		if _, err = tx.ExecContext(ctx, s.q(`DELETE FROM platform_keys WHERE id=?`), v.id); err != nil {
			return err
		}
	}
	return nil
}
func (s *Store) migrateLegacyAudit(ctx context.Context, tx *sql.Tx, owned map[string]ownership, report *MigrationReport) error {
	rows, err := tx.QueryContext(ctx, `SELECT id,tenant,subject,action,target,time_ms FROM platform_audit`)
	if err != nil {
		return err
	}
	defer rows.Close()
	type row struct {
		id, tenant, subject, action, target string
		at                                  int64
	}
	items := []row{}
	for rows.Next() {
		var v row
		if err = rows.Scan(&v.id, &v.tenant, &v.subject, &v.action, &v.target, &v.at); err != nil {
			return err
		}
		items = append(items, v)
	}
	if err = rows.Err(); err != nil {
		return err
	}
	rows.Close()
	for _, v := range items {
		owner, ok := owned[v.tenant]
		if !ok {
			raw, _ := json.Marshal(map[string]any{"id": v.id, "tenant": v.tenant, "subject": v.subject, "action": v.action, "target": v.target, "time_ms": v.at})
			if _, err = tx.ExecContext(ctx, s.q(`INSERT INTO platform_quarantine(id,source_table,legacy_key,reason,payload,quarantined) VALUES(?,?,?,?,?,?)`), randomID(), "platform_audit", v.id, "missing explicit organization/workspace ownership", string(raw), time.Now().UnixMilli()); err != nil {
				return err
			}
			report.Quarantined++
		} else {
			if _, err = tx.ExecContext(ctx, s.q(`INSERT INTO platform_audit_events(id,organization_id,workspace_id,subject_type,subject_id,action,target_type,target_id,time_ms,details) VALUES(?,?,?,?,?,?,?,?,?,?)`), v.id, owner.org, owner.workspace, "legacy", v.subject, v.action, "legacy", v.target, v.at, `{"migrated":true}`); err != nil {
				return err
			}
			report.AuditConverted++
		}
		if _, err = tx.ExecContext(ctx, s.q(`DELETE FROM platform_audit WHERE id=?`), v.id); err != nil {
			return err
		}
	}
	return nil
}
