// Action input contracts — declaring what a transition or action COLLECTS.
//
// Before this, an action's `params:` could only carry `validate:` — a list of
// rule names per parameter (`ParamsDecl.Validate`). That is enough to reject a
// bad request body, but it is not enough to BUILD one: there is no type, no
// label, no widget, so no renderer can turn it into a form. The consequence was
// visible in the UI (plan docs_internal/plan/action-input-contract.md):
//
//   - a transition button POSTed with an empty body, so a guard reading
//     `params.get('void_reason')` could never pass through the derived UI;
//   - a bulk action could only run actions that take no parameters at all,
//     because there was nothing to collect (todo 5.12.9);
//   - `ActionSummary.HasParams` existed in the bundle with zero consumers.
//
// A `ParamInput` is deliberately a REFERENCE to an Entity field whenever one of
// that name exists. The field stays the single source of truth for type, enum,
// and cardinality, so the same parameter cannot read as a `textarea` in one
// transition and a `select` in another — and a value collected for a referring
// input is written to that field, which is what makes a transition's input land
// in the record rather than evaporating.
//
// Only what is genuinely surface-specific is declared here: the caption, the
// widget, and the conditional vocabulary (`visible_when` / `readonly_when` /
// `required_when` / `compute`) — the same four predicates a Form field has, read
// by the same interpreter. That is what removes most of the need for a
// per-transition declaration: a transition with several `from` states gates its
// input with `required_when` instead of being split apart.
package spec

import "fmt"

// ParamInput is one declared input of an action or transition.
type ParamInput struct {
	// Name is the parameter name, and doubles as the wire key in the request
	// body. When it matches a field on the owning Entity the input refers to
	// that field — inheriting its type, title, enum/options, and cardinality —
	// and a collected value is persisted to it. Otherwise it is an ad-hoc
	// parameter and `type` is required.
	// @schema {minLength: 1, maxLength: 128, pattern: "^[a-z][a-z0-9_]*$", description: "Parameter name, and the wire key in the request body. Matching an Entity field name makes this a REFERENCE to that field (inheriting type/enum/cardinality, and persisting the collected value); otherwise the input is ad-hoc."}
	Name string `yaml:"name" json:"name"`

	// Type is the data type of an AD-HOC input. Required when `name` matches no
	// Entity field, and refused when it does: the field already declares a type,
	// and a second one would have no enforcement behind it to say which wins.
	Type FieldType `yaml:"type,omitempty" json:"type,omitempty"`

	// Label overrides the caption. Falls back to the Entity field's `title`,
	// then to the humanised name — the same precedence a Form field uses.
	Label string `yaml:"label,omitempty" json:"label,omitempty"`
	// Placeholder is the empty-state hint inside the input.
	Placeholder string `yaml:"placeholder,omitempty" json:"placeholder,omitempty"`
	// Help is the user-facing hint rendered under the input. On an Entity field
	// the same text is called `description`; here it is `help`, matching
	// `FormField.help`.
	Help string `yaml:"help,omitempty" json:"help,omitempty"`
	// Widget overrides the widget derived from the field's type. A CLOSED set
	// (`FormWidget`) — a typo would otherwise render a plain text input, which
	// is the silent-typo class S10 closes.
	Widget FormWidget `yaml:"widget,omitempty" json:"widget,omitempty"`

	// Required marks the input as unconditionally required. `RequiredWhen`
	// expresses the conditional case; when both are present the predicate wins,
	// since it can only narrow.
	Required bool `yaml:"required,omitempty" json:"required,omitempty"`
	// Default seeds the input before the user touches it.
	Default any `yaml:"default,omitempty" json:"default,omitempty"`

	// EnumValues / Options / Multiple describe an ad-hoc input's choice set, the
	// same way they do on a Field. On a REFERRING input they are refused:
	// choices belong to the field, so two forms cannot offer different sets for
	// one value.
	EnumValues []string      `yaml:"enum_values,omitempty" json:"enum_values,omitempty"`
	Options    []FieldOption `yaml:"options,omitempty" json:"options,omitempty"`
	Multiple   *bool         `yaml:"multiple,omitempty" json:"multiple,omitempty"`

	// Rules constrains the collected value. Merged with the Entity field's own
	// rules when the input refers to a field.
	Rules []ValidationRule `yaml:"rules,omitempty" json:"rules,omitempty"`

	// Client-behavior vocabulary (`FormSpecExpr`), identical to a Form field's
	// and evaluated by the same interpreter.
	VisibleWhen  string `yaml:"visible_when,omitempty" json:"visible_when,omitempty"`
	ReadonlyWhen string `yaml:"readonly_when,omitempty" json:"readonly_when,omitempty"`
	RequiredWhen string `yaml:"required_when,omitempty" json:"required_when,omitempty"`
	Compute      string `yaml:"compute,omitempty" json:"compute,omitempty"`

	// Persist overrides the destination of a collected value. Absent means the
	// default: persist to the Entity field when `name` matches one, otherwise
	// pass the value to the handler as a parameter only. `persist: false` on a
	// referring input collects it for the handler without writing it to the
	// record.
	Persist *bool `yaml:"persist,omitempty" json:"persist,omitempty"`
}

