package db

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"regexp"
	"strings"
	"time"

	"github.com/primadi/formspec/internal/starlark"
	"github.com/primadi/formspec/internal/validation"
	"github.com/primadi/formspec/pkg/spec"
)

// EntityStore provides CRUD operations for a single entity.
type EntityStore struct {
	db                DB
	driver            DriverType
	module            string
	entity            string
	tableName         string
	schema            string
	softDelete        bool
	fields            []spec.Field
	naturalKeyField   string                 // empty if no natural key; field name if exactly one (from spec.EntitySpec.NaturalKeyField)
	children          map[string]*ChildStore // child field name → store
	stateMachine      *spec.StateMachine     // optional state machine
	computedFields    []spec.Field           // fields with Computed != nil
	submitEnabled     bool                   // whether submit action is enabled (v0.3.0 — doc_status init)
	characteristic    spec.Characteristic    // master|transaction|reference|summary (2.3.12)
	backdatePolicy    *spec.BackdatePolicy
	forwardDatePolicy *spec.ForwardDatePolicy

	// targetTableResolver resolves a relation Resource reference
	// ("module.entity" or "entity") to a qualified table name. Used by
	// ValidateRelationTargets for cross-module relation resolution (2.2.5).
	// Defaults to naive {module}_{inflectPlural(entity)}, overridden by the
	// entity registry with the actual registered table name.
	targetTableResolver func(module, entity string) (string, error)

	// referencingEntityResolver resolves a (module, entity) pair to a list of
	// all entities that reference it via belongs_to relations. Used by
	// CheckReferencingDocuments for referential integrity enforcement on
	// delete/cancel. Set by the entity registry; when nil the check is skipped.
	referencingEntityResolver ReferencingEntityResolver

	// category is this entity's persist.category ("" = operational default).
	category string

	// targetCategoryResolver resolves a target (module, entity) pair to its
	// persist.category, used to block cross-category relation resolution
	// (4.4.2). Set by the entity registry; when nil the check is skipped.
	targetCategoryResolver func(module, entity string) string

	// extensions maps extension namespace → ext_{namespace} column name
	// (4.3.1/4.3.2). When set, extension data is merged into reads and split
	// out on writes.
	extensions map[string]string
}

// SetExtensions registers extension columns (namespace → ext_{namespace}) so
// the store merges extension data into reads and splits it out on writes
// (4.3.1/4.3.2).
func (s *EntityStore) SetExtensions(extensions map[string]string) {
	s.extensions = extensions
}

// SetTargetTableResolver installs a table-name resolver for cross-module
// relation lookups. The resolver receives a (module, entity) pair and
// returns the qualified table name. When nil (the default), the naive
// {module}_{plural} convention is used.
func (s *EntityStore) SetTargetTableResolver(fn func(module, entity string) (string, error)) {
	s.targetTableResolver = fn
}

// SetTargetCategoryResolver installs a resolver that returns a target
// entity's persist.category, used to block cross-category relation resolution
// (4.4.2).
func (s *EntityStore) SetTargetCategoryResolver(fn func(module, entity string) string) {
	s.targetCategoryResolver = fn
}

// PendingEvent is a durable event to enqueue to the outbox in the same
// transaction as the entity mutation that produced it — the caller (the
// create/update HTTP handlers) resolves the entity's declared Emits event
// against the pre-insert/pre-update data and passes it in, closing the gap
// described in docs/renderers/jsonb-persist/01-architecture.md §3 where a
// crash between mutation commit and outbox enqueue silently drops the
// event. Non-durable events are never passed here — they are delivered
// best-effort by action.DeliverEvents after the fact, as before.
type PendingEvent struct {
	Name    string
	Payload string // JSON-encoded event message (see action.BuildEventMessage)
}

// NewEntityStore creates a new EntityStore for the given entity manifest.
func NewEntityStore(db DB, driver DriverType, meta spec.Metadata, entity *spec.EntitySpec) *EntityStore {
	plural := entity.Plural
	if plural == "" {
		plural = inflectPlural(meta.Name)
	}
	tableName := sanitizeIdent(meta.Module + "_" + plural)

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

	// Determine if submit is enabled for initial doc_status
	submitEnabled := true // default: submit is enabled (document participates in lifecycle)
	for _, a := range entity.Actions {
		if a.Name == "submit" && a.Disabled {
			submitEnabled = false
			break
		}
	}

	return &EntityStore{
		db:                db,
		driver:            driver,
		module:            meta.Module,
		entity:            meta.Name,
		tableName:         tableName,
		schema:            schema,
		softDelete:        softDelete,
		fields:            entity.Fields,
		naturalKeyField:   entity.NaturalKeyField,
		children:          collectChildFields(db, driver, tableName, entity.Fields),
		stateMachine:      sm,
		computedFields:    computedFields,
		submitEnabled:     submitEnabled,
		characteristic:    entity.Characteristic,
		backdatePolicy:    entity.BackdatePolicy,
		forwardDatePolicy: entity.ForwardDatePolicy,
		category:          schema,
	}
}

// BaseDB returns the store's underlying (non-transaction-bound) DB — used
// by callers outside this package (internal/api's HandleCustomAction) to
// check TxScope.Peek against this entity's actual store identity.
func (s *EntityStore) BaseDB() DB { return s.db }

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

	// An entity with a state_machine must start in its declared initial
	// state — otherwise the very first transition looks like an invalid
	// "no old state, and new state != initial" case in
	// validateStateTransition, since Insert() never had a chance to set it.
	if s.stateMachine != nil && s.stateMachine.Initial != "" {
		field := s.stateMachine.Field
		if val, exists := data[field]; !exists || val == nil || val == "" {
			data[field] = s.stateMachine.Initial
		}
	}
}

