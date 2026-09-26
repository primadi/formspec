package spec

import (
	"testing"

	"gopkg.in/yaml.v3"
)

func TestValidateEntitySpec_BaseEntity(t *testing.T) {
	e := &EntitySpec{
		Version: "v1",
		Fields: []Field{
			{Name: "name", Type: FieldString, Required: true},
			{Name: "email", Type: FieldString, Unique: true},
		},
	}
	if err := ValidateEntitySpec(e); err != nil {
		t.Errorf("expected no error, got %v", err)
	}
}

// TestValidateEntitySpec_Scope pins the row-scope contract (S2, #6/#9): every
// entry must name a declared field and a known value source. A scope whose
// source is not understood would silently not filter — worse than no scope.
func TestValidateEntitySpec_Scope(t *testing.T) {
	base := func(scope ...FilterSpec) *EntitySpec {
		return &EntitySpec{
			Version: "v1",
			Fields: []Field{
				{Name: "branch_id", Type: FieldString},
				{Name: "guest_token", Type: FieldString},
			},
			RowScope: scope,
		}
	}

	valid := []struct {
		name  string
		scope FilterSpec
	}{
		{"session with attribute", FilterSpec{Field: "branch_id", From: "session", Attr: "branch_id"}},
		{"session without attribute (defaults to principal_id)", FilterSpec{Field: "branch_id", From: "session"}},
		{"route with parameter", FilterSpec{Field: "guest_token", From: "route", Param: "token"}},
		{"route without parameter (defaults to field name)", FilterSpec{Field: "guest_token", From: "route"}},
	}
	for _, c := range valid {
		if err := ValidateEntitySpec(base(c.scope)); err != nil {
			t.Errorf("%s: expected no error, got %v", c.name, err)
		}
	}

	invalid := []struct {
		name  string
		scope FilterSpec
	}{
		{"missing field", FilterSpec{From: "session"}},
		{"unknown field", FilterSpec{Field: "outlet_id", From: "session"}},
		{"unknown source", FilterSpec{Field: "branch_id", From: "cookie"}},
		{"missing source", FilterSpec{Field: "branch_id"}},
	}
	for _, c := range invalid {
		if err := ValidateEntitySpec(base(c.scope)); err == nil {
			t.Errorf("%s: expected an error, got none", c.name)
		}
	}
}

// TestValidateEntitySpec_ScopeDimension pins the S5 declaration: a scope names a
// dimension and the field carrying its value. `required: true` must agree with
// the field itself, otherwise the declaration claims a guarantee the row shape
// does not hold.
func TestValidateEntitySpec_ScopeDimension(t *testing.T) {
	base := func(mutate func(*EntitySpec)) *EntitySpec {
		e := &EntitySpec{
			Version: "v1",
			Fields: []Field{
				{Name: "branch_id", Type: FieldRelation, Required: true},
				{Name: "note", Type: FieldString},
			},
		}
		if mutate != nil {
			mutate(e)
		}
		return e
	}

	ok := []struct {
		name string
		e    *EntitySpec
	}{
		{"required dimension", base(func(e *EntitySpec) {
			e.Scope = &ScopeDecl{Dimension: "branch", Field: "branch_id", Required: true}
		})},
		{"optional dimension", base(func(e *EntitySpec) {
			e.Scope = &ScopeDecl{Dimension: "branch", Field: "note"}
		})},
	}
	for _, c := range ok {
		if err := ValidateEntitySpec(c.e); err != nil {
			t.Errorf("%s: expected no error, got %v", c.name, err)
		}
	}

	bad := []struct {
		name string
		e    *EntitySpec
	}{
		{"missing dimension", base(func(e *EntitySpec) {
			e.Scope = &ScopeDecl{Field: "branch_id"}
		})},
		{"bad dimension name", base(func(e *EntitySpec) {
			e.Scope = &ScopeDecl{Dimension: "Branch", Field: "branch_id"}
		})},
		{"missing field", base(func(e *EntitySpec) {
			e.Scope = &ScopeDecl{Dimension: "branch"}
		})},
		{"unknown field", base(func(e *EntitySpec) {
			e.Scope = &ScopeDecl{Dimension: "branch", Field: "outlet_id"}
		})},
		{"required disagrees with the field", base(func(e *EntitySpec) {
			e.Scope = &ScopeDecl{Dimension: "branch", Field: "note", Required: true}
		})},
	}
	for _, c := range bad {
		if err := ValidateEntitySpec(c.e); err == nil {
			t.Errorf("%s: expected an error, got none", c.name)
		}
	}
}

