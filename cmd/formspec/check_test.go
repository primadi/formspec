package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/primadi/formspec/internal/manifest"
)

// writeCheckSpec writes a spec tree with known cross-file errors:
//   - Form field referencing a nonexistent entity field
//   - FormSpecExpr referencing a nonexistent field
//   - action uses.resources referencing nonexistent entities
func writeCheckSpec(t *testing.T, dir string) {
	t.Helper()
	write := func(rel, content string) {
		path := filepath.Join(dir, rel)
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatalf("mkdir %s: %v", filepath.Dir(path), err)
		}
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			t.Fatalf("write %s: %v", rel, err)
		}
	}

	write("modules/alpha/master/order/entity.yaml", `apiVersion: formspec.dev/v1
kind: Entity
metadata:
  name: order
  module: alpha
spec:
  version: v1
  characteristic: master
  fields:
    - name: code
      type: string
    - name: total
      type: money
  actions:
    - name: ship
      uses:
        resources: [beta.customer, gamma.product]
  expose:
    - type: rest
      actions: [list, find, create, update, delete]
`)

	write("modules/alpha/forms/order-form.yaml", `apiVersion: formspec.dev/v1
kind: Form
metadata:
  name: order-form
  module: alpha
spec:
  entity: alpha.order
  sections:
    - title: Main
      fields:
        - field: code
        - field: nonexistent_field
        - field: total
          visible_when: "fields.nonexistent_field == 'x'"
`)
}

// runCheckCollect runs runCheck in-process and captures stdout/stderr.
// runCheck calls os.Exit on errors, so we run it via a helper that returns
// the issues instead. To keep it simple, we test the underlying analysis
// functions directly.
func TestCheckForms_FieldAndExprErrors(t *testing.T) {
	dir := t.TempDir()
	writeCheckSpec(t, dir)

	loader := manifest.NewLoader(dir)
	res, err := loader.LoadAll()
	if err != nil {
		t.Fatalf("LoadAll: %v", err)
	}

	idx := buildEntityIndex(res.Manifests)
	result := &checkResult{}
	checkForms(result, idx, res.Manifests)

	var fieldErr, exprErr bool
	for _, i := range result.Issues {
		if strings.Contains(i.Message, "nonexistent_field") && strings.Contains(i.Message, "references field") {
			fieldErr = true
		}
		if strings.Contains(i.Message, "FormSpecExpr") && strings.Contains(i.Message, "nonexistent_field") {
			exprErr = true
		}
	}
	if !fieldErr {
		t.Fatalf("expected Form field reference error, got issues: %+v", result.Issues)
	}
	if !exprErr {
		t.Fatalf("expected FormSpecExpr field reference error, got issues: %+v", result.Issues)
	}
}

func TestCheckUses_UnknownResourceError(t *testing.T) {
	dir := t.TempDir()
	writeCheckSpec(t, dir)

	loader := manifest.NewLoader(dir)
	res, err := loader.LoadAll()
	if err != nil {
		t.Fatalf("LoadAll: %v", err)
	}

	idx := buildEntityIndex(res.Manifests)
	result := &checkResult{}
	broken := checkUses(result, idx, res.Manifests)

	if len(broken) != 2 {
		t.Fatalf("expected 2 broken refs, got %d: %+v", len(broken), broken)
	}
	if !result.hasErrors() {
		t.Fatalf("expected errors for unknown resources")
	}
}

// writeCheckKanbanWizardSpec writes a spec tree with Kanban drag_guard and
// Wizard step expressions referencing a nonexistent entity field.
func writeCheckKanbanWizardSpec(t *testing.T, dir string) {
	t.Helper()
	write := func(rel, content string) {
		path := filepath.Join(dir, rel)
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatalf("mkdir %s: %v", filepath.Dir(path), err)
		}
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			t.Fatalf("write %s: %v", rel, err)
		}
	}

	write("modules/alpha/master/ticket/entity.yaml", `apiVersion: formspec.dev/v1
kind: Entity
metadata:
  name: ticket
  module: alpha
spec:
  version: v1
  characteristic: transaction
  fields:
    - name: status
      type: enum
      enum_values: [open, closed]
    - name: priority
      type: string
  state_machine:
    field: status
    initial: open
    states:
      - name: open
      - name: closed
    transitions:
      - from: [open]
        to: closed
        via: close
  expose:
    - type: rest
      actions: [list, find, create, update, delete]
`)

	write("modules/alpha/kanbans/ticket-board.yaml", `apiVersion: formspec.dev/v1
kind: Kanban
metadata:
  name: ticket-board
  module: alpha
spec:
  entity: alpha.ticket
  status_field: status
  drag_guard: "fields.nonexistent_field == 'x'"
`)

	write("modules/alpha/wizards/close-ticket.yaml", `apiVersion: formspec.dev/v1
kind: Wizard
metadata:
  name: close-ticket
  module: alpha
spec:
  entity: alpha.ticket
  steps:
    - title: Review
      fields:
        - field: status
          visible_when: "fields.nonexistent_field == 'x'"
`)

	write("modules/alpha/forms/ticket-form.yaml", `apiVersion: formspec.dev/v1
kind: Form
metadata:
  name: ticket-form
  module: alpha
spec:
  entity: alpha.ticket
  sections:
    - title: Main
      fields:
        - field: status
          visible_when: "ctx.db.query('x')"
`)

	write("modules/alpha/forms/bad-form.yaml", `apiVersion: formspec.dev/v1
kind: Form
metadata:
  name: bad-form
  module: alpha
spec:
  entity: alpha.ticket
  sections:
    - title: Main
      fields:
        - field: status
          visible_when: "fields.status == 'open'"
`)

	write("modules/alpha/forms/unbalanced-form.yaml", `apiVersion: formspec.dev/v1
kind: Form
metadata:
  name: unbalanced-form
  module: alpha
spec:
  entity: alpha.ticket
  sections:
    - title: Main
      fields:
        - field: status
          visible_when: "(fields.status == 'open'"
`)
}

