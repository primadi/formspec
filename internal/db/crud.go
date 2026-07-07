package db

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/forma/forma/internal/starlark"
	"github.com/forma/forma/internal/validation"
	"github.com/forma/forma/pkg/spec"
)

// EntityStore provides CRUD operations for a single entity.
type EntityStore struct {
	db             DB
	driver         DriverType
	module         string
	entity         string
	tableName      string
	schema         string
	softDelete     bool
	fields         []spec.Field
	children       map[string]*ChildStore // child field name → store
	stateMachine   *spec.StateMachine     // optional state machine
	computedFields []spec.Field           // fields with Computed != nil
}

// NewEntityStore creates a new EntityStore for the given entity manifest.
func NewEntityStore(db DB, driver DriverType, meta spec.Metadata, entity *spec.EntitySpec) *EntityStore {
	plural := entity.Plural
	if plural == "" {
		plural = inflectPlural(meta.Name)
	}
	tableName := meta.Module + "_" + plural

	schema := DefaultSchema
	if entity.Persist != nil && entity.Persist.Category != "" {
		if s, ok := CategorySchema[entity.Persist.Category]; ok {
			schema = s
		}
	}

	softDelete := true
	if entity.Persist != nil && entity.Persist.SoftDelete != nil {
		softDelete = *entity.Persist.SoftDelete
	}

	var sm *spec.StateMachine
	if entity.StateMachine != nil {
		sm = entity.StateMachine
	}

	// Collect computed fields
	var computedFields []spec.Field
	for _, f := range entity.Fields {
		if f.Computed != nil {
			computedFields = append(computedFields, f)
		}
	}

	return &EntityStore{
		db:             db,
		driver:         driver,
		module:         meta.Module,
		entity:         meta.Name,
		tableName:      tableName,
		schema:         schema,
		softDelete:     softDelete,
		fields:         entity.Fields,
		children:       collectChildFields(db, driver, tableName, entity.Fields),
		stateMachine:   sm,
		computedFields: computedFields,
	}
}

// qualifiedTable returns the table name with schema prefix for PostgreSQL.
func (s *EntityStore) qualifiedTable() string {
	if s.driver == DriverPostgres && s.schema != "" {
		return s.schema + "." + s.tableName
	}
	return s.tableName
}

// applyDefaults sets default values for fields not present in data.
// It modifies the data map in place. Only applies defaults when:
//   - field.Default is not nil
//   - the field is not already present in data
func (s *EntityStore) applyDefaults(data map[string]any) {
	for _, f := range s.fields {
		if f.Default == nil {
			continue
		}
		if _, exists := data[f.Name]; !exists {
			data[f.Name] = f.Default
		}
	}
}

// validateRequired checks that all required fields are present in data.
func (s *EntityStore) validateRequired(data map[string]any) error {
	for _, f := range s.fields {
		if !f.Required {
			continue
		}
		val, exists := data[f.Name]
		if !exists || val == nil || val == "" {
			return fmt.Errorf("%w: %q", ErrValidationRequired, f.Name)
		}
	}
	return nil
}

// InsertParams holds the data for creating a new entity record.
type InsertParams struct {
	TenantID  string
	CreatedBy string
	Data      map[string]any
}

// Insert creates a new entity record and returns its ID.
// Children with storage:table are extracted from Data and stored in child tables.
func (s *EntityStore) Insert(ctx context.Context, params InsertParams) (string, error) {
	// Apply default values for fields not present in data
	s.applyDefaults(params.Data)

	// Validate required fields
	if err := s.validateRequired(params.Data); err != nil {
		return "", fmt.Errorf("%s insert: %w", s.entity, err)
	}

	// Validate field rules
	if err := s.validateFieldRules(params.Data); err != nil {
		return "", fmt.Errorf("%s insert: %w", s.entity, err)
	}

	// Extract children from data (table storage only)
	parentData := params.Data
	childrenData := make(map[string][]map[string]any)
	for name, cs := range s.children {
		if cs.Storage() == "table" {
			ch, pd := cs.ChildrenExtract(parentData)
			childrenData[name] = ch
			parentData = pd
		}
	}

	tbl := s.qualifiedTable()
	query := fmt.Sprintf(
		`INSERT INTO %s (tenant_id, created_by, updated_by, data) VALUES (?, ?, ?, ?) RETURNING id`,
		tbl)

	var id string
	err := s.db.QueryRowContext(ctx, query,
		params.TenantID,
		params.CreatedBy,
		params.CreatedBy,
		toJSONString(parentData),
	).Scan(&id)
	if err != nil {
		return "", fmt.Errorf("%s insert: %w", s.entity, err)
	}

	// Insert children into child tables
	for name, ch := range childrenData {
		cs := s.children[name]
		if err := cs.InsertChildren(ctx, id, ch); err != nil {
			return "", fmt.Errorf("%s insert children %s: %w", s.entity, name, err)
		}
	}

	// Write audit log
	resource := s.module + "/" + s.entity
	changesJSON := toJSONString(params.Data)
	if err := writeAuditLog(ctx, s.db, s.driver, params.TenantID, resource, id,
		string(AuditActionCreate), params.CreatedBy, changesJSON); err != nil {
		// Audit failure is non-fatal — log but don't fail the insert
		_ = err
	}

	return id, nil
}