// generateNaturalKeys fills in any field with NaturalKey: true + a
// NaturalKeyRule whose value is not already present in data (a caller may
// still supply an explicit value, e.g. migrations/imports — those are left
// untouched). Uses NaturalKeyCounter for atomic, per-workspace/resource/field
// sequence generation. database is the counter's UPSERT target — Insert
// passes the transaction-bound DB so the counter increment and the row
// insert it numbers commit together or not at all (Core Basic §2,
// jsonb-persist 04-query-and-keys.md §2).
func (s *EntityStore) generateNaturalKeys(ctx context.Context, database DB, workspaceID string, data map[string]any) error {
	for _, f := range s.fields {
		if !f.NaturalKey || f.NaturalKeyRule == nil {
			continue
		}
		if val, exists := data[f.Name]; exists && val != nil && val != "" {
			continue
		}

		rule := f.NaturalKeyRule

		// strategy: custom means the framework does NOT auto-generate this
		// key — the caller (a hook, script, or import) is responsible for
		// supplying it. Leaving it absent here means validateRequired (run
		// right after this, in Insert) is the backstop that catches a
		// custom-strategy key nobody supplied (01-core-basic.md §2:
		// "sequence | custom").
		if rule.Strategy == "custom" {
			continue
		}

		prefix := ""
		if rule.Prefix != nil {
			if rule.Prefix.Value != "" {
				prefix = rule.Prefix.Value
			} else if rule.Prefix.Default != "" {
				prefix = rule.Prefix.Default
			}
			// rule.Prefix.Config (workspace-config-driven override) is not wired
			// yet — no workspace-config store exists in this codebase.
		}

		scope := ""
		if rule.ScopeField != "" {
			if val, exists := data[rule.ScopeField]; exists && val != nil {
				scope = fmt.Sprintf("%v", val)
			}
		}

		counter := NewNaturalKeyCounter(database, s.driver)
		key, err := counter.GenerateNaturalKey(ctx, workspaceID, s.entity, f.Name, scope, rule.Reset, rule.Format, prefix)
		if err != nil {
			return fmt.Errorf("generate natural key %q: %w", f.Name, err)
		}
		data[f.Name] = key
	}
	return nil
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

// validateKnownFields checks that every key in data is a declared field
// (custom field, child relation, or reserved) or a framework-set field
// (reserved field names). Unknown fields are rejected to prevent silent
// acceptance of arbitrary data — this is a security boundary, not just a
// data-quality check.
func (s *EntityStore) validateKnownFields(data map[string]any) error {
	// Build a set of known field names: custom fields + child relations.
	known := make(map[string]bool, len(s.fields)+len(s.children)+len(spec.ReservedFieldNames)+len(s.extensions))
	for _, f := range s.fields {
		known[f.Name] = true
	}
	for name := range s.children {
		known[name] = true
	}
	for _, name := range spec.ReservedFieldNames {
		known[name] = true
	}
	// Extension namespaces (4.3.2) are legitimate write inputs — they are
	// split out into their ext_{namespace} columns, not stored in base data.
	for ns := range s.extensions {
		known[ns] = true
	}

	for key := range data {
		if !known[key] {
			return fmt.Errorf("%w: %q", ErrUnknownField, key)
		}
	}
	return nil
}

// stripEnrichedRelations removes nested relation objects that
// resolveRelations injected into a record's Data map during a read
// (e.g. Data["patient"] = {...} for field patient_id). These enriched
// aliases are read-side conveniences for the frontend — they are never
// legitimate write inputs (callers write the *_id scalar, not the nested
// object) and would otherwise be rejected by validateKnownFields as
// unknown fields when the same map is written back (script resource.save()
// re-persists the whole loaded record; PATCH merges onto the fetched Data).
//
// A key is stripped only when it is a relation alias AND its value is a
// nested object — a client sending a scalar under an unknown name is still
// rejected by validateKnownFields.
func (s *EntityStore) stripEnrichedRelations(data map[string]any) {
	if len(data) == 0 {
		return
	}
	for _, f := range s.fields {
		if f.Relation == nil || f.Relation.Type != "belongs_to" {
			continue
		}
		// Determine relation alias exactly like resolveRelations:
		// "patient_id" → "patient", else fallback to resource name.
		alias := strings.TrimSuffix(f.Name, "_id")
		if alias == f.Name {
			alias = f.Relation.Resource
		}
		if v, ok := data[alias]; ok {
			if _, isMap := v.(map[string]any); isMap {
				delete(data, alias)
			}
		}
	}
}

// InsertParams holds the data for creating a new entity record.
type InsertParams struct {
	WorkspaceID   string
	CreatedBy     string
	Data          map[string]any
	Permissions   []string       // caller's effective permissions — used for backdate/forward-date override_permission
	PendingEvents []PendingEvent // durable events to enqueue atomically with this insert (see PendingEvent)
	RequestID     string         // originating request id, recorded in the audit trail (4.7.2)
}

// Insert creates a new entity record and returns its ID. The natural-key
// counter UPSERT, the row insert, child-table inserts, and any
// PendingEvents outbox enqueue all run inside one transaction — Core Basic
// §2/§7 (jsonb-persist 04-query-and-keys.md §2, 01-architecture.md §3):
// commit all of it or none of it, closing the gap where a crash between a
// committed mutation and a later best-effort outbox write silently drops
// the event, and the gap where a natural-key counter increment survives a
// failed insert as an unrecoverable numbering hole.
// Children with storage:table are extracted from Data and stored in child tables.
func (s *EntityStore) Insert(ctx context.Context, params InsertParams) (string, error) {
	// Summary entities are permanently read-only via API (§4.1.1)
	if s.characteristic == spec.CharSummary {
		return "", fmt.Errorf("%w: summary entity %s/%s is read-only (create/update/delete disabled)",
			ErrValidationRule, s.module, s.entity)
	}

	// Apply default values for fields not present in data
	s.applyDefaults(params.Data)

	// Strip read-side relation enrichment (nested objects injected by
	// resolveRelations) before validation — a script resource.create() may
	// re-persist a fetched record that carries relation aliases.
	s.stripEnrichedRelations(params.Data)

	// Reject unknown fields before entering the transaction
	if err := s.validateKnownFields(params.Data); err != nil {
		return "", fmt.Errorf("%s insert: %w", s.entity, err)
	}

	tbl := s.qualifiedTable()
	id := NewUUIDv7()

	err := runTx(ctx, s.db, func(txdb DB) error {
		// Generate natural keys (e.g. queue_number, invoice number) for
		// fields that declare natural_key: true + natural_key_rule, before
		// required-field validation — a generated key must count as
		// present. The counter UPSERT runs on txdb: if the insert below
		// fails, the counter increment rolls back with it instead of
		// leaving a permanent numbering gap.
		if err := s.generateNaturalKeys(ctx, txdb, params.WorkspaceID, params.Data); err != nil {
			return err
		}

		// Validate required fields
		if err := s.validateRequired(params.Data); err != nil {
			return err
		}

		// Validate field rules
		if err := s.validateFieldRules(params.Data); err != nil {
			return err
		}

		// Validate relation targets (referenceability guard) — reads through
		// this Insert's own transaction (txdb), never s.db: a second,
		// independent query against s.db while this transaction already
		// holds the sole SQLite connection would deadlock instead of erroring.
		if err := s.ValidateRelationTargets(ctx, txdb, params.WorkspaceID, params.Data); err != nil {
			return err
		}

		// Validate transaction_date policy (backdate/forward-date)
		if err := s.validateTransactionDatePolicy(params.Data, params.Permissions); err != nil {
			return err
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

		// Split extension data (4.3.2): namespaced keys are written to their
		// ext_{namespace} columns, not the base data JSONB.
		extData := s.splitExtensions(parentData)

		// Set initial doc_status: draft if lifecycle active, NULL if lifecycle-free
		initialDocStatus := "NULL"
		if s.submitEnabled {
			initialDocStatus = "'draft'"
		}

		query := fmt.Sprintf(
			`INSERT INTO %s (id, tenant_id, created_by, updated_by, doc_status, data) VALUES (?, ?, ?, ?, %s, ?)`,
			tbl, initialDocStatus)

		if _, err := txdb.ExecContext(ctx, query,
			id,
			params.WorkspaceID,
			params.CreatedBy,
			params.CreatedBy,
			toJSONString(parentData),
		); err != nil {
			return fmt.Errorf("insert row: %w", err)
		}

		// Write extension data (4.3.2): each namespaced payload goes to its
		// ext_{namespace} column, isolated from the base data JSONB.
		for ns, ext := range extData {
			if len(ext) == 0 {
				continue
			}
			col, ok := s.extensions[ns]
			if !ok {
				continue
			}
			if _, err := txdb.ExecContext(ctx,
				fmt.Sprintf("UPDATE %s SET %s = ? WHERE id = ?", tbl, col),
				toJSONString(ext), id); err != nil {
				return fmt.Errorf("write extension %s: %w", ns, err)
			}
		}

		// Tree/hierarchy (4.6.1): compute and store the materialized path for
		// each tree: true self-referential relation.
		if err := s.setTreePaths(ctx, txdb, params.WorkspaceID, id, params.Data); err != nil {
			return fmt.Errorf("insert tree paths: %w", err)
		}

		// Insert children into child tables
		for name, ch := range childrenData {
			cs := s.children[name]
			if err := cs.withDB(txdb).InsertChildren(ctx, id, ch); err != nil {
				return fmt.Errorf("insert children %s: %w", name, err)
			}
		}

		resource := s.module + "/" + s.entity

		// Write audit log — best-effort, same as before: a failure here is
		// logged but does not roll back the mutation (audit isn't part of
		// the mandated atomic set: entity mutation + natural-key counter +
		// durable outbox).
		changesJSON := toJSONString(params.Data)
		if err := writeAuditLog(ctx, txdb, s.driver, params.WorkspaceID, resource, id,
			string(AuditActionCreate), params.CreatedBy, changesJSON, params.RequestID); err != nil {
			log.Printf("[WARN] audit write failed (create %s/%s): %v", resource, id, err)
		}

		// Enqueue any durable events atomically with the row that produced
		// them — a failure here DOES roll back the whole insert, since a
		// durable publisher's contract requires the mutation and its
		// outbox entry to commit together or not at all.
		for _, ev := range params.PendingEvents {
			if _, err := enqueueOutbox(ctx, txdb, params.WorkspaceID, ev.Name, resource, ev.Payload); err != nil {
				return fmt.Errorf("enqueue pending event %q: %w", ev.Name, err)
			}
		}

		return nil
	})
	if err != nil {
		return "", fmt.Errorf("%s insert: %w", s.entity, err)
	}

	return id, nil
}

// GetByIDParams holds the data for fetching an entity record.
type GetByIDParams struct {
	WorkspaceID string
	ID          string
}

// hydrateAndCompute hydrates child-table data into the record, evaluates
// computed fields, and resolves belongs_to relations.
// Shared by all lookup paths in GetByID to avoid duplication.
func (s *EntityStore) hydrateAndCompute(ctx context.Context, rec *EntityRecord, workspaceID string) (*EntityRecord, error) {
	for name, cs := range s.children {
		hydrated, err := cs.Hydrate(ctx, rec.ID, rec.Data)
		if err != nil {
			return nil, fmt.Errorf("%s hydrate children %s: %w", s.entity, name, err)
		}
		rec.Data = hydrated
	}
	// Merge extension data (4.3.1): ext_{namespace} columns are merged into
	// the record's Data under their namespace key.
	if err := s.mergeExtensions(ctx, rec); err != nil {
		return nil, err
	}
	s.evaluateComputed(rec.Data)

	// Resolve belongs_to relations for this single record
	s.resolveRelations(ctx, []EntityRecord{*rec}, workspaceID)

	return rec, nil
}

// mergeExtensions reads each registered ext_{namespace} column and merges it
// into the record's Data under the namespace key (4.3.1).
func (s *EntityStore) mergeExtensions(ctx context.Context, rec *EntityRecord) error {
	if len(s.extensions) == 0 {
		return nil
	}
	tbl := s.qualifiedTable()
	for ns, col := range s.extensions {
		var raw string
		err := txReadDB(ctx, s.db).QueryRowContext(ctx,
			fmt.Sprintf("SELECT %s FROM %s WHERE id = ?", col, tbl), rec.ID).Scan(&raw)
		if err != nil {
			continue // no extension data for this record
		}
		if raw == "" || raw == "{}" {
			continue
		}
		var ext map[string]any
		if err := json.Unmarshal([]byte(raw), &ext); err != nil {
			continue
		}
		if rec.Data == nil {
			rec.Data = map[string]any{}
		}
		rec.Data[ns] = ext
	}
	return nil
}

// splitExtensions extracts extension data (keys matching registered
// namespaces) from the input data into a map of namespace → ext column data,
// and removes them from the base data (4.3.2).
func (s *EntityStore) splitExtensions(data map[string]any) map[string]map[string]any {
	out := map[string]map[string]any{}
	if len(s.extensions) == 0 {
		return out
	}
	for ns := range s.extensions {
		if ext, ok := data[ns].(map[string]any); ok {
			out[ns] = ext
			delete(data, ns)
		}
	}
	return out
}

// GetByID fetches a single entity record by ID.
//
// Lookup order:
//  1. Natural key (fast path) — when the entity declares a natural_key field
//     AND the requested ID doesn't look like a UUID, skip the UUID primary-key
//     lookup entirely. This saves one unnecessary database round-trip per
//     natural-key-based fetch (e.g. REST API calls like GET /settings/clinic).
//  2. UUID v7 primary key (WHERE id = ?) — standard path for UUID-based lookups.
//  3. Natural key fallback (WHERE _<NaturalKeyField> = ?) — only when
//     the entity declares a natural_key field and the UUID lookup returned
//     no rows. This makes the REST API transparently accept both /{uuid}
//     and /{natural_key_value}.
//
// Children with storage:table are hydrated into the Data map.
func (s *EntityStore) GetByID(ctx context.Context, params GetByIDParams) (*EntityRecord, error) {
	// Fast path: the requested ID is clearly a natural key value, not a UUID.
	if s.naturalKeyField != "" && !looksLikeUUID(params.ID) {
		rec, err := s.FindByField(ctx, params.WorkspaceID, s.naturalKeyField, params.ID)
		if err != nil {
			return nil, err
		}
		return s.hydrateAndCompute(ctx, rec, params.WorkspaceID)
	}

	rec, err := s.getByIDRaw(ctx, params)
	if err != nil && s.naturalKeyField != "" {
		// Natural key fallback: the requested ID didn't match a UUID —
		// try the natural key field.
		if nkRec, nkErr := s.FindByField(ctx, params.WorkspaceID, s.naturalKeyField, params.ID); nkErr == nil {
			return s.hydrateAndCompute(ctx, nkRec, params.WorkspaceID)
		}
	}
	if err != nil {
		return nil, err
	}

	return s.hydrateAndCompute(ctx, rec, params.WorkspaceID)
}

// getByIDRaw fetches the raw parent record without hydrating children.
// Reads through txReadDB — if a request-scoped transaction is active on
// this store, this sees that transaction's own uncommitted writes so far
// (read-your-own-writes within one action execution).
func (s *EntityStore) getByIDRaw(ctx context.Context, params GetByIDParams) (*EntityRecord, error) {
	tbl := s.qualifiedTable()
	query := fmt.Sprintf(
		`SELECT id, tenant_id, version, created_at, updated_at, created_by, updated_by, doc_status, data FROM %s WHERE id = ? AND tenant_id = ?`,
		tbl)
	if s.softDelete {
		query += " AND deleted_at IS NULL"
	}

	return s.scanRecord(ctx, txReadDB(ctx, s.db), query, params.ID, params.WorkspaceID)
}

// UpdateParams holds the data for updating an entity record.
type UpdateParams struct {
	WorkspaceID   string
	ID            string
	Version       int // optimistic concurrency: update only if version matches
	UpdatedBy     string
	Data          map[string]any
	Permissions   []string       // caller's effective permissions — used for backdate/forward-date override_permission
	PendingEvents []PendingEvent // durable events to enqueue atomically with this update (see PendingEvent)
	RequestID     string         // originating request id, recorded in the audit trail (4.7.2)
}

// Update updates an entity record with optimistic concurrency control.
// Returns the new version if successful. The row update, child-table sync,
// and any PendingEvents outbox enqueue all run inside one transaction — see
// Insert's doc comment for the atomicity contract this satisfies.
// Children with storage:table are extracted from Data, stored in child tables
// (replace-all strategy), and removed from parent JSONB.
func (s *EntityStore) Update(ctx context.Context, params UpdateParams) (int, error) {
	// Summary entities are permanently read-only via API (§4.1.1)
	if s.characteristic == spec.CharSummary {
		return 0, fmt.Errorf("%w: summary entity %s/%s is read-only (create/update/delete disabled)",
			ErrValidationRule, s.module, s.entity)
	}

	// Strip read-side relation enrichment (nested objects injected by
	// resolveRelations) before validation — PATCH merges onto the fetched
	// Data and script resource.save() re-persists the whole loaded record,
	// so both can carry relation aliases that are never legitimate writes.
	s.stripEnrichedRelations(params.Data)

	// Reject unknown fields early
	if err := s.validateKnownFields(params.Data); err != nil {
		return 0, fmt.Errorf("%s update: %w", s.entity, err)
	}
	// Validate required fields that are present in the update data
	if err := s.validateRequired(params.Data); err != nil {
		return 0, fmt.Errorf("%s update: %w", s.entity, err)
	}

	// Validate field rules
	if err := s.validateFieldRules(params.Data); err != nil {
		return 0, fmt.Errorf("%s update: %w", s.entity, err)
	}

	// Validate relation targets (referenceability guard) — no transaction is
	// open yet at this point in Update, so txReadDB's usual fallback to
	// s.db is safe; it only prefers the scope's txdb when one is already
	// active (e.g. a custom action that read/wrote this same store earlier).
	if err := s.ValidateRelationTargets(ctx, txReadDB(ctx, s.db), params.WorkspaceID, params.Data); err != nil {
		return 0, fmt.Errorf("%s update: %w", s.entity, err)
	}

	// Validate transaction_date policy (backdate/forward-date)
	if err := s.validateTransactionDatePolicy(params.Data, params.Permissions); err != nil {
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

	// Split extension data (4.3.2): namespaced keys are written to their
	// ext_{namespace} columns, not the base data JSONB.
	extData := s.splitExtensions(parentData)

	tbl := s.qualifiedTable()

	// Check immutable fields: fetch existing record and compare values.
	// Use GetByID (not getByIDRaw) so natural-key lookups are resolved
	// to the actual UUID — entities with natural_key: true use the natural
	// key value as the API-facing ID (e.g. "clinic" for settings).
	var existingData map[string]any
	var resolvedID string
	var existingDocStatus spec.DocStatus
	{
		rec, err := s.GetByID(ctx, GetByIDParams{WorkspaceID: params.WorkspaceID, ID: params.ID})
		if err != nil {
			return 0, fmt.Errorf("%s update: fetch existing: %w", s.entity, err)
		}
		existingData = rec.Data
		resolvedID = rec.ID // actual UUID (natural key was resolved)
		existingDocStatus = rec.EffectiveDocStatus()

		// Validate lifecycle guard: update requires draft or lifecycle-free
		if s.submitEnabled {
			if err := LifecycleGuard("update", existingDocStatus); err != nil {
				return 0, fmt.Errorf("%s update: %w", s.entity, err)
			}
		}
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

	var newVersion int
	err := runTx(ctx, s.db, func(txdb DB) error {
		query := fmt.Sprintf(
			`UPDATE %s SET data = ?, version = version + 1, updated_by = ?, updated_at = %s WHERE id = ? AND tenant_id = ? AND version = ?`,
			tbl, currentTimestampExpr(s.driver))

		if s.softDelete {
			query += " AND deleted_at IS NULL"
		}

		query += " RETURNING version"

		if err := txdb.QueryRowContext(ctx, query,
			toJSONString(parentData),
			params.UpdatedBy,
			resolvedID,
			params.WorkspaceID,
			params.Version,
		).Scan(&newVersion); err != nil {
			if strings.Contains(err.Error(), "no rows") {
				return fmt.Errorf("%w (version conflict or not found)", ErrNotFound)
			}
			return err
		}

		// Sync children in child tables (replace-all)
		for name, ch := range childrenData {
			cs := s.children[name]
			if err := cs.withDB(txdb).UpdateChildren(ctx, resolvedID, ch); err != nil {
				return fmt.Errorf("update children %s: %w", name, err)
			}
		}

		// Write extension data (4.3.2): each namespaced payload goes to its
		// ext_{namespace} column, isolated from the base data JSONB.
		for ns, ext := range extData {
			if len(ext) == 0 {
				continue
			}
			col, ok := s.extensions[ns]
			if !ok {
				continue
			}
			if _, err := txdb.ExecContext(ctx,
				fmt.Sprintf("UPDATE %s SET %s = ? WHERE id = ?", tbl, col),
				toJSONString(ext), resolvedID); err != nil {
				return fmt.Errorf("write extension %s: %w", ns, err)
			}
		}

		// Tree/hierarchy (4.6.1/4.6.3): recompute the materialized path on
		// reparent, and reject cycles.
		if err := s.setTreePaths(ctx, txdb, params.WorkspaceID, resolvedID, parentData); err != nil {
			return err
		}

		resource := s.module + "/" + s.entity

		// Write audit log with changes diff — best-effort, same as Insert.
		changes := computeChanges(existingData, parentData)
		if len(changes) > 0 {
			changesJSON := serializeChangeMap(changes)
			if err := writeAuditLog(ctx, txdb, s.driver, params.WorkspaceID, resource, resolvedID,
				string(AuditActionUpdate), params.UpdatedBy, changesJSON, params.RequestID); err != nil {
				log.Printf("[WARN] audit write failed (update %s/%s): %v", resource, resolvedID, err)
			}
		}

		// Enqueue any durable events atomically with the row update that
		// produced them — see Insert's doc comment.
		for _, ev := range params.PendingEvents {
			if _, err := enqueueOutbox(ctx, txdb, params.WorkspaceID, ev.Name, resource, ev.Payload); err != nil {
				return fmt.Errorf("enqueue pending event %q: %w", ev.Name, err)
			}
		}

		return nil
	})
	if err != nil {
		return 0, fmt.Errorf("%s update: %w", s.entity, err)
	}

	return newVersion, nil
}

// SoftDelete marks an entity record as deleted. Child-row removal, the
// delete/deactivate write, and the audit log entry all run inside one
// transaction (see Insert's doc comment for the atomicity contract).
// Children in child tables are cascade-deleted (ON DELETE CASCADE from DDL handles this
// for hard deletes; for soft deletes we explicitly remove child rows).
func (s *EntityStore) SoftDelete(ctx context.Context, workspaceID, id string) error {
	// Summary entities are permanently read-only via API (§4.1.1)
	if s.characteristic == spec.CharSummary {
		return fmt.Errorf("%w: summary entity %s/%s is read-only (create/update/delete disabled)",
			ErrValidationRule, s.module, s.entity)
	}

	// Fetch existing record to check lifecycle guard
	rec, err := s.GetByID(ctx, GetByIDParams{WorkspaceID: workspaceID, ID: id})
	if err != nil {
		return fmt.Errorf("%s delete: fetch existing: %w", s.entity, err)
	}
	if s.submitEnabled {
		if guardErr := LifecycleGuard("delete", rec.EffectiveDocStatus()); guardErr != nil {
			return fmt.Errorf("%s delete: %w", s.entity, guardErr)
		}
	}

	// Enforce reference guard: check if any document references this one
	if guardErr := s.EnforceReferenceGuard(ctx, txReadDB(ctx, s.db), workspaceID, rec.ID, "delete"); guardErr != nil {
		return fmt.Errorf("%s delete: %w", s.entity, guardErr)
	}

	// Archive lock (4.9.3): a master referenced by an archived transaction is
	// flagged locked_for_deletion=true and cannot be deleted.
	if locked, _ := rec.Data["locked_for_deletion"].(bool); locked {
		return fmt.Errorf("%s delete: %s", s.entity, spec.ErrorArchiveLockedForDeletion)
	}

	if !s.softDelete {
		delErr := runTx(ctx, s.db, func(txdb DB) error {
			// Delete children first (child tables have ON DELETE CASCADE,
			// but we do it explicitly for clarity).
			for name, cs := range s.children {
				if err := cs.withDB(txdb).DeleteChildren(ctx, id); err != nil {
					return fmt.Errorf("delete children %s: %w", name, err)
				}
			}

			// Hard delete
			tbl := s.qualifiedTable()
			_, txErr := txdb.ExecContext(ctx,
				fmt.Sprintf("DELETE FROM %s WHERE id = ? AND tenant_id = ?", tbl),
				id, workspaceID)
			return txErr
		})
		if delErr != nil {
			return fmt.Errorf("%s delete: %w", s.entity, delErr)
		}
		return nil
	}

	err = runTx(ctx, s.db, func(txdb DB) error {
		tbl := s.qualifiedTable()
		query := fmt.Sprintf(
			`UPDATE %s SET deleted_at = %s WHERE id = ? AND tenant_id = ? AND deleted_at IS NULL`,
			tbl, currentTimestampExpr(s.driver))

		result, err := txdb.ExecContext(ctx, query, id, workspaceID)
		if err != nil {
			return err
		}
		rows, _ := result.RowsAffected()
		if rows == 0 {
			return ErrNotFound
		}

		// Write audit log
		resource := s.module + "/" + s.entity
		if err := writeAuditLog(ctx, txdb, s.driver, workspaceID, resource, id,
			string(AuditActionDelete), "system", "{}", ""); err != nil {
			// Audit failure is non-fatal — log warning but don't fail the delete
			log.Printf("[WARN] audit write failed (delete %s/%s): %v", resource, id, err)
		}
		return nil
	})
	if err != nil {
		return fmt.Errorf("%s soft delete: %w", s.entity, err)
	}
	return nil
}

// UpdateFields atomically merges specific fields into the JSONB data column
// without fetching first — single SQL statement.
// PostgreSQL: data || ?::jsonb (jsonb merge)
// SQLite:     json_patch(data, ?)  (RFC 6902 JSON Patch merge)
func (s *EntityStore) UpdateFields(ctx context.Context, workspaceID, id string, fields map[string]any) error {
	if len(fields) == 0 {
		return nil
	}

	tbl := s.qualifiedTable()
	dataJSON := toJSONString(fields)

	softDeleteClause := ""
	if s.softDelete {
		softDeleteClause = "AND deleted_at IS NULL"
	}

	var query string
	if s.driver == DriverPostgres {
		query = fmt.Sprintf(
			`UPDATE %s SET data = data || ?::jsonb, version = version + 1, updated_at = %s WHERE id = ? AND tenant_id = ? %s`,
			tbl, currentTimestampExpr(s.driver), softDeleteClause)
	} else {
		query = fmt.Sprintf(
			`UPDATE %s SET data = json_patch(data, ?), version = version + 1, updated_at = %s WHERE id = ? AND tenant_id = ? %s`,
			tbl, currentTimestampExpr(s.driver), softDeleteClause)
	}

	database, err := writeDB(ctx, s.db)
	if err != nil {
		return fmt.Errorf("%s update fields: %w", s.entity, err)
	}
	result, err := database.ExecContext(ctx, query, dataJSON, id, workspaceID)
	if err != nil {
		return fmt.Errorf("%s update fields: %w", s.entity, err)
	}
	rows, _ := result.RowsAffected()
	if rows == 0 {
		return fmt.Errorf("%s update fields: %w (not found)", s.entity, ErrNotFound)
	}
	return nil
}

// IncrementField atomically adds amount to a numeric JSONB field.
// Single SQL statement — no read-modify-write race condition.
func (s *EntityStore) IncrementField(ctx context.Context, workspaceID, id, field string, amount float64) error {
	tbl := s.qualifiedTable()

	var query string
	if s.driver == DriverPostgres {
		query = fmt.Sprintf(
			`UPDATE %s SET data = jsonb_set(data, '{%s}', to_jsonb(COALESCE((data->>'%s')::numeric, 0) + ?)),
				version = version + 1, updated_at = %s
			WHERE id = ? AND tenant_id = ?`,
			tbl, field, field, currentTimestampExpr(s.driver))
	} else {
		query = fmt.Sprintf(
			`UPDATE %s SET data = json_set(data, '$.%s', CAST(json_extract(data, '$.%s') AS numeric) + ?),
				version = version + 1, updated_at = %s
			WHERE id = ? AND tenant_id = ?`,
			tbl, field, field, currentTimestampExpr(s.driver))
	}
	if s.softDelete {
		query += " AND deleted_at IS NULL"
	}

	database, err := writeDB(ctx, s.db)
	if err != nil {
		return fmt.Errorf("%s increment field %s: %w", s.entity, field, err)
	}
	result, err := database.ExecContext(ctx, query, amount, id, workspaceID)
	if err != nil {
		return fmt.Errorf("%s increment field %s: %w", s.entity, field, err)
	}
	rows, _ := result.RowsAffected()
	if rows == 0 {
		return fmt.Errorf("%s increment field %s: %w (not found)", s.entity, field, ErrNotFound)
	}
	return nil
}

// DecrementField atomically subtracts amount from a numeric JSONB field.
// Includes a guard (WHERE ... >= amount) to prevent negative values.
// Returns the new field value after decrement.
func (s *EntityStore) DecrementField(ctx context.Context, workspaceID, id, field string, amount float64) (float64, error) {
	tbl := s.qualifiedTable()

	softDeleteClause := ""
	if s.softDelete {
		softDeleteClause = "AND deleted_at IS NULL"
	}

	var query string
	if s.driver == DriverPostgres {
		query = fmt.Sprintf(
			`UPDATE %s SET data = jsonb_set(data, '{%s}', to_jsonb(COALESCE((data->>'%s')::numeric, 0) - ?)),
				version = version + 1, updated_at = %s
			WHERE id = ? AND tenant_id = ? %s AND COALESCE((data->>'%s')::numeric, 0) >= ?
			RETURNING COALESCE((data->>'%s')::numeric, 0)`,
			tbl, field, field, currentTimestampExpr(s.driver), softDeleteClause, field, field)
	} else {
		query = fmt.Sprintf(
			`UPDATE %s SET data = json_set(data, '$.%s', CAST(json_extract(data, '$.%s') AS numeric) - ?),
				version = version + 1, updated_at = %s
			WHERE id = ? AND tenant_id = ? %s AND CAST(COALESCE(json_extract(data, '$.%s'), '0') AS numeric) >= ?
			RETURNING CAST(COALESCE(json_extract(data, '$.%s'), '0') AS numeric)`,
			tbl, field, field, currentTimestampExpr(s.driver), softDeleteClause, field, field)
	}

	database, err := writeDB(ctx, s.db)
	if err != nil {
		return 0, fmt.Errorf("%s decrement field %s: %w", s.entity, field, err)
	}
	var newVal float64
	err = database.QueryRowContext(ctx, query, amount, id, workspaceID, amount).Scan(&newVal)
	if err != nil {
		if err == sql.ErrNoRows {
			return 0, fmt.Errorf("%s decrement field %s: %w (not found or insufficient %s)", s.entity, field, ErrNotFound, field)
		}
		return 0, fmt.Errorf("%s decrement field %s: %w", s.entity, field, err)
	}
	return newVal, nil
}

// ValidateRelationTargets checks that all relation fields in data point to
// valid targets (doc_status = NULL or 'submitted'). Draft/cancelled targets
// are rejected per Core §4.1b referenceability rule. database is the DB to
// read through — callers already inside an open transaction (Insert) MUST
// pass that transaction's txdb, not s.db: issuing a second, independent
// query against s.db while a transaction already holds the sole SQLite
// connection deadlocks instead of erroring.
func (s *EntityStore) ValidateRelationTargets(ctx context.Context, database DB, workspaceID string, data map[string]any) error {
	for _, f := range s.fields {
		if f.Relation == nil || f.Relation.Resource == "" {
			continue
		}

		targetID, exists := data[f.Name]
		if !exists || targetID == nil || targetID == "" {
			continue // optional or not set
		}

		targetIDStr, ok := targetID.(string)
		if !ok {
			continue
		}

		// Resolve the target resource reference: "module.entity" or "entity" (same module).
		targetResource := f.Relation.Resource
		targetModule := s.module
		targetEntity := targetResource
		if dotIdx := strings.Index(targetResource, "."); dotIdx >= 0 {
			targetModule = targetResource[:dotIdx]
			targetEntity = targetResource[dotIdx+1:]
		}

		// Resolve the table name — use the resolver if set, otherwise fall
		// back to the naive {module}_{plural} convention (2.2.5).
		var tbl string
		if s.targetTableResolver != nil {
			resolved, err := s.targetTableResolver(targetModule, targetEntity)
			if err != nil {
				// Resolver failed — skip guard (best-effort)
				continue
			}
			tbl = resolved
		} else {
			tbl = targetModule + "_" + inflectPlural(targetEntity)
		}

		query := fmt.Sprintf(
			`SELECT doc_status FROM %s WHERE id = ? AND tenant_id = ? AND deleted_at IS NULL`,
			tbl)

		var docStatus *string
		err := database.QueryRowContext(ctx, query, targetIDStr, workspaceID).Scan(&docStatus)
		if err != nil {
			// Target not found or table doesn't exist — skip guard
			continue
		}

		if docStatus != nil {
			switch *docStatus {
			case "draft":
				return fmt.Errorf("%w: relation target %s[%s] is draft (must be submitted or lifecycle-free)",
					ErrValidationRule, f.Relation.Resource, targetIDStr)
			case "cancelled":
				return fmt.Errorf("%w: relation target %s[%s] is cancelled (must be submitted or lifecycle-free)",
					ErrValidationRule, f.Relation.Resource, targetIDStr)
			}
		}
		// docStatus NULL = lifecycle-free → allowed
		// docStatus "submitted" → allowed
	}
	return nil
}

// Submit transitions a document from draft → submitted.
func (s *EntityStore) Submit(ctx context.Context, workspaceID, id, userID string) error {
	// Fetch existing record to check lifecycle guard
	rec, err := s.GetByID(ctx, GetByIDParams{WorkspaceID: workspaceID, ID: id})
	if err != nil {
		return fmt.Errorf("%s submit: fetch existing: %w", s.entity, err)
	}
	if s.submitEnabled {
		if guardErr := LifecycleGuard("submit", rec.EffectiveDocStatus()); guardErr != nil {
			return fmt.Errorf("%s submit: %w", s.entity, guardErr)
		}
	}

	tbl := s.qualifiedTable()
	query := fmt.Sprintf(
		`UPDATE %s SET doc_status = 'submitted', version = version + 1, updated_at = %s, updated_by = ? WHERE id = ? AND tenant_id = ? AND doc_status = 'draft'`,
		tbl, currentTimestampExpr(s.driver))

	if s.softDelete {
		query += " AND deleted_at IS NULL"
	}

	database, err := writeDB(ctx, s.db)
	if err != nil {
		return fmt.Errorf("%s submit: %w", s.entity, err)
	}
	result, err := database.ExecContext(ctx, query, userID, id, workspaceID)
	if err != nil {
		return fmt.Errorf("%s submit: %w", s.entity, err)
	}
	rows, _ := result.RowsAffected()
	if rows == 0 {
		return fmt.Errorf("%s submit: %w (not in draft status or not found)", s.entity, ErrNotFound)
	}

	// Propagate lifecycle to children (2.3.9)
	for name, cs := range s.children {
		if err := cs.withDB(s.db).SubmitChildren(ctx, id); err != nil {
			log.Printf("[WARN] submit children %s: %v", name, err)
		}
	}

	return nil
}

// Cancel transitions a document from submitted → cancelled.
func (s *EntityStore) Cancel(ctx context.Context, workspaceID, id, userID string) error {
	// Fetch existing record to check lifecycle guard
	rec, err := s.GetByID(ctx, GetByIDParams{WorkspaceID: workspaceID, ID: id})
	if err != nil {
		return fmt.Errorf("%s cancel: fetch existing: %w", s.entity, err)
	}
	if s.submitEnabled {
		if guardErr := LifecycleGuard("cancel", rec.EffectiveDocStatus()); guardErr != nil {
			return fmt.Errorf("%s cancel: %w", s.entity, guardErr)
		}
	}

	tbl := s.qualifiedTable()
	query := fmt.Sprintf(
		`UPDATE %s SET doc_status = 'cancelled', version = version + 1, updated_at = %s, updated_by = ? WHERE id = ? AND tenant_id = ? AND doc_status = 'submitted'`,
		tbl, currentTimestampExpr(s.driver))

	if s.softDelete {
		query += " AND deleted_at IS NULL"
	}

	database, err := writeDB(ctx, s.db)
	if err != nil {
		return fmt.Errorf("%s cancel: %w", s.entity, err)
	}
	result, err := database.ExecContext(ctx, query, userID, id, workspaceID)
	if err != nil {
		return fmt.Errorf("%s cancel: %w", s.entity, err)
	}
	rows, _ := result.RowsAffected()
	if rows == 0 {
		return fmt.Errorf("%s cancel: %w (not in submitted status or not found)", s.entity, ErrNotFound)
	}

	// Propagate lifecycle to children (2.3.9)
	for name, cs := range s.children {
		if err := cs.withDB(s.db).CancelChildren(ctx, id); err != nil {
			log.Printf("[WARN] cancel children %s: %v", name, err)
		}
	}

	return nil
}

// Deactivate sets is_active=false on a record (soft-deactivation pattern,
// 1.4.10 / 4.10.2, 02-core-extended.md §19). Only valid when the entity
// declares soft_deactivate: {enabled: true}. Uses optimistic concurrency via
// the record's current version.
func (s *EntityStore) Deactivate(ctx context.Context, workspaceID, id, userID string) error {
	return s.setActive(ctx, workspaceID, id, userID, false)
}

// Reactivate sets is_active=true on a record (soft-deactivation pattern).
func (s *EntityStore) Reactivate(ctx context.Context, workspaceID, id, userID string) error {
	return s.setActive(ctx, workspaceID, id, userID, true)
}

// setActive updates the is_active field in the record's data JSONB.
func (s *EntityStore) setActive(ctx context.Context, workspaceID, id, userID string, active bool) error {
	rec, err := s.GetByID(ctx, GetByIDParams{WorkspaceID: workspaceID, ID: id})
	if err != nil {
		return fmt.Errorf("%s set_active: fetch: %w", s.entity, err)
	}
	if rec == nil {
		return fmt.Errorf("%s set_active: %w", s.entity, ErrNotFound)
	}
	data := rec.Data
	if data == nil {
		data = map[string]any{}
	}
	data["is_active"] = active
	_, err = s.Update(ctx, UpdateParams{
		WorkspaceID: workspaceID,
		ID:          id,
		Version:     rec.Version,
		UpdatedBy:   userID,
		Data:        data,
	})
	if err != nil {
		return fmt.Errorf("%s set_active: %w", s.entity, err)
	}
	return nil
}

// Amend atomically cancels the original and creates a linked new document as draft.
// Sets amends (on new) and amended_by (on original) reserved fields.
func (s *EntityStore) Amend(ctx context.Context, workspaceID, originalID, userID string, newData map[string]any) (string, error) {
	// 1. Cancel the original
	if err := s.Cancel(ctx, workspaceID, originalID, userID); err != nil {
		return "", fmt.Errorf("%s amend: cancel original: %w", s.entity, err)
	}

	// 2. Create new document as draft
	newID, err := s.Insert(ctx, InsertParams{
		WorkspaceID: workspaceID,
		CreatedBy:   userID,
		Data:        newData,
	})
	if err != nil {
		return "", fmt.Errorf("%s amend: create new: %w", s.entity, err)
	}

	database, err := writeDB(ctx, s.db)
	if err != nil {
		return "", fmt.Errorf("%s amend: %w", s.entity, err)
	}

	// 3. Set amends on new document (points to cancelled original)
	tbl := s.qualifiedTable()
	_, err = database.ExecContext(ctx,
		fmt.Sprintf(`UPDATE %s SET amends = ? WHERE id = ? AND tenant_id = ?`, tbl),
		originalID, newID, workspaceID)
	if err != nil {
		log.Printf("[WARN] %s amend: failed to set amends on new doc %s: %v", s.entity, newID, err)
	}

	// 4. Set amended_by on original (points to new version)
	_, err = database.ExecContext(ctx,
		fmt.Sprintf(`UPDATE %s SET amended_by = ? WHERE id = ? AND tenant_id = ?`, tbl),
		newID, originalID, workspaceID)
	if err != nil {
		log.Printf("[WARN] %s amend: failed to set amended_by on original %s: %v", s.entity, originalID, err)
	}

	return newID, nil
}

// normativeListColumns are real table columns (Core §19) addressable in
// filters and sort without a generated-column mapping.
var normativeListColumns = map[string]bool{
	"id": true, "tenant_id": true, "version": true, "created_at": true,
	"updated_at": true, "created_by": true, "updated_by": true, "doc_status": true,
}

// IsNormativeColumn reports whether name is a framework-managed table column
// (Core §19) addressable directly in filters and sort.
func IsNormativeColumn(name string) bool { return normativeListColumns[name] }

// columnRefExpr returns the SQL column expression for a field, falling back
// to a JSONB path expression when the field has no generated column. This
// enables filtering on non-indexed fields via data->>'field' (Postgres) or
// json_extract(data, '$.field') (SQLite) — see 2.2.2.
//
// When the field type is known (numeric, boolean, date, timestamp) the JSONB
// text value is cast to the native SQL type so that sorting and comparison
// operators work correctly (numeric order, not lexicographic). Fields without
// a type match (unknown field name) are returned as plain text for backward
// compatibility with programmatic queries outside the entity schema.
func (s *EntityStore) columnRefExpr(field string) string {
	if normativeListColumns[field] {
		return field
	}
	// Check if the field has a generated column (index/unique/naturalKey)
	for _, f := range s.fields {
		if f.Name == field && (f.Index || f.Unique || f.NaturalKey) {
			return generatedColumnName(field)
		}
	}

	// Lookup field type for type-aware casting
	var fieldType spec.FieldType
	for _, f := range s.fields {
		if f.Name == field {
			fieldType = f.Type
			break
		}
	}

	// Fallback: use JSONB path expression
	var expr string
	if s.driver == DriverPostgres {
		expr = fmt.Sprintf("data->>'%s'", field)
	} else {
		expr = fmt.Sprintf("json_extract(data, '$.%s')", field)
	}

	// Cast based on type — ensures numeric/date sort is correct, not lexicographic
	castType := castTypeForField(fieldType, s.driver)
	if castType == "" {
		return expr
	}
	if s.driver == DriverPostgres {
		return fmt.Sprintf("(%s)::%s", expr, castType)
	}
	return fmt.Sprintf("CAST(%s AS %s)", expr, castType)
}

// castTypeForField returns the SQL type keyword to cast a FieldType to its
// native database type. Returns empty string when no cast is needed (text
// comparison is semantically correct).
//
// For SQLite, date/time types are stored as ISO-8601 text and sort correctly
// lexicographically without explicit cast.
func castTypeForField(ft spec.FieldType, driver DriverType) string {
	switch ft {
	case spec.FieldInteger:
		return "integer"
	case spec.FieldDecimal, spec.FieldNumber:
		if driver == DriverSQLite {
			return "REAL"
		}
		return "numeric"
	case spec.FieldBoolean:
		if driver == DriverSQLite {
			return "INTEGER"
		}
		return "boolean"
	case spec.FieldDate:
		if driver == DriverPostgres {
			return "date"
		}
		return "" // SQLite: ISO text sorts correctly
	case spec.FieldDateTime:
		if driver == DriverPostgres {
			return "timestamp"
		}
		return "" // SQLite: ISO text sorts correctly
	case spec.FieldTime:
		if driver == DriverPostgres {
			return "time"
		}
		return "" // SQLite: ISO text sorts correctly
	default:
		// string, text, richtext, enum, uuid, json, file, relation, money,
		// child — either already text or composite types that can't be cast
		return ""
	}
}

// toAnySlice normalizes a filter value into a flat []any for IN/NOT IN.
func toAnySlice(v any) []any {
	switch t := v.(type) {
	case []any:
		return t
	case []string:
		out := make([]any, len(t))
		for i, s := range t {
			out[i] = s
		}
		return out
	case nil:
		return nil
	default:
		return []any{v}
	}
}

// ListParams holds pagination, sorting, and filtering for List queries.
type ListParams struct {
	WorkspaceID string
	Page        int    // 1-based
	PerPage     int    // default 20
	Sort        string // field name, prefixed with - for DESC
	Filters     map[string]FilterOp
	Search      string // full-text search across data
}

// FilterOp represents a filter operation.
type FilterOp struct {
	Op    string // eq, neq, gt, gte, lt, lte, between, in, nin, like, ilike, null, notnull
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
	// Spec §558-559: default 20, max 100, values above max clamped to 100
	if params.PerPage > 100 {
		params.PerPage = 100
	} else if params.PerPage < 1 {
		params.PerPage = 20
	}

	tbl := s.qualifiedTable()
	var whereClauses []string
	var args []any

	// Tenant isolation
	whereClauses = append(whereClauses, "tenant_id = ?")
	args = append(args, params.WorkspaceID)

	// Soft delete filter
	if s.softDelete {
		whereClauses = append(whereClauses, "deleted_at IS NULL")
	}

	// Custom filters
	for field, filter := range params.Filters {
		col := s.columnRefExpr(field)
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
		case "ilike":
			if s.driver == DriverPostgres {
				whereClauses = append(whereClauses, fmt.Sprintf("%s ILIKE ?", col))
			} else {
				whereClauses = append(whereClauses, fmt.Sprintf("LOWER(%s) LIKE LOWER(?)", col))
			}
			args = append(args, filter.Value)
		case "null":
			whereClauses = append(whereClauses, fmt.Sprintf("%s IS NULL", col))
		case "notnull":
			whereClauses = append(whereClauses, fmt.Sprintf("%s IS NOT NULL", col))
		case "between":
			values := toAnySlice(filter.Value)
			if len(values) != 2 {
				continue
			}
			whereClauses = append(whereClauses, fmt.Sprintf("%s BETWEEN ? AND ?", col))
			args = append(args, values[0], values[1])
		case "in", "nin":
			values := toAnySlice(filter.Value)
			if len(values) == 0 {
				continue
			}
			placeholders := strings.Repeat("?,", len(values))
			placeholders = placeholders[:len(placeholders)-1]
			op := "IN"
			if filter.Op == "nin" {
				op = "NOT IN"
			}
			whereClauses = append(whereClauses, fmt.Sprintf("%s %s (%s)", col, op, placeholders))
			args = append(args, values...)
		case "descendant_of":
			// Tree operator (4.6.2): rows whose materialized path starts with
			// the given ancestor's path prefix. Value is the ancestor's path
			// (e.g. "a.b") or its id (resolved to its path).
			prefix, ok := filter.Value.(string)
			if !ok || prefix == "" {
				continue
			}
			tpath := fmt.Sprintf("_tpath_%s", field)
			whereClauses = append(whereClauses, fmt.Sprintf("%s LIKE ?", tpath))
			args = append(args, prefix+".%")
		case "child_of":
			// Tree operator (4.6.2): direct children (parent FK equals value).
			whereClauses = append(whereClauses, fmt.Sprintf("%s = ?", col))
			args = append(args, filter.Value)
		case "root":
			// Tree operator (4.6.2): root nodes (no parent).
			whereClauses = append(whereClauses, fmt.Sprintf("%s IS NULL", col))
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
	err := txReadDB(ctx, s.db).QueryRowContext(ctx, countQuery, args...).Scan(&total)
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
		orderClause = fmt.Sprintf("ORDER BY %s %s", s.columnRefExpr(field), direction)
	}

	// Pagination
	offset := (params.Page - 1) * params.PerPage
	paginationClause := "LIMIT ? OFFSET ?"
	args = append(args, params.PerPage, offset)

	// Query
	query := fmt.Sprintf(
		`SELECT id, tenant_id, version, created_at, updated_at, created_by, updated_by, doc_status, data FROM %s WHERE %s %s %s`,
		tbl, whereStr, orderClause, paginationClause)

	rows, err := txReadDB(ctx, s.db).QueryContext(ctx, query, args...)
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

	// Resolve belongs_to relations: fetch related records and nest them
	// so dot-path column accessors (e.g. patient.name) work in the frontend.
	s.resolveRelations(ctx, records, params.WorkspaceID)

	return &ListResult{
		Data:       records,
		Total:      total,
		Page:       params.Page,
		PerPage:    params.PerPage,
		TotalPages: totalPages,
	}, nil
}

// AggregateParams holds the inputs for an aggregate query (4.5.1–4.5.3).
type AggregateParams struct {
	WorkspaceID string
	// Func is the aggregate function: sum, count, avg, min, max.
	Func string
	// Field is the field to aggregate. Empty for count (COUNT(*)).
	Field string
	// GroupBy lists fields to group by (single or multi-field).
	GroupBy []string
	// DateTrunc enables time bucketing (4.5.4): truncate the given date
	// field to the given unit (day|week|month|quarter|year) and group by it.
	DateTrunc *DateTruncDecl
	// Having holds post-aggregation filters applied to the aggregate
	// expression (4.5.3), e.g. HAVING SUM(amount) > 500.
	Having []FilterOp
	// Filters holds pre-aggregation filters (same operators as List).
	Filters map[string]FilterOp
}

// DateTruncDecl declares a date_trunc time bucket (4.5.4).
type DateTruncDecl struct {
	Field string // date/datetime field to truncate
	Unit  string // day | week | month | quarter | year
}

// AggregateGroup is one row of an aggregate result.
type AggregateGroup struct {
	// Key maps each group-by field to its value (empty when no group_by).
	Key map[string]any
	// Value is the aggregate result for this group.
	Value float64
}

// AggregateResult is the outcome of an aggregate query.
type AggregateResult struct {
	Groups []AggregateGroup
}

// Aggregate runs an aggregate query (sum/count/avg/min/max) over the entity,
// optionally grouped by one or more fields (4.5.1–4.5.3). Pre-aggregation
// filters use the same operators as List; Having filters apply after
// grouping.
func (s *EntityStore) Aggregate(ctx context.Context, params AggregateParams) (*AggregateResult, error) {
	fn := strings.ToLower(params.Func)
	switch fn {
	case "sum", "count", "avg", "min", "max":
	default:
		return nil, fmt.Errorf("%s aggregate: unknown function %q (want sum|count|avg|min|max)", s.entity, params.Func)
	}

	tbl := s.qualifiedTable()
	var whereClauses []string
	var args []any

	whereClauses = append(whereClauses, "tenant_id = ?")
	args = append(args, params.WorkspaceID)
	if s.softDelete {
		whereClauses = append(whereClauses, "deleted_at IS NULL")
	}
	for field, filter := range params.Filters {
		col := s.columnRefExpr(field)
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
		case "null":
			whereClauses = append(whereClauses, fmt.Sprintf("%s IS NULL", col))
		case "notnull":
			whereClauses = append(whereClauses, fmt.Sprintf("%s IS NOT NULL", col))
		case "between":
			values := toAnySlice(filter.Value)
			if len(values) == 2 {
				whereClauses = append(whereClauses, fmt.Sprintf("%s BETWEEN ? AND ?", col))
				args = append(args, values[0], values[1])
			}
		}
	}
	whereStr := strings.Join(whereClauses, " AND ")

	// Build the aggregate expression.
	var aggExpr string
	if fn == "count" && params.Field == "" {
		aggExpr = "COUNT(*)"
	} else {
		aggExpr = fmt.Sprintf("%s(%s)", strings.ToUpper(fn), s.columnRefExpr(params.Field))
	}

	// Group-by columns.
	var groupCols []string
	for _, g := range params.GroupBy {
		groupCols = append(groupCols, s.columnRefExpr(g))
	}

	// date_trunc time bucket (4.5.4): truncate the date field and group by it.
	truncKey := ""
	if params.DateTrunc != nil {
		unit := strings.ToLower(params.DateTrunc.Unit)
		switch unit {
		case "day", "week", "month", "quarter", "year":
		default:
			return nil, fmt.Errorf("%s aggregate: unknown date_trunc unit %q (want day|week|month|quarter|year)", s.entity, params.DateTrunc.Unit)
		}
		col := s.columnRefExpr(params.DateTrunc.Field)
		var expr string
		if s.driver == DriverPostgres {
			expr = fmt.Sprintf("date_trunc('%s', %s)", unit, col)
		} else {
			// SQLite: strftime with the appropriate format.
			format := map[string]string{
				"day": "%Y-%m-%d", "week": "%Y-%W", "month": "%Y-%m",
				"quarter": "%Y-%m", "year": "%Y",
			}[unit]
			expr = fmt.Sprintf("strftime('%s', %s)", format, col)
		}
		groupCols = append(groupCols, expr)
		truncKey = params.DateTrunc.Field + ":" + unit
	}

	// Having clause (post-aggregation). The filter applies to the aggregate
	// expression itself (e.g. HAVING SUM(amount) > 500).
	var havingClauses []string
	for _, h := range params.Having {
		switch h.Op {
		case "gt":
			havingClauses = append(havingClauses, fmt.Sprintf("%s > ?", aggExpr))
			args = append(args, h.Value)
		case "gte":
			havingClauses = append(havingClauses, fmt.Sprintf("%s >= ?", aggExpr))
			args = append(args, h.Value)
		case "lt":
			havingClauses = append(havingClauses, fmt.Sprintf("%s < ?", aggExpr))
			args = append(args, h.Value)
		case "lte":
			havingClauses = append(havingClauses, fmt.Sprintf("%s <= ?", aggExpr))
			args = append(args, h.Value)
		case "eq":
			havingClauses = append(havingClauses, fmt.Sprintf("%s = ?", aggExpr))
			args = append(args, h.Value)
		}
	}

	selectParts := []string{aggExpr}
	if len(groupCols) > 0 {
		selectParts = append(groupCols, aggExpr)
	}
	query := fmt.Sprintf("SELECT %s FROM %s WHERE %s", strings.Join(selectParts, ", "), tbl, whereStr)
	if len(groupCols) > 0 {
		query += " GROUP BY " + strings.Join(groupCols, ", ")
	}
	if len(havingClauses) > 0 {
		query += " HAVING " + strings.Join(havingClauses, " AND ")
	}

	rows, err := txReadDB(ctx, s.db).QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("%s aggregate: %w", s.entity, err)
	}
	defer rows.Close()

	result := &AggregateResult{}
	for rows.Next() {
		group := AggregateGroup{Key: map[string]any{}}
		scanTargets := make([]any, 0, len(groupCols)+1)
		for range groupCols {
			var v any
			scanTargets = append(scanTargets, &v)
		}
		var aggVal float64
		scanTargets = append(scanTargets, &aggVal)
		if err := rows.Scan(scanTargets...); err != nil {
			return nil, fmt.Errorf("%s aggregate scan: %w", s.entity, err)
		}
		for i, g := range params.GroupBy {
			group.Key[g] = derefAny(scanTargets[i])
		}
		if truncKey != "" {
			group.Key[truncKey] = derefAny(scanTargets[len(params.GroupBy)])
		}
		group.Value = aggVal
		result.Groups = append(result.Groups, group)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("%s aggregate rows: %w", s.entity, err)
	}
	return result, nil
}

// derefAny unwraps a *interface{} returned by database/sql scans.
func derefAny(v any) any {
	if p, ok := v.(*any); ok {
		if p == nil {
			return nil
		}
		return *p
	}
	return v
}

// setTreePaths computes and stores the materialized path (_tpath_{field}) for
// each tree: true self-referential relation on insert (4.6.1). Path format:
// "" (root) or "parent.child.grandchild". The parent's path is looked up from
// the parent record's _tpath column.
func (s *EntityStore) setTreePaths(ctx context.Context, txdb DB, workspaceID, id string, data map[string]any) error {
	for _, f := range s.fields {
		if !f.Tree || f.Relation == nil {
			continue
		}
		parentID, _ := data[f.Name].(string)
		path := id
		if parentID != "" {
			tbl := s.qualifiedTable()
			var parentPath string
			err := txdb.QueryRowContext(ctx,
				fmt.Sprintf("SELECT _tpath_%s FROM %s WHERE id = ? AND tenant_id = ?", f.Name, tbl),
				parentID, workspaceID).Scan(&parentPath)
			if err != nil {
				// Parent not found or no path — treat as root.
				parentPath = ""
			}
			// Cycle detection (4.6.3): if the parent's path already contains
			// this record's id, reparenting would create a cycle.
			if strings.Contains(parentPath, id) {
				return fmt.Errorf("%s tree: %w: cycle detected (parent %s is a descendant of %s)",
					s.entity, ErrValidationRule, parentID, id)
			}
			if parentPath != "" {
				path = parentPath + "." + id
			}
		}
		if _, err := txdb.ExecContext(ctx,
			fmt.Sprintf("UPDATE %s SET _tpath_%s = ? WHERE id = ?", s.qualifiedTable(), f.Name),
			path, id); err != nil {
			return err
		}
	}
	return nil
}

// WindowParams holds the inputs for a window-function query (4.5.5).
type WindowParams struct {
	WorkspaceID string
	// Func is the window function: running_total, rank, row_number.
	Func string
	// Field is the field for running_total (ignored for rank/row_number).
	Field string
	// OrderBy lists fields to order the window by ("-field" for DESC).
	OrderBy []string
	// PartitionBy lists fields to partition the window by.
	PartitionBy []string
	// Filters holds pre-aggregation filters (same operators as List).
	Filters map[string]FilterOp
}

// WindowRow is one row of a window-function result.
type WindowRow struct {
	// Data is the record's business data.
	Data map[string]any
	// Value is the window value (running total, rank, or row number).
	Value float64
}

// WindowResult is the outcome of a window-function query.
type WindowResult struct {
	Rows []WindowRow
}

// Window runs a window-function query (4.5.5): running_total, rank, or
// row_number, optionally partitioned and ordered.
func (s *EntityStore) Window(ctx context.Context, params WindowParams) (*WindowResult, error) {
	fn := strings.ToLower(params.Func)
	switch fn {
	case "running_total", "rank", "row_number":
	default:
		return nil, fmt.Errorf("%s window: unknown function %q (want running_total|rank|row_number)", s.entity, params.Func)
	}

	tbl := s.qualifiedTable()
	var whereClauses []string
	var args []any

	whereClauses = append(whereClauses, "tenant_id = ?")
	args = append(args, params.WorkspaceID)
	if s.softDelete {
		whereClauses = append(whereClauses, "deleted_at IS NULL")
	}
	for field, filter := range params.Filters {
		col := s.columnRefExpr(field)
		switch filter.Op {
		case "eq":
			whereClauses = append(whereClauses, fmt.Sprintf("%s = ?", col))
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
		}
	}
	whereStr := strings.Join(whereClauses, " AND ")

	// Build the window expression.
	var winExpr string
	switch fn {
	case "running_total":
		winExpr = fmt.Sprintf("SUM(%s) OVER (%s)", s.columnRefExpr(params.Field), s.windowSpec(params))
	case "rank":
		winExpr = fmt.Sprintf("RANK() OVER (%s)", s.windowSpec(params))
	case "row_number":
		winExpr = fmt.Sprintf("ROW_NUMBER() OVER (%s)", s.windowSpec(params))
	}

	query := fmt.Sprintf("SELECT data, %s FROM %s WHERE %s", winExpr, tbl, whereStr)

	rows, err := txReadDB(ctx, s.db).QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("%s window: %w", s.entity, err)
	}
	defer rows.Close()

	result := &WindowResult{}
	for rows.Next() {
		var dataStr string
		var val float64
		if err := rows.Scan(&dataStr, &val); err != nil {
			return nil, fmt.Errorf("%s window scan: %w", s.entity, err)
		}
		var data map[string]any
		if err := json.Unmarshal([]byte(dataStr), &data); err != nil {
			return nil, fmt.Errorf("%s window data: %w", s.entity, err)
		}
		result.Rows = append(result.Rows, WindowRow{Data: data, Value: val})
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("%s window rows: %w", s.entity, err)
	}
	return result, nil
}

// windowSpec builds the OVER (...) clause: PARTITION BY + ORDER BY.
func (s *EntityStore) windowSpec(params WindowParams) string {
	var parts []string
	if len(params.PartitionBy) > 0 {
		var cols []string
		for _, p := range params.PartitionBy {
			cols = append(cols, s.columnRefExpr(p))
		}
		parts = append(parts, "PARTITION BY "+strings.Join(cols, ", "))
	}
	if len(params.OrderBy) > 0 {
		var cols []string
		for _, o := range params.OrderBy {
			dir := "ASC"
			field := o
			if strings.HasPrefix(o, "-") {
				dir = "DESC"
				field = o[1:]
			}
			cols = append(cols, s.columnRefExpr(field)+" "+dir)
		}
		parts = append(parts, "ORDER BY "+strings.Join(cols, ", "))
	}
	return strings.Join(parts, " ")
}

// resolveRelations populates nested relation data for a list of records.
// For each belongs_to relation field, it batch-fetches the referenced
// records and adds them as nested objects keyed by relation alias.
// Example: field "patient_id" with relation resource "patient" →
//
//	records[i].Data["patient"] = {"id": "...", "name": "...", ...}
func (s *EntityStore) resolveRelations(ctx context.Context, records []EntityRecord, workspaceID string) {
	if len(records) == 0 {
		return
	}

	for _, f := range s.fields {
		if f.Relation == nil || f.Relation.Type != "belongs_to" {
			continue
		}

		// Determine relation alias: "patient_id" → "patient"
		alias := strings.TrimSuffix(f.Name, "_id")
		if alias == f.Name {
			alias = f.Relation.Resource // fallback to resource name
		}

		// Collect foreign key values and track which records reference them
		var ids []string
		idSeen := make(map[string]bool) // deduplicate
		type recordRef struct {
			idx      int
			sourceID string
		}
		var refs []recordRef

		for i, rec := range records {
			idVal, ok := rec.Data[f.Name]
			if !ok || idVal == nil {
				continue
			}
			idStr, ok := idVal.(string)
			if !ok || idStr == "" {
				continue
			}
			refs = append(refs, recordRef{idx: i, sourceID: rec.ID})
			if !idSeen[idStr] {
				idSeen[idStr] = true
				ids = append(ids, idStr)
			}
		}

		if len(ids) == 0 {
			continue
		}

		// Resolve target entity and table name
		targetModule := s.module
		targetEntity := f.Relation.Resource
		if dotIdx := strings.Index(targetEntity, "."); dotIdx >= 0 {
			targetModule = targetEntity[:dotIdx]
			targetEntity = targetEntity[dotIdx+1:]
		}

		// Cross-category JOIN block (4.4.2): a relation may not resolve across
		// persist categories (FORMSPEC.PERSIST.CROSS_CATEGORY). Skipped when no
		// category resolver is wired.
		if s.targetCategoryResolver != nil {
			targetCat := s.targetCategoryResolver(targetModule, targetEntity)
			if targetCat != "" && s.category != "" && targetCat != s.category {
				log.Printf("[WARN] resolve relation %s: cross-category JOIN blocked (%s vs %s) — FORMSPEC.PERSIST.CROSS_CATEGORY",
					f.Name, s.category, targetCat)
				continue
			}
		}

		targetTable := TableName(targetModule, targetEntity, "")
		if s.driver == DriverPostgres && s.schema != "" {
			targetTable = s.schema + "." + targetTable
		}

		// Batch query: SELECT id, data FROM target WHERE id IN (?,?,...)
		// Include tenant_id filter for workspace isolation (deleted_at guard
		// omitted because not every target table has soft delete enabled).
		placeholders := strings.Repeat("?,", len(ids))
		placeholders = placeholders[:len(placeholders)-1]
		q := fmt.Sprintf("SELECT id, data FROM %s WHERE id IN (%s) AND tenant_id = ?",
			targetTable, placeholders)
		idArgs := make([]any, 0, len(ids)+1)
		for _, id := range ids {
			idArgs = append(idArgs, id)
		}
		idArgs = append(idArgs, workspaceID)

		rows, err := txReadDB(ctx, s.db).QueryContext(ctx, q, idArgs...)
		if err != nil {
			log.Printf("[WARN] resolve relation %s: query %s: %v", f.Name, targetTable, err)
			continue
		}

		// Build lookup: relatedID → full record data (including id)
		relatedData := make(map[string]map[string]any, len(ids))
		for rows.Next() {
			var relID, dataStr string
			if err := rows.Scan(&relID, &dataStr); err != nil {
				log.Printf("[WARN] resolve relation %s: scan row: %v", f.Name, err)
				continue
			}
			var data map[string]any
			if err := json.Unmarshal([]byte(dataStr), &data); err != nil {
				log.Printf("[WARN] resolve relation %s: unmarshal data: %v", f.Name, err)
				continue
			}
			if data == nil {
				data = make(map[string]any)
			}
			data["id"] = relID // include id so frontend can reference it
			relatedData[relID] = data
		}
		rows.Close()

		if err := rows.Err(); err != nil {
			log.Printf("[WARN] resolve relation %s: rows iteration: %v", f.Name, err)
			continue
		}

		// Add nested data to source records
		for _, ref := range refs {
			fkVal, _ := records[ref.idx].Data[f.Name].(string)
			if data, ok := relatedData[fkVal]; ok {
				records[ref.idx].Data[alias] = data
			}
		}
	}
}

// FindByField finds a single entity record by a specific field value.
func (s *EntityStore) FindByField(ctx context.Context, workspaceID, field, value string) (*EntityRecord, error) {
	tbl := s.qualifiedTable()
	col := generatedColumnName(field)

	query := fmt.Sprintf(
		`SELECT id, tenant_id, version, created_at, updated_at, created_by, updated_by, doc_status, data FROM %s WHERE %s = ? AND tenant_id = ?`,
		tbl, col)
	if s.softDelete {
		query += " AND deleted_at IS NULL"
	}
	query += " LIMIT 1"

	rec, err := s.scanRecord(ctx, txReadDB(ctx, s.db), query, value, workspaceID)
	if err != nil {
		return nil, err
	}

	// Evaluate computed fields
	s.evaluateComputed(rec.Data)

	return rec, nil
}

// EntityRecord represents a single row from an entity table.
type EntityRecord struct {
	ID          string
	WorkspaceID string
	Version     int
	CreatedAt   string
	UpdatedAt   string
	CreatedBy   string
	UpdatedBy   string
	DocStatus   string // "" = lifecycle-free, "draft", "submitted", "cancelled"
	Data        map[string]any
}

// EffectiveDocStatus returns the document status. Empty string means lifecycle-free.
func (r *EntityRecord) EffectiveDocStatus() spec.DocStatus {
	return spec.DocStatus(r.DocStatus)
}

// MarshalJSON flattens Data alongside the record's own fields into one
// object, keyed the way the wire contract (docs/spec/backend/01-core-basic.md §8)
// and every client SDK expect — not a Go-field-cased, Data-nested struct.
// Reserved names (id, version, ...) always win on collision, matching the
// framework-owned-column convention elsewhere in the spec.
func (r EntityRecord) MarshalJSON() ([]byte, error) {
	out := make(map[string]any, len(r.Data)+8)
	for k, v := range r.Data {
		out[k] = v
	}
	out["id"] = r.ID
	out["tenant_id"] = r.WorkspaceID
	out["version"] = r.Version
	out["created_at"] = r.CreatedAt
	out["updated_at"] = r.UpdatedAt
	out["created_by"] = r.CreatedBy
	out["updated_by"] = r.UpdatedBy
	if r.DocStatus != "" {
		out["doc_status"] = r.DocStatus
	}
	return json.Marshal(out)
}

// scanRecord scans a single entity record from a query.
func (s *EntityStore) scanRecord(ctx context.Context, database DB, query string, args ...any) (*EntityRecord, error) {
	row := database.QueryRowContext(ctx, query, args...)
	return scanEntityRecord(row)
}

// scanEntityRecord scans a row into an EntityRecord.
func scanEntityRecord(row interface {
	Scan(dest ...any) error
}) (*EntityRecord, error) {
	var id, workspaceID, createdBy, updatedBy string
	var version int
	var createdAt, updatedAt string
	var docStatus sql.NullString // NULL = lifecycle-free
	var dataStr string

	err := row.Scan(&id, &workspaceID, &version, &createdAt, &updatedAt, &createdBy, &updatedBy, &docStatus, &dataStr)
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
		ID:          id,
		WorkspaceID: workspaceID,
		Version:     version,
		CreatedAt:   createdAt,
		UpdatedAt:   updatedAt,
		CreatedBy:   createdBy,
		UpdatedBy:   updatedBy,
		DocStatus:   docStatus.String, // empty string when NULL
		Data:        data,
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
	case []string:
		b.WriteByte('[')
		for i, item := range val {
			if i > 0 {
				b.WriteByte(',')
			}
			writeJSONValue(b, item)
		}
		b.WriteByte(']')
	default:
		// Fall back to encoding/json for any other type (slices of ints,
		// nested structures, etc.) so we never emit malformed JSON.
		enc, err := json.Marshal(val)
		if err != nil {
			b.WriteString("null")
			return
		}
		b.Write(enc)
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
	// Inject the entity's backdate limit so computed fields (e.g. is_stale)
	// can reference it without hardcoding — follows config changes (3 ↔ 4 days).
	backdateLimit := 3
	if s.backdatePolicy != nil && s.backdatePolicy.MaxDaysBack > 0 {
		backdateLimit = s.backdatePolicy.MaxDaysBack
	}
	for _, f := range s.computedFields {
		if f.Computed == nil || f.Computed.Formula == "" {
			continue
		}
		env := make(map[string]any, len(data)+1)
		for k, v := range data {
			env[k] = v
		}
		env["backdate_limit_days"] = backdateLimit
		val, err := starlark.EvalExpr(f.Computed.Formula, env)
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
				// Build combined data from old+new (new values take precedence)
				combined := make(map[string]any, len(oldData)+len(newData))
				for k, v := range oldData {
					combined[k] = v
				}
				for k, v := range newData {
					combined[k] = v
				}
				// Build env from combined data, then inject resource/data
				// aliases pointing TO combined, NOT to env itself — otherwise
				// toStarlark hits infinite recursion on the circular map.
				// Matches evaluateGuard in internal/entity/state_machine.go.
				env := make(map[string]any, len(combined)+2)
				for k, v := range combined {
					env[k] = v
				}
				env["resource"] = combined
				env["data"] = combined

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
					return fmt.Errorf("%w: %s", ErrValidationRule, msg)
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
		// Check decimal places using string-based counting instead of float truncation
		// to avoid floating-point precision issues with large/float-imprecise numbers.
		decimalPlaces := countDecimalPlaces(num)
		if decimalPlaces > prec {
			return fmt.Errorf("%w: %q: maximum %d decimal places allowed, got %d", ErrValidationRule, fieldName, prec, decimalPlaces)
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

// uuidRegex matches standard UUID format (xxxxxxxx-xxxx-xxxx-xxxx-xxxxxxxxxxxx).
// Used by looksLikeUUID to distinguish UUID primary keys from natural key values.
var uuidRegex = regexp.MustCompile(`(?i)^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$`)

// looksLikeUUID reports whether s matches the standard UUID format. When an
// entity declares a natural key, GetByID uses this to skip the UUID primary-key
// lookup and go straight to the natural key lookup — saving one unnecessary
// database round-trip per natural-key-based fetch.
func looksLikeUUID(s string) bool {
	return uuidRegex.MatchString(s)
}

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

// countDecimalPlaces returns the number of decimal places in a float64 value
// using string-based counting to avoid floating-point precision issues.
func countDecimalPlaces(num float64) int {
	s := fmt.Sprintf("%.10f", num)
	// Remove trailing zeros
	for len(s) > 0 && s[len(s)-1] == '0' {
		s = s[:len(s)-1]
	}
	if dotIdx := strings.Index(s, "."); dotIdx >= 0 {
		return len(s) - dotIdx - 1
	}
	return 0
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

// ErrUnknownField is returned when data contains a field that is not declared in the entity spec.
var ErrUnknownField = fmt.Errorf("unknown field")
