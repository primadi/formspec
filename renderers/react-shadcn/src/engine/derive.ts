// ─── Derivation Engine ───
//
// Converts EntitySchema into default UI manifests (TableSpec, FormSpec, etc.)
// that are indistinguishable from authored YAML manifests.
//
// Override resolution: authored manifest > derived default.
// Callers should check for authored manifests first, then fall back to derive().
//
// Design doc §5.3

import {
  type EntitySchema,
  type Field,
  type TableSpec,
  type TableColumn,
  type TableAction,
  type FormSpec,
  type FormSection,
  type FormField,
  type MenuItem,
} from "@/types/manifest"
import { titleCase } from "@/lib/utils"
import { storageAllowsImage } from "@/lib/media"
import { isUserEnterableKey } from "@/lib/field-presence"

// ── Main derive functions ──

/**
 * Number of columns shown by default in a derived table. Columns beyond this
 * are NOT dropped — they stay in `spec.columns` and are revealed by the
 * Table renderer's row-expand (5.4.4 / 5.14.1).
 */
export const DERIVED_TABLE_VISIBLE_COLUMNS = 8

/**
 * Build the derived column list in priority order (5.4.4 / 5.14.1):
 *   1. natural key field
 *   2. label_field
 *   3. state_machine status field
 *   4. transaction_date (or any date/datetime field)
 *   5. remaining non-child, non-computed fields in declaration order
 *
 * Every eligible field is included — nothing is silently dropped. The
 * renderer decides how many to show by default and exposes the rest via
 * row expand/detail.
 */
export function deriveTableColumns(entity: EntitySchema): TableColumn[] {
  const eligible = entity.fields.filter(
    (f) => f.type !== "child" && !f.computed,
  )

  const priority = (f: Field): number => {
    if (f.natural_key) return 0
    if (f.name === entity.label_field) return 1
    if (entity.state_machine && f.name === entity.state_machine.field) return 2
    if (f.name === "transaction_date") return 3
    return 4
  }

  // Stable sort by priority (declaration order preserved within a tier).
  const ordered = [...eligible].sort((a, b) => priority(a) - priority(b))

  return ordered.map((field) => {
    // For belongs_to relation fields, use dot-path notation to resolve the
    // related entity's display name instead of showing the raw foreign key.
    // Example: polyclinic_id → polyclinic.name
    let colField = field.name
    if (field.type === "relation" && field.relation?.type === "belongs_to") {
      const alias = field.name.endsWith("_id")
        ? field.name.slice(0, -3)
        : field.relation.resource
      colField = `${alias}.name`
    }
    return {
      field: colField,
      label: fieldLabel(field),
      sortable: isSortable(field),
      widget: tableWidget(field),
      format: tableFormat(field),
    }
  })
}

/**
 * Derive a default TableSpec from an entity schema.
 */
export function deriveTable(entity: EntitySchema): TableSpec {
  const columns = deriveTableColumns(entity)
  const rowActions: TableAction[] = []

  // Row actions: view, edit, delete + custom actions
  // Summary entities are read-only projections — view only.
  // Reference entities are locked-structure config records (Configuration
  // pattern) — view + edit, no delete (matches engine/lifecycle.ts's
  // hasDelete/hasCreate rule for `characteristic: reference`; the backend
  // also never generates a delete route for them, internal/api/generator.go).
  if (entity.characteristic === "summary") {
    rowActions.push({ action: "view", label: "View", icon: "Eye" })
  } else if (entity.characteristic === "reference") {
    rowActions.push(
      { action: "view", label: "View", icon: "Eye" },
      { action: "edit", label: "Edit", icon: "Pencil" },
    )
  } else {
    rowActions.push(
      { action: "view", label: "View", icon: "Eye" },
      { action: "edit", label: "Edit", icon: "Pencil" },
      {
        action: "delete",
        label: "Delete",
        icon: "Trash2",
        confirm_msg: "Are you sure you want to delete this item?",
      },
    )
  }

  // Add custom actions that have UI hints
  for (const action of entity.actions ?? []) {
    if (isBuiltinAction(action.name)) continue
    if (action.ui) {
      rowActions.push({
        action: action.name,
        label: action.ui.button_label ?? action.name,
        icon: action.ui.icon,
        confirm_msg: action.ui.confirm,
      })
    }
  }

  const hasStringField = entity.fields.some(
    (f) => f.type === "string" && !f.enum_values?.length,
  )

  return {
    entity: `${entity.module}.${entity.name}`,
    columns,
    default_sort: hasField(entity, "created_at") ? "-created_at" : undefined,
    page_size: 25,
    search: hasStringField,
    row_actions: rowActions,
  }
}