// ParamsRenderHint carries the container decision for an input form, and is
// deliberately a design-time choice — the same locking `Form.render` has, and
// for the same reason: whether a dialog or a page appears is a layout decision
// a manifest author makes, not something the runtime should second-guess.
//
// When absent the renderer derives the container from the input count, matching
// `deriveFormRenderMode` (≤5 modal, 5–12 drawer, >12 separate_page).
type ParamsRenderHint struct {
	// @schema {description: "Input-form container. Design-time decision, not switchable at runtime.", enum: ["modal", "drawer", "separate_page"]}
	Mode string `yaml:"mode,omitempty" json:"mode,omitempty"`
}

// InputSet is a named, reusable list of inputs declared on the Entity.
//
// Transitions that collect the same parameters share one set through
// `params.inputs_from` instead of repeating the declaration. The set lives on
// the Entity rather than on an action because a transition's parameters are
// drawn from the same field pool as the record's — which is also why a "one
// shared form for every transition" is not expressible: permission, conditions,
// approval interception, and event emission all attach to individual
// transitions, not to the entity.
type InputSet struct {
	// @schema {minLength: 1, maxLength: 128, pattern: "^[a-z][a-z0-9_]*$", description: "Set name, referenced by a transition's or action's `params.inputs_from`."}
	Name   string       `yaml:"name" json:"name"`
	Inputs []ParamInput `yaml:"inputs" json:"inputs"`
}

// ValidateActionInputs checks every input contract an Entity declares: the
// reusable `input_sets:`, the inputs on declared actions, and the inputs on
// state-machine transitions.
//
// It is called from ValidateEntitySpec and is ADDITIVE — a manifest that only
// carries `params.validate` (every manifest written before this contract) has no
// inputs to check and passes unchanged.
//
// Transitions are checked even when their `via` is also declared under
// `actions:`: the two are separate declarations, and the transition's is the one
// the PATCH path reads.
func ValidateActionInputs(d *EntitySpec) error {
	if d == nil {
		return nil
	}
	byField := make(map[string]*Field, len(d.Fields))
	for i := range d.Fields {
		byField[d.Fields[i].Name] = &d.Fields[i]
	}

	sets := make(map[string]*InputSet, len(d.InputSets))
	for i := range d.InputSets {
		s := &d.InputSets[i]
		if s.Name == "" {
			return fmt.Errorf("input_sets[%d]: name is required", i)
		}
		if sets[s.Name] != nil {
			return fmt.Errorf("input_sets: %q is declared twice — `params.inputs_from` resolves by name, so only one declaration can ever be reached", s.Name)
		}
		sets[s.Name] = s
		if len(s.Inputs) == 0 {
			return fmt.Errorf("input_sets %q: declares no inputs, so `inputs_from: [%s]` would collect nothing while looking like it does", s.Name, s.Name)
		}
		if err := validateParamInputs(s.Inputs, byField, fmt.Sprintf("input_sets %q", s.Name)); err != nil {
			return err
		}
	}

	for i := range d.Actions {
		a := &d.Actions[i]
		if err := validateParamsDecl(a.Params, byField, sets, fmt.Sprintf("action %q", a.Name)); err != nil {
			return err
		}
	}

	if d.StateMachine != nil {
		for i := range d.StateMachine.Transitions {
			t := &d.StateMachine.Transitions[i]
			where := fmt.Sprintf("state_machine transition %s->%s", t.From, t.To)
			if err := validateParamsDecl(t.Params, byField, sets, where); err != nil {
				return err
			}
		}
	}
	return nil
}