// TestValidateEntitySpec_Assignments pins the S5 principal→dimension mapping.
func TestValidateEntitySpec_Assignments(t *testing.T) {
	base := func(a ...AssignmentDecl) *EntitySpec {
		return &EntitySpec{
			Version: "v1",
			Fields: []Field{
				{Name: "username", Type: FieldString},
				{Name: "branch_id", Type: FieldRelation},
				{Name: "region_id", Type: FieldRelation},
			},
			Assignments: a,
		}
	}

	if err := ValidateEntitySpec(base(AssignmentDecl{
		Dimension: "branch", Field: "branch_id", PrincipalField: "username",
	})); err != nil {
		t.Errorf("valid assignment: expected no error, got %v", err)
	}

	bad := []struct {
		name string
		a    AssignmentDecl
	}{
		{"missing dimension", AssignmentDecl{Field: "branch_id", PrincipalField: "username"}},
		{"bad dimension name", AssignmentDecl{Dimension: "Branch", Field: "branch_id", PrincipalField: "username"}},
		{"missing field", AssignmentDecl{Dimension: "branch", PrincipalField: "username"}},
		{"unknown field", AssignmentDecl{Dimension: "branch", Field: "outlet_id", PrincipalField: "username"}},
		{"missing principal_field", AssignmentDecl{Dimension: "branch", Field: "branch_id"}},
		{"unknown principal_field", AssignmentDecl{Dimension: "branch", Field: "branch_id", PrincipalField: "login"}},
	}
	for _, c := range bad {
		if err := ValidateEntitySpec(base(c.a)); err == nil {
			t.Errorf("%s: expected an error, got none", c.name)
		}
	}

	// Two mappings for the same dimension would be ambiguous at resolution time.
	if err := ValidateEntitySpec(base(
		AssignmentDecl{Dimension: "branch", Field: "branch_id", PrincipalField: "username"},
		AssignmentDecl{Dimension: "branch", Field: "region_id", PrincipalField: "username"},
	)); err == nil {
		t.Error("duplicate dimension: expected an error, got none")
	}
}