/**
 * Derive a default FormSpec from an entity schema for a given mode.
 */
export function deriveForm(
  entity: EntitySchema,
  mode: "create" | "edit" | "view" = "create",
): FormSpec {
  const sections: FormSection[] = []
  const editableFields = entity.fields.filter(
    (f) =>
      (mode !== "create" || !f.computed) &&
      // A key the engine alone authors (`natural_key_entry: auto_generated`) is
      // not an input — the store drops a supplied value, so rendering one would
      // offer the user a field whose content is discarded. `user_entry` and
      // `auto_generated_if_empty` stay, because a person does supply those.
      isUserEnterableKey(f),
  )

  const section: FormSection = {
    title:
      mode === "create"
        ? "New Entry"
        : mode === "edit"
          ? "Edit Entry"
          : "Details",
    fields: editableFields.map((f) => formField(f, mode)),
  }

  // Add description if provided
  if (entity.description) {
    section.description = entity.description
  }

  sections.push(section)

  // Determine render mode based on field count heuristic (§1.6)
  const renderMode = deriveFormRenderMode(editableFields)

  return {
    entity: `${entity.module}.${entity.name}`,
    mode,
    sections,
    render: { mode: renderMode },
  }
}

/**
 * Determine form render mode based on field count and characteristics.
 * - modal: ≤5 fields
 * - drawer: 6-12 fields
 * - separate_page: >12 fields or has child tables
 */
function deriveFormRenderMode(
  fields: Field[],
): "modal" | "drawer" | "separate_page" {
  const hasChildTable = fields.some(
    (f) => f.type === "child" && f.child?.storage === "table",
  )
  const fieldCount = fields.length

  if (fieldCount > 12 || hasChildTable) return "separate_page"
  if (fieldCount > 5) return "drawer"
  return "modal"
}

/**
 * Derive default menu entries for an entity.
 */
export function deriveMenuItems(entities: EntitySchema[]): MenuItem[] {
  const byModule = new Map<string, EntitySchema[]>()
  for (const e of entities) {
    const list = byModule.get(e.module) ?? []
    list.push(e)
    byModule.set(e.module, list)
  }

  const menus: MenuItem[] = []
  for (const [module, ents] of byModule) {
    const children: MenuItem[] = ents
      .filter((e) => e.characteristic !== "summary")
      .map((e) => ({
        label: entityDisplayName(e),
        icon: entityIcon(e),
        route: `/${module}/${e.plural}`,
      }))

    if (children.length === 0) continue

    menus.push({
      label: moduleDisplayName(module),
      icon: moduleIcon(module),
      children,
    })
  }

  return menus
}

/**
 * Derive Kanban columns when the manifest declares none (5.5.5 zero-config).
 * Order follows the state machine's declared states (transition order), or
 * falls back to the `status_field` enum's declaration order. When neither
 * exists, returns an empty list (renderer shows a hint).
 */
