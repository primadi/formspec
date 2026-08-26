package spec

import (
	"fmt"
	"strings"

	"gopkg.in/yaml.v3"
)

// DocStatus is the built-in document lifecycle state (Core §4.1).
// NULL means lifecycle-free — the document does not participate in the lifecycle.
// @schema {description: "Document lifecycle state (empty = lifecycle-free)", enum: ["draft", "submitted", "cancelled"]}
type DocStatus string

const (
	DocStatusDraft     DocStatus = "draft"
	DocStatusSubmitted DocStatus = "submitted"
	DocStatusCancelled DocStatus = "cancelled"
)

// IsValidDocStatus returns true if s is a recognized doc_status value (including empty for lifecycle-free).
func IsValidDocStatus(s string) bool {
	switch DocStatus(s) {
	case DocStatusDraft, DocStatusSubmitted, DocStatusCancelled, "":
		return true
	default:
		return false
	}
}

// ReservedFieldNames lists fields auto-managed by the framework. Custom fields MUST NOT use these names.
var ReservedFieldNames = []string{"owner", "created_at", "modified", "doc_status", "amends", "amended_by", "version"}

// IsReservedField returns true if name is a reserved field.
func IsReservedField(name string) bool {
	for _, r := range ReservedFieldNames {
		if name == r {
			return true
		}
	}
	return false
}

// ReservedActionNames lists built-in actions with framework-enforced guards.
var ReservedActionNames = []string{"create", "update", "submit", "cancel", "delete", "amend", "create-submit", "amend-submit"}

// IsReservedAction returns true if name is a reserved action.
func IsReservedAction(name string) bool {
	for _, a := range ReservedActionNames {
		if a == name {
			return true
		}
	}
	return false
}

// IsDerivedReservedAction returns true for auto-derived actions (create-submit, amend-submit).
func IsDerivedReservedAction(name string) bool {
	return name == "create-submit" || name == "amend-submit"
}

// OnDelete specifies behavior when a referenced document is deleted (Core §10.5).
// @schema {description: "Referential action on target delete", enum: ["restrict", "cascade", "set_null"]}
type OnDelete string

const (
	OnDeleteRestrict OnDelete = "restrict" // default — block deletion
	OnDeleteCascade  OnDelete = "cascade"  // delete child rows
	OnDeleteSetNull  OnDelete = "set_null" // set FK to NULL
)

// EntitySpec defines a stateful, persisted business data resource (Core §4.1).
type EntitySpec struct {
	// @schema {example: "v1"}
	Version string `yaml:"version" json:"version"`
	// @schema {example: "invoices"}
	Plural            string              `yaml:"plural,omitempty" json:"plural,omitempty"`
	Characteristic    Characteristic      `yaml:"characteristic,omitempty" json:"characteristic,omitempty"`
	Auth              *EntityAuth         `yaml:"auth,omitempty" json:"auth,omitempty"`
	Persist           *PersistSpec        `yaml:"persist,omitempty" json:"persist,omitempty"`
	Fields            []Field             `yaml:"fields" json:"fields"`
	Actions           []Action            `yaml:"actions" json:"actions"`
	StateMachine      *StateMachine       `yaml:"state_machine,omitempty" json:"state_machine,omitempty"`
	Events            []EventDecl         `yaml:"events,omitempty" json:"events,omitempty"`
	Deliver           []DeliveryDecl      `yaml:"deliver,omitempty" json:"deliver,omitempty"`
	Indexes           []IndexDecl         `yaml:"indexes,omitempty" json:"indexes,omitempty"`
	ExtendStorage     *ExtendStorage      `yaml:"extend_storage,omitempty" json:"extend_storage,omitempty"`
	Expose            []ExposeConfig      `yaml:"expose,omitempty" json:"expose,omitempty"`
	BackdatePolicy    *BackdatePolicy     `yaml:"backdate_policy,omitempty" json:"backdate_policy,omitempty"`
	ForwardDatePolicy *ForwardDatePolicy  `yaml:"forward_date_policy,omitempty" json:"forward_date_policy,omitempty"`
	Hooks             []HookDecl          `yaml:"hooks,omitempty" json:"hooks,omitempty"`
	RateLimit         *RateLimitSpec      `yaml:"rate_limit,omitempty" json:"rate_limit,omitempty"`           // 1.4.1 resource-level rate limit (02-core-extended.md §17)
	SoftDeactivate    *SoftDeactivateDecl `yaml:"soft_deactivate,omitempty" json:"soft_deactivate,omitempty"` // 1.4.10
	// @schema {example: "plain_crud", enum: ["two_step_autosave", "two_step_manual", "plain_crud"]}
	Lifecycle string `yaml:"lifecycle,omitempty" json:"lifecycle,omitempty"` // explicit frontend lifecycle: two_step_autosave | two_step_manual | plain_crud (default derived from actions)
	// @schema {example: "name"}
	DisplayField    string `yaml:"display_field,omitempty" json:"display_field,omitempty"` // field name used as display label for this entity in relation pickers & detail pages
	NaturalKeyField string `yaml:"-" json:"-"`                                             // resolved in ValidateEntitySpec: empty if none, field name if exactly one natural_key
}

// DocumentSpec defines a stateful, persisted business data resource (Core §4.1).
// Deprecated: Renamed to EntitySpec in v0.3.0; kept for backward compatibility.
type DocumentSpec = EntitySpec

// ExposeConfig declares one external protocol surface for an Entity (D49).
// Each entry opts the entity into one protocol type. Without any expose entries,
// the entity is purely internal (same-process services, Starlark, events only).
type ExposeConfig struct {
	Type    ProtocolType `yaml:"type" json:"type"`                           // rest | grpc | ws
	Enabled bool         `yaml:"enabled,omitempty" json:"enabled,omitempty"` // for grpc/ws: true = full service
	Actions []string     `yaml:"actions,omitempty" json:"actions,omitempty"` // REST: list, find, create, update, delete
}

// ProtocolType identifies the external protocol for API delivery (§16).
// @schema {description: "External protocol type", enum: ["rest", "grpc", "ws"]}
type ProtocolType string

const (
	ProtocolREST      ProtocolType = "rest"
	ProtocolGRPC      ProtocolType = "grpc"
	ProtocolWebSocket ProtocolType = "ws"
)

// EntityAuth configures authentication for an entity.
type EntityAuth struct {
	Required   bool     `yaml:"required" json:"required"`
	Strategies []string `yaml:"strategies" json:"strategies"`
}

