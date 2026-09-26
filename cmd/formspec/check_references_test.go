package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/primadi/formspec/internal/manifest"
)

// writeRefSpec writes a spec tree whose references are a mix of declared and
// dangling NAMES. It mirrors the three shapes that were green under
// `formspec validate` before this check existed (kafe 10.10 / todo 10.12):
//
//	entity:   a Report pointing at `ledger` (never declared)
//	kind:     a Table pointing at `journal_entry` (the manifest is journal-entry)
//	widget:   a Dashboard placing `recent-journals` (never written)
//
// It also carries two refs that MUST NOT be reported, because a checker that
// cries wolf is worse than none:
//
//   - a cross-module Widget named by a plain name (the Clinic-UI-Showcase case
//     — a `clinic` dashboard placing `pharmacy-queue-count` from `pharmacy`)
//   - a Page whose block targets a declared Form through a `mode: edit` ref
func writeRefSpec(t *testing.T, dir string) {
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
  expose:
    - type: rest
      actions: [list, find]
`)

	// Declared in ANOTHER module, referenced by bare name from alpha.
	write("modules/beta/master/queue-item/entity.yaml", `apiVersion: formspec.dev/v1
kind: Entity
metadata:
  name: queue-item
  module: beta
spec:
  version: v1
  characteristic: master
  fields:
    - name: label
      type: string
  expose:
    - type: rest
      actions: [list, find]
`)

	write("modules/beta/master/queue/widgets/queue-count.yaml", `apiVersion: formspec.dev/v1
kind: Widget
metadata:
  name: queue-count
  module: beta
spec:
  title: Queue
  type: metric
  entity: beta.queue-item
`)

	// A real form/table pair for the valid Page block.
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
`)

	write("modules/alpha/pages/order-page.yaml", `apiVersion: formspec.dev/v1
kind: Page
metadata:
  name: order-page
  module: alpha
spec:
  route: /orders
  title: Orders
  blocks:
    - form:
        ref: order-form
        mode: edit
`)

	// DANGLING: report entity that exists nowhere.
	write("modules/alpha/reports/trial-balance.yaml", `apiVersion: formspec.dev/v1
kind: Report
metadata:
  name: trial-balance
  module: alpha
spec:
  title: Trial Balance
  entity: ledger
  columns:
    - field: account
`)

	// DANGLING: table ref (manifest is `journal-entry`, spelled `journal_entry`).
	write("modules/alpha/pages/journal-page.yaml", `apiVersion: formspec.dev/v1
kind: Page
metadata:
  name: journal-page
  module: alpha
spec:
  route: /journals
  title: Journals
  blocks:
    - table:
        ref: journal_entry
`)

	// DANGLING: widget placed on a dashboard but declared nowhere.
	write("modules/alpha/dashboards/overview.yaml", `apiVersion: formspec.dev/v1
kind: Dashboard
metadata:
  name: overview
  module: alpha
spec:
  title: Overview
  widgets:
    - { ref: recent-journals, layout: { x: 0, y: 0, w: 1, h: 1 } }
`)

	// VALID: a bare-named widget declared by another module.
	write("modules/alpha/dashboards/mixed.yaml", `apiVersion: formspec.dev/v1
kind: Dashboard
metadata:
  name: mixed
  module: alpha
spec:
  title: Mixed
  widgets:
    - { ref: queue-count, layout: { x: 0, y: 0, w: 1, h: 1 } }
`)
}

// issueMessages returns every reported message joined for substring assertions.
func issueMessages(result *checkResult) string {
	var b strings.Builder
	for _, i := range result.Issues {
		b.WriteString(i.Message)
		b.WriteString("\n")
	}
	return b.String()
}

func TestCheckReferences_DanglingNames(t *testing.T) {
	dir := t.TempDir()
	writeRefSpec(t, dir)

	loader := manifest.NewLoader(dir)
	res, err := loader.LoadAll()
	if err != nil {
		t.Fatalf("LoadAll: %v", err)
	}

	idx := buildEntityIndex(res.Manifests)
	result := &checkResult{}
	checkReferences(result, idx, res.Manifests)
	msgs := issueMessages(result)

	for _, want := range []string{
		`Report "trial-balance" references unknown entity "ledger"`,
		`Page "journal-page" block table references unknown table "journal_entry"`,
		`Dashboard "overview" widget references unknown widget "recent-journals"`,
	} {
		if !strings.Contains(msgs, want) {
			t.Errorf("missing finding %q\n--- got ---\n%s", want, msgs)
		}
	}
}

// A checker that reports working manifests as broken is worse than no checker:
// the author learns to ignore it. These two shapes are declared cross-module by
// a plain name, and both are legitimate.
func TestCheckReferences_NoFalsePositives(t *testing.T) {
	dir := t.TempDir()
	writeRefSpec(t, dir)

	loader := manifest.NewLoader(dir)
	res, err := loader.LoadAll()
	if err != nil {
		t.Fatalf("LoadAll: %v", err)
	}

	idx := buildEntityIndex(res.Manifests)
	result := &checkResult{}
	checkReferences(result, idx, res.Manifests)
	msgs := issueMessages(result)

	for _, unwanted := range []string{
		`unknown widget "queue-count"`,
		`unknown form "order-form"`,
		`Page "order-page" block`,
	} {
		if strings.Contains(msgs, unwanted) {
			t.Errorf("false positive matching %s:\n%s", unwanted, msgs)
		}
	}
}

// Every shape that SHOULD be clean must stay clean; `formspec check` is run on
// the shipped examples and a regression here breaks other work.
func TestCheckReferences_CleanSpecIsSilent(t *testing.T) {
	dir := t.TempDir()
	write := func(rel, content string) {
		path := filepath.Join(dir, rel)
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatalf("mkdir: %v", err)
		}
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			t.Fatalf("write: %v", err)
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
  expose:
    - type: rest
      actions: [list, find]
`)
	// An auth form has no entity by design (auth_action replaces it) — it must
	// not be reported as a dangling ref.
	write("modules/alpha/forms/login.yaml", `apiVersion: formspec.dev/v1
kind: Form
metadata:
  name: login
  module: alpha
spec:
  auth_action: login
  sections:
    - title: Sign in
      fields:
        - field: username
`)
	write("modules/alpha/tables/orders.yaml", `apiVersion: formspec.dev/v1
kind: Table
metadata:
  name: orders
  module: alpha
spec:
  entity: alpha.order
  columns:
    - field: code
`)

	loader := manifest.NewLoader(dir)
	res, err := loader.LoadAll()
	if err != nil {
		t.Fatalf("LoadAll: %v", err)
	}
	idx := buildEntityIndex(res.Manifests)
	result := &checkResult{}
	checkReferences(result, idx, res.Manifests)

	if len(result.Issues) != 0 {
		t.Fatalf("clean spec produced findings:\n%s", issueMessages(result))
	}
}
