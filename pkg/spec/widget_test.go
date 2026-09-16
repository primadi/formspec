package spec

import (
	"strings"
	"testing"
)

// The widget vocabulary is a closed set (S10 / kafe TODO 1.4). These tests pin
// the members and the shape of the validation errors: the failure this closes is
// a typo (`relaion-picker`) validating fine and silently rendering a text
// input.
//
// Parity with the renderer lives in
// renderers/react-shadcn/src/widgets/catalog.test.tsx, which reads the JSON
// Schema generated from these const blocks.

// TestFormWidgets_ClosedSet pins the members — updating this list is the
// deliberate act that makes a widget name public.
func TestFormWidgets_ClosedSet(t *testing.T) {
	want := []string{
		"child-grid",
		"combobox",
		"datepicker",
		"datetimeinput",
		"decimalinput",
		"fileinput",
		"grants-editor",
		"hidden",
		"input",
		"json",
		"moneyinput",
		"number",
		"password",
		"qrcode",
		"radio-group",
		"relation-picker",
		"richtext",
		"select",
		"slider",
		"switch",
		"tags",
		"textarea",
		"timeinput",
		"uuid",
	}
	got := SortedFormWidgets()
	if len(got) != len(want) {
		t.Fatalf("form widget count: want %d, got %d (%v)", len(want), len(got), got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("form widget set differs at %d: want %q, got %q", i, want[i], got[i])
		}
	}
}

// TestTableCellWidgets_ClosedSet — table cells are their own (much smaller)
// surface; a form widget there is silently ignored by the cell renderer.
// `image` (gap #4) renders a file cell as an inline preview, `qrcode` (gap #3)
// makes a token scannable in a list/printout; the cell renderer has a branch
// for both, enforced by renderers/react-shadcn/src/widgets/catalog.test.tsx.
func TestTableCellWidgets_ClosedSet(t *testing.T) {
	got := AllTableCellWidgets()
	if len(got) != 4 || got[0] != WidgetBadge || got[1] != WidgetBoolean ||
		got[2] != WidgetImage || got[3] != WidgetQrCodeCell {
		t.Fatalf("table cell widget set: want [badge boolean image qrcode], got %v", got)
	}
}

func TestIsFormWidget(t *testing.T) {
	for _, ok := range []string{"input", "relation-picker", "grants-editor", "child-grid"} {
		if !IsFormWidget(ok) {
			t.Errorf("IsFormWidget(%q) = false, want true", ok)
		}
	}
	for _, bad := range []string{
		"",               // empty means "derive from the field type"
		"badge",          // table-only surface
		"relaion-picker", // typo (gap #1's example)
		"money-input",    // not implemented yet (known gap)
		"textinput",      // the name docs used to promise
		"toggle",         // ditto
		"relation",       // field *type* name, a legacy router alias
		"Input",          // case-sensitive
	} {
		if IsFormWidget(bad) {
			t.Errorf("IsFormWidget(%q) = true, want false", bad)
		}
	}
}

func TestIsTableCellWidget(t *testing.T) {
	if !IsTableCellWidget("badge") || !IsTableCellWidget("boolean") {
		t.Fatal("badge/boolean must be members")
	}
	for _, bad := range []string{"relation-picker", "select", "relaion-badge", ""} {
		if IsTableCellWidget(bad) {
			t.Errorf("IsTableCellWidget(%q) = true, want false", bad)
		}
	}
}