// Field defines a data field on an Entity.
// @schema {title: "Field Definition"}
type Field struct {
	// @schema {minLength: 1, maxLength: 128, pattern: "^[a-z][a-z0-9_]*$", description: "Field name — lowercase snake_case, unique within entity"}
	Name string `yaml:"name" json:"name"`
	// @schema {description: "Data type — determines storage class, renderer widget, and validation"}
	Type           FieldType           `yaml:"type" json:"type"`
	Title          string              `yaml:"title,omitempty" json:"title,omitempty"`
	Description    string              `yaml:"description,omitempty" json:"description,omitempty"`
	Default        any                 `yaml:"default,omitempty" json:"default,omitempty"`
	Required       bool                `yaml:"required,omitempty" json:"required,omitempty"`
	Immutable      bool                `yaml:"immutable,omitempty" json:"immutable,omitempty"`
	ReadOnly       bool                `yaml:"read_only,omitempty" json:"read_only,omitempty"`
	Unique         bool                `yaml:"unique,omitempty" json:"unique,omitempty"`
	Index          bool                `yaml:"index,omitempty" json:"index,omitempty"`
	NaturalKey     bool                `yaml:"natural_key,omitempty" json:"natural_key,omitempty"`
	NaturalKeyRule *NaturalKeyRuleDecl `yaml:"natural_key_rule,omitempty" json:"natural_key_rule,omitempty"`
	Audited        bool                `yaml:"audited,omitempty" json:"audited,omitempty"`
	EnumValues     []string            `yaml:"enum_values,omitempty" json:"enum_values,omitempty"`
	Rules          []ValidationRule    `yaml:"rules,omitempty" json:"rules,omitempty"`
	Relation       *RelationDecl       `yaml:"relation,omitempty" json:"relation,omitempty"`
	Child          *ChildDecl          `yaml:"child,omitempty" json:"child,omitempty"`
	Computed       *ComputedDecl       `yaml:"computed,omitempty" json:"computed,omitempty"`

	// Decimal precision/scale (05-field-types.md §1.2) — only meaningful for
	// type: decimal. Precision = total significant digits (left + right of the
	// point); Scale = digits after the decimal point (drives client-side input
	// limiting and server-side rounding/validation).
	Precision *int `yaml:"precision,omitempty" json:"precision,omitempty"`
	Scale     *int `yaml:"scale,omitempty" json:"scale,omitempty"`

	// Client-behavior vocabulary (FormSpecExpr, frontend 08-formspec-expr.md).
	// Normally declared in Form manifests; on entity fields they act as
	// defaults — and are the ONLY way to configure per-column behavior on
	// child fields (ChildTable reads child.fields from the entity).
	VisibleWhen  string `yaml:"visible_when,omitempty" json:"visible_when,omitempty"`
	ReadonlyWhen string `yaml:"readonly_when,omitempty" json:"readonly_when,omitempty"`
	RequiredWhen string `yaml:"required_when,omitempty" json:"required_when,omitempty"`

	// AutoFill — client-side lookup: when the relation field named in `from`
	// (same child row) changes, copy the related record's `field` into this
	// field. Declared on the target field (e.g. unit_price auto-filled from
	// menu_item_id → price). Server-side hooks remain the authority on write.
	AutoFill *AutoFillDecl `yaml:"auto_fill,omitempty" json:"auto_fill,omitempty"`

	// Extended field type structs (05-field-types.md §4–§5)

	// 1.4.9 TreeDecl — self-referential hierarchy marker (05-field-types.md §4).
	Tree bool `yaml:"tree,omitempty" json:"tree,omitempty"`

	// 1.4.3 FieldClassification — governance label (05-field-types.md §5.4).
	Classification FieldClassification `yaml:"classification,omitempty" json:"classification,omitempty"`

	// 1.4.4 FieldPermission — field-level required_permission (05-field-types.md §5.3).
	RequiredPermission string `yaml:"required_permission,omitempty" json:"required_permission,omitempty"`

	// 1.4.5 FieldExclude — per-surface field exclusion (05-field-types.md §5.3).
	Exclude []string `yaml:"exclude,omitempty" json:"exclude,omitempty"`

	// 1.4.6 EncryptedField — at-rest encryption marker (05-field-types.md §5.2).
	Encrypted bool `yaml:"encrypted,omitempty" json:"encrypted,omitempty"`

	// 1.4.7 MaskedField — auto-mask in response/log (05-field-types.md §5.2).
	Masked bool `yaml:"masked,omitempty" json:"masked,omitempty"`

	// 1.4.11 StorageSpec — file field configuration (05-field-types.md §1.3).
	// Only valid when Type == FieldFile.
	Storage *StorageSpec `yaml:"storage,omitempty" json:"storage,omitempty"`

	// 4.2.2 RenamedFrom — declares that this field was renamed from the given
	// old field name, so the migration engine treats it as a rename (two-phase
	// removal) instead of drop+add (01-core-basic.md §4).
	RenamedFrom string `yaml:"renamed_from,omitempty" json:"renamed_from,omitempty"`

	// Money (05-field-types.md §2) — only meaningful for type: money.
	// Currency is the explicit ISO-4217 code for this field; when empty the
	// field inherits settings.currency (never guessed). DecimalPlaces is the
	// fixed minor-unit scale — REQUIRED when Currency overrides the global
	// default (no currency catalog to look it up from).
	Currency      string `yaml:"currency,omitempty" json:"currency,omitempty"`
	DecimalPlaces *int   `yaml:"decimal_places,omitempty" json:"decimal_places,omitempty"`
}

// FieldType is the data type of a field (Core §10.1, 05-field-types.md §1.1).
// @schema {title: "Field Type", description: "Data type for a field — determines storage class, renderer widget, and validation rules"}
type FieldType string

const (
	FieldString     FieldType = "string"
	FieldText       FieldType = "text"     // multi-line text (05-field-types.md §1.1)
	FieldRichText   FieldType = "richtext" // rich markup (sanitized server-side)
	FieldInteger    FieldType = "integer"  // 64-bit integer
	FieldDecimal    FieldType = "decimal"  // arbitrary-precision — MUST be used for money, never float
	FieldMoney      FieldType = "money"    // {amount, currency} pair (05-field-types.md §2)
	FieldBoolean    FieldType = "boolean"
	FieldEnum       FieldType = "enum"
	FieldDate       FieldType = "date"
	FieldDateTime   FieldType = "datetime"
	FieldTime       FieldType = "time" // time-of-day (HH:MM:SS)
	FieldUUID       FieldType = "uuid"
	FieldJSON       FieldType = "json"
	FieldFile       FieldType = "file"       // reference to ctx.storage object (05-field-types.md §1.3)
	FieldAttachment FieldType = "attachment" // alias for FieldFile (05-field-types.md §1.3) — normalized to FieldFile at validate time
	FieldRelation   FieldType = "relation"
	FieldChild      FieldType = "child"

	// Deprecated: FieldNumber predates the spec's integer/decimal split (Core §10.1).
	// It is kept for backward compatibility and maps to the same storage as decimal.
	FieldNumber FieldType = "number"
)