export function deriveKanbanColumns(
  entity: EntitySchema | undefined,
  statusField: string,
): import("@/types/manifest").KanbanColumn[] {
  if (!entity) return []

  // 1. State machine states — declared order is authoritative.
  if (entity.state_machine && entity.state_machine.field === statusField) {
    return entity.state_machine.states.map((s) => ({
      status: s.name,
      label: s.label || titleCase(s.name),
    }))
  }

  // 2. Enum values on the status field — declaration order.
  const statusFieldDef = entity.fields.find((f) => f.name === statusField)
  if (statusFieldDef?.enum_values?.length) {
    return statusFieldDef.enum_values.map((v) => ({
      status: v,
      label: titleCase(v),
    }))
  }

  // 3. A declared choice set on the status field — declaration order, and the
  // only branch that carries real captions (`options[].label`). Without it a
  // Kanban board on an `options` field would show no columns at all.
  if (statusFieldDef?.options?.length) {
    return statusFieldDef.options.map((o) => ({
      status: String(o.value),
      label: o.label?.trim() || humanizeFieldName(String(o.value)),
    }))
  }

  return []
}

/**
 * Derive a default detail page field list (readonly).
 * Returns fields grouped for display: main fields, then child tables.
 */
export function deriveDetailFields(entity: EntitySchema): {
  mainFields: Field[]
  childFields: Field[]
} {
  return {
    mainFields: entity.fields.filter((f) => f.type !== "child"),
    childFields: entity.fields.filter((f) => f.type === "child"),
  }
}

/**
 * Resolve authored vs derived: check if a manifest exists for this entity.
 * If authored manifest name matches entity name, use authored; else derive.
 */
export function resolveTable(
  entity: EntitySchema,
  authoredTables: ReadonlyMap<
    string,
    import("@/types/manifest").Entry<TableSpec>
  >,
): TableSpec {
  const authored = authoredTables.get(entity.name)
  if (!authored) return deriveTable(entity)
  // Authored wins, but a column with no `label` still inherits the entity
  // field's `title` — otherwise declaring a Table to reorder/adjust two
  // columns drops every other caption to a raw field name.
  return {
    ...authored.spec,
    columns: withEntityColumnLabels(authored.spec.columns ?? [], entity),
  }
}

/**
 * Resolve authored vs derived form with mode-aware lookup:
 *
 * Create mode:
 *   1. `{entity.name}-create`  (e.g. `visit-create`)
 *   2. `{entity.name}-form`    (e.g. `visit-form`, generic for any mode)
 *   3. auto-generate from entity schema
 *
 * Edit mode:
 *   1. `{entity.name}-edit`    (e.g. `visit-edit`)
 *   2. `{entity.name}-form`
 *   3. auto-generate
 *
 * View mode:
 *   1. `{entity.name}-view`    (e.g. `visit-view`)
 *   2. `{entity.name}-form`
 *   3. auto-generate
 */
export function resolveForm(
  entity: EntitySchema,
  mode: "create" | "edit" | "view",
  authoredForms: ReadonlyMap<
    string,
    import("@/types/manifest").Entry<FormSpec>
  >,
  // Explicit override — a Page/Tab block's `form.ref` names a specific
  // authored Form by name, bypassing the naming-convention guess below.
  // Needed whenever more than one Form targets the same entity (e.g. a
  // Configuration Page split across tabs, each with its own curated Form).
  explicitRef?: string,
): FormSpec {
  if (explicitRef) {
    const named = authoredForms.get(explicitRef)
    if (named) return withEntityFieldDefaults(named.spec, entity)
  }

  const modeSuffix =
    mode === "create" ? "create" : mode === "edit" ? "edit" : "view"

  // 1. Mode-specific: entity-create, entity-edit, entity-view
  const modeForm = authoredForms.get(`${entity.name}-${modeSuffix}`)
  if (modeForm) return withEntityFieldDefaults(modeForm.spec, entity)

  // 2. Generic entity form: entity-form
  const genericForm = authoredForms.get(`${entity.name}-form`)
  if (genericForm) return withEntityFieldDefaults(genericForm.spec, entity)

  // 3. Fallback: auto-generate
  return deriveForm(entity, mode)
}

