package main

import (
	"strings"
	"testing"

	"gopkg.in/yaml.v3"

	"github.com/primadi/formspec/internal/manifest"
)

// validateSchemaString runs the JSON Schema layer over a single YAML manifest.
func validateSchemaString(t *testing.T, ksc *kindSchemaCompiler, yamlDoc string) string {
	t.Helper()
	var raw manifest.RawManifest
	if err := yaml.Unmarshal([]byte(yamlDoc), &raw); err != nil {
		t.Fatalf("parse manifest: %v", err)
	}
	raw.Source = "test.yaml#0"
	return validateSchema(ksc, raw)
}

func newTestCompiler(t *testing.T) *kindSchemaCompiler {
	t.Helper()
	// Compiled against the repo's generated schemas (../../schemas from the
	// cmd/formspec package dir). The registry/cache path is exercised by the
	// schemaregistry package tests.
	ksc, err := newKindSchemaCompiler("../../schemas")
	if err != nil {
		t.Fatalf("compiler: %v", err)
	}
	return ksc
}

func TestValidateSchema_ValidEntity(t *testing.T) {
	ksc := newTestCompiler(t)
	doc := `
apiVersion: formspec.dev/v1
kind: Entity
metadata: { name: widget, module: inventory }
spec:
  version: v1
  characteristic: master
  fields:
    - { name: sku, type: string, required: true }
`
	if errMsg := validateSchemaString(t, ksc, doc); errMsg != "" {
		t.Fatalf("expected valid, got: %s", errMsg)
	}
}

func TestValidateSchema_ExposeShorthandRejected(t *testing.T) {
	ksc := newTestCompiler(t)
	doc := `
apiVersion: formspec.dev/v1
kind: Entity
metadata: { name: widget, module: inventory }
spec:
  version: v1
  characteristic: master
  expose: all
  fields:
    - { name: sku, type: string }
`
	errMsg := validateSchemaString(t, ksc, doc)
	if errMsg == "" {
		t.Fatal("expected expose: all to be rejected by schema")
	}
	if !strings.Contains(errMsg, "expose") {
		t.Fatalf("expected message about expose, got: %s", errMsg)
	}
}

func TestValidateSchema_LifecycleMapRejected(t *testing.T) {
	ksc := newTestCompiler(t)
	doc := `
apiVersion: formspec.dev/v1
kind: Entity
metadata: { name: widget, module: inventory }
spec:
  version: v1
  characteristic: master
  lifecycle: { doc_status: true }
  fields:
    - { name: sku, type: string }
`
	errMsg := validateSchemaString(t, ksc, doc)
	if errMsg == "" {
		t.Fatal("expected lifecycle map to be rejected by schema")
	}
}

func TestValidateSchema_RelationShorthandRejected(t *testing.T) {
	ksc := newTestCompiler(t)
	// `target` is not a valid Field property; canonical is a sibling `relation`.
	doc := `
apiVersion: formspec.dev/v1
kind: Entity
metadata: { name: widget, module: inventory }
spec:
  version: v1
  characteristic: master
  fields:
    - { name: owner_id, type: relation, target: inventory.customer }
`
	errMsg := validateSchemaString(t, ksc, doc)
	if errMsg == "" {
		t.Fatal("expected relation with target: to be rejected by schema")
	}
}

func TestValidateSchema_CanonicalRelationAccepted(t *testing.T) {
	ksc := newTestCompiler(t)
	doc := `
apiVersion: formspec.dev/v1
kind: Entity
metadata: { name: widget, module: inventory }
spec:
  version: v1
  characteristic: master
  fields:
    - name: owner_id
      type: relation
      relation: { type: belongs_to, resource: inventory.customer }
`
	if errMsg := validateSchemaString(t, ksc, doc); errMsg != "" {
		t.Fatalf("expected canonical relation to pass, got: %s", errMsg)
	}
}

func TestValidateSchema_WorkflowStatesRejected(t *testing.T) {
	ksc := newTestCompiler(t)
	// The stale Workflow shape (custom states/transitions) must be rejected.
	doc := `
apiVersion: formspec.dev/v1
kind: Workflow
metadata: { name: wf, module: gl }
spec:
  entity: gl.journal-entry
  states:
    - { name: draft }
  transitions:
    - { from: draft, to: posted, action: post }
`
	errMsg := validateSchemaString(t, ksc, doc)
	if errMsg == "" {
		t.Fatal("expected stale Workflow shape to be rejected by schema")
	}
}

func TestValidateSchema_CanonicalWorkflowAccepted(t *testing.T) {
	ksc := newTestCompiler(t)
	doc := `
apiVersion: formspec.dev/v1
kind: Workflow
metadata: { name: wf, module: gl }
spec:
  entity: gl.journal-entry
  on: { transition: { from: draft, to: posted } }
  steps:
    - { roles: [gl.supervisor], approvers: 1 }
  on_reject: { to: rejected }
`
	if errMsg := validateSchemaString(t, ksc, doc); errMsg != "" {
		t.Fatalf("expected canonical Workflow to pass, got: %s", errMsg)
	}
}