// ValidationRule is a validation constraint on a field.
// @schema {description: "Validation constraint. Supports 4 YAML formats: string (\"required\"), colon (\"after:end_date\"), map-shorthand ({min_length: 1}), or full ({name: \"min_length\", value: 1})"}
type ValidationRule struct {
	Name  string `yaml:"name" json:"name"`
	Value any    `yaml:"value,omitempty" json:"value,omitempty"`
}

// UnmarshalYAML implements custom YAML decoding for ValidationRule.
// Supports four formats:
//   - String shorthand: "required" → {Name: "required"}
//   - Colon shorthand: "after:end_date" → {Name: "after", Value: "end_date"}
//   - Map shorthand: {min_length: 1} → {Name: "min_length", Value: 1}
//   - Full format: {name: "required", value: true} → {Name: "required", Value: true}
func (r *ValidationRule) UnmarshalYAML(value *yaml.Node) error {
	// Try string shorthand: "required"
	var s string
	if err := value.Decode(&s); err == nil {
		// Check for colon shorthand: "after:end_date", "before:start_date", "exists:customers"
		if colonIdx := strings.Index(s, ":"); colonIdx > 0 {
			prefix := s[:colonIdx]
			suffix := s[colonIdx+1:]
			switch prefix {
			case "after", "before", "exists":
				r.Name = prefix
				r.Value = suffix
				return nil
			}
		}
		r.Name = s
		return nil
	}

	// Try map format: {name: "required", value: 1} or {min_length: 1}
	var m map[string]any
	if err := value.Decode(&m); err == nil {
		// Check for full format: {name: "...", value: ...}
		if name, ok := m["name"]; ok {
			r.Name = fmt.Sprint(name)
			if v, ok := m["value"]; ok {
				r.Value = v
			}
			return nil
		}

		// Single-key shorthand: {min_length: 1}
		for k, v := range m {
			r.Name = k
			r.Value = v
			return nil // take first key only
		}
	}

	return fmt.Errorf("cannot unmarshal into ValidationRule: expected string or map")
}

// FieldRef references another entity's field.
type FieldRef struct {
	Entity string `yaml:"entity" json:"entity"`
	Field  string `yaml:"field" json:"field"`
}

// ChildDecl defines a child relationship.
type ChildDecl struct {
	Entity        string  `yaml:"entity,omitempty" json:"entity,omitempty"`
	Storage       string  `yaml:"storage" json:"storage"` // "jsonb" or "table"
	SequenceField string  `yaml:"sequence_field,omitempty" json:"sequence_field,omitempty"`
	Fields        []Field `yaml:"fields,omitempty" json:"fields,omitempty"`
}

// ComputedDecl defines a computed/derived field.
type ComputedDecl struct {
	Formula string `yaml:"formula" json:"formula"`
}

// AutoFillDecl defines a client-side lookup/auto-fill on a field.
// When the relation field named in `from` (same child row) changes, the
// related record is fetched and its `field` value is copied into the field
// that declares this AutoFillDecl.
type AutoFillDecl struct {
	// From — name of the relation field in the same row that triggers the fill.
	From string `yaml:"from" json:"from"`
	// Field — name of the field on the related entity to copy from.
	Field string `yaml:"field" json:"field"`
}

// ValidateEvents validates event naming conventions per Core §12:
//   - Events named before_* are sync gates and must not claim to be async.
//   - Events named on_* are async notifications and must not claim to be sync.
//   - Custom event names (no before_/on_ prefix) require an explicit type field.
func ValidateEvents(events []EventDecl) error {
	for _, e := range events {
		if err := ValidateEventNaming(e); err != nil {
			return err
		}
	}
	return nil
}