// GetByIDParams holds the data for fetching an entity record.
type GetByIDParams struct {
	TenantID string
	ID       string
}

// GetByID fetches a single entity record by ID.
// Children with storage:table are hydrated into the Data map.
func (s *EntityStore) GetByID(ctx context.Context, params GetByIDParams) (*EntityRecord, error) {
	rec, err := s.getByIDRaw(ctx, params)
	if err != nil {
		return nil, err
	}

	// Hydrate children from child tables
	for name, cs := range s.children {
		hydrated, err := cs.Hydrate(ctx, rec.ID, rec.Data)
		if err != nil {
			return nil, fmt.Errorf("%s hydrate children %s: %w", s.entity, name, err)
		}
		rec.Data = hydrated
	}

	// Evaluate computed fields
	s.evaluateComputed(rec.Data)

	return rec, nil
}

// getByIDRaw fetches the raw parent record without hydrating children.
func (s *EntityStore) getByIDRaw(ctx context.Context, params GetByIDParams) (*EntityRecord, error) {
	tbl := s.qualifiedTable()
	query := fmt.Sprintf(
		`SELECT id, tenant_id, version, created_at, updated_at, created_by, updated_by, data FROM %s WHERE id = ? AND tenant_id = ?`,
		tbl)
	if s.softDelete {
		query += " AND deleted_at IS NULL"
	}

	return s.scanRecord(ctx, query, params.ID, params.TenantID)
}

// UpdateParams holds the data for updating an entity record.
type UpdateParams struct {
	TenantID  string
	ID        string
	Version   int // optimistic concurrency: update only if version matches
	UpdatedBy string
	Data      map[string]any
}

// Update updates an entity record with optimistic concurrency control.
// Returns the new version if successful.
// Children with storage:table are extracted from Data, stored in child tables
// (replace-all strategy), and removed from parent JSONB.
func (s *EntityStore) Update(ctx context.Context, params UpdateParams) (int, error) {
	// Validate required fields that are present in the update data
	if err := s.validateRequired(params.Data); err != nil {
		return 0, fmt.Errorf("%s update: %w", s.entity, err)
	}

	// Validate field rules
	if err := s.validateFieldRules(params.Data); err != nil {
		return 0, fmt.Errorf("%s update: %w", s.entity, err)
	}

	// Extract children from data (table storage only)
	parentData := params.Data
	childrenData := make(map[string][]map[string]any)
	for name, cs := range s.children {
		if cs.Storage() == "table" {
			ch, pd := cs.ChildrenExtract(parentData)
			childrenData[name] = ch
			parentData = pd
		}
	}

	tbl := s.qualifiedTable()

	// Check immutable fields: fetch existing record and compare values
	var existingData map[string]any
	{
		rec, err := s.getByIDRaw(ctx, GetByIDParams{TenantID: params.TenantID, ID: params.ID})
		if err != nil {
			return 0, fmt.Errorf("%s update: fetch existing: %w", s.entity, err)
		}
		existingData = rec.Data
	}
	for _, f := range s.fields {
		if !f.Immutable {
			continue
		}
		oldVal, oldExists := existingData[f.Name]
		newVal, newExists := parentData[f.Name]
		if newExists {
			if !oldExists || oldVal != newVal {
				return 0, fmt.Errorf("%s update: %w: %q", s.entity, ErrImmutableFieldChanged, f.Name)
			}
		}
	}

	// Validate state machine transition if the state field is changing
	if s.stateMachine != nil {
		if err := s.validateStateTransition(existingData, parentData); err != nil {
			return 0, fmt.Errorf("%s update: %w", s.entity, err)
		}
	}

	query := fmt.Sprintf(
		`UPDATE %s SET data = ?, version = version + 1, updated_by = ?, updated_at = %s WHERE id = ? AND tenant_id = ? AND version = ?`,
		tbl, currentTimestampExpr(s.driver))

	if s.softDelete {
		query += " AND deleted_at IS NULL"
	}

	query += " RETURNING version"

	var newVersion int
	err := s.db.QueryRowContext(ctx, query,
		toJSONString(parentData),
		params.UpdatedBy,
		params.ID,
		params.TenantID,
		params.Version,
	).Scan(&newVersion)
	if err != nil {
		if strings.Contains(err.Error(), "no rows") {
			return 0, fmt.Errorf("%s update: %w (version conflict or not found)", s.entity, ErrNotFound)
		}
		return 0, fmt.Errorf("%s update: %w", s.entity, err)
	}

	// Sync children in child tables (replace-all)
	for name, ch := range childrenData {
		cs := s.children[name]
		if err := cs.UpdateChildren(ctx, params.ID, ch); err != nil {
			return 0, fmt.Errorf("%s update children %s: %w", s.entity, name, err)
		}
	}

	// Write audit log with changes diff
	resource := s.module + "/" + s.entity
	changes := computeChanges(existingData, parentData)
	if len(changes) > 0 {
		changesJSON := serializeChangeMap(changes)
		if err := writeAuditLog(ctx, s.db, s.driver, params.TenantID, resource, params.ID,
			string(AuditActionUpdate), params.UpdatedBy, changesJSON); err != nil {
			// Audit failure is non-fatal
			_ = err
		}
	}

	return newVersion, nil
}

