package spec

import (
	"testing"
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

	if err := ValidateEntitySpec(base(&UnitDecl{Base: "gram", Convertible: []string{"kg"}})); err != nil {
		t.Errorf("valid unit: expected no error, got %v", err)
	}

	bad := []struct {
		name string
		u    *UnitDecl
	}{
		{"missing base", &UnitDecl{Convertible: []string{"kg"}}},
		{"base not in enum_values", &UnitDecl{Base: "ounce"}},
		{"convertible not in enum_values", &UnitDecl{Base: "gram", Convertible: []string{"ounce"}}},
		{"convertible repeats base", &UnitDecl{Base: "gram", Convertible: []string{"gram"}}},
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
