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
    (f) => mode !== "create" || !f.computed,
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
  if (authored) return authored.spec
  return deriveTable(entity)
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
    if (named) return named.spec
  }

  const modeSuffix =
    mode === "create" ? "create" : mode === "edit" ? "edit" : "view"

  // 1. Mode-specific: entity-create, entity-edit, entity-view
  const modeForm = authoredForms.get(`${entity.name}-${modeSuffix}`)
  if (modeForm) return modeForm.spec

  // 2. Generic entity form: entity-form
  const genericForm = authoredForms.get(`${entity.name}-form`)
  if (genericForm) return genericForm.spec

  // 3. Fallback: auto-generate
  return deriveForm(entity, mode)
}

// ── Field mapping helpers ──

function fieldLabel(field: Field): string {
  if (field.title) return field.title
  return field.name
    .replace(/_/g, " ")
    .replace(/\bid\b/i, "ID")
    .replace(/\b\w/g, (c) => c.toUpperCase())
}

function isSortable(field: Field): boolean {
  return [
    "string",
    "integer",
    "decimal",
    "date",
    "datetime",
    "enum",
    "boolean",
  ].includes(field.type)
}

function tableWidget(field: Field): string | undefined {
  if (field.type === "enum" || field.name === "doc_status") return "badge"
  if (field.type === "boolean") return "boolean"
  return undefined
}

function tableFormat(field: Field): string | undefined {
  if (field.type === "datetime") return "relative"
  if (field.type === "date") return "date"
  if (field.type === "decimal") {
    // Check if the field has currency-like rules
    if (
      field.rules?.some(
        (r) => r.name === "min" && typeof (r.value as number) === "number",
      )
    )
      return "currency"
  }
  return undefined
}

function formField(field: Field, mode: "create" | "edit" | "view"): FormField {
  const ff: FormField = {
    name: field.name,
    label: fieldLabel(field),
    widget: formWidget(field),
  }

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
      return "input"
    case "text":
      return "textarea"
    case "richtext":
      return "richtext"
    case "integer":
      return "number"
    case "decimal":
      return "decimalinput"
    case "boolean":
      return "switch"
    case "enum":
      return "select"
    case "date":
      return "datepicker"
    case "datetime":
      return "datetimeinput"
    case "uuid":
      return "uuid"
    case "json":
      return "json"
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
