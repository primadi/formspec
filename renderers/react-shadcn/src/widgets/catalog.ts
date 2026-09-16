// ─── Manifest widget catalog (S10 / kafe TODO 1.4) ───
//
// The closed set of `widget:` names a manifest may write. This mirrors
// pkg/spec/widget.go, which is what generates the JSON Schema enum that
// `formspec validate` and the YAML editor consume.
//
// Why it exists: `widget:` used to be a free-form string, so a typo
// (`widget: relaion-picker`) validated fine and silently rendered a plain text
// input — indistinguishable from "this widget does not exist yet".
//
// Two sets, one per surface — deliberately NOT one merged list: a form widget
// on a *table column* is a mistake the cell renderer would silently ignore and
// print raw text.
//
//   form  (FormField.widget, Wizard step fields)  → FormFieldWidget switch
//   table (TableColumn.widget, Listing columns)   → renderCellValue
//
// `catalog.test.ts` fails if this file, the generated JSON Schema, or the
// renderers' own dispatch drift apart — that test is the parity gate.

/** Widget names a form field may declare (mirrors pkg/spec/widget.go). */
export const FORM_WIDGETS = [
  "input",
  "textarea",
  "richtext",
  "number",
  "decimalinput",
  "select",
  "switch",
  "radio-group",
  "combobox",
  "password",
  "slider",
  "tags",
  "uuid",
  "json",
  "fileinput",
  "relation-picker",
  "datepicker",
  "datetimeinput",
  "child-grid",
  "grants-editor",
  "hidden",
  "qrcode",
  "moneyinput",
  "timeinput",
] as const

export type FormWidgetName = (typeof FORM_WIDGETS)[number]

/** Widget names a table/listing column may declare. */
export const TABLE_CELL_WIDGETS = [
  "badge",
  "boolean",
  "image",
  "qrcode",
] as const

export type TableCellWidgetName = (typeof TABLE_CELL_WIDGETS)[number]

/**
 * Field *type* names the router still renders (legacy aliases: a spec tree
 * deployed before the vocabulary was closed may have written
 * `widget: relation`). They are NOT part of the published catalog — validation
 * rejects them and points at the canonical name — but the router keeps
 * handling them so such specs keep rendering instead of breaking.
 */
const LEGACY_TYPE_ALIASES: Record<string, FormWidgetName> = {
  string: "input",
  text: "textarea",
  integer: "number",
  decimal: "decimalinput",
  boolean: "switch",
  enum: "select",
  date: "datepicker",
  datetime: "datetimeinput",
  relation: "relation-picker",
  child: "child-grid",
  file: "fileinput",
}

/** True when `name` is a member of the form widget catalog. */
export function isFormWidget(name: string): name is FormWidgetName {
  return (FORM_WIDGETS as readonly string[]).includes(name)
}

/** True when `name` is a member of the table cell widget catalog. */
export function isTableCellWidget(name: string): name is TableCellWidgetName {
  return (TABLE_CELL_WIDGETS as readonly string[]).includes(name)
}

/** The canonical widget a legacy type-name alias maps to, if any. */
export function formWidgetAliasTarget(
  name: string,
): FormWidgetName | undefined {
  return LEGACY_TYPE_ALIASES[name]
}

/** The closed set, for error messages. */
export function formWidgetNames(): string {
  return FORM_WIDGETS.join(", ")
}

/** The closed set, for error messages. */
export function tableCellWidgetNames(): string {
  return TABLE_CELL_WIDGETS.join(", ")
}