// ── Field caption helpers ──
//
// One vocabulary for "what do we call this field on screen", shared by every
// renderer that draws a field caption. Before this, four spellings coexisted:
// `fieldLabel()` (derived Form/Table), `field.label ?? field.name` (authored
// Form/Table/Wizard/SearchSelect), the Wizard's `label ?? description ?? name`,
// and `field.name.replace(/_/g, " ")` (DetailPage — which ignored the entity's
// `title` entirely).
//
// Only the derived spelling consulted the entity's `title`. So declaring
// `kind: Form` — usually for field order, sections or `visible_when`, not for
// labels — silently downgraded every caption from "Minimum Belanja" to
// "min_purchase", and DetailPage showed "min purchase" even with no authored
// manifest at all. 111 of 161 authored form fields in `examples/` declare no
// `label`, so the fallback is the common path, not an edge case.
//
// Precedence, identical everywhere: manifest `label` → entity `title` →
// humanised field name.

/** Humanise a snake_case identifier: "min_purchase" → "Min Purchase". */
export function humanizeFieldName(name: string): string {
  return name
    .replace(/_/g, " ")
    .replace(/\bid\b/i, "ID")
    .replace(/\b\w/g, (c) => c.toUpperCase())
}

function fieldLabel(field: Field): string {
  if (field.title) return field.title
  return humanizeFieldName(field.name)
}

/**
 * Effective caption for a field referenced by name: the manifest's own `label`
 * when declared, otherwise the entity field's `title`, otherwise a humanised
 * field name.
 *
 * `name` may be a dot-path column (`polyclinic.name`, `patient.name`); the
 * entity field is then the root segment, matching either `polyclinic` or the
 * relation's own `polyclinic_id`.
 */
export function entityFieldLabel(
  entity: EntitySchema | undefined,
  name: string,
  label?: string,
): string {
  if (label) return label
  const root = name.split(".")[0]
  const field = entity?.fields.find(
    (f) => f.name === root || f.name === `${root}_id`,
  )
  if (field?.title) return field.title
  return humanizeFieldName(field?.name ?? root)
}

/**
 * Effective help text for a field referenced by name: the manifest's own
 * `help` when declared (the caller passes it), otherwise the entity field's
 * `description`.
 *
 * Returns `undefined` when neither exists so callers can distinguish "no help
 * declared" from "help deliberately emptied" — the same distinction
 * `entityFieldLabel` does not need, because a caption always has to render
 * something.
 *
 * Resolution is shared with the derived path (`formField()`), which is what
 * keeps the two vocabularies from drifting apart again: before this, four
 * spellings coexisted for captions and only one of them read the entity.
 */
export function entityFieldHelp(
  entity: EntitySchema | undefined,
  name: string,
): string | undefined {
  const root = name.split(".")[0]
  const field = entity?.fields.find(
    (f) => f.name === root || f.name === `${root}_id`,
  )
  return field?.description || undefined
}

/**
 * Fill in the missing per-field `label` and `help` of an authored Form from
 * the target entity's field `title`/`description`. Explicit declarations
 * always win.
 *
 * The `help` half exists because the two vocabularies were disconnected:
 * `Field.description` (Entity) and `FormField.help` (Form) mean the same
 * thing to a user, but only `deriveForm()` read the former (via
 * `formField()`). So the moment an Entity gained a `kind: Form` — usually for
 * ordering, sections or `visible_when`, *not* for help — every description
 * vanished, and authors had to copy it by hand. They did: `promo-form.yaml`
 * and `promo/entity.yaml` carried the same sentence verbatim. Declaring help
 * via the Entity keeps one declaration for both surfaces.
 *
 * `sections[0].description` likewise falls back to the Entity's own
 * `metadata.description`, matching what `deriveForm()` has always done for
 * the derived section — that is the string `OverlayHost` shows as the
 * drawer/dialog subtitle, which otherwise degrades to the generic English
 * "Fill in the details for this <entity>." even when the Entity is described.
 *
 * Returns a new spec: the bundle entry is shared (zustand) and a Page can
 * embed the same Form twice, so mutating it in place would leak across
 * renders.
 */