// TestValidateEntitySpec_Unit pins S12: a unit declaration must agree with the
// field's own values, so a typo cannot silently disable conversion.
func TestValidateEntitySpec_Unit(t *testing.T) {
	base := func(u *UnitDecl) *EntitySpec {
		return &EntitySpec{
			Version: "v1",
			Fields: []Field{{
				Name:       "unit",
				Type:       FieldEnum,
				EnumValues: []string{"gram", "kg", "pcs"},
				Unit:       u,
			}},
		}
	}

	if err := ValidateEntitySpec(base(&UnitDecl{Base: "gram", Convertible: []string{"kg"}, Factors: map[string]float64{"kg": 1000}})); err != nil {
		t.Errorf("valid unit: expected no error, got %v", err)
	}

	bad := []struct {
		name string
		u    *UnitDecl
	}{
		{"missing base", &UnitDecl{Convertible: []string{"kg"}, Factors: map[string]float64{"kg": 1000}}},
		{"base not in enum_values", &UnitDecl{Base: "ounce"}},
		{"convertible not in enum_values", &UnitDecl{Base: "gram", Convertible: []string{"ounce"}, Factors: map[string]float64{"ounce": 28.35}}},
		{"convertible repeats base", &UnitDecl{Base: "gram", Convertible: []string{"gram"}, Factors: map[string]float64{"gram": 1}}},
		{"convertible without factor", &UnitDecl{Base: "gram", Convertible: []string{"kg"}}},
		{"factor for base unit", &UnitDecl{Base: "gram", Convertible: []string{"kg"}, Factors: map[string]float64{"kg": 1000, "gram": 1}}},
		{"factor not in convertible", &UnitDecl{Base: "gram", Convertible: []string{"kg"}, Factors: map[string]float64{"kg": 1000, "pcs": 1}}},
		{"non-positive factor", &UnitDecl{Base: "gram", Convertible: []string{"kg"}, Factors: map[string]float64{"kg": 0}}},
	}
	for _, c := range bad {
		if err := ValidateEntitySpec(base(c.u)); err == nil {
			t.Errorf("%s: expected an error, got none", c.name)
		}
	}

	// `unit` describes unit names, so it is meaningless on a numeric field.
	badType := &EntitySpec{
		Version: "v1",
		Fields:  []Field{{Name: "quantity", Type: FieldDecimal, Unit: &UnitDecl{Base: "gram"}}},
	}
	if err := ValidateEntitySpec(badType); err == nil {
		t.Error("unit on a decimal field: expected an error, got none")
	}
}

// TestUnitDecl_Convert pins S12 conversion (item 4.6): conversion goes through
// the base unit using declared factors, and a unit outside the declared group is
// an error (a different dimension, not a silent no-op).
func TestUnitDecl_Convert(t *testing.T) {
	u := &UnitDecl{Base: "gram", Convertible: []string{"kg"}, Factors: map[string]float64{"kg": 1000}}

	cases := []struct {
		value    float64
		from, to string
		want     float64
	}{
		{2, "kg", "gram", 2000},
		{2000, "gram", "kg", 2},
		{5, "gram", "gram", 5},
		{1.5, "kg", "kg", 1.5},
	}
	for _, c := range cases {
		got, err := u.Convert(c.value, c.from, c.to)
		if err != nil {
			t.Errorf("Convert(%v, %s, %s): unexpected error %v", c.value, c.from, c.to, err)
			continue
		}
		if got != c.want {
			t.Errorf("Convert(%v, %s, %s) = %v, want %v", c.value, c.from, c.to, got, c.want)
		}
	}

	// A unit outside the declared group is a different dimension — error, not 0.
	if _, err := u.Convert(1, "pcs", "gram"); err == nil {
		t.Error("Convert from an undeclared unit: expected an error, got none")
	}
	if _, err := u.Convert(1, "gram", "ounce"); err == nil {
		t.Error("Convert to an undeclared unit: expected an error, got none")
	}
}

