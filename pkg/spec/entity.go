package spec

import (
	"fmt"
	"strings"

	"gopkg.in/yaml.v3"
)

// DocStatus is the built-in document lifecycle state (Core §4.1).
// NULL means lifecycle-free — the document does not participate in the lifecycle.
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
type OnDelete string

const (
	OnDeleteRestrict OnDelete = "restrict" // default — block deletion
	OnDeleteCascade  OnDelete = "cascade"  // delete child rows
	OnDeleteSetNull  OnDelete = "set_null" // set FK to NULL
)

// EntitySpec defines a stateful, persisted business data resource (Core §4.1).
// Deprecated: Use DocumentSpec for new code. EntitySpec is kept for backward compatibility.
type EntitySpec = DocumentSpec

// DocumentSpec defines a stateful, persisted business data resource (Core §4.1).
// Renamed from EntitySpec in v0.3.0.
type DocumentSpec struct {
	Version           string             `yaml:"version" json:"version"`
	Plural            string             `yaml:"plural,omitempty" json:"plural,omitempty"`
	Characteristic    Characteristic     `yaml:"characteristic,omitempty" json:"characteristic,omitempty"`
	Auth              *EntityAuth        `yaml:"auth,omitempty" json:"auth,omitempty"`
	Persist           *PersistSpec       `yaml:"persist,omitempty" json:"persist,omitempty"`
	Fields            []Field            `yaml:"fields" json:"fields"`
	Actions           []Action           `yaml:"actions" json:"actions"`
	StateMachine      *StateMachine      `yaml:"state_machine,omitempty" json:"state_machine,omitempty"`
	Events            []EventDecl        `yaml:"events,omitempty" json:"events,omitempty"`
	Deliver           []DeliveryDecl     `yaml:"deliver,omitempty" json:"deliver,omitempty"`
	Indexes           []IndexDecl        `yaml:"indexes,omitempty" json:"indexes,omitempty"`
	Tenant            *TenantDecl        `yaml:"tenant,omitempty" json:"tenant,omitempty"`
	ExtendStorage     *ExtendStorage     `yaml:"extend_storage,omitempty" json:"extend_storage,omitempty"`
	Expose            []ExposeConfig     `yaml:"expose,omitempty" json:"expose,omitempty"`
	BackdatePolicy    *BackdatePolicy    `yaml:"backdate_policy,omitempty" json:"backdate_policy,omitempty"`
	ForwardDatePolicy *ForwardDatePolicy `yaml:"forward_date_policy,omitempty" json:"forward_date_policy,omitempty"`
	Hooks             []HookDecl         `yaml:"hooks,omitempty" json:"hooks,omitempty"`
}

// ExposeConfig declares one external protocol surface for an Entity (D49).
// Each entry opts the entity into one protocol type. Without any expose entries,
// the entity is purely internal (same-process services, Starlark, events only).
type ExposeConfig struct {
	Type    ProtocolType `yaml:"type" json:"type"`                           // rest | grpc | ws
	Enabled bool         `yaml:"enabled,omitempty" json:"enabled,omitempty"` // for grpc/ws: true = full service
	Actions []string     `yaml:"actions,omitempty" json:"actions,omitempty"` // REST: list, find, create, update, delete
}

// ProtocolType identifies the external protocol for API delivery (§16).
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
type Field struct {
	Name           string              `yaml:"name" json:"name"`
	Type           FieldType           `yaml:"type" json:"type"`
	Title          string              `yaml:"title,omitempty" json:"title,omitempty"`
	Description    string              `yaml:"description,omitempty" json:"description,omitempty"`
	Default        any                 `yaml:"default,omitempty" json:"default,omitempty"`
	Required       bool                `yaml:"required,omitempty" json:"required,omitempty"`
	Immutable      bool                `yaml:"immutable,omitempty" json:"immutable,omitempty"`
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
}

// FieldType is the data type of a field (Core §10.1).
type FieldType string

const (
	FieldString   FieldType = "string"
	FieldInteger  FieldType = "integer" // 64-bit integer
	FieldDecimal  FieldType = "decimal" // arbitrary-precision — MUST be used for money, never float
	FieldBoolean  FieldType = "boolean"
	FieldEnum     FieldType = "enum"
	FieldDate     FieldType = "date"
	FieldDateTime FieldType = "datetime"
	FieldJSON     FieldType = "json"
	FieldUUID     FieldType = "uuid"
	FieldRelation FieldType = "relation"
	FieldChild    FieldType = "child"

	// Deprecated: FieldNumber predates the spec's integer/decimal split (Core §10.1).
	// It is kept for backward compatibility and maps to the same storage as decimal.
	FieldNumber FieldType = "number"
)

// ValidationRule is a validation constraint on a field.
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