// ValidateEntitySpec validates an EntitySpec, returning an error if any constraint is violated.
// In addition to extension validation, it enforces:
//   - Reserved field names MUST NOT be reused.
//   - transaction_date field MUST be declared when characteristic: transaction.
//   - Event naming convention: before_* = sync, on_* = async (Core §12).
func ValidateEntitySpec(d *EntitySpec) error {
	// Normalize alias field types: `attachment` is an alias for `file`
	// (05-field-types.md §1.3). Do this before any storage/widget mapping so
	// every downstream path treats them identically.
	for i := range d.Fields {
		if d.Fields[i].Type == FieldAttachment {
			d.Fields[i].Type = FieldFile
		}
	}

	// Reserved field name check
	for _, f := range d.Fields {
		if IsReservedField(f.Name) {
			return fmt.Errorf("field %q is a reserved field name and cannot be used as a custom field", f.Name)
		}
	}

	// renamed_from (4.2.2): the old name must not collide with another field
	// in the same entity, and must not be a reserved name.
	seen := make(map[string]bool, len(d.Fields))
	for _, f := range d.Fields {
		seen[f.Name] = true
	}
	for _, f := range d.Fields {
		if f.RenamedFrom == "" {
			continue
		}
		if IsReservedField(f.RenamedFrom) {
			return fmt.Errorf("field %q: renamed_from %q is a reserved field name", f.Name, f.RenamedFrom)
		}
		if seen[f.RenamedFrom] {
			return fmt.Errorf("field %q: renamed_from %q collides with an existing field", f.Name, f.RenamedFrom)
		}
	}

	// natural_key_rule: strategy/reset enums (01-core-basic.md §2)
	for _, f := range d.Fields {
		if f.NaturalKeyRule == nil {
			continue
		}
		if !f.NaturalKey {
			return fmt.Errorf("field %q declares natural_key_rule without natural_key: true", f.Name)
		}
		switch f.NaturalKeyRule.Strategy {
		case "", "sequence", "custom":
		default:
			return fmt.Errorf("field %q: natural_key_rule.strategy must be \"sequence\" or \"custom\", got %q", f.Name, f.NaturalKeyRule.Strategy)
		}
		switch f.NaturalKeyRule.Reset {
		case "", "never", "yearly", "monthly", "daily":
		default:
			return fmt.Errorf("field %q: natural_key_rule.reset must be one of never|yearly|monthly|daily, got %q", f.Name, f.NaturalKeyRule.Reset)
		}

		// Reset requires a date placeholder in the format string —
		// otherwise the counter resets but still produces the same value
		// across periods, causing a UNIQUE constraint violation at insert
		// time (01-core-basic.md §2: natural key uniqueness per tenant).
		if f.NaturalKeyRule.Reset != "" && f.NaturalKeyRule.Reset != "never" {
			fmt_ := f.NaturalKeyRule.Format
			if fmt_ == "" {
				fmt_ = "{prefix}-{period}-{seq:05d}" // default format
			}
			hasPeriod := strings.Contains(fmt_, "{period}")
			hasYear := strings.Contains(fmt_, "{year}")
			hasMonth := strings.Contains(fmt_, "{month}")
			hasDay := strings.Contains(fmt_, "{day}")

			switch f.NaturalKeyRule.Reset {
			case "daily":
				if !hasPeriod && !hasDay {
					return fmt.Errorf("field %q: natural_key_rule.reset=daily requires {day} or {period} in format, got %q", f.Name, fmt_)
				}
			case "monthly":
				if !hasPeriod && !hasMonth {
					return fmt.Errorf("field %q: natural_key_rule.reset=monthly requires {month} or {period} in format, got %q", f.Name, fmt_)
				}
			case "yearly":
				if !hasPeriod && !hasYear {
					return fmt.Errorf("field %q: natural_key_rule.reset=yearly requires {year} or {period} in format, got %q", f.Name, fmt_)
				}
			}
		}
	}

	// Natural key cardinality (§2): at most one natural_key field per entity.
	// Resolve NaturalKeyField for fast O(1) lookup at the store layer.
	{
		var nkField string
		for _, f := range d.Fields {
			if f.NaturalKey {
				if nkField != "" {
					return fmt.Errorf("only one natural_key field allowed per entity (found %q and %q)", nkField, f.Name)
				}
				nkField = f.Name
			}
		}
		d.NaturalKeyField = nkField
	}

	// Money fields (05-field-types.md §2, todo 7.16): a field that overrides
	// `currency` to a code other than the global default MUST declare its own
	// `decimal_places` — no currency catalog to look it up from. The global
	// default comes from the workspace settings; at entity-apply time we
	// validate the structural rule (explicit currency + missing scale) which
	// is independent of the resolved settings value.
	for _, f := range d.Fields {
		if f.Type != FieldMoney {
			continue
		}
		if f.Currency != "" && f.DecimalPlaces == nil {
			return &MoneyFieldError{
				Field:   f.Name,
				Message: fmt.Sprintf("money field overrides currency to %q — must declare `decimal_places` (no currency catalog to look it up from)", f.Currency),
			}
		}
	}

	// Soft-deactivation pattern (1.4.10 / 4.10.2, 02-core-extended.md §19):
	// soft_deactivate: {enabled: true} adds an `is_active` boolean field
	// (default true) plus deactivate/reactivate actions. Inject the field if
	// the author did not declare it, so DDL generation, field validation, and
	// the store all see it uniformly.
	if d.SoftDeactivate != nil && d.SoftDeactivate.Enabled {
		hasIsActive := false
		for _, f := range d.Fields {
			if f.Name == "is_active" {
				hasIsActive = true
				break
			}
		}
		if !hasIsActive {
			d.Fields = append(d.Fields, Field{
				Name:    "is_active",
				Type:    FieldBoolean,
				Title:   "Active",
				Default: true,
			})
		}
	}

	// transaction_date required for characteristic: transaction
	if d.Characteristic == CharTransaction {
		hasTransactionDate := false
		for _, f := range d.Fields {
			if f.Name == "transaction_date" && (f.Type == FieldDate || f.Type == FieldDateTime) {
				hasTransactionDate = true
				break
			}
		}
		if !hasTransactionDate {
			return fmt.Errorf("document has characteristic: transaction but no transaction_date field declared (type: date or datetime)")
		}
	}

	// Extension validation
	if d.ExtendStorage != nil {
		for _, f := range d.Fields {
			if f.Required {
				return fmt.Errorf("extension field %q cannot be required", f.Name)
			}
		}

		target := d.ExtendStorage.Target
		if target == "" {
			return fmt.Errorf("extend_storage.target is required")
		}
		parts := strings.Split(target, "/")
		if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
			return fmt.Errorf("extend_storage.target must be in format \"module/entity\", got %q", target)
		}

		ns := d.ExtendStorage.Namespace
		if ns == "" {
			return fmt.Errorf("extend_storage.namespace is required")
		}
		if len(ns) < 3 || len(ns) > 32 {
			return fmt.Errorf("extend_storage.namespace must be 3-32 characters, got %d", len(ns))
		}
		for i, r := range ns {
			if i == 0 && !(r >= 'a' && r <= 'z') {
				return fmt.Errorf("extend_storage.namespace must start with a lowercase letter")
			}
			if !((r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '_') {
				return fmt.Errorf("extend_storage.namespace must contain only lowercase letters, digits, and underscores")
			}
		}
	}

	// Event naming validation (Core §12)
	if err := ValidateEvents(d.Events); err != nil {
		return err
	}

	// Event durability contract (2.4.3): publisher non-durable + subscriber durable = error
	if err := ValidateEventDurability(d.Events); err != nil {
		return err
	}

	// emits: must reference a declared event (Core §12)
	if err := ValidateActionEmits(d.Actions, d.Events); err != nil {
		return err
	}

	// Hooks spec validation (Core Extended §8)
	if err := ValidateHooks(d.Hooks, d.Actions); err != nil {
		return err
	}

	return nil
}

// ValidateDocumentSpec validates a DocumentSpec.
// Deprecated: renamed to ValidateEntitySpec; kept for backward compatibility.
func ValidateDocumentSpec(d *DocumentSpec) error {
	return ValidateEntitySpec(d)
}