// TestValidateEntitySpec_SummaryInvariants pins S14: `maintained_by` and
// `invariants` belong to summary projections, name real fields, and — crucially
// — an invariant must be backed by a unique index. A summary has no action
// pipeline, so an unbacked invariant would be a claim nothing enforces.
func TestValidateEntitySpec_SummaryInvariants(t *testing.T) {
	base := func(mutate func(*EntitySpec)) *EntitySpec {
		e := &EntitySpec{
			Version:        "v1",
			Characteristic: CharSummary,
			Fields: []Field{
				{Name: "branch_id", Type: FieldRelation},
				{Name: "ingredient_id", Type: FieldRelation},
			},
			Indexes: []IndexDecl{
				{Fields: []string{"branch_id", "ingredient_id"}, Unique: true},
			},
		}
		if mutate != nil {
			mutate(e)
		}
		return e
	}

	ok := base(func(e *EntitySpec) {
		e.MaintainedBy = "cafe-stock/scripts/stock_level_apply"
		e.Invariants = []InvariantDecl{{
			Unique:  []string{"branch_id", "ingredient_id"},
			Message: "satu saldo per cabang/bahan",
		}}
	})
	if err := ValidateEntitySpec(ok); err != nil {
		t.Errorf("valid summary contract: expected no error, got %v", err)
	}

	noIndex := base(func(e *EntitySpec) {
		e.Indexes = nil
		e.Invariants = []InvariantDecl{{Unique: []string{"branch_id", "ingredient_id"}}}
	})
	if err := ValidateEntitySpec(noIndex); err == nil {
		t.Error("invariant without a unique index: expected an error, got none")
	}

	nonSummary := &EntitySpec{
		Version:        "v1",
		Characteristic: CharMaster,
		Fields:         []Field{{Name: "name", Type: FieldString}},
		MaintainedBy:   "m/scripts/x",
	}
	if err := ValidateEntitySpec(nonSummary); err == nil {
		t.Error("maintained_by on a master entity: expected an error, got none")
	}

	badRef := base(func(e *EntitySpec) { e.MaintainedBy = "stock_level_apply" })
	if err := ValidateEntitySpec(badRef); err == nil {
		t.Error("maintained_by without a module qualifier: expected an error, got none")
	}

	unknownField := base(func(e *EntitySpec) {
		e.Invariants = []InvariantDecl{{Unique: []string{"branch_id", "nope"}}}
	})
	if err := ValidateEntitySpec(unknownField); err == nil {
		t.Error("invariant naming an unknown field: expected an error, got none")
	}
}

// TestValidateEntitySpec_SummaryRejectsHooks pins GAP-33 (TODO 4.2): a summary
// entity is written by its maintainer script, never through the action pipeline,
// so `hooks:`/`conditions:` declared on it would never run. Rejecting them is
// the honest answer — a manifest that looks protected but is not is worse than
// one that fails validation.
func TestValidateEntitySpec_SummaryRejectsHooks(t *testing.T) {
	base := func(mutate func(*EntitySpec)) *EntitySpec {
		e := &EntitySpec{
			Version:        "v1",
			Characteristic: CharSummary,
			Fields:         []Field{{Name: "branch_id", Type: FieldRelation}},
		}
		if mutate != nil {
			mutate(e)
		}
		return e
	}

	withHook := base(func(e *EntitySpec) {
		e.Hooks = []HookDecl{{On: HookOnBefore, Action: "*", Impl: &ImplDecl{Ref: "m/scripts/x.star"}}}
	})
	if err := ValidateEntitySpec(withHook); err == nil {
		t.Error("summary entity with hooks: expected an error, got none")
	}

	withCondition := base(func(e *EntitySpec) {
		e.Actions = []Action{{Name: "apply", Conditions: []ConditionDecl{{Expression: "true"}}}}
	})
	if err := ValidateEntitySpec(withCondition); err == nil {
		t.Error("summary entity action with conditions: expected an error, got none")
	}

	// A master entity may still declare hooks — the rule is summary-specific.
	master := &EntitySpec{
		Version:        "v1",
		Characteristic: CharMaster,
		Fields:         []Field{{Name: "name", Type: FieldString}},
		Hooks:          []HookDecl{{On: HookOnBefore, Action: "*", Impl: &ImplDecl{Ref: "m/scripts/x.star"}}},
	}
	if err := ValidateEntitySpec(master); err != nil {
		t.Errorf("master entity with hooks: expected no error, got %v", err)
	}
}

// TestValidateEntitySpec_TransitionEmits pins S13 (item 6.1): a transition's
// `emit` must name a declared event, so the transition↔event link is verifiable
// rather than implied by naming.
func TestValidateEntitySpec_TransitionEmits(t *testing.T) {
	base := func(emit string) *EntitySpec {
		return &EntitySpec{
			Version: "v1",
			Fields:  []Field{{Name: "status", Type: FieldString}},
			Events:  []EventDecl{{Name: "on_paid", Type: EventTypeAsync}},
			StateMachine: &StateMachine{
				Field:   "status",
				Initial: "draft",
				States:  []StateDecl{{Name: "draft"}, {Name: "paid"}},
				Transitions: []TransitionDecl{
					{From: StateList{"draft"}, To: "paid", Action: "confirm", Emit: emit},
				},
			},
		}
	}

	if err := ValidateEntitySpec(base("on_paid")); err != nil {
		t.Errorf("emit naming a declared event: expected no error, got %v", err)
	}
	if err := ValidateEntitySpec(base("")); err != nil {
		t.Errorf("transition without emit: expected no error, got %v", err)
	}
	if err := ValidateEntitySpec(base("on_nope")); err == nil {
		t.Error("emit naming an undeclared event: expected an error, got none")
	}
}

