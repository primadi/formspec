package spec

import (
	"fmt"
	"strings"

	"gopkg.in/yaml.v3"
)

// EntitySpec defines a stateful, persisted business data resource (Core §4.1).
type EntitySpec struct {
	Version         string           `yaml:"version" json:"version"`
	Plural          string           `yaml:"plural,omitempty" json:"plural,omitempty"`
	Characteristics []Characteristic `yaml:"characteristics,omitempty" json:"characteristics,omitempty"`
	Auth            *EntityAuth      `yaml:"auth,omitempty" json:"auth,omitempty"`
	Persist         *PersistSpec     `yaml:"persist,omitempty" json:"persist,omitempty"`
	Fields          []Field          `yaml:"fields" json:"fields"`
	Actions         []Action         `yaml:"actions" json:"actions"`
	StateMachine    *StateMachine    `yaml:"state_machine,omitempty" json:"state_machine,omitempty"`
	Events          []EventDecl      `yaml:"events,omitempty" json:"events,omitempty"`
	Deliver         []DeliveryDecl   `yaml:"deliver,omitempty" json:"deliver,omitempty"`
	Indexes         []IndexDecl      `yaml:"indexes,omitempty" json:"indexes,omitempty"`
	Tenant          *TenantDecl      `yaml:"tenant,omitempty" json:"tenant,omitempty"`
	ExtendStorage   *ExtendStorage   `yaml:"extend_storage,omitempty" json:"extend_storage,omitempty"`
	Expose          []ExposeConfig   `yaml:"expose,omitempty" json:"expose,omitempty"` // D49 — absent = no external access
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

// ValidateEntitySpec validates an EntitySpec, returning an error if any constraint is violated.
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
	Resources  []string         `yaml:"resources,omitempty" json:"resources,omitempty"`
	Db         *UsesDbDecl      `yaml:"db,omitempty" json:"db,omitempty"`
	Config     *UsesConfigDecl  `yaml:"config,omitempty" json:"config,omitempty"`
	Kvstore    []KvstoreUseDecl `yaml:"kvstore,omitempty" json:"kvstore,omitempty"`
	Primitives []string         `yaml:"primitives,omitempty" json:"primitives,omitempty"`
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

// EventDecl declares an event an entity can publish (Core §12).
type EventDecl struct {
	Name        string              `yaml:"name" json:"name"`
	Description string              `yaml:"description,omitempty" json:"description,omitempty"`
	Publish     *PublishDecl        `yaml:"publish,omitempty" json:"publish,omitempty"`
	Payload     *PayloadDecl        `yaml:"payload,omitempty" json:"payload,omitempty"`
	Deliver     []EventDeliveryDecl `yaml:"deliver,omitempty" json:"deliver,omitempty"`
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
	Type       string `yaml:"type" json:"type"` // belongs_to | has_many | has_one
	Resource   string `yaml:"resource" json:"resource"`
	ForeignKey string `yaml:"foreign_key,omitempty" json:"foreign_key,omitempty"`
}

// NaturalKeyRuleDecl defines how a natural key is generated.
type NaturalKeyRuleDecl struct {
	Strategy string            `yaml:"strategy" json:"strategy"` // sequence | custom
	Format   string            `yaml:"format,omitempty" json:"format,omitempty"`
	Prefix   *NaturalKeyPrefix `yaml:"prefix,omitempty" json:"prefix,omitempty"`
	Reset    string            `yaml:"reset,omitempty" json:"reset,omitempty"` // never | yearly | monthly | daily
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

// PersistSpec controls how an entity is stored (Core §19).
type PersistSpec struct {
	Table string `yaml:"table,omitempty" json:"table,omitempty"`
	// SoftDelete defaults to true; nil means "not set" so a persist block
	// that omits soft_delete does not silently disable it.
	SoftDelete *bool       `yaml:"soft_delete,omitempty" json:"soft_delete,omitempty"`
	Category   string      `yaml:"category,omitempty" json:"category,omitempty"` // operational | financial | compliance | analytics | master | archive
	Indexes    []IndexDecl `yaml:"indexes,omitempty" json:"indexes,omitempty"`
}