// validateParamsDecl checks one `params:` block, resolving any `inputs_from`
// references so the checks below see exactly the inputs a renderer would.
func validateParamsDecl(p *ParamsDecl, byField map[string]*Field, sets map[string]*InputSet, where string) error {
	if p == nil {
		return nil
	}

	if p.Render != nil {
		switch p.Render.Mode {
		case "", "modal", "drawer", "separate_page":
		default:
			return fmt.Errorf("%s: params.render.mode must be one of modal|drawer|separate_page, got %q", where, p.Render.Mode)
		}
	}

	merged := make([]ParamInput, 0, len(p.Inputs))
	merged = append(merged, p.Inputs...)
	for _, name := range p.InputsFrom {
		set := sets[name]
		if set == nil {
			return fmt.Errorf("%s: params.inputs_from names %q, which is not declared under `input_sets:` on this entity — the inputs would silently not be collected", where, name)
		}
		merged = append(merged, set.Inputs...)
	}
	if len(merged) == 0 {
		return nil
	}
	return validateParamInputs(merged, byField, where)
}

// validateParamInputs checks a resolved input list: required names, no
// duplicates, the referring-versus-ad-hoc rule, and the widget contract.
func validateParamInputs(inputs []ParamInput, byField map[string]*Field, where string) error {
	seen := make(map[string]bool, len(inputs))
	for i := range inputs {
		in := &inputs[i]
		if in.Name == "" {
			return fmt.Errorf("%s: inputs[%d] has no name — a parameter without one has no wire key to arrive on", where, i)
		}
		at := fmt.Sprintf("%s input %q", where, in.Name)
		if seen[in.Name] {
			return fmt.Errorf("%s: declared twice — one parameter cannot have two declarations, and the renderer would emit two inputs bound to the same key", at)
		}
		seen[in.Name] = true

		field := byField[in.Name]
		if field == nil {
			if in.Type == "" {
				return fmt.Errorf("%s: does not name a field on this entity, so it is an ad-hoc parameter and `type` is required — without it there is nothing to render or validate", at)
			}
			if !ValidFieldType(in.Type) {
				return fmt.Errorf("%s: unknown ad-hoc type %q", at, in.Type)
			}
			if in.Persist != nil && *in.Persist {
				return fmt.Errorf("%s: sets `persist: true` but names no field on this entity, so there is nowhere to write it — add the field, or drop `persist` to pass it to the handler only", at)
			}
			// Synthesize the shape the cardinality check reads, so an ad-hoc
			// input is held to the same widget contract as a real field.
			field = &Field{
				Name:       in.Name,
				Type:       in.Type,
				Options:    in.Options,
				EnumValues: in.EnumValues,
				Multiple:   in.Multiple,
			}
		} else if in.Type != "" {
			return fmt.Errorf("%s: names the entity field %q, so `type` must be left out — the field already declares %q, and a second type has no enforcement behind it to say which wins", at, in.Name, field.Type)
		}

		if err := ValidateFormWidget(in.Widget, at); err != nil {
			return err
		}
		if in.Widget != "" {
			if err := WidgetCardinalityMismatch(field, in.Widget); err != nil {
				return fmt.Errorf("%s: %w", at, err)
			}
		}
	}
	return nil
}

// EffectiveActionSpec resolves the action that a transition's `via` names,
// preferring a declared `actions:` entry and falling back to the transition
// itself — the same union `EntitySpec.ActionSources` exposes — with the
// TRANSITION's own `params` and `conditions` overlaid on top.
//
// Precedence, per field:
//   - `Params`     — the transition's when it declares one, else the action's;
//   - `Conditions` — the transition's FIRST, then the action's (both are gates).
//
// Callers that read a transition's input contract MUST go through this function
// rather than picking one declaration site, so the server and the renderer
// resolve the same thing (`src/lib/actionParams.ts` uses the mirrored rule
// `transition.params ?? action.params`).
//
// Returns nil when the transition names no `via`, in which case there is no
// action contract to apply.
//
// It exists because a transition's input contract can be written in two places
// (on the transition, or on a declared action sharing its `via`), and the write
// paths must read ONE of them. Reading the transition's own fields only would
// silently ignore a contract declared on the action — which is exactly how a
// `conditions: len(params.get('void_reason',”)) > 0` guard stayed unenforced on
// the PATCH path while looking perfectly declarative in the manifest.
//
// Returns nil when the transition names no `via`, in which case there is no
// action contract to apply.
func EffectiveActionSpec(es *EntitySpec, t *TransitionDecl) *Action {
	if es == nil || t == nil || t.Action == "" {
		return nil
	}
	sources := es.ActionSources()
	for i := range sources {
		if sources[i].Name != t.Action {
			continue
		}
		// The transition's own declarations are overlaid on whatever
		// `ActionSources` picked, because "which entry is listed" and "what
		// contract governs this transition" are different questions.
		//
		// A declared `actions:` entry survives `ValidateActionTransitionDuplication`
		// exactly when it adds something the transition LACKS — kafe's
		// `void-order` action contributes `description` + `audit` + `conditions`,
		// while the transition carries `params.inputs`. Reading the declared entry
		// alone therefore dropped the inputs: the reason stayed in the request
		// body, `params.get('void_reason')` saw nothing, and every void was
		// refused with "Alasan void wajib diisi" while the body plainly contained
		// it (found by the kafe e2e, todo 5.24.4).
		merged := sources[i]
		if t.Params != nil {
			merged.Params = t.Params
		}
		if len(t.Conditions) > 0 {
			// Both gate the same transition, so neither may be dropped: the
			// declared gates are appended, not replaced, and replacement would
			// let a declared gate stop being enforced with no error anywhere.
			merged.Conditions = append(append([]ConditionDecl{}, t.Conditions...), merged.Conditions...)
		}
		return &merged
	}
	return nil
}