export function withEntityFieldDefaults(
  spec: FormSpec,
  entity: EntitySchema,
): FormSpec {
  if (!spec.sections?.length) return spec
  return {
    ...spec,
    sections: spec.sections.map((section, idx) => ({
      ...section,
      description:
        section.description || (idx === 0 ? entity.description : undefined),
      fields: section.fields.map((f) => {
        const label = f.label ?? entityFieldLabel(entity, f.name)
        const help = f.help ?? entityFieldHelp(entity, f.name)
        // Only allocate a new object when something is actually filled in —
        // an authored field that declares both stays referentially equal.
        if (label === f.label && help === f.help) return f
        const next: FormField = { ...f, label }
        if (help !== undefined) next.help = help
        return next
      }),
    })),
  }
}

/**
 * Fill in the missing `label`s of authored Table/Listing columns from the
 * target entity's field `title`s. Explicit `label` declarations always win.
 */
export function withEntityColumnLabels(
  columns: TableColumn[],
  entity: EntitySchema | undefined,
): TableColumn[] {
  return columns.map((c) =>
    c.label ? c : { ...c, label: entityFieldLabel(entity, c.field) },
  )
}

function isSortable(field: Field): boolean {
  return [
    "string",
    "integer",
    "decimal",
    "percent",
    "date",
    "datetime",
    "enum",
    "boolean",
  ].includes(field.type)
}

function tableWidget(field: Field): string | undefined {
  if (field.type === "enum" || field.name === "doc_status") return "badge"
  if (field.type === "boolean") return "boolean"
  // A field with a declared choice set renders as a labelled badge: a single
  // value is exactly what `badge` is for (10.30), and the label is substituted
  // before the cell is drawn (`renderOptionCell` in lib/renderCell.tsx). A
  // *set* stays plain text — one pill carrying "Senin, Selasa, Jumat" reads
  // worse than the comma-separated labels already do.
  if (field.options?.length && !isMultiValue(field)) return "badge"
  // Image files render as an inline preview (#4). Non-image files keep the
  // plain download link, so they get no widget hint here.
  if (
    (field.type === "file" || field.type === "attachment") &&
    storageAllowsImage(field.storage)
  )
    return "image"
  return undefined
}

function tableFormat(field: Field): string | undefined {
  if (field.type === "datetime") return "relative"
  if (field.type === "date") return "date"
  // A percentage is numerically a decimal (S11); the difference is rendering.
  if (field.type === "percent") return "percent"
  // `money` carries its own unit (05-field-types.md §2), so the cell must be
  // formatted as an amount — a derived column previously fell through to
  // `JSON.stringify` and rendered `{"amount":"50000","currency":"IDR"}`.
  // This is the same mapping `cellHintsForField()` (lib/renderCell.tsx) already
  // applies to child-grid cells; the two vocabularies must stay in step.
  if (field.type === "money") return "currency"
  return undefined
}

function formField(field: Field, mode: "create" | "edit" | "view"): FormField {
  const ff: FormField = {
    name: field.name,
    label: fieldLabel(field),
    widget: formWidget(field),
  }

  // Same rule the authored path applies through `withEntityFieldDefaults()`
  // — the entity's `description` is the field's default help text. Keeping
  // the single rule in one place is what stops the two paths diverging.
  if (field.description) ff.help = field.description
  if (field.required) {
    // required is handled by zod schema, not FormField
  }

  // Immutable fields are readonly in edit mode
  if (mode === "edit" && field.immutable) {
    ff.read_only = true
  }

  // Computed fields are always readonly
  if (field.computed) {
    ff.read_only = true
    ff.compute = field.computed.formula
  }

  return ff
}

