package db

import (
	"context"
	"encoding/json"
	"fmt"
)

// ExtensionStore handles CRUD for entity extension columns (ext_{namespace}).
// Extension data is stored in a separate JSONB column on the target table,
// not in the base data JSONB column.
type ExtensionStore struct {
	db         DB
	driver     DriverType
	tableName  string // target table name (module_entity)
	namespace  string // extension namespace
	columnName string // ext_{namespace}
}

// NewExtensionStore creates a store for an extension column.
//
// Parameters:
//   - db: database connection
//   - driver: SQLite or PostgreSQL
//   - tableName: target table name (e.g. "billing_invoices")
//   - namespace: extension namespace (e.g. "custext")
func NewExtensionStore(db DB, driver DriverType, tableName, namespace string) *ExtensionStore {
	return &ExtensionStore{
		db:         db,
		driver:     driver,
		tableName:  tableName,
		namespace:  namespace,
		columnName: "ext_" + namespace,
	}
}

// GetExtensionData reads the extension JSONB column for a record.
// Returns nil if the column is empty or the record doesn't have extension data.
func (s *ExtensionStore) GetExtensionData(ctx context.Context, id string) (map[string]any, error) {
	query := fmt.Sprintf(`SELECT %s FROM %s WHERE id = ?`, s.columnName, s.tableName)

	var raw string
	err := s.db.QueryRowContext(ctx, query, id).Scan(&raw)
	if err != nil {
		return nil, fmt.Errorf("extension %s get: %w", s.namespace, err)
	}

	if raw == "" || raw == "{}" {
		return make(map[string]any), nil
	}

	var data map[string]any
	if err := json.Unmarshal([]byte(raw), &data); err != nil {
		return nil, fmt.Errorf("extension %s unmarshal: %w", s.namespace, err)
	}
	return data, nil
}

// SetExtensionData writes the extension JSONB column for a record.
func (s *ExtensionStore) SetExtensionData(ctx context.Context, id string, data map[string]any) error {
	raw, err := json.Marshal(data)
	if err != nil {
		return fmt.Errorf("extension %s marshal: %w", s.namespace, err)
	}

	query := fmt.Sprintf(`UPDATE %s SET %s = ? WHERE id = ?`, s.tableName, s.columnName)
	_, err = s.db.ExecContext(ctx, query, string(raw), id)
	if err != nil {
		return fmt.Errorf("extension %s set: %w", s.namespace, err)
	}
	return nil
}

// PatchExtensionData merges the provided data into the existing extension JSONB column.
func (s *ExtensionStore) PatchExtensionData(ctx context.Context, id string, patch map[string]any) error {
	existing, err := s.GetExtensionData(ctx, id)
	if err != nil {
		return err
	}

	if existing == nil {
		existing = make(map[string]any)
	}
	for k, v := range patch {
		existing[k] = v
	}

	return s.SetExtensionData(ctx, id, existing)
}

// SetExtensionDataOnInsert writes extension data during the initial INSERT.
// This is used when creating the base entity and extension data simultaneously.
func (s *ExtensionStore) SetExtensionDataOnInsert(ctx context.Context, id string, data map[string]any) error {
	return s.SetExtensionData(ctx, id, data)
}

// ColumnName returns the extension column name (ext_{namespace}).
func (s *ExtensionStore) ColumnName() string {
	return s.columnName
}

// Namespace returns the extension namespace.
func (s *ExtensionStore) Namespace() string {
	return s.namespace
}