// SoftDelete marks an entity record as deleted.
// Children in child tables are cascade-deleted (ON DELETE CASCADE from DDL handles this
// for hard deletes; for soft deletes we explicitly remove child rows).
func (s *EntityStore) SoftDelete(ctx context.Context, tenantID, id string) error {
	if !s.softDelete {
		// Delete children first (child tables have ON DELETE CASCADE,
		// but we do it explicitly for clarity).
		for name, cs := range s.children {
			if err := cs.DeleteChildren(ctx, id); err != nil {
				return fmt.Errorf("%s delete children %s: %w", s.entity, name, err)
			}
		}

		// Hard delete
		tbl := s.qualifiedTable()
		_, err := s.db.ExecContext(ctx,
			fmt.Sprintf("DELETE FROM %s WHERE id = ? AND tenant_id = ?", tbl),
			id, tenantID)
		return err
	}

	tbl := s.qualifiedTable()
	query := fmt.Sprintf(
		`UPDATE %s SET deleted_at = %s WHERE id = ? AND tenant_id = ? AND deleted_at IS NULL`,
		tbl, currentTimestampExpr(s.driver))

	result, err := s.db.ExecContext(ctx, query, id, tenantID)
	if err != nil {
		return fmt.Errorf("%s soft delete: %w", s.entity, err)
	}
	rows, _ := result.RowsAffected()
	if rows == 0 {
		return fmt.Errorf("%s soft delete: %w", s.entity, ErrNotFound)
	}

	// Write audit log
	resource := s.module + "/" + s.entity
	if err := writeAuditLog(ctx, s.db, s.driver, tenantID, resource, id,
		string(AuditActionDelete), "system", "{}"); err != nil {
		// Audit failure is non-fatal
		_ = err
	}
	return nil
}

// ListParams holds pagination, sorting, and filtering for List queries.
type ListParams struct {
	TenantID string
	Page     int    // 1-based
	PerPage  int    // default 20
	Sort     string // field name, prefixed with - for DESC
	Filters  map[string]FilterOp
	Search   string // full-text search across data
}

// FilterOp represents a filter operation.
type FilterOp struct {
	Op    string // eq, neq, gt, gte, lt, lte, like, in, nin
	Value any
}

// ListResult holds the result of a List query.
type ListResult struct {
	Data       []EntityRecord
	Total      int
	Page       int
	PerPage    int
	TotalPages int
}

