package api

import (
	"testing"

	"github.com/primadi/formspec/internal/auth"
	"github.com/primadi/formspec/pkg/spec"
)

func TestSanitizeData_Masked(t *testing.T) {
	es := &spec.EntitySpec{Fields: []spec.Field{
		{Name: "password_hash", Type: spec.FieldString, Masked: true},
		{Name: "name", Type: spec.FieldString},
	}}
	data := map[string]any{"password_hash": "secret123", "name": "alice"}
	out := sanitizeData(es, nil, "ui", data)
	if out["password_hash"] == "secret123" {
		t.Error("expected password_hash masked")
	}
	if out["name"] != "alice" {
		t.Error("expected name unchanged")
	}
}

func TestSanitizeData_RequiredPermission(t *testing.T) {
	es := &spec.EntitySpec{Fields: []spec.Field{
		{Name: "salary", Type: spec.FieldInteger, RequiredPermission: "hr.salary.view"},
		{Name: "name", Type: spec.FieldString},
	}}
	data := map[string]any{"salary": 100, "name": "alice"}

	// Identity without the field permission → salary removed.
	id := &auth.Identity{Permissions: []string{"hr.employee.view"}}
	out := sanitizeData(es, id, "ui", data)
	if _, ok := out["salary"]; ok {
		t.Error("expected salary removed (no permission)")
	}

	// Identity with the field permission → salary present.
	id2 := &auth.Identity{Permissions: []string{"hr.salary.view"}}
	out2 := sanitizeData(es, id2, "ui", data)
	if out2["salary"] != 100 {
		t.Error("expected salary present (has permission)")
	}
}

func TestSanitizeData_ExcludeSurface(t *testing.T) {
	es := &spec.EntitySpec{Fields: []spec.Field{
		{Name: "secret", Type: spec.FieldString, Exclude: []string{"public_api"}},
		{Name: "name", Type: spec.FieldString},
	}}
	data := map[string]any{"secret": "x", "name": "alice"}

	// public_api surface → secret excluded.
	out := sanitizeData(es, nil, "public_api", data)
	if _, ok := out["secret"]; ok {
		t.Error("expected secret excluded on public_api")
	}

	// ui surface → secret present.
	out2 := sanitizeData(es, nil, "ui", data)
	if out2["secret"] != "x" {
		t.Error("expected secret present on ui")
	}
}

func TestSanitizeData_DoesNotMutateInput(t *testing.T) {
	es := &spec.EntitySpec{Fields: []spec.Field{
		{Name: "password_hash", Type: spec.FieldString, Masked: true},
	}}
	data := map[string]any{"password_hash": "secret123"}
	_ = sanitizeData(es, nil, "ui", data)
	if data["password_hash"] != "secret123" {
		t.Error("sanitizeData must not mutate the input map")
	}
}