// TestValidateEntitySpec_Percent pins S11: `percent` is a first-class field type
// that validates like the decimal it numerically is.
func TestValidateEntitySpec_Percent(t *testing.T) {
	e := &EntitySpec{
		Version: "v1",
		Fields:  []Field{{Name: "tax_percent", Type: FieldPercent, Scale: ptrInt(2)}},
	}
	if err := ValidateEntitySpec(e); err != nil {
		t.Errorf("percent field: expected no error, got %v", err)
	}
}

func ptrInt(v int) *int { return &v }

func TestValidateEntitySpec_RenamedFrom(t *testing.T) {
	// Valid rename.
	e := &EntitySpec{
		Version: "v1",
		Fields: []Field{
			{Name: "full_name", Type: FieldString, RenamedFrom: "name"},
		},
	}
	if err := ValidateEntitySpec(e); err != nil {
		t.Errorf("expected no error for valid rename, got %v", err)
	}

	// renamed_from collides with an existing field.
	e2 := &EntitySpec{
		Version: "v1",
		Fields: []Field{
			{Name: "full_name", Type: FieldString, RenamedFrom: "name"},
			{Name: "name", Type: FieldString},
		},
	}
	if err := ValidateEntitySpec(e2); err == nil {
		t.Error("expected error when renamed_from collides with existing field")
	}

	// renamed_from is a reserved name.
	e3 := &EntitySpec{
		Version: "v1",
		Fields: []Field{
			{Name: "full_name", Type: FieldString, RenamedFrom: "version"},
		},
	}
	if err := ValidateEntitySpec(e3); err == nil {
		t.Error("expected error when renamed_from is a reserved name")
	}
}

func TestValidateEntitySpec_ExtensionNoRequired(t *testing.T) {
	e := &EntitySpec{
		Version: "v1",
		ExtendStorage: &ExtendStorage{
			Target:    "billing/invoice",
			Namespace: "custext",
		},
		Fields: []Field{
			{Name: "project_code", Type: FieldString, Required: false},
			{Name: "cost_center", Type: FieldString},
		},
	}
	if err := ValidateEntitySpec(e); err != nil {
		t.Errorf("expected no error, got %v", err)
	}
}

func TestValidateEntitySpec_ExtensionWithRequired(t *testing.T) {
	e := &EntitySpec{
		Version: "v1",
		ExtendStorage: &ExtendStorage{
			Target:    "billing/invoice",
			Namespace: "custext",
		},
		Fields: []Field{
			{Name: "project_code", Type: FieldString, Required: true},
		},
	}
	if err := ValidateEntitySpec(e); err == nil {
		t.Error("expected error for required field in extension, got nil")
	}
}

func TestValidateEntitySpec_ExtensionMissingTarget(t *testing.T) {
	e := &EntitySpec{
		Version: "v1",
		ExtendStorage: &ExtendStorage{
			Namespace: "custext",
		},
		Fields: []Field{
			{Name: "project_code", Type: FieldString},
		},
	}
	if err := ValidateEntitySpec(e); err == nil {
		t.Error("expected error for missing target, got nil")
	}
}