// List queries multiple entity records with pagination, sorting, and filtering.
func (s *EntityStore) List(ctx context.Context, params ListParams) (*ListResult, error) {
	if params.Page < 1 {
		params.Page = 1
	}
	if params.PerPage < 1 || params.PerPage > 100 {
		params.PerPage = 20
	}

	tbl := s.qualifiedTable()
	var whereClauses []string
	var args []any

	// Tenant isolation
	whereClauses = append(whereClauses, "tenant_id = ?")
	args = append(args, params.TenantID)

	// Soft delete filter
	if s.softDelete {
		whereClauses = append(whereClauses, "deleted_at IS NULL")
	}

	// Custom filters
	for field, filter := range params.Filters {
		col := generatedColumnName(field)
		switch filter.Op {
		case "eq":
			whereClauses = append(whereClauses, fmt.Sprintf("%s = ?", col))
			args = append(args, filter.Value)
		case "neq":
			whereClauses = append(whereClauses, fmt.Sprintf("%s != ?", col))
			args = append(args, filter.Value)
		case "gt":
			whereClauses = append(whereClauses, fmt.Sprintf("%s > ?", col))
			args = append(args, filter.Value)
		case "gte":
			whereClauses = append(whereClauses, fmt.Sprintf("%s >= ?", col))
			args = append(args, filter.Value)
		case "lt":
			whereClauses = append(whereClauses, fmt.Sprintf("%s < ?", col))
			args = append(args, filter.Value)
		case "lte":
			whereClauses = append(whereClauses, fmt.Sprintf("%s <= ?", col))
			args = append(args, filter.Value)
		case "like":
			whereClauses = append(whereClauses, fmt.Sprintf("%s LIKE ?", col))
			args = append(args, filter.Value)
		case "in":
			whereClauses = append(whereClauses, fmt.Sprintf("%s IN (?)", col)) // simplified
			args = append(args, filter.Value)
		}
	}

	// Search across data JSONB
	if params.Search != "" {
		whereClauses = append(whereClauses, "data LIKE ?")
		args = append(args, "%"+params.Search+"%")
	}

	whereStr := strings.Join(whereClauses, " AND ")

	// Count total
	var total int
	countQuery := fmt.Sprintf("SELECT COUNT(*) FROM %s WHERE %s", tbl, whereStr)
	err := s.db.QueryRowContext(ctx, countQuery, args...).Scan(&total)
	if err != nil {
		return nil, fmt.Errorf("%s list count: %w", s.entity, err)
	}

	totalPages := (total + params.PerPage - 1) / params.PerPage
	if totalPages < 1 {
		totalPages = 1
	}

	// Sorting
	orderClause := "ORDER BY created_at DESC"
	if params.Sort != "" {
		direction := "ASC"
		field := params.Sort
		if strings.HasPrefix(field, "-") {
			direction = "DESC"
			field = field[1:]
		}
		orderClause = fmt.Sprintf("ORDER BY %s %s", generatedColumnName(field), direction)
	}

	// Pagination
	offset := (params.Page - 1) * params.PerPage
	paginationClause := "LIMIT ? OFFSET ?"
	args = append(args, params.PerPage, offset)

	// Query
	query := fmt.Sprintf(
		`SELECT id, tenant_id, version, created_at, updated_at, created_by, updated_by, data FROM %s WHERE %s %s %s`,
		tbl, whereStr, orderClause, paginationClause)

	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("%s list: %w", s.entity, err)
	}
	defer rows.Close()

	var records []EntityRecord
	for rows.Next() {
		rec, err := scanEntityRecord(rows)
		if err != nil {
			return nil, fmt.Errorf("%s list scan: %w", s.entity, err)
		}
		s.evaluateComputed(rec.Data)
		records = append(records, *rec)
	}

	return &ListResult{
		Data:       records,
		Total:      total,
		Page:       params.Page,
		PerPage:    params.PerPage,
		TotalPages: totalPages,
	}, nil
}

// FindByField finds a single entity record by a specific field value.
func (s *EntityStore) FindByField(ctx context.Context, tenantID, field, value string) (*EntityRecord, error) {
	tbl := s.qualifiedTable()
	col := generatedColumnName(field)

	query := fmt.Sprintf(
		`SELECT id, tenant_id, version, created_at, updated_at, created_by, updated_by, data FROM %s WHERE %s = ? AND tenant_id = ?`,
		tbl, col)
	if s.softDelete {
		query += " AND deleted_at IS NULL"
	}
	query += " LIMIT 1"

	rec, err := s.scanRecord(ctx, query, value, tenantID)
	if err != nil {
		return nil, err
	}

	// Evaluate computed fields
	s.evaluateComputed(rec.Data)

	return rec, nil
}