func TestCheckKanban_DragGuardExprError(t *testing.T) {
	dir := t.TempDir()
	writeCheckKanbanWizardSpec(t, dir)

	loader := manifest.NewLoader(dir)
	res, err := loader.LoadAll()
	if err != nil {
		t.Fatalf("LoadAll: %v", err)
	}

	idx := buildEntityIndex(res.Manifests)
	result := &checkResult{}
	checkKanban(result, idx, res.Manifests)

	var found bool
	for _, i := range result.Issues {
		if strings.Contains(i.Message, "drag_guard") && strings.Contains(i.Message, "nonexistent_field") {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected drag_guard field reference error, got issues: %+v", result.Issues)
	}
}

func TestCheckWizard_StepExprError(t *testing.T) {
	dir := t.TempDir()
	writeCheckKanbanWizardSpec(t, dir)

	loader := manifest.NewLoader(dir)
	res, err := loader.LoadAll()
	if err != nil {
		t.Fatalf("LoadAll: %v", err)
	}

	idx := buildEntityIndex(res.Manifests)
	result := &checkResult{}
	checkWizard(result, idx, res.Manifests)

	var found bool
	for _, i := range result.Issues {
		if strings.Contains(i.Message, "visible_when") && strings.Contains(i.Message, "nonexistent_field") {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected wizard step expression error, got issues: %+v", result.Issues)
	}
}

func TestCheckExpr_GrammarValidation(t *testing.T) {
	dir := t.TempDir()
	writeCheckKanbanWizardSpec(t, dir)

	loader := manifest.NewLoader(dir)
	res, err := loader.LoadAll()
	if err != nil {
		t.Fatalf("LoadAll: %v", err)
	}

	idx := buildEntityIndex(res.Manifests)
	result := &checkResult{}
	checkForms(result, idx, res.Manifests)

	var ctxErr, unbalancedErr bool
	for _, i := range result.Issues {
		if strings.Contains(i.Message, "ctx access") {
			ctxErr = true
		}
		if strings.Contains(i.Message, "unbalanced delimiter") {
			unbalancedErr = true
		}
	}
	if !ctxErr {
		t.Fatalf("expected ctx access grammar error, got issues: %+v", result.Issues)
	}
	if !unbalancedErr {
		t.Fatalf("expected unbalanced delimiter grammar error, got issues: %+v", result.Issues)
	}
}

func TestApplyUsesFix_RemovesBrokenRefs(t *testing.T) {
	dir := t.TempDir()
	writeCheckSpec(t, dir)

	entityFile := filepath.Join(dir, "modules", "alpha", "master", "order", "entity.yaml")
	broken := []brokenRef{
		{source: entityFile + "#0", file: entityFile, action: "ship", resource: "beta.customer"},
		{source: entityFile + "#0", file: entityFile, action: "ship", resource: "gamma.product"},
	}
	removed := applyUsesFix(broken)
	if len(removed) != 2 {
		t.Fatalf("expected 2 removed, got %d", len(removed))
	}

	data, err := os.ReadFile(entityFile)
	if err != nil {
		t.Fatalf("read entity: %v", err)
	}
	if strings.Contains(string(data), "beta.customer") || strings.Contains(string(data), "gamma.product") {
		t.Fatalf("broken refs not removed:\n%s", data)
	}
	if strings.Contains(string(data), "uses:") {
		t.Fatalf("empty uses block should be removed:\n%s", data)
	}
}