func TestValidateEntitySpec_ExtensionInvalidTarget(t *testing.T) {
	e := &EntitySpec{
		Version: "v1",
		ExtendStorage: &ExtendStorage{
			Target:    "billing",
			Namespace: "custext",
		},
		Fields: []Field{
			{Name: "code", Type: FieldString},
		},
	}
	if err := ValidateEntitySpec(e); err == nil {
		t.Error("expected error for invalid target format, got nil")
	}
}

func TestValidateEntitySpec_ExtensionMissingNamespace(t *testing.T) {
	e := &EntitySpec{
		Version: "v1",
		ExtendStorage: &ExtendStorage{
			Target: "billing/invoice",
		},
		Fields: []Field{
			{Name: "code", Type: FieldString},
		},
	}
	if err := ValidateEntitySpec(e); err == nil {
		t.Error("expected error for missing namespace, got nil")
	}
}

func TestValidateEntitySpec_ExtensionInvalidNamespace(t *testing.T) {
	tests := []struct {
		ns   string
		desc string
	}{
		{"123abc", "starts with digit"},
		{"UPPERCASE", "uppercase"},
		{"ab", "too short"},
		{"a-b", "contains hyphen"},
		{"a.b", "contains dot"},
		{"this_namespace_is_way_too_long_for_sure", "too long"},
	}
	for _, tt := range tests {
		t.Run(tt.desc, func(t *testing.T) {
			e := &EntitySpec{
				Version: "v1",
				ExtendStorage: &ExtendStorage{
					Target:    "billing/invoice",
					Namespace: tt.ns,
				},
				Fields: []Field{
					{Name: "code", Type: FieldString},
				},
			}
			if err := ValidateEntitySpec(e); err == nil {
				t.Errorf("expected error for namespace %q (%s), got nil", tt.ns, tt.desc)
			}
		})
	}
}

func TestValidateEntitySpec_NilExtendStorage(t *testing.T) {
	// nil ExtendStorage should not cause any extension validation
	e := &EntitySpec{
		Version: "v1",
		Fields: []Field{
			{Name: "name", Type: FieldString, Required: true},
		},
	}
	if err := ValidateEntitySpec(e); err != nil {
		t.Errorf("expected no error for nil extend_storage, got %v", err)
	}
}

func TestValidateEntitySpec_SummaryRebuildContract(t *testing.T) {
	valid := &EntitySpec{
		Version:        "v1",
		Characteristic: CharSummary,
		Fields: []Field{
			{Name: "customer_id", Type: FieldString},
			{Name: "order_total", Type: FieldDecimal},
		},
		Sources: []SummarySource{
			{Entity: "sales.order", Alias: "o", Filter: map[string]string{"status": "paid"}},
			{Entity: "sales.customer", Alias: "c"},
		},
		JoinKey: "o.customer_id = c.id",
		Rebuild: &RebuildSpec{Strategy: "partial"},
	}
	if err := ValidateEntitySpec(valid); err != nil {
		t.Fatalf("expected valid summary rebuild contract, got %v", err)
	}

	invalid := &EntitySpec{
		Version:        "v1",
		Characteristic: CharSummary,
		Fields:         []Field{{Name: "total", Type: FieldDecimal}},
		Rebuild:        &RebuildSpec{Strategy: "unknown"},
	}
	if err := ValidateEntitySpec(invalid); err == nil {
		t.Fatal("expected error for invalid summary rebuild strategy")
	}
}