// ValidateDocumentSpec validates a DocumentSpec, returning an error if any constraint is violated.
// In addition to extension validation, it enforces:
//   - Reserved field names MUST NOT be reused.
//   - transaction_date field MUST be declared when characteristic: transaction.
//   - Event naming convention: before_* = sync, on_* = async (Core §12).
func ValidateDocumentSpec(d *DocumentSpec) error {
	// Reserved field name check
	for _, f := range d.Fields {
		if IsReservedField(f.Name) {
			return fmt.Errorf("field %q is a reserved field name and cannot be used as a custom field", f.Name)
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

// Currently checks:
//   - Extension (ExtendStorage) fields cannot be Required.
//   - Extension target must be in "module/entity" format.
//   - Extension namespace must be valid (non-empty, lowercase, alphanumeric+underscore).
func ValidateEntitySpec(e *EntitySpec) error {
	if e.ExtendStorage != nil {
		// Extension fields must not be Required
		for _, f := range e.Fields {
			if f.Required {
				return fmt.Errorf("extension field %q cannot be required", f.Name)
			}
		}

		// Validate target format: "module/entity"
		target := e.ExtendStorage.Target
		if target == "" {
			return fmt.Errorf("extend_storage.target is required")
		}
		parts := strings.Split(target, "/")
		if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
			return fmt.Errorf("extend_storage.target must be in format \"module/entity\", got %q", target)
		}

		// Validate namespace format
		ns := e.ExtendStorage.Namespace
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
	return nil
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
	Emits              string           `yaml:"emits,omitempty" json:"emits,omitempty"`       // event name linked per §12
	Expose             []string         `yaml:"expose,omitempty" json:"expose,omitempty"`     // per-action protocol filter: rest | grpc | ws
	Impl               *ImplDecl        `yaml:"impl,omitempty" json:"impl,omitempty"`
	Uses               *UsesDecl        `yaml:"uses,omitempty" json:"uses,omitempty"`
	Params             *ParamsDecl      `yaml:"params,omitempty" json:"params,omitempty"`
	Conditions         []ConditionDecl  `yaml:"conditions,omitempty" json:"conditions,omitempty"`
	UI                 *ActionUIHint    `yaml:"ui,omitempty" json:"ui,omitempty"` // Frontend §1.7 — button rendering hints
}

// ActionUIHint carries frontend rendering hints for an action button (Frontend §1.7).
type ActionUIHint struct {
	ButtonLabel string `yaml:"button_label,omitempty" json:"button_label,omitempty"`
	Style       string `yaml:"style,omitempty" json:"style,omitempty"` // primary | secondary | danger
	Icon        string `yaml:"icon,omitempty" json:"icon,omitempty"`
	Confirm     string `yaml:"confirm,omitempty" json:"confirm,omitempty"`
	ShowWhen    string `yaml:"show_when,omitempty" json:"show_when,omitempty"` // FormaExpr
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
	Scope  string `yaml:"scope,omitempty" json:"scope,omitempty"`   // tenant | module | global
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
func ValidateEventNaming(event EventDecl) error {
	isBefore := strings.HasPrefix(event.Name, "before_")
	isOn := strings.HasPrefix(event.Name, "on_")

	if !isBefore && !isOn {
		if event.Type == "" {
			return fmt.Errorf("event %q: type is required for custom events (not prefixed before_/on_)", event.Name)
		}
		if event.Type != EventTypeSync && event.Type != EventTypeAsync {
			return fmt.Errorf("event %q: type must be 'sync' or 'async', got %q", event.Name, event.Type)
		}
		return nil
	}

	if isBefore {
		if event.Type != "" && event.Type != EventTypeSync {
			return fmt.Errorf("event %q: before_* events must be 'sync', got %q", event.Name, event.Type)
		}
	}
	if isOn {
		if event.Type != "" && event.Type != EventTypeAsync {
			return fmt.Errorf("event %q: on_* events must be 'async', got %q", event.Name, event.Type)
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
	Scope    string `yaml:"scope,omitempty" json:"scope,omitempty"`       // websocket: tenant | user | ...
	Resource string `yaml:"resource,omitempty" json:"resource,omitempty"` // reliable_event: "module.entity"
	Action   string `yaml:"action,omitempty" json:"action,omitempty"`
}

// RetryDecl configures retry behavior for reliable event delivery.
type RetryDecl struct {
	Max            int    `yaml:"max,omitempty" json:"max,omitempty"`
	Backoff        string `yaml:"backoff,omitempty" json:"backoff,omitempty"` // exponential | linear | fixed
	InitialDelayMs int    `yaml:"initial_delay_ms,omitempty" json:"initial_delay_ms,omitempty"`
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

// TenantDecl configures tenant isolation for an Entity.
type TenantDecl struct {
	Isolated bool `yaml:"isolated" json:"isolated"`
}

// RelationDecl defines a relation to another entity.
type RelationDecl struct {
	Type       string   `yaml:"type" json:"type"` // belongs_to | has_many | has_one
	Resource   string   `yaml:"resource" json:"resource"`
	ForeignKey string   `yaml:"foreign_key,omitempty" json:"foreign_key,omitempty"`
	OnDelete   OnDelete `yaml:"on_delete,omitempty" json:"on_delete,omitempty"` // restrict | cascade | set_null (v0.3.0)
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
	// across the whole tenant.
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