func TestValidateSchema_AppRequiresRootURL(t *testing.T) {
	ksc := newTestCompiler(t)
	doc := `
apiVersion: formspec.dev/v1
kind: App
metadata: { name: myapp, description: "demo" }
spec:
  modules: [inventory]
`
	errMsg := validateSchemaString(t, ksc, doc)
	if errMsg == "" {
		t.Fatal("expected App without version/vendor/root_url to be rejected")
	}
}

// ─── Integrator cross-manifest validation (7.7.2, 7.7.3) ───

func TestValidateIntegrators_SymmetricCancel(t *testing.T) {
	manifests := []manifest.RawManifest{
		{
			Kind:     "Integrator",
			Metadata: manifest.RawMetadata{Name: "invoice-to-gl", Module: "billing"},
			Spec: map[string]any{
				"listen": map[string]any{"resource": "billing.invoice", "event": "on_submit"},
				"call":   map[string]any{"resource": "gl.journal-entry", "action": "create"},
			},
		},
	}

	rejects := validateIntegrators(manifests)
	if len(rejects) == 0 {
		t.Fatal("expected integrator without symmetric cancel handler to be rejected (7.7.2)")
	}
}

func TestValidateIntegrators_SymmetricCancel_OK(t *testing.T) {
	manifests := []manifest.RawManifest{
		{
			Kind:     "Integrator",
			Metadata: manifest.RawMetadata{Name: "invoice-to-gl", Module: "billing"},
			Spec: map[string]any{
				"listen": map[string]any{"resource": "billing.invoice", "event": "on_submit"},
				"call":   map[string]any{"resource": "gl.journal-entry", "action": "create"},
			},
		},
		{
			Kind:     "Integrator",
			Metadata: manifest.RawMetadata{Name: "invoice-cancel-to-gl", Module: "billing"},
			Spec: map[string]any{
				"listen": map[string]any{"resource": "billing.invoice", "event": "on_cancel"},
				"call":   map[string]any{"resource": "gl.journal-entry", "action": "cancel"},
			},
		},
	}

	rejects := validateIntegrators(manifests)
	if len(rejects) != 0 {
		t.Fatalf("expected no rejects with symmetric cancel handler, got %v", rejects)
	}
}

func TestValidateIntegrators_IdempotentTarget(t *testing.T) {
	manifests := []manifest.RawManifest{
		{
			Kind:     "Integrator",
			Metadata: manifest.RawMetadata{Name: "invoice-to-gl", Module: "billing"},
			Spec: map[string]any{
				"listen": map[string]any{"resource": "billing.invoice", "event": "on_submit"},
				"call":   map[string]any{"resource": "gl.journal-entry", "action": "create"},
			},
		},
		{
			Kind:     "Integrator",
			Metadata: manifest.RawMetadata{Name: "invoice-cancel-to-gl", Module: "billing"},
			Spec: map[string]any{
				"listen": map[string]any{"resource": "billing.invoice", "event": "on_cancel"},
				"call":   map[string]any{"resource": "gl.journal-entry", "action": "cancel"},
			},
		},
		{
			Kind:     "Entity",
			Metadata: manifest.RawMetadata{Name: "journal-entry", Module: "gl"},
			Spec: map[string]any{
				"actions": []any{
					map[string]any{"name": "create"}, // NOT idempotent
					map[string]any{"name": "cancel", "idempotent": true},
				},
			},
		},
	}

	rejects := validateIntegrators(manifests)
	// The on_submit integrator targets gl.journal-entry.create (not idempotent)
	// → must be rejected (7.7.3).
	found := false
	for _, msg := range rejects {
		if strings.Contains(msg, "idempotent") {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected idempotent-target rejection, got %v", rejects)
	}
}

func TestValidateIntegrators_IdempotentTarget_OK(t *testing.T) {
	manifests := []manifest.RawManifest{
		{
			Kind:     "Integrator",
			Metadata: manifest.RawMetadata{Name: "invoice-to-gl", Module: "billing"},
			Spec: map[string]any{
				"listen": map[string]any{"resource": "billing.invoice", "event": "on_submit"},
				"call":   map[string]any{"resource": "gl.journal-entry", "action": "create"},
			},
		},
		{
			Kind:     "Integrator",
			Metadata: manifest.RawMetadata{Name: "invoice-cancel-to-gl", Module: "billing"},
			Spec: map[string]any{
				"listen": map[string]any{"resource": "billing.invoice", "event": "on_cancel"},
				"call":   map[string]any{"resource": "gl.journal-entry", "action": "cancel"},
			},
		},
		{
			Kind:     "Entity",
			Metadata: manifest.RawMetadata{Name: "journal-entry", Module: "gl"},
			Spec: map[string]any{
				"actions": []any{
					map[string]any{"name": "create", "idempotent": true},
					map[string]any{"name": "cancel", "idempotent": true},
				},
			},
		},
	}

	rejects := validateIntegrators(manifests)
	if len(rejects) != 0 {
		t.Fatalf("expected no rejects with idempotent targets, got %v", rejects)
	}
}
