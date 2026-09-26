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

// The Form must follow the Entity's cardinality. Before this gate, a Form could
// declare `widget: select` on a field the Entity declared a set (or the reverse)
// and `formspec check` reported nothing — the contradiction only surfaced as a
// control the data could not honour, in the browser.
func TestCheckForms_WidgetCardinalityContradiction(t *testing.T) {
	dir := t.TempDir()
	write := func(rel, content string) {
		path := filepath.Join(dir, rel)
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatalf("mkdir %s: %v", filepath.Dir(path), err)
		}
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			t.Fatalf("write %s: %v", rel, err)
		}
	}

	// The Entity is the authority: `days_of_week` is a set, `channel` one value.
	write("modules/alpha/master/promo/entity.yaml", `apiVersion: formspec.dev/v1
kind: Entity
metadata:
  name: promo
  module: alpha
spec:
  version: v1
  characteristic: master
  fields:
    - name: code
      type: string
    - name: days_of_week
      type: json
      multiple: true
      options:
        - { value: 1, label: Senin }
        - { value: 2, label: Selasa }
    - name: channel
      type: string
      multiple: false
      options:
        - { value: pos, label: POS }
        - { value: qris, label: QRIS }
`)

	write("modules/alpha/forms/promo-form.yaml", `apiVersion: formspec.dev/v1
kind: Form
metadata:
  name: promo-form
  module: alpha
spec:
  entity: alpha.promo
  sections:
    - title: Main
      fields:
        - field: code
        - { field: days_of_week, widget: select }
        - { field: channel, widget: select-multi-tag }
`)

	loader := manifest.NewLoader(dir)
	res, err := loader.LoadAll()
	if err != nil {
		t.Fatalf("LoadAll: %v", err)
	}

	idx := buildEntityIndex(res.Manifests)
	result := &checkResult{}
	checkForms(result, idx, res.Manifests)

	var setAsSingle, singleAsTags bool
	for _, i := range result.Issues {
		if i.Kind != "error" {
			continue
		}
		if strings.Contains(i.Message, "days_of_week") &&
			strings.Contains(i.Message, "`widget: select` picks one value") {
			setAsSingle = true
		}
		if strings.Contains(i.Message, "channel") &&
			strings.Contains(i.Message, "needs a field that holds a set") {
			singleAsTags = true
		}
	}
	if !setAsSingle {
		t.Errorf("a one-value select on a set field must be reported, got: %+v", result.Issues)
	}
	if !singleAsTags {
		t.Errorf("the tag widget on a single-value field must be reported, got: %+v", result.Issues)
	}
	if len(result.Issues) != 2 {
		t.Errorf("want exactly the two contradictions, got %d: %+v", len(result.Issues), result.Issues)
	}
}