func TestValidateDocumentSpec_NaturalKeyRuleFormat(t *testing.T) {
	tests := []struct {
		name    string
		spec    *EntitySpec
		wantErr bool
	}{
		{
			name: "daily with day placeholder",
			spec: &EntitySpec{Fields: []Field{
				{Name: "q", NaturalKey: true, NaturalKeyRule: &NaturalKeyRuleDecl{
					Strategy: "sequence", Format: "{prefix}{year}{month}{day}-{seq:03d}", Reset: "daily",
				}},
			}},
			wantErr: false,
		},
		{
			name: "daily with period placeholder",
			spec: &EntitySpec{Fields: []Field{
				{Name: "q", NaturalKey: true, NaturalKeyRule: &NaturalKeyRuleDecl{
					Strategy: "sequence", Format: "{prefix}-{period}-{seq:03d}", Reset: "daily",
				}},
			}},
			wantErr: false,
		},
		{
			name: "daily without date placeholder (should fail)",
			spec: &EntitySpec{Fields: []Field{
				{Name: "q", NaturalKey: true, NaturalKeyRule: &NaturalKeyRuleDecl{
					Strategy: "sequence", Format: "{prefix}-{seq:03d}", Reset: "daily",
				}},
			}},
			wantErr: true,
		},
		{
			name: "monthly with month placeholder",
			spec: &EntitySpec{Fields: []Field{
				{Name: "n", NaturalKey: true, NaturalKeyRule: &NaturalKeyRuleDecl{
					Strategy: "sequence", Format: "PAY-{year}{month}-{seq:05d}", Reset: "monthly",
				}},
			}},
			wantErr: false,
		},
		{
			name: "monthly with period placeholder",
			spec: &EntitySpec{Fields: []Field{
				{Name: "n", NaturalKey: true, NaturalKeyRule: &NaturalKeyRuleDecl{
					Strategy: "sequence", Format: "ORD-{period}-{seq:03d}", Reset: "monthly",
				}},
			}},
			wantErr: false,
		},
		{
			name: "monthly without date placeholder (should fail)",
			spec: &EntitySpec{Fields: []Field{
				{Name: "n", NaturalKey: true, NaturalKeyRule: &NaturalKeyRuleDecl{
					Strategy: "sequence", Format: "PAY-{seq:05d}", Reset: "monthly",
				}},
			}},
			wantErr: true,
		},
		{
			name: "yearly with year placeholder",
			spec: &EntitySpec{Fields: []Field{
				{Name: "n", NaturalKey: true, NaturalKeyRule: &NaturalKeyRuleDecl{
					Strategy: "sequence", Format: "INV-{year}-{seq:05d}", Reset: "yearly",
				}},
			}},
			wantErr: false,
		},
		{
			name: "yearly without year placeholder (should fail)",
			spec: &EntitySpec{Fields: []Field{
				{Name: "n", NaturalKey: true, NaturalKeyRule: &NaturalKeyRuleDecl{
					Strategy: "sequence", Format: "INV-{seq:05d}", Reset: "yearly",
				}},
			}},
			wantErr: true,
		},
		{
			name: "never reset without date (ok)",
			spec: &EntitySpec{Fields: []Field{
				{Name: "n", NaturalKey: true, NaturalKeyRule: &NaturalKeyRuleDecl{
					Strategy: "sequence", Format: "INV-{seq:05d}", Reset: "never",
				}},
			}},
			wantErr: false,
		},
		{
			name: "empty reset without date (ok)",
			spec: &EntitySpec{Fields: []Field{
				{Name: "n", NaturalKey: true, NaturalKeyRule: &NaturalKeyRuleDecl{
					Strategy: "sequence", Format: "INV-{seq:05d}",
				}},
			}},
			wantErr: false,
		},
		{
			name: "daily with default format (empty, uses default which has period)",
			spec: &EntitySpec{Fields: []Field{
				{Name: "q", NaturalKey: true, NaturalKeyRule: &NaturalKeyRuleDecl{
					Strategy: "sequence", Reset: "daily",
				}},
			}},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateDocumentSpec(tt.spec)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateDocumentSpec() error = %v, wantErr = %v", err, tt.wantErr)
			}
		})
	}
}

