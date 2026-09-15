// ─── Widget vocabulary — closed sets (S10 / kafe TODO 1.4) ───
//
// `widget:` was a free-form string, so a typo was indistinguishable from a
// widget that does not exist yet: `widget: relaion-picker` validated fine and
// silently rendered a plain text input. These closed sets are the contract.
//
// Two sets, one per surface — deliberately NOT one shared list:
//
//   - FormWidget      — FormField.widget (Form, Wizard step, …)
//   - TableCellWidget — TableColumn.widget (Table, Listing)
//
// A single merged list would let `widget: relation-picker` on a *table column*
// through validation, where the cell renderer ignores it and prints raw text —
// exactly the silent-typo class S10 closes.
//
// Canonical names are the ones the renderer actually switches on. Field *type*
// names (`relation`, `date`, `child`, `boolean`, …) are legacy aliases the
// router still accepts so already-deployed spec trees keep rendering, but they
// are NOT part of the published vocabulary; aliasHint turns them into an
// actionable error.
//
// The JSON Schema enum is generated from these const blocks
// (internal/genjsonschema `enrichEnumValues`), which is what gives
// `formspec validate` its typo rejection and the editor its autocomplete.
// Parity with the renderer is enforced by
// renderers/react-shadcn/src/widgets/catalog.test.ts.

package spec

import (
	"fmt"
	"sort"
	"strings"
)

// FormWidget is the manifest name of a form-field widget (closed set).
// @schema {title: "Form Widget", description: "Widget rendering a form field. Closed set — every name is implemented by the renderer; omit to use the default derived from the field type."}
type FormWidget string

// Form widget names — keep this block in the same order as the renderer switch
// (renderers/react-shadcn/src/kinds/form/FormRenderer.tsx → FormFieldWidget) so
// diffs between the two stay readable.
const (
	WidgetInput          FormWidget = "input" // single-line text (default)
	WidgetTextarea       FormWidget = "textarea"
	WidgetRichText       FormWidget = "richtext"
	WidgetNumber         FormWidget = "number"
	WidgetDecimalInput   FormWidget = "decimalinput"
	WidgetSelect         FormWidget = "select"
	WidgetSwitch         FormWidget = "switch"
	WidgetRadioGroup     FormWidget = "radio-group"
	WidgetCombobox       FormWidget = "combobox"
	WidgetPassword       FormWidget = "password"
	WidgetSlider         FormWidget = "slider"
	WidgetTags           FormWidget = "tags"
	WidgetUUID           FormWidget = "uuid"
	WidgetJSON           FormWidget = "json"
	WidgetFileInput      FormWidget = "fileinput"
	WidgetRelationPicker FormWidget = "relation-picker"
	WidgetDatePicker     FormWidget = "datepicker"
	WidgetDateTimeInput  FormWidget = "datetimeinput"
	WidgetChildGrid      FormWidget = "child-grid"
	WidgetGrantsEditor   FormWidget = "grants-editor"
	// WidgetHidden renders nothing but keeps the value in form state and in the
	// submitted payload — for fields seeded from the render context
	// (`default_from`) that the user must not see.
	WidgetHidden FormWidget = "hidden"
)

// TableCellWidget is the manifest name of a table/list cell widget (closed set).
// @schema {title: "Table Cell Widget", description: "Widget rendering a table or listing cell. Closed set — every name is implemented by the cell renderer; omit to render the raw value (optionally via `format`)."}
type TableCellWidget string

const (
	WidgetBadge   TableCellWidget = "badge"
	WidgetBoolean TableCellWidget = "boolean"
)

// formWidgets is the authoritative ordered list (see the const block above).
var formWidgets = []FormWidget{
	WidgetInput,
	WidgetTextarea,
	WidgetRichText,
	WidgetNumber,
	WidgetDecimalInput,
	WidgetSelect,
	WidgetSwitch,
	WidgetRadioGroup,
	WidgetCombobox,
	WidgetPassword,
	WidgetSlider,
	WidgetTags,
	WidgetUUID,
	WidgetJSON,
	WidgetFileInput,
	WidgetRelationPicker,
	WidgetDatePicker,
	WidgetDateTimeInput,
	WidgetChildGrid,
	WidgetGrantsEditor,
	WidgetHidden,
}

var tableCellWidgets = []TableCellWidget{
	WidgetBadge,
	WidgetBoolean,
}

