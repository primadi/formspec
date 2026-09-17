package db

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
)

// SnapshotTable is the system table holding the applied shape of every entity
// (see EntitySnapshot). It is the memory the migration gate needs: without it a
// removed field and a field that never existed look identical, so a refusal
// could only be based on a guess.
const SnapshotTable = "formspec_schema_snapshot"

// snapshotQuerier is the slice of DB/Tx the snapshot IO needs. Taking it as an
// interface is what lets the snapshot be written inside the same transaction as
// the migration record it describes — a snapshot that survives a rolled-back
// migration would claim a shape the database never had.
type snapshotQuerier interface {
	QueryContext(ctx context.Context, query string, args ...any) (*sql.Rows, error)
	QueryRowContext(ctx context.Context, query string, args ...any) *sql.Row
	ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error)
}

// snapshotRow is one stored snapshot plus the identity it was stored under.
type snapshotRow struct {
	Module   string
	Entity   string
	Checksum string
	Shape    EntitySnapshot
}

// AllSnapshots lists every recorded shape, keyed by "module/entity". It is how a
// plan notices an entity that the manifests no longer declare but whose table is
// still there — the one case the migration gate can never resolve on its own.
func (r *MigrationRunner) AllSnapshots(ctx context.Context, q snapshotQuerier) (map[string]snapshotRow, error) {
	rows, err := q.QueryContext(ctx, "SELECT module, entity, checksum, shape FROM "+SnapshotTable)
	if err != nil {
		return nil, fmt.Errorf("list snapshots: %w", err)
	}
	defer rows.Close()

	out := map[string]snapshotRow{}
	for rows.Next() {
		var row snapshotRow
		var shape string
		if err := rows.Scan(&row.Module, &row.Entity, &row.Checksum, &shape); err != nil {
			return nil, fmt.Errorf("scan snapshot: %w", err)
		}
		if err := json.Unmarshal([]byte(shape), &row.Shape); err != nil {
			// A snapshot nobody can read must not be skipped quietly: skipping it
			// would silently disable the gate for that entity, which is exactly
			// the failure mode this table exists to prevent.
			return nil, fmt.Errorf("decode snapshot %s/%s: %w", row.Module, row.Entity, err)
		}
		out[row.Module+"/"+row.Entity] = row
	}
	return out, rows.Err()
}

// LoadSnapshot returns the applied shape of one entity, the checksum it was
// applied with, and whether it was found.
func (r *MigrationRunner) LoadSnapshot(ctx context.Context, q snapshotQuerier, module, entity string) (*EntitySnapshot, string, bool, error) {
	var shape, checksum string
	err := q.QueryRowContext(ctx,
		"SELECT shape, checksum FROM "+SnapshotTable+" WHERE module = ? AND entity = ?",
		module, entity).Scan(&shape, &checksum)
	if err == sql.ErrNoRows {
		return nil, "", false, nil
	}
	if err != nil {
		return nil, "", false, fmt.Errorf("load snapshot %s/%s: %w", module, entity, err)
	}
	var snap EntitySnapshot
	if err := json.Unmarshal([]byte(shape), &snap); err != nil {
		return nil, "", false, fmt.Errorf("load snapshot %s/%s: decode shape: %w", module, entity, err)
	}
	return &snap, checksum, true, nil
}

// SaveSnapshot records the shape an entity was just migrated to. Called inside
// the migration transaction (4.2.3).
func (r *MigrationRunner) SaveSnapshot(ctx context.Context, q snapshotQuerier, module, entity, checksum string, snap *EntitySnapshot) error {
	shape, err := json.Marshal(snap)
	if err != nil {
		return fmt.Errorf("save snapshot %s/%s: encode shape: %w", module, entity, err)
	}
	if _, err := q.ExecContext(ctx,
		"DELETE FROM "+SnapshotTable+" WHERE module = ? AND entity = ?", module, entity); err != nil {
		return fmt.Errorf("save snapshot %s/%s: clear: %w", module, entity, err)
	}
	if _, err := q.ExecContext(ctx,
		"INSERT INTO "+SnapshotTable+" (module, entity, driver, checksum, shape) VALUES (?, ?, ?, ?, ?)",
		module, entity, string(r.driver), checksum, string(shape)); err != nil {
		return fmt.Errorf("save snapshot %s/%s: insert: %w", module, entity, err)
	}
	return nil
}

// ForgetSnapshot drops an entity's snapshot. Used when the table is gone and the
// manifest no longer declares the entity: the operator removed both by hand, so
// there is nothing left to diff against and keeping the entry would refuse the
// same deployment forever with no way to satisfy it.
func (r *MigrationRunner) ForgetSnapshot(ctx context.Context, q snapshotQuerier, module, entity string) error {
	if _, err := q.ExecContext(ctx,
		"DELETE FROM "+SnapshotTable+" WHERE module = ? AND entity = ?", module, entity); err != nil {
		return fmt.Errorf("forget snapshot %s/%s: %w", module, entity, err)
	}
	return nil
}