// EntityRecord represents a single row from an entity table.
type EntityRecord struct {
	ID        string
	TenantID  string
	Version   int
	CreatedAt string
	UpdatedAt string
	CreatedBy string
	UpdatedBy string
	Data      map[string]any
}

// scanRecord scans a single entity record from a query.
func (s *EntityStore) scanRecord(ctx context.Context, query string, args ...any) (*EntityRecord, error) {
	row := s.db.QueryRowContext(ctx, query, args...)
	return scanEntityRecord(row)
}

// scanEntityRecord scans a row into an EntityRecord.
func scanEntityRecord(row interface {
	Scan(dest ...any) error
}) (*EntityRecord, error) {
	var id, tenantID, createdBy, updatedBy string
	var version int
	var createdAt, updatedAt string
	var dataStr string

	err := row.Scan(&id, &tenantID, &version, &createdAt, &updatedAt, &createdBy, &updatedBy, &dataStr)
	if err != nil {
		if strings.Contains(err.Error(), "no rows") {
			return nil, fmt.Errorf("%w", ErrNotFound)
		}
		return nil, fmt.Errorf("scan: %w", err)
	}

	data, err := parseJSON(dataStr)
	if err != nil {
		data = make(map[string]any)
	}

	return &EntityRecord{
		ID:        id,
		TenantID:  tenantID,
		Version:   version,
		CreatedAt: createdAt,
		UpdatedAt: updatedAt,
		CreatedBy: createdBy,
		UpdatedBy: updatedBy,
		Data:      data,
	}, nil
}

// toJSONString converts a map to a JSON string.
func toJSONString(data map[string]any) string {
	if data == nil {
		return "{}"
	}
	// Simple JSON serialization — handles basic types
	var b strings.Builder
	b.WriteByte('{')
	first := true
	for k, v := range data {
		if !first {
			b.WriteByte(',')
		}
		first = false
		b.WriteByte('"')
		b.WriteString(k)
		b.WriteString(`":`)
		writeJSONValue(&b, v)
	}
	b.WriteByte('}')
	return b.String()
}

func writeJSONValue(b *strings.Builder, v any) {
	switch val := v.(type) {
	case string:
		// Escape JSON string
		b.WriteByte('"')
		for _, c := range val {
			switch c {
			case '"':
				b.WriteString(`\"`)
			case '\\':
				b.WriteString(`\\`)
			case '\n':
				b.WriteString(`\n`)
			case '\r':
				b.WriteString(`\r`)
			case '\t':
				b.WriteString(`\t`)
			default:
				b.WriteRune(c)
			}
		}
		b.WriteByte('"')
	case float64:
		fmt.Fprintf(b, "%g", val)
	case int:
		fmt.Fprintf(b, "%d", val)
	case int64:
		fmt.Fprintf(b, "%d", val)
	case bool:
		fmt.Fprintf(b, "%t", val)
	case nil:
		b.WriteString("null")
	case map[string]any:
		b.WriteString(toJSONString(val))
	case []any:
		b.WriteByte('[')
		for i, item := range val {
			if i > 0 {
				b.WriteByte(',')
			}
			writeJSONValue(b, item)
		}
		b.WriteByte(']')
	case []map[string]any:
		b.WriteByte('[')
		for i, item := range val {
			if i > 0 {
				b.WriteByte(',')
			}
			b.WriteString(toJSONString(item))
		}
		b.WriteByte(']')
	default:
		fmt.Fprintf(b, "%v", val)
	}
}

// parseJSON parses a JSON string into a map.
func parseJSON(s string) (map[string]any, error) {
	if s == "" || s == "{}" || s == "null" {
		return make(map[string]any), nil
	}
	result := make(map[string]any)
	if err := json.Unmarshal([]byte(s), &result); err != nil {
		return nil, fmt.Errorf("parse json: %w", err)
	}
	return result, nil
}

// evaluateComputed evaluates all computed fields for a record's data.
// It modifies the data map in place by injecting computed values.
func (s *EntityStore) evaluateComputed(data map[string]any) {
	for _, f := range s.computedFields {
		if f.Computed == nil || f.Computed.Formula == "" {
			continue
		}
		val, err := starlark.EvalExpr(f.Computed.Formula, data)
		if err != nil {
			// If evaluation fails, don't set the field — leave it absent
			continue
		}
		data[f.Name] = val
	}
}