// Action defines an operation on an Entity or Service.
type Action struct {
	Name               string           `yaml:"name" json:"name"`
	Description        string           `yaml:"description,omitempty" json:"description,omitempty"`
	RequiredPermission string           `yaml:"required_permission,omitempty" json:"required_permission,omitempty"`
	Idempotent         bool             `yaml:"idempotent,omitempty" json:"idempotent,omitempty"`
	IdempotencyKey     *IdempotencyDecl `yaml:"idempotency_key,omitempty" json:"idempotency_key,omitempty"`
	Audit              bool             `yaml:"audit,omitempty" json:"audit,omitempty"`
	Disabled           bool             `yaml:"disabled,omitempty" json:"disabled,omitempty"` // §11.1 — removes a standard action from every surface
	Call               string           `yaml:"call,omitempty" json:"call,omitempty"`         // sync (default) | async (§11.4)
	Track              bool             `yaml:"track,omitempty" json:"track,omitempty"`       // §13: call:async + track:true = tracked async job (job_id + progress)
	Callback           *CallbackDecl    `yaml:"callback,omitempty" json:"callback,omitempty"` // §13.1: callback webhook delivery for tracked async jobs
	Emits              string           `yaml:"emits,omitempty" json:"emits,omitempty"`       // event name linked per §12
	Expose             []string         `yaml:"expose,omitempty" json:"expose,omitempty"`     // per-action protocol filter: rest | grpc | ws
	Impl               *ImplDecl        `yaml:"impl,omitempty" json:"impl,omitempty"`
	Uses               *UsesDecl        `yaml:"uses,omitempty" json:"uses,omitempty"`
	Params             *ParamsDecl      `yaml:"params,omitempty" json:"params,omitempty"`
	Conditions         []ConditionDecl  `yaml:"conditions,omitempty" json:"conditions,omitempty"`
	UI                 *ActionUIHint    `yaml:"ui,omitempty" json:"ui,omitempty"`                 // Backend §5.1 — button rendering hints (confirm, icon, style, etc.)
	RateLimit          *RateLimitSpec   `yaml:"rate_limit,omitempty" json:"rate_limit,omitempty"` // 1.4.1 per-action override (02-core-extended.md §17)
}

// ActionUIHint carries frontend rendering hints for an action button (Backend §5.1).
type ActionUIHint struct {
	ButtonLabel string `yaml:"button_label,omitempty" json:"button_label,omitempty"`
	// @schema {description: "Button style variant", enum: ["primary", "secondary", "danger"]}
	Style   string `yaml:"style,omitempty" json:"style,omitempty"`
	Icon    string `yaml:"icon,omitempty" json:"icon,omitempty"`
	Confirm string `yaml:"confirm,omitempty" json:"confirm,omitempty"`
	// @schema {description: "FormSpecExpr — condition to show this action button"}
	ShowWhen string `yaml:"show_when,omitempty" json:"show_when,omitempty"`
}

// ImplDecl specifies how an action is implemented.
type ImplDecl struct {
	Type ImplType `yaml:"type" json:"type"`
	Ref  string   `yaml:"ref,omitempty" json:"ref,omitempty"`
}

// IdempotencyDecl specifies how idempotency keys are derived.
type IdempotencyDecl struct {
	From  string `yaml:"from" json:"from"`
	Field string `yaml:"field,omitempty" json:"field,omitempty"`
}

// UsesDecl declares what resources/primitives/config an action needs (§11.2).
type UsesDecl struct {
	Resources  []string          `yaml:"resources,omitempty" json:"resources,omitempty"`
	Db         *UsesDbDecl       `yaml:"db,omitempty" json:"db,omitempty"`
	Config     *UsesConfigDecl   `yaml:"config,omitempty" json:"config,omitempty"`
	Kvstore    []KvstoreUseDecl  `yaml:"kvstore,omitempty" json:"kvstore,omitempty"`
	Secrets    []string          `yaml:"secrets,omitempty" json:"secrets,omitempty"` // 1.4.2 ctx.secrets access keys (02-core-extended.md §18)
	Primitives []string          `yaml:"primitives,omitempty" json:"primitives,omitempty"`
	Datastores map[string]string `yaml:"datastores,omitempty" json:"datastores,omitempty"` // primitive → datastore name binding
}

// UsesDbDecl specifies raw database access per category/module (§11.2).
// Defaults to own module; cross-module write is high-risk consent (D46).
type UsesDbDecl struct {
	Read  []string `yaml:"read,omitempty" json:"read,omitempty"`
	Write []string `yaml:"write,omitempty" json:"write,omitempty"`
}

// UsesConfigDecl specifies config access.
type UsesConfigDecl struct {
	Read  []string `yaml:"read,omitempty" json:"read,omitempty"`
	Write []string `yaml:"write,omitempty" json:"write,omitempty"`
}

// KvstoreUseDecl specifies durable key-value store access (§11.2).
type KvstoreUseDecl struct {
	Scope  string `yaml:"scope,omitempty" json:"scope,omitempty"`   // workspace | module | global
	Access string `yaml:"access,omitempty" json:"access,omitempty"` // read | read_write
	Module string `yaml:"module,omitempty" json:"module,omitempty"`
}

// ParamsDecl defines input parameters for an action.
type ParamsDecl struct {
	Validate []ParamValidation `yaml:"validate,omitempty" json:"validate,omitempty"`
}

// ParamValidation validates an action parameter.
type ParamValidation struct {
	Field string           `yaml:"field" json:"field"`
	Rules []ValidationRule `yaml:"rules" json:"rules"`
}

// ConditionDecl is a guard condition for an action (§13 level 3).
// The canonical manifest form is {script, message}; {field, expression}
// is an older equivalent kept for backward compatibility.
type ConditionDecl struct {
	Script     string `yaml:"script,omitempty" json:"script,omitempty"`
	Message    string `yaml:"message,omitempty" json:"message,omitempty"`
	Field      string `yaml:"field,omitempty" json:"field,omitempty"`
	Expression string `yaml:"expression,omitempty" json:"expression,omitempty"`
}

// StateMachine defines entity lifecycle states and transitions.
type StateMachine struct {
	Field       string           `yaml:"field" json:"field"`
	Initial     string           `yaml:"initial" json:"initial"`
	States      []StateDecl      `yaml:"states" json:"states"`
	Transitions []TransitionDecl `yaml:"transitions" json:"transitions"`
}

// StateDecl defines a named state.
type StateDecl struct {
	Name        string `yaml:"name" json:"name"`
	Label       string `yaml:"label" json:"label"`
	Description string `yaml:"description,omitempty" json:"description,omitempty"`
}

// TransitionDecl defines a transition between states (Core §14).
// The canonical manifest key for the triggering action is `via`;
// `action` is accepted as a legacy alias.
type TransitionDecl struct {
	From   StateList  `yaml:"from" json:"from"`
	To     string     `yaml:"to" json:"to"`
	Action string     `yaml:"via" json:"via"`
	Guard  *GuardDecl `yaml:"guard,omitempty" json:"guard,omitempty"`
}

// UnmarshalYAML accepts both the canonical `via:` key and the legacy `action:` alias.
func (t *TransitionDecl) UnmarshalYAML(value *yaml.Node) error {
	var raw struct {
		From   StateList  `yaml:"from"`
		To     string     `yaml:"to"`
		Via    string     `yaml:"via"`
		Action string     `yaml:"action"`
		Guard  *GuardDecl `yaml:"guard"`
	}
	if err := value.Decode(&raw); err != nil {
		return err
	}
	t.From = raw.From
	t.To = raw.To
	t.Guard = raw.Guard
	t.Action = raw.Via
	if t.Action == "" {
		t.Action = raw.Action
	}
	return nil
}

