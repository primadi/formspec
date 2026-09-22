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
	// WidgetQrCode renders the field's value as a QR code (gap #3/S4). The value
	// IS the payload (a token or URL), so there is nothing to type: the widget
	// exists so a table card, receipt, or public page can turn a stored string
	// into something a phone can scan. Read-only.
	WidgetQrCode FormWidget = "qrcode"
	// WidgetMoneyInput edits a `money` field (05-field-types.md §2, gap #1): it
	// keeps the currency alongside the amount, uses a numpad on touch devices,
	// and shows a formatted preview. Before it, a money field rendered as a
	// plain text input and the cashier typed an amount as free text.
	WidgetMoneyInput FormWidget = "moneyinput"
	// WidgetTimeInput edits a `time` field (time-of-day, `HH:MM[:SS]`).
	WidgetTimeInput FormWidget = "timeinput"
)

// TableCellWidget is the manifest name of a table/list cell widget (closed set).
// @schema {title: "Table Cell Widget", description: "Widget rendering a table or listing cell. Closed set — every name is implemented by the cell renderer; omit to render the raw value (optionally via `format`)."}
type TableCellWidget string

const (
	WidgetBadge   TableCellWidget = "badge"
	WidgetBoolean TableCellWidget = "boolean"
	// WidgetImage renders a `file`/`attachment` cell as an inline image (#4) —
	// the download route is the `src`, so a thumbnail in a table costs nothing
	// extra. A non-image value falls back to the download link.
	WidgetImage TableCellWidget = "image"
	// WidgetQrCodeCell renders the cell's value as a scannable QR code — the
	// same `qrcode` name as the form widget, because it means the same thing:
	// "this string, scannable". Listed in the cell set too so a dining-table
	// list or a printed card does not need a bespoke kind.
	WidgetQrCodeCell TableCellWidget = "qrcode"
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
	WidgetQrCode,
	WidgetMoneyInput,
	WidgetTimeInput,
}
var tableCellWidgets = []TableCellWidget{
	WidgetBadge,
	WidgetBoolean,
	WidgetImage,
	// A QR cell is how a table/card prints a scannable token (gap #3).
	WidgetQrCodeCell,
}

// ReportAggregate is the closed set of aggregation functions a ReportColumn may
// declare (S16). It mirrors the Query Builder's aggregate functions
// (02-core-extended.md §16) so a report column cannot name an aggregate the
// engine does not implement.
// @schema {title: "Report Aggregate", description: "Aggregation applied to a report column. Closed set — mirrors the Query Builder's aggregate functions."}
type ReportAggregate string

const (
	AggSum   ReportAggregate = "sum"
	AggAvg   ReportAggregate = "avg"
	AggCount ReportAggregate = "count"
	AggMin   ReportAggregate = "min"
	AggMax   ReportAggregate = "max"
)

var reportAggregates = []ReportAggregate{AggSum, AggAvg, AggCount, AggMin, AggMax}

// ReportFormat is the closed set of value formatters a ReportColumn may declare
// (S16). It mirrors what the report renderer actually implements, so a format
// the renderer does not know cannot be written (it would silently print the raw
// value).
// @schema {title: "Report Format", description: "Value formatter for a report column. Closed set — every name is implemented by the report renderer."}
type ReportFormat string

const (
	FormatCurrency   ReportFormat = "currency"
	FormatDate       ReportFormat = "date"
	FormatDateTime   ReportFormat = "datetime"
	FormatPercentage ReportFormat = "percentage"
)

var reportFormats = []ReportFormat{FormatCurrency, FormatDate, FormatDateTime, FormatPercentage}

// IsReportAggregate reports whether v is a declared aggregate.
func IsReportAggregate(v string) bool {
	for _, a := range reportAggregates {
		if string(a) == v {
			return true
		}
	}
	return false
}

// IsReportFormat reports whether v is a declared format.
func IsReportFormat(v string) bool {
	for _, f := range reportFormats {
		if string(f) == v {
			return true
		}
	}
	return false
}

// ReportAggregateValues returns the aggregate names (for error messages).
func ReportAggregateValues() []string {
	out := make([]string, len(reportAggregates))
	for i, a := range reportAggregates {
		out[i] = string(a)
	}
	return out
}