// validateStateTransition checks that a state transition is valid according to
// the entity's state machine definition.
// It only validates when the state machine field is present in both old and new data.
func (s *EntityStore) validateStateTransition(oldData, newData map[string]any) error {
	if s.stateMachine == nil {
		return nil
	}

	field := s.stateMachine.Field
	oldVal, oldExists := oldData[field]
	newVal, newExists := newData[field]

	// If state field is not being updated, skip validation
	if !newExists {
		return nil
	}

	// If this is a new record (no old state), only verify initial state
	if !oldExists {
		if s.stateMachine.Initial != "" && newVal != s.stateMachine.Initial {
			return fmt.Errorf("initial state must be %q, got %v", s.stateMachine.Initial, newVal)
		}
		return nil
	}

	oldState := fmt.Sprintf("%v", oldVal)
	newState := fmt.Sprintf("%v", newVal)

	// No state change — skip
	if oldState == newState {
		return nil
	}

	// Look for a matching transition
	for _, t := range s.stateMachine.Transitions {
		if t.From.Matches(oldState) && t.To == newState {
			// Evaluate guard if present
			if t.Guard != nil && t.Guard.Expression != "" {
				// Build env from combined old+new data (new values take precedence)
				env := make(map[string]any, len(oldData)+len(newData))
				for k, v := range oldData {
					env[k] = v
				}
				for k, v := range newData {
					env[k] = v
				}

				result, err := starlark.EvalExpr(t.Guard.Expression, env)
				if err != nil {
					msg := t.Guard.Message
					if msg == "" {
						msg = fmt.Sprintf("guard %q evaluation error", t.Guard.Expression)
					}
					return fmt.Errorf("%w: %s (from %s -> %s): %v", ErrValidationRule, msg, oldState, newState, err)
				}

				guardPassed, ok := result.(bool)
				if !ok {
					return fmt.Errorf("%w: guard %q must return bool, got %T", ErrValidationRule, t.Guard.Expression, result)
				}

				if !guardPassed {
					msg := t.Guard.Message
					if msg == "" {
						msg = fmt.Sprintf("guard %q rejected transition", t.Guard.Expression)
					}
					return fmt.Errorf("%w: %s (from %s -> %s)", ErrValidationRule, msg, oldState, newState)
				}
			}
			return nil // Valid transition
		}
	}

	// No valid transition found
	return fmt.Errorf("%w: invalid state transition from %q to %q", ErrValidationRule, oldState, newState)
}

// validateFieldRules checks all field validation rules against the data.
// Returns an error wrapping ErrValidationRule if any rule is violated.
func (s *EntityStore) validateFieldRules(data map[string]any) error {
	for _, f := range s.fields {
		val, exists := data[f.Name]
		if !exists || val == nil {
			continue
		}

		for _, rule := range f.Rules {
			if err := validateSingleRule(f.Name, val, rule); err != nil {
				return err
			}
			// Cross-field validators need access to the full data map
			if rule.Name == "after" || rule.Name == "after_field" ||
				rule.Name == "before" || rule.Name == "before_field" ||
				rule.Name == "exists" {
				if err := validation.ValidateCrossField(f.Name, val, rule, data); err != nil {
					return err
				}
			}
		}
	}
	return nil
}