// StateList holds one or more source states for a transition.
// YAML accepts either a scalar ("draft") or a sequence ([draft, awaiting_payment]).
type StateList []string

// UnmarshalYAML accepts a scalar state name or a list of state names.
func (s *StateList) UnmarshalYAML(value *yaml.Node) error {
	var one string
	if err := value.Decode(&one); err == nil {
		*s = StateList{one}
		return nil
	}
	var many []string
	if err := value.Decode(&many); err == nil {
		*s = StateList(many)
		return nil
	}
	return fmt.Errorf("cannot unmarshal into StateList: expected string or list of strings")
}

// Matches reports whether the given state is a valid source for this transition.
// The wildcard "*" matches any state.
func (s StateList) Matches(state string) bool {
	for _, from := range s {
		if from == state || from == "*" {
			return true
		}
	}
	return false
}

// GuardDecl is a condition that must be met for a transition.
// YAML accepts either an inline expression string (canonical, Core §14)
// or a map with explicit expression/message keys.
type GuardDecl struct {
	Expression string `yaml:"expression" json:"expression"`
	Message    string `yaml:"message,omitempty" json:"message,omitempty"`
}

// UnmarshalYAML accepts a scalar expression or the {expression, message} map form.
func (g *GuardDecl) UnmarshalYAML(value *yaml.Node) error {
	var expr string
	if err := value.Decode(&expr); err == nil {
		g.Expression = expr
		return nil
	}
	var raw struct {
		Expression string `yaml:"expression"`
		Message    string `yaml:"message"`
	}
	if err := value.Decode(&raw); err != nil {
		return err
	}
	g.Expression = raw.Expression
	g.Message = raw.Message
	return nil
}

// HookTiming is when a hook runs relative to its action or event (Core Extended §8).
// @schema {description: "When the hook executes relative to the action/event", enum: ["before", "after", "on_error", "before_deliver", "after_deliver"]}
type HookTiming string

const (
	HookOnBefore        HookTiming = "before"         // sync, gates the action — may abort via fail()
	HookOnAfter         HookTiming = "after"          // sync, runs post-commit — best-effort, cannot abort
	HookOnError         HookTiming = "on_error"       // runs when the action (or a before hook) failed
	HookOnBeforeDeliver HookTiming = "before_deliver" // sync, gates event delivery — may suppress it
	HookOnAfterDeliver  HookTiming = "after_deliver"  // runs after an event was delivered
)

// HookDecl attaches handler code to an action's before/after/on_error point,
// or an event's before_deliver/after_deliver point (Core Extended §8). A
// reserved action's own Impl (Action.Impl) is equivalent to one hook scoped
// to that action, running last in the before-phase — hooks: entries let
// other modules/scripts attach additional code to the same points, by name
// or via the "*" wildcard, with priority ordering (default 10; lower runs
// first).
type HookDecl struct {
	On       HookTiming `yaml:"on" json:"on"`
	Action   string     `yaml:"action,omitempty" json:"action,omitempty"` // action name or "*" — before/after/on_error only
	Event    string     `yaml:"event,omitempty" json:"event,omitempty"`   // event name or "*" — before_deliver/after_deliver only
	Impl     *ImplDecl  `yaml:"impl" json:"impl"`
	Priority int        `yaml:"priority,omitempty" json:"priority,omitempty"` // 0 → default 10
}

// ValidateHooks checks each hook's on/action/event shape against actions.
// A hook on: before|after|on_error must set action (name or "*") and must
// not set event; a hook on: before_deliver|after_deliver is the reverse.
func ValidateHooks(hooks []HookDecl, actions []Action) error {
	for _, h := range hooks {
		switch h.On {
		case HookOnBefore, HookOnAfter, HookOnError:
			if h.Action == "" {
				return fmt.Errorf("hook on:%s requires action (name or \"*\")", h.On)
			}
			if h.Event != "" {
				return fmt.Errorf("hook on:%s must not set event", h.On)
			}
			if h.Action != "*" && !actionExists(h.Action, actions) {
				return fmt.Errorf("hook action %q does not match any declared action", h.Action)
			}
		case HookOnBeforeDeliver, HookOnAfterDeliver:
			if h.Event == "" {
				return fmt.Errorf("hook on:%s requires event (name or \"*\")", h.On)
			}
			if h.Action != "" {
				return fmt.Errorf("hook on:%s must not set action", h.On)
			}
		default:
			return fmt.Errorf("hook on:%q invalid — must be before|after|on_error|before_deliver|after_deliver", h.On)
		}
		if h.Impl == nil {
			return fmt.Errorf("hook on:%s action:%s event:%s has no impl", h.On, h.Action, h.Event)
		}
	}
	return nil
}

func actionExists(name string, actions []Action) bool {
	if IsReservedAction(name) {
		return true // reserved actions (create/update/...) are always implicitly available as hook points, even if not explicitly declared in actions:
	}
	for _, a := range actions {
		if a.Name == name {
			return true
		}
	}
	return false
}

// ValidateActionEmits checks that every action's emits (if set) names a
// declared event.
func ValidateActionEmits(actions []Action, events []EventDecl) error {
	for _, a := range actions {
		if a.Emits == "" {
			continue
		}
		found := false
		for _, e := range events {
			if e.Name == a.Emits {
				found = true
				break
			}
		}
		if !found {
			return fmt.Errorf("action %q emits %q, which is not declared in events:", a.Name, a.Emits)
		}
	}
	return nil
}

// ValidateEventDurability checks durability contract (2.4.3):
// publisher non-durable + subscriber durable = validation error.
// A subscriber is durable if any of its deliver channels use durable delivery
// (reliable_event, queue, or websocket when the event is durable).
func ValidateEventDurability(events []EventDecl) error {
	for _, e := range events {
		isDurable := e.Publish != nil && e.Publish.Durable
		for _, d := range e.Deliver {
			if d.Channel == "reliable_event" || d.Channel == "queue" {
				if !isDurable {
					return fmt.Errorf("event %q: channel %q requires publish.durable: true", e.Name, d.Channel)
				}
			}
		}
	}
	return nil
}

// EventDecl declares an event an entity can publish (Core §12).
type EventDecl struct {
	Name        string              `yaml:"name" json:"name"`
	Description string              `yaml:"description,omitempty" json:"description,omitempty"`
	Type        string              `yaml:"type,omitempty" json:"type,omitempty"` // sync | async — auto-derived from name prefix if not explicit
	Publish     *PublishDecl        `yaml:"publish,omitempty" json:"publish,omitempty"`
	Payload     *PayloadDecl        `yaml:"payload,omitempty" json:"payload,omitempty"`
	Deliver     []EventDeliveryDecl `yaml:"deliver,omitempty" json:"deliver,omitempty"`
}