// The gate must not fire on the shapes it is not about: a widget that says
// nothing about cardinality, and a field with no declared set at all.
func TestCheckForms_WidgetCardinalityAllowsUnrelatedWidgets(t *testing.T) {
	dir := t.TempDir()
	write := func(rel, content string) {
		path := filepath.Join(dir, rel)
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatalf("mkdir %s: %v", filepath.Dir(path), err)
		}
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			t.Fatalf("write %s: %v", rel, err)
		}
	}

	write("modules/alpha/master/promo/entity.yaml", `apiVersion: formspec.dev/v1
kind: Entity
metadata:
  name: promo
  module: alpha
spec:
  version: v1
  characteristic: master
  fields:
    - name: days_of_week
      type: json
      multiple: true
      options:
        - { value: 1, label: Senin }
    - name: code
      type: string
    - name: payload
      type: json
`)

	write("modules/alpha/forms/promo-form.yaml", `apiVersion: formspec.dev/v1
kind: Form
metadata:
  name: promo-form
  module: alpha
spec:
  entity: alpha.promo
  sections:
    - title: Main
      fields:
        # A cardinality-neutral widget on a set field is fine — only the
        # one-value pickers contradict it.
        - { field: days_of_week, widget: input }
        - { field: code, widget: input }
        # No declared options: nothing to be multiple of, so tags is allowed.
        - { field: payload, widget: select-multi-tag }
`)

	loader := manifest.NewLoader(dir)
	res, err := loader.LoadAll()
	if err != nil {
		t.Fatalf("LoadAll: %v", err)
	}

	idx := buildEntityIndex(res.Manifests)
	result := &checkResult{}
	checkForms(result, idx, res.Manifests)

	if len(result.Issues) != 0 {
		t.Fatalf("no cardinality contradiction expected, got: %+v", result.Issues)
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

// TestValidateExprGrammar_AcceptsValidExpressions pins the expressions that
// real manifests use (including the kafe promo-form, which regressed at runtime
// because the previous gate only balanced delimiters and never tokenized).
func TestValidateExprGrammar_AcceptsValidExpressions(t *testing.T) {
	valid := []string{
		// Single-quoted literals — the kafe promo-form regression. Starlark
		// accepts both quote styles, so these must survive the gate.
		"fields.type == 'percentage'",
		"fields.type == 'fixed'",
		"fields.type == 'buy_x_get_y'",
		"fields.applies_to == 'menu_item'",
		"fields.applies_to == 'category'",
		// Double-quoted literals keep working.
		`fields.method == "cash"`,
		// Compound conditions, list comprehensions, arithmetic.
		"fields.status == 'draft' or fields.status == 'awaiting_payment'",
		"fields.manual_discount_amount != null",
		"fields.items != None and len(fields.items) > 0",
		"sum([i.quantity * i.unit_price for i in fields.items])",
		"sum([t.quantity * t.price for t in fields.treatments])",
		"fields.tendered >= fields.amount",
		"fields.total * 0.1",
		"!!true",
		"fields.status != 'open' and len(fields.notes) > 0",
		// Delimiters inside a string literal are opaque, not nesting.
		`fields.name != "("`,
		"fields.name != '('",
		// An apostrophe inside a double-quoted string does not end it.
		`fields.name != "it's"`,
		// The closed callable set — every member must stay accepted. A `(` that
		// only LOOKS like a call (inside a string literal) is content, not a
		// call, so `fields.name == 'f('` must survive the callable gate.
		"len(fields.items) > 0",
		"sum([i.qty for i in fields.items]) > 0",
		"amount(fields.total) > 100",
		"currency(fields.total) == 'IDR'",
		"today() >= '2026-01-01'",
		"fields.name == 'f('",
		"fields.note == \"sum(x)\"",
		// Whitespace between the callee and `(` is still a call.
		"len (fields.items) > 0",
		"",
	}
	for _, expr := range valid {
		if err := validateExprGrammar(expr); err != "" {
			t.Errorf("validateExprGrammar(%q) = %q, want accepted", expr, err)
		}
	}
}

// TestValidateExprGrammar_RejectsUnknownCallables pins the closed callable set.
//
// The character scan alone cannot tell `len(x)` from `user.has('x')` — both are
// identifiers, dots and parens — so before this gate existed, a member call
// passed `formspec check` and then died at RUNTIME as "unknown function:
// undefined" in the client evaluator (which resolves `node.callee.name` over an
// Identifier). That is exactly the false guarantee 08-formspec-expr.md §4
// forbids: an expression surviving apply is supposed to be resolvable.
//
// Measured on the live example: examples/Clinic-UI-Showcase shipped
// `when: "user.has('clinic.settings.update')"`, which validated green (0
// problems) and hid nothing — identity-based conditions are also outside the
// expression subset per §3.
func TestValidateExprGrammar_RejectsUnknownCallables(t *testing.T) {
	cases := []struct {
		expr string
		want string
	}{
		// The live regression: a member call the evaluator cannot resolve.
		{"user.has('clinic.settings.update')", "unknown function \"user.has\""},
		{"session.x()", "unknown function \"session.x\""},
		// A plain typo is caught too — same class, no member access needed.
		{"leng(fields.items) > 0", "unknown function \"leng\""},
		{"lenn(fields.items) > 0", "unknown function \"lenn\""},
		// Server-only Starlark builtins are NOT part of the client subset.
		{"days_ago(1)", "unknown function \"days_ago\""},
		{"empty(fields.note)", "unknown function \"empty\""},
		{"sum_line('debit') > 0", "unknown function \"sum_line\""},
	}
	for _, tc := range cases {
		got := validateExprGrammar(tc.expr)
		if got == "" {
			t.Errorf("validateExprGrammar(%q) accepted, want error containing %q", tc.expr, tc.want)
			continue
		}
		if !strings.Contains(got, tc.want) {
			t.Errorf("validateExprGrammar(%q) = %q, want error containing %q", tc.expr, got, tc.want)
		}
		// The message must say what IS allowed, so the author can fix it without
		// reading the source.
		if !strings.Contains(got, "closed set") {
			t.Errorf("validateExprGrammar(%q) = %q, want the message to name the closed callable set", tc.expr, got)
		}
	}
}

// TestCheckMenuExpr pins the deploy-time gate for `MenuItem.When`, the one
// FormSpecExpr site that had no gate at all before this.
func TestCheckMenuExpr(t *testing.T) {
	loader := manifest.NewLoader("")
	src := `
apiVersion: formspec.dev/v1alpha1
kind: App
metadata: { name: demo, module: demo }
spec:
  root_url: /app/demo
  modules: [demo]
  menu:
    - { label: "Broken", route: /x, when: "user.has('demo.x')" }
    - { label: "Fine", route: /y, when: "today() >= '2026-01-01'" }
`
	raws, errs := loader.ParseBytes([]byte(src), "app.yaml")
	if len(errs) > 0 {
		t.Fatalf("parse: %v", errs)
	}

	result := &checkResult{}
	checkMenuExpr(result, raws)

	if len(result.Issues) != 1 {
		t.Fatalf("want exactly 1 issue (the broken `when`), got %d: %+v", len(result.Issues), result.Issues)
	}
	msg := result.Issues[0].Message
	if !strings.Contains(msg, "unknown function") || !strings.Contains(msg, "Broken") {
		t.Errorf("issue should name the offending callable and the item, got %q", msg)
	}
}

// TestCheckMenuExpr_ModuleMenu covers the other manifest that carries a menu:
// a Module's default suggestion is spliced into Apps, so a broken `when` there
// must be caught at its source.
func TestCheckMenuExpr_ModuleMenu(t *testing.T) {
	loader := manifest.NewLoader("")
	src := `
apiVersion: formspec.dev/v1alpha1
kind: Module
metadata: { name: demo }
spec:
  version: 1.0.0
  menu:
    - label: "Group"
      children:
        - { label: "Nested broken", route: /x, when: "leng(1) > 0" }
`
	raws, errs := loader.ParseBytes([]byte(src), "module.yaml")
	if len(errs) > 0 {
		t.Fatalf("parse: %v", errs)
	}

	result := &checkResult{}
	checkMenuExpr(result, raws)

	if len(result.Issues) != 1 {
		t.Fatalf("want 1 issue from the nested item, got %d: %+v", len(result.Issues), result.Issues)
	}
	if !strings.Contains(result.Issues[0].Message, "Nested broken") {
		t.Errorf("issue should be attributed to the nested item, got %q", result.Issues[0].Message)
	}
}

// TestValidateExprGrammar_RejectsUnparsableExpressions pins the class the
// previous gate let through: expressions the client renderer cannot parse, so
// they surfaced as a runtime "Expression error" banner instead of failing at
// deploy time (08-formspec-expr.md §4).
func TestValidateExprGrammar_RejectsUnparsableExpressions(t *testing.T) {
	cases := []struct {
		expr string
		want string
	}{
		// A quote typo — the exact shape that broke promo-form.
		{"fields.status == 'open", "unterminated string literal"},
		{`fields.status == "open`, "unterminated string literal"},
		// A lone `=` is not FormSpecExpr (the client lexer marks it ILLEGAL).
		{"fields.status = 'open'", `operator "="`},
		// Unbalanced / mismatched delimiters.
		{"(fields.status == 'open'", "unbalanced delimiter"},
		{"fields.status == 'open')", "unbalanced delimiter"},
		{"len(fields.items]", "mismatched delimiter"},
		// Dict/set literals and blocks are outside the subset.
		{"len({})", "dict/set literals"},
		// Forbidden constructs.
		{"ctx.db.query('x')", "ctx access"},
		// Unparseable characters.
		{"fields.status == 'open' # comment", "not part of FormSpecExpr"},
		{"fields.status == 'open' @ 1", "not part of FormSpecExpr"},
	}
	for _, tc := range cases {
		got := validateExprGrammar(tc.expr)
		if got == "" {
			t.Errorf("validateExprGrammar(%q) accepted, want error containing %q", tc.expr, tc.want)
			continue
		}
		if !strings.Contains(got, tc.want) {
			t.Errorf("validateExprGrammar(%q) = %q, want error containing %q", tc.expr, got, tc.want)
		}
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
