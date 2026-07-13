package db

import (
	"context"
	"database/sql"
	"fmt"
	"time"
)

// AuditAction represents the type of operation being audited.
type AuditAction string

const (
	AuditActionCreate AuditAction = "create"
	AuditActionUpdate AuditAction = "update"
	AuditActionDelete AuditAction = "delete"
	AuditActionAction AuditAction = "action" // custom entity action
)

// AuditRecord represents a single audit log entry.
type AuditRecord struct {
	ID        string `json:"id"`
	WorkspaceID  string `json:"tenant_id"`
	Entity    string `json:"entity"`    // resource name (e.g. "billing/invoice")
	EntityID  string `json:"entity_id"` // PK of the entity record
	Action    string `json:"action"`    // create | update | delete | action
	Actor     string `json:"actor"`     // user or system identifier
	Changes   string `json:"changes"`   // JSON: {"field": {"old": ..., "new": ...}}
	CreatedAt string `json:"created_at"`
}

// AuditStore provides read access to the audit log.
// Writes happen via the CRUD layer (EntityStore), not directly.
type AuditStore struct {
	db     DB
	driver DriverType
}

// NewAuditStore creates a new audit store for querying audit logs.
func NewAuditStore(db DB, driver DriverType) *AuditStore {
	return &AuditStore{db: db, driver: driver}
}

// ListByEntity returns audit records for a specific entity record.
func (s *AuditStore) ListByEntity(ctx context.Context, workspaceID, entity, entityID string, limit, offset int) ([]AuditRecord, error) {
	if limit < 1 || limit > 100 {
		limit = 50
	}

	rows, err := s.db.QueryContext(ctx, `
		SELECT id, tenant_id, entity, entity_id, action, actor, changes, created_at
		FROM forma_audit_log
		WHERE tenant_id = ? AND entity = ? AND entity_id = ?
		ORDER BY created_at DESC
		LIMIT ? OFFSET ?
	`, workspaceID, entity, entityID, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("audit list: %w", err)
	}
	defer rows.Close()

	var records []AuditRecord
	for rows.Next() {
		var rec AuditRecord
		if err := rows.Scan(&rec.ID, &rec.WorkspaceID, &rec.Entity, &rec.EntityID,
			&rec.Action, &rec.Actor, &rec.Changes, &rec.CreatedAt); err != nil {
			return nil, fmt.Errorf("audit scan: %w", err)
		}
		records = append(records, rec)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("audit rows: %w", err)
	}
	return records, nil
}

// ListByWorkspace returns audit records for a workspace, optionally filtered by entity type.
func (s *AuditStore) ListByWorkspace(ctx context.Context, workspaceID, entity string, limit, offset int) ([]AuditRecord, error) {
	if limit < 1 || limit > 100 {
		limit = 50
	}

	var err error
	var rows *sql.Rows

	if entity != "" {
		rows, err = s.db.QueryContext(ctx, `
			SELECT id, tenant_id, entity, entity_id, action, actor, changes, created_at
			FROM forma_audit_log
			WHERE tenant_id = ? AND entity = ?
			ORDER BY created_at DESC
			LIMIT ? OFFSET ?
		`, workspaceID, entity, limit, offset)
	} else {
		rows, err = s.db.QueryContext(ctx, `
			SELECT id, tenant_id, entity, entity_id, action, actor, changes, created_at
			FROM forma_audit_log
			WHERE tenant_id = ?
			ORDER BY created_at DESC
			LIMIT ? OFFSET ?
		`, workspaceID, limit, offset)
	}
	if err != nil {
		return nil, fmt.Errorf("audit list workspace: %w", err)
	}
	defer rows.Close()

	var records []AuditRecord
	for rows.Next() {
		var rec AuditRecord
		if err := rows.Scan(&rec.ID, &rec.WorkspaceID, &rec.Entity, &rec.EntityID,
			&rec.Action, &rec.Actor, &rec.Changes, &rec.CreatedAt); err != nil {
			return nil, fmt.Errorf("audit scan: %w", err)
		}
		records = append(records, rec)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("audit rows: %w", err)
	}
	return records, nil
}

// writeAuditLog inserts an audit record. This is called internally by the CRUD layer.
func writeAuditLog(ctx context.Context, db DB, driver DriverType, workspaceID, entity, entityID, action, actor, changes string) error {
	now := time.Now().UTC().Format(time.RFC3339Nano)

	var err error
	if driver == DriverPostgres {
		_, err = db.ExecContext(ctx, `
			INSERT INTO forma_audit_log (tenant_id, entity, entity_id, action, actor, changes, created_at)
			VALUES ($1, $2, $3, $4, $5, $6, $7)
		`, workspaceID, entity, entityID, action, actor, changes, now)
	} else {
		_, err = db.ExecContext(ctx, `
			INSERT INTO forma_audit_log (tenant_id, entity, entity_id, action, actor, changes, created_at)
			VALUES (?, ?, ?, ?, ?, ?, ?)
		`, workspaceID, entity, entityID, action, actor, changes, now)
	}

	if err != nil {
		return fmt.Errorf("write audit log: %w", err)
	}
	return nil
}