// widgetAliasHints maps a field *type* name to the canonical widget an author
// most likely meant, so the validation error is actionable rather than just
// "unknown value".
var widgetAliasHints = map[string]FormWidget{
	"string":   WidgetInput,
	"text":     WidgetTextarea,
	"integer":  WidgetNumber,
	"decimal":  WidgetDecimalInput,
	"boolean":  WidgetSwitch,
	"enum":     WidgetSelect,
	"date":     WidgetDatePicker,
	"datetime": WidgetDateTimeInput,
	"relation": WidgetRelationPicker,
	"child":    WidgetChildGrid,
	"file":     WidgetFileInput,
	// No money/time widget exists yet (known gap: MoneyInput/TimeInput) — the
	// closest implemented widget is named so the message is still actionable.
	"money": WidgetDecimalInput,
	"time":  WidgetDateTimeInput,
}

// AllFormWidgets returns the closed set of form widget names (copy).
func AllFormWidgets() []FormWidget {
	out := make([]FormWidget, len(formWidgets))
	copy(out, formWidgets)
	return out
}

// AllTableCellWidgets returns the closed set of table cell widget names (copy).
func AllTableCellWidgets() []TableCellWidget {
	out := make([]TableCellWidget, len(tableCellWidgets))
	copy(out, tableCellWidgets)
	return out
}

// IsFormWidget reports whether name is a member of the form widget set.
func IsFormWidget(name string) bool {
	for _, w := range formWidgets {
		if string(w) == name {
			return true
		}
	}
	return false
}

// IsTableCellWidget reports whether name is a member of the table cell set.
func IsTableCellWidget(name string) bool {
	for _, w := range tableCellWidgets {
		if string(w) == name {
			return true
		}
	}
	return false
}

// FormWidgetNames renders the closed set for an error message.
func FormWidgetNames() string {
	names := make([]string, len(formWidgets))
	for i, w := range formWidgets {
		names[i] = string(w)
	}
	return strings.Join(names, ", ")
}

// TableCellWidgetNames renders the closed set for an error message.
func TableCellWidgetNames() string {
	names := make([]string, len(tableCellWidgets))
	for i, w := range tableCellWidgets {
		names[i] = string(w)
	}
	return strings.Join(names, ", ")
}

// ValidateFormWidget checks an explicit `widget:` value on a form field.
// `where` locates the field in the manifest (e.g. `section "Main" field
// "total"`). An empty widget is valid — the renderer derives one from the
// field type.
func ValidateFormWidget(w FormWidget, where string) error {
	if w == "" || IsFormWidget(string(w)) {
		return nil
	}
	return fmt.Errorf("%s: unknown widget %q (allowed: %s)%s",
		where, w, FormWidgetNames(), aliasHint(string(w)))
}

// ValidateTableCellWidget checks an explicit `widget:` value on a table column.
func ValidateTableCellWidget(w TableCellWidget, where string) error {
	if w == "" || IsTableCellWidget(string(w)) {
		return nil
	}
	// A form widget on a table column is a common, quiet mistake: the cell
	// renderer ignores it and prints the raw value.
	hint := aliasHint(string(w))
	if hint == "" && IsFormWidget(string(w)) {
		hint = fmt.Sprintf(" — %q is a form widget, not a table cell widget", w)
	}
	return fmt.Errorf("%s: unknown table cell widget %q (allowed: %s)%s",
		where, w, TableCellWidgetNames(), hint)
}

// ValidateTableColumns checks the `widget:` of every column on a Table or
// Listing manifest. `where` names the kind (e.g. `table "order-table"`).
func ValidateTableColumns(columns []TableColumn, where string) error {
	for _, c := range columns {
		if err := ValidateTableCellWidget(c.Widget, fmt.Sprintf("%s, column %q", where, c.Field)); err != nil {
			return err
		}
	}
	return nil
}

// ValidateFormSections checks the `widget:` of every field in every section of
// a Form (or Wizard step). `where` names the manifest.
func ValidateFormSections(sections []FormSection, where string) error {
	for _, s := range sections {
		label := s.Title
		if label == "" {
			label = "(untitled section)"
		}
		for _, f := range s.Fields {
			if err := ValidateFormWidget(f.Widget, fmt.Sprintf("%s, section %q, field %q", where, label, f.Field)); err != nil {
				return err
			}
		}
	}
	return nil
}

// aliasHint suggests the canonical widget when the value looks like a field
// type name.
func aliasHint(value string) string {
	if suggestion, ok := widgetAliasHints[value]; ok {
		return fmt.Sprintf(" — %q is a field type; the widget is %q", value, suggestion)
	}
	return ""
}

// SortedFormWidgets returns the closed set sorted (stable output for tests and
// generated docs).
func SortedFormWidgets() []string {
	out := make([]string, 0, len(formWidgets))
	for _, w := range formWidgets {
		out = append(out, string(w))
	}
	sort.Strings(out)
	return out
}
