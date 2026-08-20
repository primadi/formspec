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
