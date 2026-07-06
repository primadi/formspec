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