// isMultiValue mirrors `spec.FieldIsMultiple` (`pkg/spec/entity.go`): the
// Entity's declared cardinality, read by both the widget derivation and the
// read-only renderers (re-exported through `lib/field-options.ts`).
//
// It lives here — not in field-options.ts — because field-options already
// depends on this module for `humanizeFieldName`; the reverse import would make
// the two mutually dependent.
//
// Absent means single: `json` and `string` can hold either shape, so the Go
// validator *requires* `multiple` on them whenever `options` is declared — an
// undeclared cardinality reaching the renderer is therefore a scalar field, not
// a set.
export function isMultiValue(field: Field | undefined): boolean {
  if (!field) return false
  return field.multiple === true
}

// deriveFormWidget is the single source of "which widget renders this field".
//
// Both paths read it — the derived Form (`formField` below) and the authored
// Form (`FormRenderer`'s field router) — because when the two disagree, the
// same field renders differently depending on whether someone wrote a
// `kind: Form`. That divergence is exactly what forced authors to restate the
// Entity's decision as `widget: select-multi-tag` in the manifest.
//
// Cardinality (the Entity's `multiple`) picks between the set and single
// widgets; without a declared `options` nothing is inferred, so no existing
// spec changes behaviour.
export function deriveFormWidget(field: Field): string {
  return formWidget(field)
}

function formWidget(field: Field): string {
  switch (field.type) {
    case "string":
      if (
        field.rules?.some(
          (r) =>
            r.name === "max_length" &&
            typeof r.value === "number" &&
            (r.value as number) > 120,
        )
      ) {
        return "textarea"
      }
      return field.options?.length ? optionWidget(field) : "input"
    case "text":
      return "textarea"
    case "richtext":
      return "richtext"
    case "integer":
      return field.options?.length ? optionWidget(field) : "number"
    case "decimal":
      return "decimalinput"
    case "money":
      return "moneyinput"
    case "time":
      return "timeinput"
    case "percent":
      // S11: same numeric input as decimal — the `%` is a display concern
      // (`format: percent`), not a different editing surface.
      return "decimalinput"
    case "boolean":
      return "switch"
    case "enum":
      return "select"
    case "date":
      return field.options?.length ? optionWidget(field) : "datepicker"
    case "datetime":
      return "datetimeinput"
    case "uuid":
      return "uuid"
    case "json":
      // A json field that declares a choice set is a *set of declared values*,
      // not free-form JSON — deriving the picker here is what saves the author
      // from writing `widget:` just to avoid a raw JSON editor. A json field
      // without `options` keeps the JSON editor (no behaviour change for
      // existing specs).
      return field.options?.length ? optionWidget(field) : "json"
    case "file":
      return "fileinput"
    case "relation":
      return "relation-picker"
    case "child":
      return "child-grid"
    default:
      return "input"
  }
}

// optionWidget picks the single vs set picker from the Entity's cardinality.
function optionWidget(field: Field): string {
  return isMultiValue(field) ? "select-multi-tag" : "select"
}

function hasField(entity: EntitySchema, name: string): boolean {
  return entity.fields.some((f) => f.name === name)
}

function isBuiltinAction(name: string): boolean {
  return [
    "create",
    "update",
    "submit",
    "cancel",
    "delete",
    "amend",
    "create-submit",
    "amend-submit",
    "view",
    "edit",
  ].includes(name)
}

function entityDisplayName(entity: EntitySchema): string {
  return titleCase(entity.name)
}

function entityIcon(_entity: EntitySchema): string | undefined {
  return "FileText"
}

function moduleDisplayName(module: string): string {
  return titleCase(module)
}

function moduleIcon(_module: string): string | undefined {
  return "Folder"
}
