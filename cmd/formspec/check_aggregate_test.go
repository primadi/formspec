package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/primadi/formspec/internal/manifest"
)

// writeCheckAggregateSpec writes a spec tree with a report whose aggregates are
// declared over the wrong kinds of fields (S7 / gap #28).
func writeCheckAggregateSpec(t *testing.T, dir string) {
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
    - name: note
      type: text
    - name: qty
      type: integer
    - name: total
      type: money
`)

	write("modules/alpha/reports/sales.yaml", `apiVersion: formspec.dev/v1
kind: Report
metadata:
  name: sales
  module: alpha
spec:
  title: Sales
  entity: alpha.order
  columns:
    - { field: code, label: Code }
    - { field: total, label: Total, format: currency, aggregate: sum }
    - { field: note, label: Note, aggregate: sum }
    - { field: nope, label: Nope, aggregate: avg }
    - { field: qty, label: Qty, aggregate: mediansum }
  totals:
    - { label: Total, field: total, fn: sum }
    - { label: Codes, field: code, fn: min }
  export: [csv]
`)

	write("modules/alpha/widgets/omzet.yaml", `apiVersion: formspec.dev/v1
kind: Widget
metadata:
  name: omzet
  module: alpha
spec:
  type: metric
  entity: alpha.order
  config: { field: total, aggregate: sum }
`)

	write("modules/alpha/widgets/bad-metric.yaml", `apiVersion: formspec.dev/v1
kind: Widget
metadata:
  name: bad-metric
  module: alpha
spec:
  type: metric
  entity: alpha.order
  config: { field: note, aggregate: sum }
`)
}

// TestCheckAggregates_RejectsNonNumericFields pins the static gate: a sum over
// a text column (or an unknown field, or an unknown aggregate verb) is a spec
// error, not something to discover as a confident 0 in a report.
func TestCheckAggregates_RejectsNonNumericFields(t *testing.T) {
	dir := t.TempDir()
	writeCheckAggregateSpec(t, dir)

	loader := manifest.NewLoader(dir)
	res, err := loader.LoadAll()
	if err != nil {
		t.Fatalf("LoadAll: %v", err)
	}

	idx := buildEntityIndex(res.Manifests)
	result := &checkResult{}
	checkAggregates(result, idx, res.Manifests)

	// Sanity: a sum over a money field and over an integer is fine — that is
	// the shape the kafe reports use. Only the *type* complaints are checked
	// here (the `mediansum` column legitimately errors on its unknown verb).
	for _, issue := range result.Issues {
		if !strings.Contains(issue.Message, "which is not numeric") {
			continue
		}
		if strings.Contains(issue.Message, `"total"`) || strings.Contains(issue.Message, `"qty"`) {
			t.Fatalf("numeric/money fields must be aggregatable, got: %s", issue.Message)
		}
	}

	want := []string{
		`sum(note) is not defined`,                 // text column cannot be summed
		`aggregate "avg" references unknown field`, // unknown field
		`aggregate "mediansum" is not defined`,     // unknown aggregate verb
		`min(code) is not defined`,                 // string column cannot be min'd
		`Widget "bad-metric" config: sum(note)`,    // widget metric over a text column
	}
	for _, w := range want {
		var found bool
		for _, issue := range result.Issues {
			if strings.Contains(issue.Message, w) {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("expected an issue mentioning %q, got: %+v", w, result.Issues)
		}
	}
}