func TestValidateFormWidget(t *testing.T) {
	// Omitted widget is legal — the renderer derives one from the field type.
	if err := ValidateFormWidget("", `field "total"`); err != nil {
		t.Fatalf("empty widget must be valid, got: %v", err)
	}
	if err := ValidateFormWidget(WidgetRelationPicker, `field "customer"`); err != nil {
		t.Fatalf("catalogued widget must be valid, got: %v", err)
	}

	err := ValidateFormWidget("relaion-picker", `field "customer"`)
	if err == nil {
		t.Fatal("a typo must be rejected")
	}
	msg := err.Error()
	if !strings.Contains(msg, "relaion-picker") || !strings.Contains(msg, "unknown widget") {
		t.Errorf("error must name the offending value: %s", msg)
	}
	// The message must be actionable without opening the docs: it lists the
	// closed set and names the nearest canonical widget.
	if !strings.Contains(msg, "relation-picker") || !strings.Contains(msg, "allowed:") {
		t.Errorf("error must list the closed set: %s", msg)
	}
}

func TestValidateFormWidget_AliasHint(t *testing.T) {
	err := ValidateFormWidget("relation", `field "customer"`)
	if err == nil {
		t.Fatal("a field type name is not a widget name")
	}
	if !strings.Contains(err.Error(), `"relation" is a field type; the widget is "relation-picker"`) {
		t.Errorf("want an alias hint, got: %v", err)
	}
}

func TestValidateTableCellWidget(t *testing.T) {
	if err := ValidateTableCellWidget("", "column \"status\""); err != nil {
		t.Fatalf("empty cell widget must be valid, got: %v", err)
	}
	if err := ValidateTableCellWidget(WidgetBadge, "column \"status\""); err != nil {
		t.Fatalf("badge must be valid, got: %v", err)
	}

	// A form widget on a table column is the quiet mistake: the cell renderer
	// ignores it and prints the raw value.
	err := ValidateTableCellWidget("select", `column "status"`)
	if err == nil {
		t.Fatal("a form widget must not be accepted on a table column")
	}
	if !strings.Contains(err.Error(), "form widget, not a table cell widget") {
		t.Errorf("want a cross-surface hint, got: %v", err)
	}
	if !strings.Contains(err.Error(), "allowed: badge, boolean") {
		t.Errorf("want the closed set in the message, got: %v", err)
	}
}

func TestValidateTableColumns(t *testing.T) {
	cols := []TableColumn{
		{Field: "code"},
		{Field: "status", Widget: WidgetBadge},
	}
	if err := ValidateTableColumns(cols, `table "order-table"`); err != nil {
		t.Fatalf("valid columns: %v", err)
	}

	bad := []TableColumn{{Field: "code"}, {Field: "total", Widget: "money-input"}}
	err := ValidateTableColumns(bad, `listing "catalog"`)
	if err == nil {
		t.Fatal("want an error for the unknown column widget")
	}
	// The message must locate the column, not just the manifest.
	if !strings.Contains(err.Error(), `listing "catalog"`) || !strings.Contains(err.Error(), `column "total"`) {
		t.Errorf("want the column located in the message, got: %v", err)
	}
}

func TestValidateFormSections(t *testing.T) {
	sections := []FormSection{{
		Title: "Main",
		Fields: []FormField{
			{Field: "code", Widget: WidgetInput},
			{Field: "status", Widget: WidgetSelect},
		},
	}}
	if err := ValidateFormSections(sections, "form"); err != nil {
		t.Fatalf("valid sections: %v", err)
	}

	// `moneyinput` used to be the example of an unknown widget; it is a real
	// widget now (gap #1), so the example has to be something that stays wrong.
	bad := []FormSection{{Fields: []FormField{{Field: "total", Widget: "monei-nput"}}}}
	err := ValidateFormSections(bad, "form")
	if err == nil {
		t.Fatal("want an error for the unknown field widget")
	}
	if !strings.Contains(err.Error(), `field "total"`) || !strings.Contains(err.Error(), "monei-nput") {
		t.Errorf("want the field located in the message, got: %v", err)
	}

	// A section without a title is still located (name the position instead).
	err = ValidateFormSections(bad, "wizard \"close-shift\"")
	if err == nil || !strings.Contains(err.Error(), "wizard") {
		t.Errorf("want the manifest named in the message, got: %v", err)
	}
}
