package db

import (
	"context"
	"database/sql"
	"fmt"
	"time"
)

// EventLogRecord represents a single delivered-event log entry — the
// durable record behind an event's deliver: {channel: audit_log} contract.
// Distinct from AuditRecord (forma_audit_log), which tracks CRUD field
// diffs, not arbitrary named business events.
type EventLogRecord struct {
	ID          string `json:"id"`
	TenantID    string `json:"tenant_id"`
	EventName   string `json:"event_name"`
	Resource    string `json:"resource"` // "module/entity", e.g. "clinic/visit"
	Payload     string `json:"payload"`  // JSON
	DeliveredAt string `json:"delivered_at"`
}

// EventLogStore records delivered business events.
type EventLogStore struct {
	db     DB
	driver DriverType
}

// NewEventLogStore creates a new event log store.
func NewEventLogStore(db DB, driver DriverType) *EventLogStore {
	return &EventLogStore{db: db, driver: driver}
}

// Write records one delivered event. Best-effort — this channel carries no
// retry semantics of its own; a durable event instead flows through the
// outbox (see internal/action.DeliverEvents).
func (s *EventLogStore) Write(ctx context.Context, tenantID, eventName, resource string, payload []byte) error {
	now := time.Now().UTC().Format(time.RFC3339Nano)

	var err error
	if s.driver == DriverPostgres {
		_, err = s.db.ExecContext(ctx, `
			INSERT INTO forma_event_log (tenant_id, event_name, resource, payload, delivered_at)
			VALUES ($1, $2, $3, $4, $5)
		`, tenantID, eventName, resource, string(payload), now)
	} else {
		_, err = s.db.ExecContext(ctx, `
			INSERT INTO forma_event_log (tenant_id, event_name, resource, payload, delivered_at)
			VALUES (?, ?, ?, ?, ?)
		`, tenantID, eventName, resource, string(payload), now)
	}
	if err != nil {
		return fmt.Errorf("write event log: %w", err)
	}
	return nil
}

// ListByTenant returns event log records for a tenant, optionally filtered
// by resource ("module/entity").
func (s *EventLogStore) ListByTenant(ctx context.Context, tenantID, resource string, limit, offset int) ([]EventLogRecord, error) {
	if limit < 1 || limit > 100 {
		limit = 50
	}

	var rows *sql.Rows
	var err error

	if resource != "" {
		rows, err = s.db.QueryContext(ctx, `
			SELECT id, tenant_id, event_name, resource, payload, delivered_at
			FROM forma_event_log
			WHERE tenant_id = ? AND resource = ?
			ORDER BY delivered_at DESC
			LIMIT ? OFFSET ?
		`, tenantID, resource, limit, offset)
	} else {
		rows, err = s.db.QueryContext(ctx, `
			SELECT id, tenant_id, event_name, resource, payload, delivered_at
			FROM forma_event_log
			WHERE tenant_id = ?
			ORDER BY delivered_at DESC
			LIMIT ? OFFSET ?
		`, tenantID, limit, offset)
	}
	if err != nil {
		return nil, fmt.Errorf("event log list: %w", err)
	}
	defer rows.Close()

	var records []EventLogRecord
	for rows.Next() {
		var rec EventLogRecord
		if err := rows.Scan(&rec.ID, &rec.TenantID, &rec.EventName, &rec.Resource,
			&rec.Payload, &rec.DeliveredAt); err != nil {
			return nil, fmt.Errorf("event log scan: %w", err)
		}
		records = append(records, rec)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("event log rows: %w", err)
	}
	return records, nil
}