// validateSingleRule checks a single validation rule against a value.
func validateSingleRule(fieldName string, val any, rule spec.ValidationRule) error {
	switch rule.Name {
	case "email":
		str, ok := val.(string)
		if !ok {
			return fmt.Errorf("%w: %q: email must be a string", ErrValidationRule, fieldName)
		}
		if !emailRegex.MatchString(str) {
			return fmt.Errorf("%w: %q: invalid email format", ErrValidationRule, fieldName)
		}

	case "pattern":
		str, ok := val.(string)
		if !ok {
			return fmt.Errorf("%w: %q: pattern requires a string value", ErrValidationRule, fieldName)
		}
		pattern, ok := rule.Value.(string)
		if !ok {
			return fmt.Errorf("%w: %q: pattern rule value must be a string regex", ErrValidationRule, fieldName)
		}
		matched, err := regexp.MatchString(pattern, str)
		if err != nil {
			return fmt.Errorf("%w: %q: invalid regex pattern %q: %w", ErrValidationRule, fieldName, pattern, err)
		}
		if !matched {
			return fmt.Errorf("%w: %q: does not match pattern %q", ErrValidationRule, fieldName, pattern)
		}

	case "min_length":
		str, ok := val.(string)
		if !ok {
			return fmt.Errorf("%w: %q: min_length requires a string value", ErrValidationRule, fieldName)
		}
		minLen := toInt(rule.Value)
		if len(str) < minLen {
			return fmt.Errorf("%w: %q: minimum length is %d, got %d", ErrValidationRule, fieldName, minLen, len(str))
		}

	case "max_length":
		str, ok := val.(string)
		if !ok {
			return fmt.Errorf("%w: %q: max_length requires a string value", ErrValidationRule, fieldName)
		}
		maxLen := toInt(rule.Value)
		if len(str) > maxLen {
			return fmt.Errorf("%w: %q: maximum length is %d, got %d", ErrValidationRule, fieldName, maxLen, len(str))
		}

	case "min":
		num := toFloat(val)
		minVal := toFloat(rule.Value)
		if num < minVal {
			return fmt.Errorf("%w: %q: minimum value is %v", ErrValidationRule, fieldName, minVal)
		}

	case "max":
		num := toFloat(val)
		maxVal := toFloat(rule.Value)
		if num > maxVal {
			return fmt.Errorf("%w: %q: maximum value is %v", ErrValidationRule, fieldName, maxVal)
		}

	case "positive":
		num := toFloat(val)
		if num <= 0 {
			return fmt.Errorf("%w: %q: must be a positive number", ErrValidationRule, fieldName)
		}

	case "url":
		str, ok := val.(string)
		if !ok {
			return fmt.Errorf("%w: %q: url must be a string", ErrValidationRule, fieldName)
		}
		if !urlRegex.MatchString(str) {
			return fmt.Errorf("%w: %q: invalid URL format", ErrValidationRule, fieldName)
		}

	case "precision":
		num := toFloat(val)
		prec := toInt(rule.Value)
		if prec < 0 {
			return fmt.Errorf("%w: %q: precision value must be non-negative", ErrValidationRule, fieldName)
		}
		// Check decimal places: shift by 10^prec and compare with truncated version
		multiplier := 1.0
		for i := 0; i < prec; i++ {
			multiplier *= 10
		}
		truncated := float64(int(num*multiplier)) / multiplier
		if num != truncated {
			return fmt.Errorf("%w: %q: maximum %d decimal places allowed", ErrValidationRule, fieldName, prec)
		}

	case "future":
		str, ok := val.(string)
		if !ok {
			return fmt.Errorf("%w: %q: future requires a datetime string", ErrValidationRule, fieldName)
		}
		t, err := parseDateTime(str)
		if err != nil {
			return fmt.Errorf("%w: %q: invalid datetime format for future check: %v", ErrValidationRule, fieldName, err)
		}
		if !t.After(timeNow()) {
			return fmt.Errorf("%w: %q: must be in the future", ErrValidationRule, fieldName)
		}

	case "past":
		str, ok := val.(string)
		if !ok {
			return fmt.Errorf("%w: %q: past requires a datetime string", ErrValidationRule, fieldName)
		}
		t, err := parseDateTime(str)
		if err != nil {
			return fmt.Errorf("%w: %q: invalid datetime format for past check: %v", ErrValidationRule, fieldName, err)
		}
		if !t.Before(timeNow()) {
			return fmt.Errorf("%w: %q: must be in the past", ErrValidationRule, fieldName)
		}

	case "min_items":
		items, ok := val.([]any)
		if !ok {
			return fmt.Errorf("%w: %q: min_items requires an array value", ErrValidationRule, fieldName)
		}
		minLen := toInt(rule.Value)
		if len(items) < minLen {
			return fmt.Errorf("%w: %q: minimum %d items required, got %d", ErrValidationRule, fieldName, minLen, len(items))
		}

	case "max_items":
		items, ok := val.([]any)
		if !ok {
			return fmt.Errorf("%w: %q: max_items requires an array value", ErrValidationRule, fieldName)
		}
		maxLen := toInt(rule.Value)
		if len(items) > maxLen {
			return fmt.Errorf("%w: %q: maximum %d items allowed, got %d", ErrValidationRule, fieldName, maxLen, len(items))
		}
	}

	return nil
}