// ReportFormatValues returns the format names (for error messages).
func ReportFormatValues() []string {
	out := make([]string, len(reportFormats))
	for i, f := range reportFormats {
		out[i] = string(f)
	}
	return out
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
	"money":    WidgetMoneyInput,
	"time":     WidgetTimeInput,
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

// ─── S10 lanjutan (item 8.5): closed sets for the remaining free-string
// properties. Each mirrors what the engine/renderer actually implements, so a
// typo cannot silently do nothing. ───

// ReportParamType is the closed set of report parameter input types (S10).
// @schema {title: "Report Param Type", description: "Input type for a report parameter. Closed set — every name is implemented by the report renderer."}
type ReportParamType string

const (
	ParamText     ReportParamType = "text"
	ParamDate     ReportParamType = "date"
	ParamDateTime ReportParamType = "datetime"
	ParamSelect   ReportParamType = "select"
	ParamRelation ReportParamType = "relation"
)

var reportParamTypes = []ReportParamType{ParamText, ParamDate, ParamDateTime, ParamSelect, ParamRelation}

// EventChannel is the closed set of event delivery channels (S10).
// @schema {title: "Event Channel", description: "Delivery channel for an event. Closed set — every name is implemented by the delivery pipeline."}
type EventChannel string

const (
	ChannelAuditLog      EventChannel = "audit_log"
	ChannelWebsocket     EventChannel = "websocket"
	ChannelQueue         EventChannel = "queue"
	ChannelReliableEvent EventChannel = "reliable_event"
)

var eventChannels = []EventChannel{ChannelAuditLog, ChannelWebsocket, ChannelQueue, ChannelReliableEvent}

// PrintFormat is the closed set of print output formats (S10).
// @schema {title: "Print Format", description: "Output format for a Print document. Closed set — pdf/thermal are served server-side, html client-side; dotmatrix is not implemented and is rejected."}
type PrintFormat string

const (
	PrintPDF       PrintFormat = "pdf"
	PrintThermal   PrintFormat = "thermal"
	PrintDotMatrix PrintFormat = "dotmatrix"
	PrintHTML      PrintFormat = "html"
)

var printFormats = []PrintFormat{PrintPDF, PrintThermal, PrintDotMatrix, PrintHTML}

// WorkflowStepMode is the closed set of approval quorum modes (S10).
// @schema {title: "Workflow Step Mode", description: "How a step's quorum is collected. Closed set."}
type WorkflowStepMode string

const (
	StepModeAll        WorkflowStepMode = "all"
	StepModeAny        WorkflowStepMode = "any"
	StepModeSequential WorkflowStepMode = "sequential"
)

var workflowStepModes = []WorkflowStepMode{StepModeAll, StepModeAny, StepModeSequential}

// IsReportParamType reports whether v is a declared report parameter type.
func IsReportParamType(v string) bool {
	for _, t := range reportParamTypes {
		if string(t) == v {
			return true
		}
	}
	return false
}

// IsEventChannel reports whether v is a declared event channel.
func IsEventChannel(v string) bool {
	for _, c := range eventChannels {
		if string(c) == v {
			return true
		}
	}
	return false
}

// IsPrintFormat reports whether v is a declared print format.
func IsPrintFormat(v string) bool {
	for _, f := range printFormats {
		if string(f) == v {
			return true
		}
	}
	return false
}

// IsWorkflowStepMode reports whether v is a declared workflow step mode.
func IsWorkflowStepMode(v string) bool {
	for _, m := range workflowStepModes {
		if string(m) == v {
			return true
		}
	}
	return false
}

// ReportParamTypeValues returns the parameter type names (for error messages).
func ReportParamTypeValues() []string { return stringifyAll(reportParamTypes) }

// EventChannelValues returns the channel names (for error messages).
func EventChannelValues() []string { return stringifyAll(eventChannels) }

// PrintFormatValues returns the print format names (for error messages).
func PrintFormatValues() []string { return stringifyAll(printFormats) }

// WorkflowStepModeValues returns the step mode names (for error messages).
func WorkflowStepModeValues() []string { return stringifyAll(workflowStepModes) }

// stringifyAll renders a slice of string-kinded values as []string.
func stringifyAll[T ~string](vals []T) []string {
	out := make([]string, len(vals))
	for i, v := range vals {
		out[i] = string(v)
	}
	return out
}
