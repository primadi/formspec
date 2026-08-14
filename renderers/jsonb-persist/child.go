package db

import (
	"context"
	"fmt"
	"strings"

	"github.com/primadi/formspec/pkg/spec"
)

// ChildStore manages children of a single child field on a parent entity.
// Supports two storage modes:
//   - "jsonb": children live inside parent's data JSONB column
//   - "table": children live in a separate child table
type ChildStore struct {
	db          DB
	driver      DriverType
	parentTable string
	childTable  string
	field       spec.Field
	storage     string // "jsonb" or "table"
}

// NewChildStore creates a ChildStore for a child field.
func NewChildStore(db DB, driver DriverType, parentTable string, field spec.Field) *ChildStore {
	childTable := parentTable + "__" + field.Name
	storage := "jsonb"
	if field.Child != nil && field.Child.Storage != "" {
		storage = field.Child.Storage
	}
	return &ChildStore{
		db:          db,
		driver:      driver,
		parentTable: parentTable,
		childTable:  childTable,
		field:       field,
		storage:     storage,
	}
}

// Storage returns the storage mode.
func (c *ChildStore) Storage() string { return c.storage }

// withDB returns a shallow copy of c bound to a different DB — used to run
// child writes on the same transaction as the parent row mutation (see
// EntityStore.Insert/Update in crud.go, InTx in tx.go).
func (c *ChildStore) withDB(txdb DB) *ChildStore {
	cp := *c
	cp.db = txdb
	return &cp
}

// ChildTable returns the child table name (only meaningful for table storage).
func (c *ChildStore) ChildTable() string { return c.childTable }

// ChildrenExtract removes child-typed fields from parent data and returns them.
// For jsonb storage, the children stay in the parent data — this is a no-op.
// For table storage, children are removed from parent data and returned for separate storage.
func (c *ChildStore) ChildrenExtract(data map[string]any) (children []map[string]any, parentData map[string]any) {
	parentData = make(map[string]any)
	for k, v := range data {
		parentData[k] = v
	}

	if c.storage == "jsonb" {
		return nil, parentData
	}

	// Table storage: extract children from data
	raw, ok := parentData[c.field.Name]
	if !ok {
		return nil, parentData
	}

	// raw should be []any or []map[string]any
	arr, ok := raw.([]any)
	if !ok {
		return nil, parentData
	}

	for _, item := range arr {
		if m, ok := item.(map[string]any); ok {
			children = append(children, m)
		}
	}

	// Remove children from parent data (they live in child table)
	delete(parentData, c.field.Name)

	return children, parentData
}

// InsertChildren inserts child rows into the child table.
// For jsonb storage, this is a no-op (children are already in parent JSONB).
// When SequenceField is set, validates monotonic ordering (2.3.8).
func (c *ChildStore) InsertChildren(ctx context.Context, parentID string, children []map[string]any) error {
	if c.storage == "jsonb" || len(children) == 0 {
		return nil
	}

	// Validate or auto-assign sequence_field if configured (2.3.8)
	seqField := ""
	if c.field.Child != nil && c.field.Child.SequenceField != "" {
		seqField = c.field.Child.SequenceField
		// Check if client provided sequence values
		clientProvided := false
		for _, ch := range children {
			if v, ok := ch[seqField]; ok && v != nil {
				clientProvided = true
				break
			}
		}
		if clientProvided {
			// Validate monotonic ordering
			if err := validateSequenceField(children, seqField); err != nil {
				return err
			}
		} else {
			// Auto-assign sequence values starting from 1
			for i, ch := range children {
				ch[seqField] = int64(i + 1)
			}
		}
	}

	for i, child := range children {
		dataStr := toJSONString(child)
		id := NewUUIDv7()

		if seqField != "" {
			// Use client-supplied sequence value, not auto-increment
			seqVal := extractSeqValue(child, seqField, i+1)
			query := fmt.Sprintf(
				"INSERT INTO %s (id, parent_id, %s, data) VALUES (?, ?, ?, ?)",
				c.childTable, seqField)
			_, err := c.db.ExecContext(ctx, query, id, parentID, seqVal, dataStr)
			if err != nil {
				return fmt.Errorf("insert child[%d] into %s: %w", i, c.childTable, err)
			}
		} else {
			query := fmt.Sprintf(
				"INSERT INTO %s (id, parent_id, data) VALUES (?, ?, ?)",
				c.childTable)
			_, err := c.db.ExecContext(ctx, query, id, parentID, dataStr)
			if err != nil {
				return fmt.Errorf("insert child[%d] into %s: %w", i, c.childTable, err)
			}
		}
	}

	return nil
}

// extractSeqValue extracts the sequence value from child data, falling back to
// index+1 if not present (for backward compatibility with clients that omit it).
func extractSeqValue(child map[string]any, seqField string, defaultVal int) int64 {
	if v, ok := child[seqField]; ok {
		switch tv := v.(type) {
		case float64:
			return int64(tv)
		case int64:
			return tv
		case int:
			return int64(tv)
		}
	}
	return int64(defaultVal)
}

// validateSequenceField checks that the sequence_field values are monotonically
// increasing (strictly). Duplicates or non-increasing values → VALIDATION_ERROR.
func validateSequenceField(children []map[string]any, seqField string) error {
	var prev *int64
	for i, child := range children {
		raw, ok := child[seqField]
		if !ok || raw == nil {
			return &validationErr{
				field: seqField,
				msg:   fmt.Sprintf("child[%d]: sequence field %q is required", i, seqField),
			}
		}
		var curr int64
		switch v := raw.(type) {
		case float64:
			curr = int64(v)
		case int64:
			curr = v
		case int:
			curr = int64(v)
		default:
			return &validationErr{
				field: seqField,
				msg:   fmt.Sprintf("child[%d]: %q must be a number", i, seqField),
			}
		}

		if prev != nil && curr <= *prev {
			return &validationErr{
				field: seqField,
				msg:   fmt.Sprintf("child[%d]: %q value %d is not monotonically increasing (previous: %d)", i, seqField, curr, *prev),
			}
		}
		prev = &curr
	}
	return nil
}

