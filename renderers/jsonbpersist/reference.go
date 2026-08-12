package db

import (
	"context"
	"fmt"
)

// ReferencingEntity describes another entity that has a belongs_to relation
// pointing to this entity. Used by CheckReferencingDocuments to discover
// which tables to query for referencing rows.
type ReferencingEntity struct {
	Module    string // owning module of the referencing entity
	Entity    string // name of the referencing entity
	FieldName string // field name on the referencing entity that stores the foreign key (e.g. "polyclinic_id")
	TableName string // fully qualified table name (with schema prefix if applicable)
}

// ReferencingEntityResolver resolves a (module, entity) pair to a list of
// all entities that reference it via belongs_to relations. Returns an empty
// slice if no references exist. Set by the entity registry via
// SetReferencingEntityResolver; when nil, CheckReferencingDocuments skips
// the check (best-effort).
type ReferencingEntityResolver func(module, entity string) ([]ReferencingEntity, error)

// CheckReferencingDocuments checks whether any other registered relation field
// points to this entity record. Used by SoftDelete and Cancel to enforce
// relation.on_delete: restrict (default), cascade, or set_null.
//
// It queries each referencing entity's table for rows whose data->>'field'
// matches the given ID. If any row exists, the delete/cancel is blocked.
//
// Returns (blockingResource, blockingID, error). If no references exist,
// blockingResource is empty.
func (s *EntityStore) CheckReferencingDocuments(ctx context.Context, database DB, workspaceID, id string) (string, string, error) {
	if s.referencingEntityResolver == nil {
		return "", "", nil // resolver not configured — skip check (best-effort)
	}

	refs, err := s.referencingEntityResolver(s.module, s.entity)
	if err != nil {
		return "", "", fmt.Errorf("resolve referencing entities: %w", err)
	}

	for _, ref := range refs {
		// Query the referencing table for rows where data->>'fieldName' = id
		tbl := ref.TableName
		fieldExpr := jsonbExtractText(s.driver, "data", ref.FieldName)

		query := fmt.Sprintf(
			`SELECT id FROM %s WHERE %s = ? AND tenant_id = ? AND deleted_at IS NULL LIMIT 1`,
			tbl, fieldExpr)

		var blockingID string
		err := database.QueryRowContext(ctx, query, id, workspaceID).Scan(&blockingID)
		if err == nil {
			// Found a referencing row — block the delete
			blockingResource := ref.Module + "/" + ref.Entity
			return blockingResource, blockingID, nil
		}
		// No rows or other error — continue checking
	}

	return "", "", nil
}

// SetReferencingEntityResolver installs a resolver that returns all entities
// referencing this entity via belongs_to relations. Must be set before any
// delete or cancel operation that needs referential integrity enforcement.
func (s *EntityStore) SetReferencingEntityResolver(fn ReferencingEntityResolver) {
	s.referencingEntityResolver = fn
}

// jsonbExtractText returns a SQL expression that extracts a text field from
// a JSONB column. PostgreSQL uses the -> operator; SQLite uses json_extract().
func jsonbExtractText(driver DriverType, column, field string) string {
	if driver == DriverPostgres {
		return fmt.Sprintf("%s->>'%s'", column, field)
	}
	return fmt.Sprintf("json_extract(%s, '$.%s')", column, field)
}

// EnforceReferenceGuard checks whether deleting or cancelling a record is
// blocked by reference constraints. Returns nil if allowed, or a LifecycleError
// with FORMSPEC.REF.DELETE_BLOCKED/FORMSPEC.REF.CANCEL_BLOCKED if blocked.
func (s *EntityStore) EnforceReferenceGuard(ctx context.Context, txdb DB, workspaceID, id, actionName string) error {
	if actionName != "delete" && actionName != "cancel" {
		return nil
	}

	blockingResource, _, err := s.CheckReferencingDocuments(ctx, txdb, workspaceID, id)
	if err != nil {
		return err
	}
	if blockingResource == "" {
		return nil // no references found
	}

	code := "FORMSPEC.REF.DELETE_BLOCKED"
	if actionName == "cancel" {
		code = "FORMSPEC.REF.CANCEL_BLOCKED"
	}

	return &LifecycleError{
		Action: actionName,
		Code:   code,
	}
}
