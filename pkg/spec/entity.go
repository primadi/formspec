package spec

import (
	"fmt"
	"strings"
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
}

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
	EnumValues     []string            `yaml:"enum_values,omitempty" json:"enum_values,omitempty"`
	Rules          []ValidationRule    `yaml:"rules,omitempty" json:"rules,omitempty"`
	Relation       *RelationDecl       `yaml:"relation,omitempty" json:"relation,omitempty"`
	Child          *ChildDecl          `yaml:"child,omitempty" json:"child,omitempty"`
	Computed       *ComputedDecl       `yaml:"computed,omitempty" json:"computed,omitempty"`
}

// FieldType is the data type of a field.
type FieldType string

const (
	FieldString   FieldType = "string"
	FieldNumber   FieldType = "number"
	FieldBoolean  FieldType = "boolean"
	FieldEnum     FieldType = "enum"
	FieldDate     FieldType = "date"
	FieldDateTime FieldType = "datetime"
	FieldJSON     FieldType = "json"
	FieldUUID     FieldType = "uuid"
	FieldRelation FieldType = "relation"
	FieldChild    FieldType = "child"
)

// ValidationRule is a validation constraint on a field.
type ValidationRule struct {
	Name  string `yaml:"name" json:"name"`
	Value any    `yaml:"value,omitempty" json:"value,omitempty"`
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

// UsesDecl declares what resources/primitives/config an action needs.
type UsesDecl struct {
	Resources  []string        `yaml:"resources,omitempty" json:"resources,omitempty"`
	Config     *UsesConfigDecl `yaml:"config,omitempty" json:"config,omitempty"`
	Primitives []string        `yaml:"primitives,omitempty" json:"primitives,omitempty"`
}

// UsesConfigDecl specifies config access.
type UsesConfigDecl struct {
	Read  []string `yaml:"read,omitempty" json:"read,omitempty"`
	Write []string `yaml:"write,omitempty" json:"write,omitempty"`
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

// ConditionDecl is a guard condition for an action.
type ConditionDecl struct {
	Field      string `yaml:"field" json:"field"`
	Expression string `yaml:"expression" json:"expression"`
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

// TransitionDecl defines a transition between states.
type TransitionDecl struct {
	From   string     `yaml:"from" json:"from"`
	To     string     `yaml:"to" json:"to"`
	Action string     `yaml:"action" json:"action"`
	Guard  *GuardDecl `yaml:"guard,omitempty" json:"guard,omitempty"`
}

// GuardDecl is a condition that must be met for a transition.
type GuardDecl struct {
	Expression string `yaml:"expression" json:"expression"`
	Message    string `yaml:"message,omitempty" json:"message,omitempty"`
}

// EventDecl declares an event an entity can publish.
type EventDecl struct {
	Name        string       `yaml:"name" json:"name"`
	Description string       `yaml:"description,omitempty" json:"description,omitempty"`
	Publish     *PublishDecl `yaml:"publish,omitempty" json:"publish,omitempty"`
}

// PublishDecl configures how an event is published.
type PublishDecl struct {
	Durable bool `yaml:"durable,omitempty" json:"durable,omitempty"`
}

// DeliveryDecl configures what happens when an event fires.
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
	Table      string      `yaml:"table,omitempty" json:"table,omitempty"`
	SoftDelete bool        `yaml:"soft_delete,omitempty" json:"soft_delete,omitempty"`
	Category   string      `yaml:"category,omitempty" json:"category,omitempty"` // operational | financial | compliance | analytics | master | archive
	Indexes    []IndexDecl `yaml:"indexes,omitempty" json:"indexes,omitempty"`
}