// validationErr is a simple validation error that wraps ErrValidationRule.
type validationErr struct {
	field string
	msg   string
}

func (e *validationErr) Error() string {
	return fmt.Sprintf("%s: %s", ErrValidationRule.Error(), e.msg)
}

func (e *validationErr) Unwrap() error { return ErrValidationRule }

// GetChildren fetches all children for a parent from the child table.
// For jsonb storage, this is a no-op (children come from parent data JSONB).
func (c *ChildStore) GetChildren(ctx context.Context, parentID string) ([]map[string]any, error) {
	if c.storage == "jsonb" {
		return nil, nil
	}

	query := fmt.Sprintf(
		`SELECT id, data FROM %s WHERE parent_id = ? ORDER BY created_at ASC`,
		c.childTable)

	rows, err := c.db.QueryContext(ctx, query, parentID)
	if err != nil {
		if strings.Contains(err.Error(), "no such table") {
			return nil, nil
		}
		return nil, fmt.Errorf("get children from %s: %w", c.childTable, err)
	}
	defer rows.Close()

	var children []map[string]any
	for rows.Next() {
		var id, dataStr string
		if err := rows.Scan(&id, &dataStr); err != nil {
			return nil, fmt.Errorf("scan child from %s: %w", c.childTable, err)
		}

		data, err := parseJSON(dataStr)
		if err != nil {
			data = map[string]any{"_raw": dataStr}
		}

		// Include child row ID
		data["_child_id"] = id
		children = append(children, data)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("rows iteration %s: %w", c.childTable, err)
	}

	return children, nil
}

// UpdateChildren replaces all children for a parent.
// Strategy: delete all existing, then insert new.
// For jsonb storage, this is a no-op (updated via parent data JSONB).
func (c *ChildStore) UpdateChildren(ctx context.Context, parentID string, children []map[string]any) error {
	if c.storage == "jsonb" {
		return nil
	}

	// Delete all existing children
	_, err := c.db.ExecContext(ctx,
		fmt.Sprintf("DELETE FROM %s WHERE parent_id = ?", c.childTable),
		parentID)
	if err != nil {
		return fmt.Errorf("delete children from %s: %w", c.childTable, err)
	}

	// Insert new children
	return c.InsertChildren(ctx, parentID, children)
}

// DeleteChildren removes all children for a parent.
// For jsonb storage, this is a no-op (parent soft delete handles it).
func (c *ChildStore) DeleteChildren(ctx context.Context, parentID string) error {
	if c.storage == "jsonb" {
		return nil
	}

	_, err := c.db.ExecContext(ctx,
		fmt.Sprintf("DELETE FROM %s WHERE parent_id = ?", c.childTable),
		parentID)
	if err != nil {
		return fmt.Errorf("delete children from %s: %w", c.childTable, err)
	}
	return nil
}

// SubmitChildren propagates submit lifecycle to all child rows (2.3.9).
// Children follow parent lifecycle — when parent is submitted, all children
// transition to submitted as well. For jsonb storage this is a no-op.
func (c *ChildStore) SubmitChildren(ctx context.Context, parentID string) error {
	if c.storage == "jsonb" {
		return nil
	}
	_, err := c.db.ExecContext(ctx,
		fmt.Sprintf("UPDATE %s SET doc_status = 'submitted' WHERE parent_id = ? AND (doc_status IS NULL OR doc_status = 'draft')", c.childTable),
		parentID)
	return err
}

// CancelChildren propagates cancel lifecycle to all child rows (2.3.9).
// When parent is cancelled, all children transition to cancelled as well.
func (c *ChildStore) CancelChildren(ctx context.Context, parentID string) error {
	if c.storage == "jsonb" {
		return nil
	}
	_, err := c.db.ExecContext(ctx,
		fmt.Sprintf("UPDATE %s SET doc_status = 'cancelled' WHERE parent_id = ? AND doc_status = 'submitted'", c.childTable),
		parentID)
	return err
}

// Hydrate merges table-stored children back into a parent data map.
// For jsonb storage, this is a no-op.
// Call this after GetByID to include children in the response.
func (c *ChildStore) Hydrate(ctx context.Context, parentID string, parentData map[string]any) (map[string]any, error) {
	if c.storage == "jsonb" {
		return parentData, nil
	}

	children, err := c.GetChildren(ctx, parentID)
	if err != nil {
		return parentData, err
	}

	if len(children) > 0 {
		// Strip _child_id from children
		var clean []map[string]any
		for _, ch := range children {
			cleanChild := make(map[string]any)
			for k, v := range ch {
				if k != "_child_id" {
					cleanChild[k] = v
				}
			}
			clean = append(clean, cleanChild)
		}
		parentData[c.field.Name] = clean
	}

	return parentData, nil
}

// collectChildFields scans an entity's fields and returns the child fields,
// mapping field name → *ChildStore.
func collectChildFields(db DB, driver DriverType, tableName string, fields []spec.Field) map[string]*ChildStore {
	children := make(map[string]*ChildStore)
	for _, f := range fields {
		if f.Type == spec.FieldChild && f.Child != nil {
			children[f.Name] = NewChildStore(db, driver, tableName, f)
		}
	}
	return children
}