// TransitionInputParams returns the values a transition's declared inputs read
// from, given the request body and the record being transitioned.
//
// Only names the contract DECLARES are read: an undeclared key in the body is
// never promoted, so a caller cannot smuggle an arbitrary record field into a
// guard by adding it to the payload. Each declared name resolves to the body's
// value when present, else the record's — an optional input left blank therefore
// reads back as the value already stored, instead of failing a required-check
// that the caller had no way to satisfy.
//
// The result is what `params.get(...)` sees in conditions, and what
// ValidateActionParams validates.
func TransitionInputParams(es *EntitySpec, t *TransitionDecl, body, record map[string]any) map[string]any {
	action := EffectiveActionSpec(es, t)
	if action == nil || action.Params == nil {
		return nil
	}

	names := make([]string, 0, len(action.Params.Inputs))
	names = append(names, declaredInputNames(es, action.Params)...)
	if len(names) == 0 {
		// A contract expressed only as `validate:` still names its parameters;
		// those are the ones a condition can read.
		for _, v := range action.Params.Validate {
			names = append(names, v.Field)
		}
	}
	if len(names) == 0 {
		return nil
	}

	out := make(map[string]any, len(names))
	for _, name := range names {
		if v, ok := body[name]; ok {
			out[name] = v
			continue
		}
		if v, ok := record[name]; ok {
			out[name] = v
		}
	}
	return out
}

// declaredInputNames lists the input names a contract declares, resolving
// `inputs_from` against the entity's `input_sets:`.
func declaredInputNames(es *EntitySpec, p *ParamsDecl) []string {
	names := make([]string, 0, len(p.Inputs))
	for i := range p.Inputs {
		names = append(names, p.Inputs[i].Name)
	}
	if es == nil || len(p.InputsFrom) == 0 {
		return names
	}
	for _, setName := range p.InputsFrom {
		for i := range es.InputSets {
			if es.InputSets[i].Name != setName {
				continue
			}
			for j := range es.InputSets[i].Inputs {
				names = append(names, es.InputSets[i].Inputs[j].Name)
			}
		}
	}
	return names
}

// PersistableInputs returns the declared inputs whose collected value belongs on
// the RECORD — the default when the input's name matches an Entity field, or an
// explicit `persist: true`.
//
// This is what makes a transition's reason land in the record instead of
// evaporating: the value arrives in the request body, but the write paths only
// copy keys they are told about, and a PATCH merges whatever it is given. Naming
// them here keeps the rule in one place, next to the declaration it reads.
func PersistableInputs(es *EntitySpec, p *ParamsDecl) []string {
	if es == nil || p == nil {
		return nil
	}
	out := make([]string, 0, len(p.Inputs))
	for _, name := range declaredInputNames(es, p) {
		in := findInput(es, p, name)
		if in == nil {
			continue
		}
		persist := fieldByName(es, name) != nil // referring inputs persist by default
		if in.Persist != nil {
			persist = *in.Persist
		}
		if persist && fieldByName(es, name) != nil {
			out = append(out, name)
		}
	}
	return out
}