// EventType constants.
const (
	EventTypeSync  = "sync"
	EventTypeAsync = "async"
)

// ValidateEventNaming checks event name prefix convention and returns the implied type.
// Rules per §12:
//   - before_* → sync
//   - on_* → async
//   - custom names → type MUST be explicit
//
// Returns error with FORMSPEC.EVENT.* codes (2.4.5).
func ValidateEventNaming(event EventDecl) error {
	isBefore := strings.HasPrefix(event.Name, "before_")
	isOn := strings.HasPrefix(event.Name, "on_")

	if !isBefore && !isOn {
		if event.Type == "" {
			return fmt.Errorf("[FORMSPEC.EVENT.TYPE_MISSING] event %q: type is required for custom events (not prefixed before_/on_)", event.Name)
		}
		if event.Type != EventTypeSync && event.Type != EventTypeAsync {
			return fmt.Errorf("[FORMSPEC.EVENT.TYPE_MISMATCH] event %q: type must be 'sync' or 'async', got %q", event.Name, event.Type)
		}
		return nil
	}

	if isBefore {
		if event.Type != "" && event.Type != EventTypeSync {
			return fmt.Errorf("[FORMSPEC.EVENT.TYPE_MISMATCH] event %q: before_* events must be 'sync', got %q", event.Name, event.Type)
		}
	}
	if isOn {
		if event.Type != "" && event.Type != EventTypeAsync {
			return fmt.Errorf("[FORMSPEC.EVENT.TYPE_MISMATCH] event %q: on_* events must be 'async', got %q", event.Name, event.Type)
		}
	}

	return nil
}

// PublishDecl configures how an event is published.
type PublishDecl struct {
	Durable bool `yaml:"durable,omitempty" json:"durable,omitempty"`
}

// PayloadDecl declares which entity fields are carried in the event payload (§12).
type PayloadDecl struct {
	Fields []string `yaml:"fields" json:"fields"`
}

// EventDeliveryDecl is one delivery target of an event — its "consequence map" entry (§12).
type EventDeliveryDecl struct {
	Channel        string          `yaml:"channel" json:"channel"` // audit_log | websocket | queue | reliable_event
	Target         *DeliveryTarget `yaml:"target,omitempty" json:"target,omitempty"`
	Job            string          `yaml:"job,omitempty" json:"job,omitempty"`
	Retry          *RetryDecl      `yaml:"retry,omitempty" json:"retry,omitempty"`
	DeadLetter     *DeliveryTarget `yaml:"dead_letter,omitempty" json:"dead_letter,omitempty"`
	IdempotencyKey string          `yaml:"idempotency_key,omitempty" json:"idempotency_key,omitempty"`
}

// DeliveryTarget addresses the receiver of an event delivery.
type DeliveryTarget struct {
	Scope    string `yaml:"scope,omitempty" json:"scope,omitempty"`       // websocket: workspace | user | ...
	Resource string `yaml:"resource,omitempty" json:"resource,omitempty"` // reliable_event: "module.entity"
	Action   string `yaml:"action,omitempty" json:"action,omitempty"`
}

// RetryDecl configures retry behavior for reliable event delivery.
type RetryDecl struct {
	Max            int    `yaml:"max,omitempty" json:"max,omitempty"`
	Backoff        string `yaml:"backoff,omitempty" json:"backoff,omitempty"` // exponential | linear | fixed
	InitialDelayMs int    `yaml:"initial_delay_ms,omitempty" json:"initial_delay_ms,omitempty"`
}

// CallbackDecl configures callback webhook delivery for a tracked async job
// (02-core-extended.md §13.1, todo 7.13.4). The callback URL is supplied by
// the caller via a request header; the result is delivered HMAC-signed.
type CallbackDecl struct {
	Channel string     `yaml:"channel" json:"channel"`                       // webhook
	URLFrom string     `yaml:"url_from,omitempty" json:"url_from,omitempty"` // header
	Header  string     `yaml:"header,omitempty" json:"header,omitempty"`     // header carrying the callback URL (e.g. X-Callback-URL)
	Sign    bool       `yaml:"sign,omitempty" json:"sign,omitempty"`
	Retry   *RetryDecl `yaml:"retry,omitempty" json:"retry,omitempty"`
}

// DeliveryDecl configures what happens when an event fires.
//
// Deprecated: predates the nested event `deliver:` list (Core §12); kept for
// backward compatibility with manifests using a top-level `deliver:` block.
type DeliveryDecl struct {
	Event         string `yaml:"event" json:"event"`
	ReliableEvent bool   `yaml:"reliable_event,omitempty" json:"reliable_event,omitempty"`
	WebSocket     bool   `yaml:"websocket,omitempty" json:"websocket,omitempty"`
	Channel       string `yaml:"channel,omitempty" json:"channel,omitempty"`
}

// IndexDecl declares a database index.
type IndexDecl struct {
	Fields []string `yaml:"fields" json:"fields"`
	Unique bool     `yaml:"unique,omitempty" json:"unique,omitempty"`
}

// RelationDecl defines a relation to another entity.
// @schema {title: "Relation Declaration"}
type RelationDecl struct {
	// @schema {description: "Relation type: belongs_to (FK on this entity), has_many (FK on target), has_one (FK on target, unique)", enum: ["belongs_to", "has_many", "has_one"]}
	Type string `yaml:"type" json:"type"`
	// @schema {description: "Target entity \"module/entity\" or just \"entity\" within same module"}
	Resource   string   `yaml:"resource" json:"resource"`
	ForeignKey string   `yaml:"foreign_key,omitempty" json:"foreign_key,omitempty"`
	OnDelete   OnDelete `yaml:"on_delete,omitempty" json:"on_delete,omitempty"` // restrict | cascade | set_null (v0.3.0)
	// Snapshot copies master financial fields to the transaction at
	// create/submit (02-core-extended.md §1.1, todo 7.10) — denormalisasi
	// finansial. Values are copied (not live-joined), so old transactions are
	// unaffected by later master changes.
	Snapshot []SnapshotField `yaml:"snapshot,omitempty" json:"snapshot,omitempty"`
}

// SnapshotField copies one master field into the transaction at create/submit
// (02-core-extended.md §1.1, todo 7.10).
type SnapshotField struct {
	// From is the master field name to copy.
	From string `yaml:"from" json:"from"`
	// As is the target field name on the transaction (default = From).
	As string `yaml:"as,omitempty" json:"as,omitempty"`
}

