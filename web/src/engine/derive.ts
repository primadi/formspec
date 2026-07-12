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
  type MenuSpec,
} from "@/types/manifest"

// ── Main derive functions ──

/**
 * Derive a default TableSpec from an entity schema.
 */
export function deriveTable(entity: EntitySchema): TableSpec {
  const columns: TableColumn[] = []
  const rowActions: TableAction[] = []

  // Columns: non-child fields, declaration order, max ~8
  const visibleFields = entity.fields
    .filter((f) => f.type !== "child" && !f.computed)
    .slice(0, 8)

  for (const field of visibleFields) {
    columns.push({
      field: field.name,
      label: fieldLabel(field),
      sortable: isSortable(field),
      widget: tableWidget(field),
      format: tableFormat(field),
    })
  }

  // Row actions: view, edit, delete + custom actions
  // Summary entities are read-only projections — no edit/delete
  if (entity.characteristic !== "summary") {
    rowActions.push(
      { action: "view", label: "View", icon: "Eye" },
      { action: "edit", label: "Edit", icon: "Pencil" },
      { action: "delete", label: "Delete", icon: "Trash2", confirm_msg: "Are you sure you want to delete this item?" },
    )
  } else {
    rowActions.push({ action: "view", label: "View", icon: "Eye" })
  }

  // Add custom actions that have UI hints
  for (const action of (entity.actions ?? [])) {
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
export function deriveForm(entity: EntitySchema, mode: "create" | "edit" | "view" = "create"): FormSpec {
  const sections: FormSection[] = []
  const editableFields = entity.fields.filter(
    (f) => mode !== "create" || !f.computed,
  )

  const section: FormSection = {
    title: mode === "create" ? "New Entry" : mode === "edit" ? "Edit Entry" : "Details",
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
function deriveFormRenderMode(fields: Field[]): "modal" | "drawer" | "separate_page" {
  const hasChildTable = fields.some((f) => f.type === "child" && f.child?.storage === "table")
  const fieldCount = fields.length

  if (fieldCount > 12 || hasChildTable) return "separate_page"
  if (fieldCount > 5) return "drawer"
  return "modal"
}

/**
 * Derive default menu entries for an entity.
 */
export function deriveMenuItems(
  entities: EntitySchema[],
): MenuSpec[] {
  const byModule = new Map<string, EntitySchema[]>()
  for (const e of entities) {
    const list = byModule.get(e.module) ?? []
    list.push(e)
    byModule.set(e.module, list)
  }

  const menus: MenuSpec[] = []
  for (const [module, ents] of byModule) {
    const children: MenuSpec[] = ents
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
  authoredTables: ReadonlyMap<string, import("@/types/manifest").Entry<TableSpec>>,
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
  authoredForms: ReadonlyMap<string, import("@/types/manifest").Entry<FormSpec>>,
): FormSpec {
  const modeSuffix = mode === "create" ? "create" : mode === "edit" ? "edit" : "view"

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
  return ["string", "integer", "decimal", "date", "datetime", "enum", "boolean"].includes(field.type)
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
    if (field.rules?.some((r) => r.name === "min" && typeof (r.value as number) === "number")) return "currency"
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
      if (field.rules?.some((r) => r.name === "max_length" && typeof r.value === "number" && (r.value as number) > 120)) {
        return "textarea"
      }
      return "input"
    case "integer":
    case "decimal":
      return "number"
    case "boolean":
      return "switch"
    case "enum":
      return "select"
    case "date":
    case "datetime":
      return "datepicker"
    case "uuid":
      return "uuid"
    case "json":
      return "json"
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
  return ["create", "update", "submit", "cancel", "delete", "amend", "create-submit", "amend-submit", "view", "edit"].includes(name)
}

function entityDisplayName(entity: EntitySchema): string {
  return entity.name.charAt(0).toUpperCase() + entity.name.slice(1).replace(/-/g, " ")
}

function entityIcon(_entity: EntitySchema): string | undefined {
  return "FileText"
}

function moduleDisplayName(module: string): string {
  return module.charAt(0).toUpperCase() + module.slice(1).replace(/-/g, " ")
}

function moduleIcon(_module: string): string | undefined {
  return "Folder"
}