// findInput locates the declaration of a resolved input name, searching the
// inline list first and then the referenced sets.
func findInput(es *EntitySpec, p *ParamsDecl, name string) *ParamInput {
	for i := range p.Inputs {
		if p.Inputs[i].Name == name {
			return &p.Inputs[i]
		}
	}
	if es == nil {
		return nil
	}
	for _, setName := range p.InputsFrom {
		for i := range es.InputSets {
			if es.InputSets[i].Name != setName {
				continue
			}
			for j := range es.InputSets[i].Inputs {
				if es.InputSets[i].Inputs[j].Name == name {
					return &es.InputSets[i].Inputs[j]
				}
			}
		}
	}
	return nil
}

// fieldByName returns the entity field with the given name, or nil.
func fieldByName(es *EntitySpec, name string) *Field {
	if es == nil {
		return nil
	}
	for i := range es.Fields {
		if es.Fields[i].Name == name {
			return &es.Fields[i]
		}
	}
	return nil
}

// EffectiveParamValidation returns every validation rule an action's contract
// implies, in one list the write paths can hand to ValidateActionParams.
//
// It exists because the contract has two halves that grew up separately:
//
//   - `params.validate` — the original, rule-only form (`{field, rules}`);
//   - `params.inputs`   — the renderable form added with the action-input
//     contract, which states requiredness as `required: true` and/or a
//     `rules: [required]` list.
//
// A declaration is only a contract if the SERVER enforces it. Rendering a
// required field the server accepts as missing merely moves the failure to a
// later reader — measured: an approval-gated transition declaring
// `inputs: [{name: void_reason, required: true}]` accepted a bare
// `{"status":"voided"}`, started its approval, and would have voided the order
// with no reason at all.
//
// Rules declared on an input are appended, never substituted: an input may add
// a constraint to a field whose own rules are stricter, and duplicate
// `required` entries are harmless (the rule is idempotent).
func EffectiveParamValidation(es *EntitySpec, p *ParamsDecl) []ParamValidation {
	if p == nil {
		return nil
	}
	out := make([]ParamValidation, 0, len(p.Validate)+len(p.Inputs))
	out = append(out, p.Validate...)

	for _, name := range declaredInputNames(es, p) {
		in := findInput(es, p, name)
		if in == nil {
			continue
		}
		rules := make([]ValidationRule, 0, len(in.Rules)+1)
		if in.Required {
			rules = append(rules, ValidationRule{Name: "required"})
		}
		for _, r := range in.Rules {
			if in.Required && r.Name == "required" {
				continue // already implied; one entry is enough
			}
			rules = append(rules, r)
		}
		// An input that names an existing field inherits that field's own
		// rules: `void_reason` declared `required` on the Entity is required
		// here too, so the two cannot disagree about the same value.
		if f := fieldByName(es, name); f != nil {
			for _, r := range f.Rules {
				if hasRule(rules, r.Name) {
					continue
				}
				rules = append(rules, r)
			}
		}
		if len(rules) == 0 {
			continue
		}
		if existing := indexOfValidation(out, name); existing >= 0 {
			// A name may be described by several declarations (an ad-hoc
			// `validate:` entry, an input, the Entity field). Append only what is
			// not already stated, so one requirement does not read as three —
			// harmless at runtime, but an error list that repeats itself makes
			// the real constraints look inconsistent.
			for _, r := range rules {
				if hasRule(out[existing].Rules, r.Name) {
					continue
				}
				out[existing].Rules = append(out[existing].Rules, r)
			}
			continue
		}
		out = append(out, ParamValidation{Field: name, Rules: rules})
	}
	return out
}

// indexOfValidation finds the entry for a field, or -1.
func indexOfValidation(list []ParamValidation, field string) int {
	for i := range list {
		if list[i].Field == field {
			return i
		}
	}
	return -1
}

// hasRule reports whether a rule name is already present.
func hasRule(rules []ValidationRule, name string) bool {
	for _, r := range rules {
		if r.Name == name {
			return true
		}
	}
	return false
}

// ValidFieldType reports whether t is a declared field type.
//
// It exists for ad-hoc inputs, which have no `Field` to have been validated
// already: an entity field's type reaches storage, where an unknown one fails
// loudly, but an ad-hoc input's type only reaches a renderer, where an unknown
// one silently falls back to a text box.
func ValidFieldType(t FieldType) bool {
	switch t {
	case FieldString, FieldText, FieldRichText,
		FieldInteger, FieldDecimal, FieldMoney, FieldNumber, FieldPercent,
		FieldBoolean, FieldEnum,
		FieldDate, FieldDateTime, FieldTime,
		FieldUUID, FieldJSON,
		FieldFile, FieldAttachment,
		FieldRelation, FieldChild:
		return true
	}
	return false
}