// NaturalKeyRuleDecl defines how a natural key is generated.
type NaturalKeyRuleDecl struct {
	Strategy   string            `yaml:"strategy" json:"strategy"` // sequence | custom
	Format     string            `yaml:"format,omitempty" json:"format,omitempty"`
	Prefix     *NaturalKeyPrefix `yaml:"prefix,omitempty" json:"prefix,omitempty"`
	Reset      string            `yaml:"reset,omitempty" json:"reset,omitempty"` // never | yearly | monthly | daily
	ScopeField string            `yaml:"scope_field,omitempty" json:"scope_field,omitempty"`
	// ScopeField names a field on the same entity (e.g. "branch_id") whose value
	// isolates the counter — the counter table is already keyed by
	// (tenant_id, resource, field, scope, period); ScopeField supplies that
	// scope. Empty (the default) reproduces prior behavior: one counter shared
	// across the whole workspace.
}

// NaturalKeyPrefix defines the prefix for a natural key.
type NaturalKeyPrefix struct {
	Config  string `yaml:"config,omitempty" json:"config,omitempty"`
	Default string `yaml:"default,omitempty" json:"default,omitempty"`
	Value   string `yaml:"value,omitempty" json:"value,omitempty"`
}

// ExtendStorage declares that this Entity extends another entity's storage
// by adding a new JSONB column (namespace-isolated) to the target table.
//
// target:  "module/entity" of the base entity being extended
// namespace:  unique identifier for this extension column (ext_{namespace})
//
// Extension fields MUST NOT be required (required=true is rejected at validation).
// Extension data is stored in a separate column ext_{namespace}, not in the base data JSONB.
// This allows clean uninstall via DROP COLUMN without affecting base data.
type ExtendStorage struct {
	Target    string `yaml:"target" json:"target"`
	Namespace string `yaml:"namespace" json:"namespace"`
	// Validate is an additive business rule (4.3.5): a Starlark script ref
	// that runs after the base entity's L1–L6 validation. It never overrides
	// base validation, has read-only access to base fields, and may only
	// require its own namespaced fields.
	Validate string `yaml:"validate,omitempty" json:"validate,omitempty"`
}

// BackdatePolicy controls how far back a transaction_date can be set (§14a).
type BackdatePolicy struct {
	MaxDaysBack        int    `yaml:"max_days_back,omitempty" json:"max_days_back,omitempty"`
	OverridePermission string `yaml:"override_permission,omitempty" json:"override_permission,omitempty"`
}

// ForwardDatePolicy controls how far forward a transaction_date can be set (§14a).
// Default max_days_forward = 0 (no forward dating allowed).
type ForwardDatePolicy struct {
	MaxDaysForward     int    `yaml:"max_days_forward,omitempty" json:"max_days_forward,omitempty"`
	OverridePermission string `yaml:"override_permission,omitempty" json:"override_permission,omitempty"`
}

// PersistSpec controls how an entity is stored (Core §19).
type PersistSpec struct {
	Table string `yaml:"table,omitempty" json:"table,omitempty"`
	// SoftDelete defaults to true; nil means "not set" so a persist block
	// that omits soft_delete does not silently disable it.
	SoftDelete *bool       `yaml:"soft_delete,omitempty" json:"soft_delete,omitempty"`
	Category   string      `yaml:"category,omitempty" json:"category,omitempty"` // operational | financial | compliance | analytics | master | archive
	Indexes    []IndexDecl `yaml:"indexes,omitempty" json:"indexes,omitempty"`
}

// ─── 1.4 Extended Field Type Structs ───

// 1.4.1 RateLimitSpec — per-resource or per-action rate limit (02-core-extended.md §17).
type RateLimitSpec struct {
	Max      int    `yaml:"max" json:"max"`
	Per      string `yaml:"per" json:"per"`                               // e.g. "1s", "1m", "1h"
	Scope    string `yaml:"scope,omitempty" json:"scope,omitempty"`       // tenant | user | ip | global
	Strategy string `yaml:"strategy,omitempty" json:"strategy,omitempty"` // sliding_window | token_bucket
}

// 1.4.3 FieldClassification — governance label for field-level data sensitivity
// (05-field-types.md §5.4).
// @schema {description: "Data sensitivity classification for governance", enum: ["pii", "financial", "internal"]}
type FieldClassification string

const (
	ClassificationPII       FieldClassification = "pii"
	ClassificationFinancial FieldClassification = "financial"
	ClassificationInternal  FieldClassification = "internal"
)

// 1.4.9 TreeDecl — self-referential hierarchy marker (05-field-types.md §4).
// Applied via Field.Tree = true on a relation field.
// No separate struct needed — the `tree: true` boolean on Field is sufficient.
// This type exists for documentation and potential future expansion.
type TreeDecl struct {
	Enabled bool `yaml:"enabled,omitempty" json:"enabled,omitempty"`
}

// 1.4.10 SoftDeactivateDecl — soft-deactivation pattern for master entities
// (02-core-extended.md §19). Adds is_active field + deactivate/reactivate actions.
type SoftDeactivateDecl struct {
	Enabled bool `yaml:"enabled,omitempty" json:"enabled,omitempty"`
}

// 1.4.11 StorageSpec — file field configuration (05-field-types.md §1.3).
// Only valid on Field.Type == FieldFile.
type StorageSpec struct {
	AllowedTypes []string           `yaml:"allowed_types,omitempty" json:"allowed_types,omitempty"`
	MaxSizeMB    int                `yaml:"max_size_mb,omitempty" json:"max_size_mb,omitempty"`
	MaxCount     int                `yaml:"max_count,omitempty" json:"max_count,omitempty"`
	Visibility   string             `yaml:"visibility,omitempty" json:"visibility,omitempty"`         // public | private | signed
	SignedURLTTL string             `yaml:"signed_url_ttl,omitempty" json:"signed_url_ttl,omitempty"` // e.g. "15m"
	CDN          bool               `yaml:"cdn,omitempty" json:"cdn,omitempty"`
	Transform    []StorageTransform `yaml:"transform,omitempty" json:"transform,omitempty"`
}

// StorageTransform declares an image transformation preset for a file field.
type StorageTransform struct {
	Name   string `yaml:"name" json:"name"`
	Width  int    `yaml:"width,omitempty" json:"width,omitempty"`
	Height int    `yaml:"height,omitempty" json:"height,omitempty"`
	Fit    string `yaml:"fit,omitempty" json:"fit,omitempty"` // cover | contain | fill
}