// emailRegex validates basic email format.
var emailRegex = regexp.MustCompile(`^[a-zA-Z0-9._%+\-]+@[a-zA-Z0-9.\-]+\.[a-zA-Z]{2,}$`)

// urlRegex validates basic URL format.
var urlRegex = regexp.MustCompile(`^https?://[^\s/$.?#].[^\s]*$`)

// timeNow returns the current UTC time. Extracted for testability.
var timeNow = func() time.Time {
	return time.Now().UTC()
}

// parseDateTime attempts to parse a datetime string in ISO 8601 or similar formats.
func parseDateTime(s string) (time.Time, error) {
	formats := []string{
		time.RFC3339,
		"2006-01-02T15:04:05Z",
		"2006-01-02T15:04:05",
		"2006-01-02 15:04:05",
		"2006-01-02",
	}
	for _, f := range formats {
		if t, err := time.Parse(f, s); err == nil {
			return t, nil
		}
	}
	return time.Time{}, fmt.Errorf("cannot parse %q as datetime", s)
}

// toInt converts a value to int for validation comparisons.
func toInt(v any) int {
	switch n := v.(type) {
	case int:
		return n
	case int64:
		return int(n)
	case float64:
		return int(n)
	case string:
		// Try to parse as number string from YAML
		var f float64
		fmt.Sscanf(n, "%f", &f)
		return int(f)
	default:
		return 0
	}
}

// toFloat converts a value to float64 for validation comparisons.
func toFloat(v any) float64 {
	switch n := v.(type) {
	case float64:
		return n
	case int:
		return float64(n)
	case int64:
		return float64(n)
	case string:
		var f float64
		fmt.Sscanf(n, "%f", &f)
		return f
	default:
		return 0
	}
}

// serializeChangeMap converts a map[string]map[string]any to a JSON string.
func serializeChangeMap(changes map[string]map[string]any) string {
	if len(changes) == 0 {
		return "{}"
	}
	// Convert to a flat map for JSON serialization
	flat := make(map[string]any, len(changes))
	for k, v := range changes {
		flat[k] = v
	}
	return toJSONString(flat)
}

// computeChanges returns a map of field → {old, new} for changed values.
// Used for audit log change tracking.
func computeChanges(old, new map[string]any) map[string]map[string]any {
	changes := make(map[string]map[string]any)

	// Check for added or modified fields
	for k, newVal := range new {
		oldVal, exists := old[k]
		if !exists {
			changes[k] = map[string]any{"old": nil, "new": newVal}
		} else if !valuesEqual(oldVal, newVal) {
			changes[k] = map[string]any{"old": oldVal, "new": newVal}
		}
	}

	// Check for removed fields
	for k, oldVal := range old {
		if _, exists := new[k]; !exists {
			changes[k] = map[string]any{"old": oldVal, "new": nil}
		}
	}

	return changes
}

// valuesEqual compares two Go values for audit diff purposes.
// Handles basic types, maps, and slices.
func valuesEqual(a, b any) bool {
	switch va := a.(type) {
	case map[string]any:
		vb, ok := b.(map[string]any)
		if !ok {
			return false
		}
		return mapsEqual(va, vb)
	case []any:
		vb, ok := b.([]any)
		if !ok {
			return false
		}
		if len(va) != len(vb) {
			return false
		}
		for i := range va {
			if !valuesEqual(va[i], vb[i]) {
				return false
			}
		}
		return true
	default:
		return a == b
	}
}

// mapsEqual checks if two maps have equal key-value pairs.
func mapsEqual(a, b map[string]any) bool {
	if len(a) != len(b) {
		return false
	}
	for k, va := range a {
		vb, ok := b[k]
		if !ok || !valuesEqual(va, vb) {
			return false
		}
	}
	return true
}

// currentTimestampExpr returns the current timestamp expression for the driver.
func currentTimestampExpr(driver DriverType) string {
	if driver == DriverSQLite {
		return "(datetime('now'))"
	}
	return "now()"
}

// ErrNotFound is returned when a record is not found.
var ErrNotFound = fmt.Errorf("not found")

// ErrValidationRequired is returned when a required field is missing.
var ErrValidationRequired = fmt.Errorf("required field missing")

// ErrImmutableFieldChanged is returned when an immutable field is modified.
var ErrImmutableFieldChanged = fmt.Errorf("immutable field cannot be changed")

// ErrValidationRule is returned when a field violates a validation rule.
var ErrValidationRule = fmt.Errorf("field validation failed")