func TestValidateEntitySpec_NaturalKeyImpliesUnique(t *testing.T) {
	spec := &EntitySpec{Fields: []Field{
		{Name: "code", Type: FieldString, NaturalKey: true},
	}}
	if err := ValidateEntitySpec(spec); err != nil {
		t.Fatalf("natural_key alone must be valid (unique is implied), got %v", err)
	}
	if !spec.Fields[0].Unique {
		t.Error("natural_key must resolve to Unique=true for the storage layer")
	}
	// Presence is the author's choice, NOT an engine implication: the two modes
	// are "generated on insert" and "filled by a person, maybe later". Asserting
	// required here would demand a value the author never declared.
	if spec.Fields[0].Required {
		t.Error("natural_key must NOT imply Required — presence is the author's decision")
	}
	if spec.NaturalKeyField != "code" {
		t.Errorf("NaturalKeyField = %q, want %q", spec.NaturalKeyField, "code")
	}
}

// TestValidateEntitySpec_NaturalKeyPresenceModes pins the two supported modes:
// a generated key (rule) and a user-filled key, the latter with or without
// `required`. All three must load — the choice belongs to the manifest.
func TestValidateEntitySpec_NaturalKeyPresenceModes(t *testing.T) {
	generated := &EntitySpec{Fields: []Field{
		{Name: "number", Type: FieldString, NaturalKey: true, NaturalKeyRule: &NaturalKeyRuleDecl{Strategy: "sequence"}},
	}}
	userFilledOptional := &EntitySpec{Fields: []Field{
		// A chart-of-accounts code: the user MAY fill it in.
		{Name: "code", Type: FieldString, NaturalKey: true},
	}}
	userFilledRequired := &EntitySpec{Fields: []Field{
		{Name: "code", Type: FieldString, NaturalKey: true, Required: true},
	}}

	for _, tc := range []struct {
		name string
		spec *EntitySpec
	}{
		{"generated", generated},
		{"user-filled optional", userFilledOptional},
		{"user-filled required", userFilledRequired},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if err := ValidateEntitySpec(tc.spec); err != nil {
				t.Fatalf("mode must be valid, got %v", err)
			}
			if !tc.spec.Fields[0].Unique {
				t.Error("every mode must still be unique")
			}
		})
	}
}

// TestValidateEntitySpec_NaturalKeyRejectsExplicitUniqueFalse pins the
// presence-flag contract: uniqueness is the ONE guarantee `natural_key` makes,
// so writing `unique: false` contradicts the declaration and is refused — while
// omitting it stays valid (the normal case, and what every core manifest does).
// Only a presence flag can tell those two apart; a plain bool decodes both to
// the same value.
func TestValidateEntitySpec_NaturalKeyRejectsExplicitUniqueFalse(t *testing.T) {
	contradiction := Field{Name: "code", Type: FieldString, NaturalKey: true, uniqueSet: true}
	if err := ValidateEntitySpec(&EntitySpec{Fields: []Field{contradiction}}); err == nil {
		t.Fatal("expected a contradiction error for explicit `unique: false`")
	}
}

// TestFieldUnmarshalYAML_RecordsExplicitKeys proves the presence flag is
// actually set by decoding — the validator above is only as good as this.
func TestFieldUnmarshalYAML_RecordsExplicitKeys(t *testing.T) {
	var doc struct {
		Fields []Field `yaml:"fields"`
	}
	src := `
fields:
  - { name: a, type: string, natural_key: true, unique: false }
  - { name: b, type: string, natural_key: true }
`
	if err := yaml.Unmarshal([]byte(src), &doc); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len(doc.Fields) != 2 {
		t.Fatalf("expected 2 fields, got %d", len(doc.Fields))
	}
	if !doc.Fields[0].uniqueSet {
		t.Error("field a: explicit `unique: false` must set uniqueSet")
	}
	if doc.Fields[1].uniqueSet {
		t.Error("field b: omitted key must not set the presence flag")
	}
}

func TestValidateEntitySpec_OnlyOneNaturalKey(t *testing.T) {
	spec := &EntitySpec{Fields: []Field{
		{Name: "code", Type: FieldString, NaturalKey: true},
		{Name: "sku", Type: FieldString, NaturalKey: true},
	}}
	if err := ValidateEntitySpec(spec); err == nil {
		t.Fatal("expected an error for two natural_key fields")
	}
}
