// ─── Grants Editor Widget ───
//
// Checkbox-tree editor for a Role's `grants` field (page → tab → action).
// Builds the tree from the meta bundle:
//   - authored pages (block/tab actions)
//   - derived entity CRUD pages ({entity}-page)
//   - navigation-only kinds ({kind}:{name} — Dashboard/Report/Wizard/Kanban/
//     Timeline/Print)
// Writes the grants JSON in the same shape the backend Materializer consumes:
//
//   [{ "page": "...", "actions": [{"name": "create"}],
//      "tabs": [{ "tab": "...", "actions": [{"name": "list"}] }] }]
//
// An action may also carry a `row_scope` — the per-role row restriction that a
// per-entity `row_scope` cannot express (kafe 10.67: only paid orders reach the
// kitchen). The conversion lives in `@/lib/grants` because `grants` is free JSON
// on the role entity: nothing validates it, and the backend resolver SKIPS what
// it cannot read, so a wrong key silently enforces nothing.

import { useEffect, useMemo, useState } from "react"
import {
  BarChart3,
  Clock,
  File,
  FileText,
  KanbanSquare,
  LayoutDashboard,
  Printer,
  Search,
  Wand2,
} from "lucide-react"
import { Checkbox } from "@/components/ui/checkbox"
import { Input } from "@/components/ui/input"
import { fetchMetaBundle } from "@/lib/api"
import {
  SCOPE_OPERATORS,
  describePredicate,
  grantKey,
  grantsToSelection,
  prunePredicates,
  selectionToGrants,
  type Grant,
  type ScopePredicate,
} from "@/lib/grants"
import { useMetaStore } from "@/stores/meta"
import { useSessionStore } from "@/stores/session"
import type {
  BlockRef,
  DashboardSpec,
  EntitySchema,
  Entry,
  MenuItem,
  MetaBundle,
  ReportSpec,
} from "@/types/manifest"

interface GrantsEditorProps {
  value?: unknown
  onChange?: (value: unknown) => void
  readonly?: boolean
  error?: string
}

interface ActionModel {
  name: string
  label: string
  permissions: string[]
}

interface TabModel {
  label: string
  actions: ActionModel[]
}

interface PageModel {
  page: string
  label: string
  module: string
  route: string
  kind: string
  actions: ActionModel[]
  tabs: TabModel[]
}

// ── Labels & icons ──

const ACTION_LABELS: Record<string, string> = {
  list: "Lihat daftar",
  view: "Lihat detail",
  create: "Buat",
  update: "Ubah",
  delete: "Hapus",
  submit: "Submit",
  cancel: "Batalkan",
  amend: "Amend",
  "create-submit": "Buat & Submit",
  "amend-submit": "Amend & Submit",
  deactivate: "Nonaktifkan",
  reactivate: "Aktifkan kembali",
}

const KIND_ICONS: Record<string, typeof File> = {
  page: File,
  entity: FileText,
  dashboard: LayoutDashboard,
  report: BarChart3,
  wizard: Wand2,
  kanban: KanbanSquare,
  timeline: Clock,
  print: Printer,
}

function actionLabel(name: string): string {
  return ACTION_LABELS[name] ?? titleCase(name)
}

function titleCase(s: string): string {
  return s.replace(/[-_]/g, " ").replace(/\b\w/g, (c) => c.toUpperCase())
}

// ── Permission derivation (mirrors backend Materializer) ──

function entityPlural(
  bundle: MetaBundle,
  module: string,
  entityRef: string,
): string {
  let m = module
  let name = entityRef
  if (entityRef.includes(".")) {
    const i = entityRef.indexOf(".")
    m = entityRef.slice(0, i)
    name = entityRef.slice(i + 1)
  }
  const e = (bundle.entities ?? []).find(
    (x) => x.module === m && x.name === name,
  )
  return e?.plural ?? (name.endsWith("s") ? name : `${name}s`)
}

function entityPerm(
  bundle: MetaBundle,
  module: string,
  entityRef: string,
  action: string,
): string {
  const plural = entityPlural(bundle, module, entityRef)
  let m = module
  if (entityRef.includes(".")) m = entityRef.slice(0, entityRef.indexOf("."))
  return `${m}.${plural}.${action}`
}

function qualifyPerm(module: string, perm: string): string {
  if (perm.split(".").length >= 3 || !module) return perm
  return `${module}.${perm}`
}

// Derive the actions a page exposes (mirrors the backend Materializer:
// form mode + custom form actions, table list/view + custom table actions).
function blockActions(
  block: { form?: BlockRef; table?: BlockRef },
  bundle: MetaBundle,
  module: string,
): ActionModel[] {
  const out: ActionModel[] = []
  const push = (action: string, entityRef: string) => {
    if (!out.some((a) => a.name === action)) {
      out.push({
        name: action,
        label: actionLabel(action),
        permissions: [entityPerm(bundle, module, entityRef, action)],
      })
    }
  }
  if (block.form?.ref) {
    const form = bundle.forms.find((f) => f.name === block.form?.ref)
    if (form) {
      // spec.entity is optional in the manifest type — a form without one
      // grants nothing entity-scoped.
      const entityRef = form.spec.entity ?? ""
      const mode = block.form?.mode ?? form.spec.mode ?? "create"
      push(mode === "edit" || mode === "view" ? "update" : "create", entityRef)
      form.spec.actions?.forEach((a) => push(a.action, entityRef))
    }
  }
  if (block.table?.ref) {
    const table = bundle.tables.find((t) => t.name === block.table?.ref)
    if (table) {
      push("list", table.spec.entity)
      push("view", table.spec.entity)
      push("create", table.spec.entity)
      push("update", table.spec.entity)
      push("delete", table.spec.entity)
      table.spec.row_actions?.forEach((a) => push(a.action, table.spec.entity))
      table.spec.bulk_actions?.forEach((a) => push(a.action, table.spec.entity))
    }
  }
  return out
}

function entityPageModel(entity: EntitySchema): PageModel {
  const actions: ActionModel[] = []
  const module = entity.module
  const plural = entity.plural
  const isSummary = entity.characteristic === "summary"
  const add = (name: string, permission: string) => {
    if (!actions.some((a) => a.name === name)) {
      actions.push({
        name,
        label: actionLabel(name),
        permissions: [permission],
      })
    }
  }
  // Standard CRUD actions (mirrors backend entityFootprint). Summary entities
  // are read-only projections — no create/update/delete.
  for (const name of ["list", "view", "create", "update", "delete"]) {
    if (
      isSummary &&
      (name === "create" || name === "update" || name === "delete")
    )
      continue
    add(name, `${module}.${plural}.${name}`)
  }
  // Custom actions from the schema (already carry their permission).
  for (const a of entity.actions ?? []) {
    if (a.permission) add(a.name, a.permission)
  }
  return {
    page: `${entity.name}-page`,
    label: titleCase(entity.name),
    module,
    route: `/${module}/${plural}`,
    kind: "entity",
    actions,
    tabs: [],
  }
}

function navPageModel(
  kind: string,
  name: string,
  label: string,
  module: string,
  route: string,
  permissions: string[],
): PageModel {
  return {
    page: `${kind}:${name}`,
    label,
    module,
    route,
    kind,
    actions: [
      {
        name: "view",
        label: "Lihat",
        permissions: permissions.length ? permissions : ["view"],
      },
    ],
    tabs: [],
  }
}

function dashboardPermissions(
  bundle: MetaBundle,
  d: Entry<DashboardSpec>,
): string[] {
  const out: string[] = []
  for (const w of d.spec.widgets ?? []) {
    const widget = (bundle.widgets ?? []).find((x) => x.name === w.ref)
    if (widget?.spec.entity) {
      const p = entityPerm(bundle, d.module, widget.spec.entity, "view")
      if (!out.includes(p)) out.push(p)
    }
  }
  return out
}

function reportPermissions(bundle: MetaBundle, r: Entry<ReportSpec>): string[] {
  if (r.spec.required_permission) {
    return [qualifyPerm(r.module, r.spec.required_permission)]
  }
  return [entityPerm(bundle, r.module, r.spec.entity, "list")]
}

function entityViewPermissions(
  bundle: MetaBundle,
  module: string,
  entityRef?: string,
): string[] {
  if (!entityRef) return []
  return [entityPerm(bundle, module, entityRef, "view")]
}

// Flatten the app menu tree into an ordered list of leaf routes (depth-first).
// Used to order grant pages to match the menu order.
function flattenMenuRoutes(menu: MenuItem[]): string[] {
  const out: string[] = []
  const walk = (items: MenuItem[]) => {
    for (const item of items) {
      if (item.route) out.push(item.route)
      if (item.children?.length) walk(item.children)
    }
  }
  walk(menu ?? [])
  return out
}

function buildPageModels(bundle: MetaBundle): PageModel[] {
  const models: PageModel[] = []

  // 1. Authored pages.
  for (const page of bundle.pages ?? []) {
    if (page.spec.tabs?.length) {
      models.push({
        page: page.name,
        label: page.spec.title || page.name,
        module: page.module,
        route: page.spec.route || `/${page.name}`,
        kind: "page",
        actions: [],
        tabs: page.spec.tabs.map((tab) => ({
          label: tab.label,
          actions: blockActions(tab, bundle, page.module),
        })),
      })
    } else {
      models.push({
        page: page.name,
        label: page.spec.title || page.name,
        module: page.module,
        route: page.spec.route || `/${page.name}`,
        kind: "page",
        actions: (page.spec.blocks ?? []).flatMap((b) =>
          blockActions(b, bundle, page.module),
        ),
        tabs: [],
      })
    }
  }

  // 2. Derived entity CRUD pages.
  for (const entity of bundle.entities ?? []) {
    models.push(entityPageModel(entity))
  }

  // 3. Navigation-only kinds.
  for (const d of bundle.dashboards ?? []) {
    models.push(
      navPageModel(
        "dashboard",
        d.name,
        d.spec.title || d.name,
        d.module,
        `/dashboard/${d.name}`,
        dashboardPermissions(bundle, d),
      ),
    )
  }
  for (const r of bundle.reports ?? []) {
    models.push(
      navPageModel(
        "report",
        r.name,
        r.spec.title || r.name,
        r.module,
        `/report/${r.name}`,
        reportPermissions(bundle, r),
      ),
    )
  }
  for (const w of bundle.wizards ?? []) {
    models.push(
      navPageModel(
        "wizard",
        w.name,
        w.spec.title || w.name,
        w.module,
        `/wizard/${w.name}`,
        entityViewPermissions(bundle, w.module, w.spec.entity),
      ),
    )
  }
  for (const k of bundle.kanbans ?? []) {
    models.push(
      navPageModel(
        "kanban",
        k.name,
        titleCase(k.name),
        k.module,
        `/kanban/${k.name}`,
        entityViewPermissions(bundle, k.module, k.spec.entity),
      ),
    )
  }
  for (const t of bundle.timelines ?? []) {
    models.push(
      navPageModel(
        "timeline",
        t.name,
        titleCase(t.name),
        t.module,
        `/timeline/${t.name}`,
        entityViewPermissions(bundle, t.module, t.spec.entity),
      ),
    )
  }
  for (const p of bundle.prints ?? []) {
    models.push(
      navPageModel(
        "print",
        p.name,
        titleCase(p.name),
        p.module,
        `/print/${p.name}`,
        entityViewPermissions(bundle, p.module, p.spec.entity),
      ),
    )
  }

  // Order pages to match the app menu (depth-first leaf routes); pages not
  // present in the menu keep their build order at the end.
  const menuRoutes = flattenMenuRoutes(bundle.menu ?? [])
  const menuIndex = new Map(menuRoutes.map((r, i) => [r, i]))
  models.sort((a, b) => {
    const ia = menuIndex.get(a.route)
    const ib = menuIndex.get(b.route)
    if (ia !== undefined && ib !== undefined) return ia - ib
    if (ia !== undefined) return -1
    if (ib !== undefined) return 1
    return 0
  })

  return models
}

// Key format: "page" | "page::action" | "page::tab::action"
// ── Selection helpers ──

function pageChecked(
  m: PageModel,
  keys: Set<string>,
): boolean | "indeterminate" {
  const all: string[] = []
  for (const a of m.actions) all.push(grantKey(m.page, undefined, a.name))
  for (const t of m.tabs)
    for (const a of t.actions) all.push(grantKey(m.page, t.label, a.name))
  if (!all.length) return false
  const checked = all.filter((k) => keys.has(k)).length
  if (checked === all.length) return true
  if (checked > 0) return "indeterminate"
  return false
}

function tabChecked(
  m: PageModel,
  t: TabModel,
  keys: Set<string>,
): boolean | "indeterminate" {
  if (!t.actions.length) return false
  const checked = t.actions.filter((a) =>
    keys.has(grantKey(m.page, t.label, a.name)),
  ).length
  if (checked === t.actions.length) return true
  if (checked > 0) return "indeterminate"
  return false
}

function selectedPermissions(models: PageModel[], keys: Set<string>): string[] {
  const out: string[] = []
  for (const m of models) {
    for (const a of m.actions) {
      if (keys.has(grantKey(m.page, undefined, a.name)))
        out.push(...a.permissions)
    }
    for (const t of m.tabs) {
      for (const a of t.actions) {
        if (keys.has(grantKey(m.page, t.label, a.name)))
          out.push(...a.permissions)
      }
    }
  }
  return [...new Set(out)]
}

// ── Search ──

function actionMatches(a: ActionModel, q: string): boolean {
  return (
    a.name.toLowerCase().includes(q) ||
    a.label.toLowerCase().includes(q) ||
    a.permissions.some((p) => p.toLowerCase().includes(q))
  )
}

function filterModels(models: PageModel[], query: string): PageModel[] {
  const q = query.trim().toLowerCase()
  if (!q) return models
  return models
    .map((m) => {
      const pageMatch =
        m.label.toLowerCase().includes(q) ||
        m.page.toLowerCase().includes(q) ||
        m.module.toLowerCase().includes(q) ||
        m.route.toLowerCase().includes(q)
      if (pageMatch) return m
      const actions = m.actions.filter((a) => actionMatches(a, q))
      const tabs = m.tabs
        .map((t) => ({
          ...t,
          actions: t.actions.filter((a) => actionMatches(a, q)),
        }))
        .filter((t) => t.actions.length)
      if (actions.length || tabs.length) return { ...m, actions, tabs }
      return null
    })
    .filter((m): m is PageModel => m !== null)
}

export function GrantsEditor({
  value,
  onChange,
  readonly = false,
  error,
}: GrantsEditorProps) {
  const workspace = useSessionStore((s) => s.workspace)
  const appName = useMetaStore((s) => s.bundle?.app.name)
  const metaBundle = useMetaStore((s) => s.bundle)
  // The grants editor is an admin tool: it must list every page/action in the
  // App regardless of the caller's own permissions. Fetch the app-scoped but
  // unfiltered bundle (`?grants=true`); fall back to the filtered meta bundle
  // if the fetch fails (e.g. caller lacks role-management permission).
  const [grantsBundle, setGrantsBundle] = useState<MetaBundle | null>(null)
  useEffect(() => {
    if (!workspace || !appName) return
    let cancelled = false
    fetchMetaBundle(workspace, { appName, grants: true })
      .then((b) => {
        if (!cancelled) setGrantsBundle(b)
      })
      .catch(() => {
        /* fall back to the filtered meta bundle */
      })
    return () => {
      cancelled = true
    }
  }, [workspace, appName])

  const bundle = grantsBundle ?? metaBundle
  const models = useMemo(
    () => (bundle ? buildPageModels(bundle) : []),
    [bundle],
  )
  const initial = useMemo(
    () => grantsToSelection(value as Grant[] | undefined),
    [value],
  )
  const [keys, setKeys] = useState<Set<string>>(() => initial.keys)
  // Row scopes being edited, keyed exactly like the checkboxes. Kept as raw
  // predicate rows (not as FilterSpec) so a half-typed row can exist in the UI
  // without ever reaching the manifest — `prunePredicates` is what turns them
  // into a declaration, and it drops anything inert.
  const [scopeEdits, setScopeEdits] = useState<
    Record<string, ScopePredicate[]>
  >({})
  const [query, setQuery] = useState("")

  // Sync internal selection when the external value changes (e.g. role data
  // loads after the form mounts).
  useEffect(() => {
    setKeys(grantsToSelection(value as Grant[] | undefined).keys)
  }, [value])

  /** The row scope of one action, preferring in-editor edits over stored data. */
  const predicatesFor = (k: string): ScopePredicate[] => {
    const edited = scopeEdits[k]
    if (edited) return edited
    return (initial.scopes[k] ?? []).map((s) => ({
      field: s.field,
      op: s.op ?? "eq",
      from: s.from,
      value: s.value,
      attr: s.attr,
      param: s.param,
    }))
  }

  const setPredicates = (k: string, next: ScopePredicate[], keys2?: Set<string>) => {
    const nextEdits = { ...scopeEdits, [k]: next }
    setScopeEdits(nextEdits)
    commit(keys2 ?? keys, nextEdits)
  }

  const visibleModels = useMemo(
    () => filterModels(models, query),
    [models, query],
  )
  const preview = useMemo(
    () => selectedPermissions(models, keys),
    [models, keys],
  )

  const commit = (
    next: Set<string>,
    edits: Record<string, ScopePredicate[]> = scopeEdits,
  ) => {
    setKeys(next)
    onChange?.(selectionToGrants(models, { keys: next, scopes: initial.scopes }, edits))
  }

  const toggle = (key: string, checked: boolean) => {
    const next = new Set(keys)
    if (checked) next.add(key)
    else next.delete(key)
    commit(next)
  }

  const toggleAll = (pageName: string, checked: boolean) => {
    const m = models.find((x) => x.page === pageName)
    if (!m) return
    const next = new Set(keys)
    for (const a of m.actions) {
      if (checked) next.add(grantKey(m.page, undefined, a.name))
      else next.delete(grantKey(m.page, undefined, a.name))
    }
    for (const t of m.tabs) {
      for (const a of t.actions) {
        if (checked) next.add(grantKey(m.page, t.label, a.name))
        else next.delete(grantKey(m.page, t.label, a.name))
      }
    }
    commit(next)
  }

  const toggleTab = (pageName: string, tabLabel: string, checked: boolean) => {
    const m = models.find((x) => x.page === pageName)
    if (!m) return
    const t = m.tabs.find((x) => x.label === tabLabel)
    if (!t) return
    const next = new Set(keys)
    for (const a of t.actions) {
      if (checked) next.add(grantKey(m.page, t.label, a.name))
      else next.delete(grantKey(m.page, t.label, a.name))
    }
    commit(next)
  }

  if (readonly) {
    const grants = selectionToGrants(models, { keys, scopes: initial.scopes })
    return (
      <pre className="py-1 text-xs font-mono whitespace-pre-wrap wrap-break-word text-muted-foreground">
        {grants.length ? JSON.stringify(grants, null, 2) : "-"}
      </pre>
    )
  }

  if (!models.length) {
    return (
      <p className="text-xs text-muted-foreground">
        Tidak ada page yang tersedia untuk grants.
      </p>
    )
  }

  return (
    <div className="space-y-3 rounded-lg border p-3">
      {error && <p className="text-sm text-destructive">{error}</p>}

      <div className="relative">
        <Search className="absolute top-2 left-2.5 h-4 w-4 text-muted-foreground" />
        <Input
          value={query}
          onChange={(e) => setQuery(e.target.value)}
          placeholder="Cari page atau permission..."
          className="pl-8"
        />
      </div>

      <div className="space-y-2">
        {visibleModels.map((m) => {
          const Icon = KIND_ICONS[m.kind] ?? File
          return (
            <div key={m.page} className="space-y-1.5 rounded-md border p-2">
              {/* Page header — page apa yang sedang disetting */}
              <div className="flex items-center gap-2">
                <Checkbox
                  checked={pageChecked(m, keys) === true}
                  onCheckedChange={(c: boolean | "indeterminate") =>
                    toggleAll(m.page, !!c)
                  }
                  disabled={readonly}
                />
                <Icon className="h-4 w-4 shrink-0 text-muted-foreground" />
                <span className="text-sm font-medium">{m.label}</span>
                <span className="rounded bg-muted px-1.5 py-0.5 font-mono text-[10px] text-muted-foreground">
                  {m.module}
                </span>
                <span className="hidden font-mono text-[10px] text-muted-foreground sm:inline">
                  {m.route}
                </span>
              </div>

              {/* Tabs */}
              {m.tabs.length > 0 ? (
                <div className="ml-6 space-y-1.5 border-l pl-3">
                  {m.tabs.map((t) => (
                    <div key={t.label} className="space-y-1">
                      <div className="flex items-center gap-2">
                        <Checkbox
                          checked={tabChecked(m, t, keys) === true}
                          onCheckedChange={(c: boolean | "indeterminate") =>
                            toggleTab(m.page, t.label, !!c)
                          }
                          disabled={readonly}
                        />
                        <span className="text-sm">{t.label}</span>
                      </div>
                      <div className="ml-6 space-y-1">
                        {t.actions.map((a) => {
                          const k = grantKey(m.page, t.label, a.name)
                          return (
                            <ActionRow
                              key={a.name}
                              action={a}
                              checked={keys.has(k)}
                              onToggle={(c) => toggle(k, c)}
                              readonly={readonly}
                              predicates={predicatesFor(k)}
                              onPredicates={(next) => setPredicates(k, next)}
                            />
                          )
                        })}
                      </div>
                    </div>
                  ))}
                </div>
              ) : (
                <div className="ml-6 space-y-1">
                  {m.actions.map((a) => {
                    const k = grantKey(m.page, undefined, a.name)
                    return (
                      <ActionRow
                        key={a.name}
                        action={a}
                        checked={keys.has(k)}
                        onToggle={(c) => toggle(k, c)}
                        readonly={readonly}
                        predicates={predicatesFor(k)}
                        onPredicates={(next) => setPredicates(k, next)}
                      />
                    )
                  })}
                </div>
              )}
            </div>
          )
        })}
      </div>

      {/* Preview permission termaterialisasi */}
      <div className="rounded-md bg-muted/50 p-2">
        <p className="text-xs font-medium text-muted-foreground">
          Preview permission termaterialisasi ({preview.length})
        </p>
        {preview.length ? (
          <pre className="mt-1 max-h-40 overflow-auto font-mono text-[10px] leading-relaxed text-muted-foreground">
            {preview.join("\n")}
          </pre>
        ) : (
          <p className="mt-0.5 text-xs text-muted-foreground">
            Belum ada permission dipilih.
          </p>
        )}
      </div>
    </div>
  )
}

function ActionRow({
  action,
  checked,
  onToggle,
  readonly,
  predicates,
  onPredicates,
}: {
  action: ActionModel
  checked: boolean
  onToggle: (checked: boolean) => void
  readonly: boolean
  predicates: ScopePredicate[]
  onPredicates: (next: ScopePredicate[]) => void
}) {
  const [open, setOpen] = useState(false)
  const active = predicates.length > 0

  return (
    <div className="space-y-1">
      <div className="flex items-start gap-1.5 text-xs">
        <Checkbox
          checked={checked}
          onCheckedChange={(c: boolean | "indeterminate") => onToggle(!!c)}
          disabled={readonly}
          className="mt-0.5"
        />
        <button
          type="button"
          onClick={() => checked && setOpen((v) => !v)}
          disabled={!checked}
          className="min-w-0 flex-1 text-left disabled:cursor-default"
        >
          <span className="block font-medium">
            {action.label}
            {active && (
              <span className="ml-1.5 rounded bg-muted px-1 py-0.5 font-normal text-[10px] text-muted-foreground">
                {predicates.length} batasan baris
              </span>
            )}
          </span>
          <span className="block font-mono text-[10px] text-muted-foreground">
            {action.permissions.join(", ")}
          </span>
        </button>
        {checked && !readonly && (
          <button
            type="button"
            onClick={() => setOpen((v) => !v)}
            className="shrink-0 text-[10px] text-muted-foreground underline-offset-2 hover:underline"
          >
            {open ? "tutup" : active ? "ubah batasan" : "batasi baris"}
          </button>
        )}
      </div>

      {checked && open && !readonly && (
        <RowScopeEditor predicates={predicates} onChange={onPredicates} />
      )}
    </div>
  )
}

/**
 * Edits the row scope of ONE granted action.
 *
 * Why this exists at all: a row restriction is the only way to state a
 * per-ROLE rule like "only paid orders reach the kitchen" (kafe 10.67). An
 * entity `row_scope` cannot express it — that one filters by session attribute
 * and applies to every caller, so using it would blind the cashier to the
 * drafts they are composing.
 *
 * The editor writes what the backend reads: `field` + `op` + exactly one value
 * source (`value`, or `from: session|route`). Anything inert is dropped by
 * `prunePredicates` before it can reach the manifest, because a `row_scope` that
 * cannot be resolved makes the backend refuse every read (fail-closed 403).
 */
function RowScopeEditor({
  predicates,
  onChange,
}: {
  predicates: ScopePredicate[]
  onChange: (next: ScopePredicate[]) => void
}) {
  const update = (i: number, patch: Partial<ScopePredicate>) => {
    onChange(predicates.map((p, j) => (i === j ? { ...p, ...patch } : p)))
  }
  const remove = (i: number) => onChange(predicates.filter((_, j) => j !== i))
  const add = () => onChange([...predicates, { field: "", op: "eq", value: "" }])

  return (
    <div className="ml-6 space-y-1 rounded border bg-muted/30 p-1.5">
      <p className="text-[10px] leading-snug text-muted-foreground">
        Hanya baris yang cocok SEMUA batasan di bawah yang boleh diakses peran
        ini. `value` = konstanta (mis. <span className="font-mono">paid,ready</span> untuk
        operator <span className="font-mono">in</span>);{" "}
        <span className="font-mono">session</span> = atribut sesi (mis. cabang).
      </p>

      {predicates.map((p, i) => {
        const source = p.from ?? "literal"
        return (
          <div key={i} className="flex flex-wrap items-center gap-1">
            <Input
              value={p.field}
              onChange={(e) => update(i, { field: e.target.value })}
              placeholder="field"
              className="h-7 w-28 font-mono text-[11px]"
            />
            <select
              value={p.op}
              onChange={(e) => update(i, { op: e.target.value })}
              className="h-7 rounded border bg-background px-1 font-mono text-[11px]"
            >
              {SCOPE_OPERATORS.map((op) => (
                <option key={op} value={op}>
                  {op}
                </option>
              ))}
            </select>
            <select
              value={source}
              onChange={(e) => {
                const v = e.target.value
                if (v === "literal") update(i, { from: undefined, attr: undefined, param: undefined })
                else if (v === "session") update(i, { from: "session", value: undefined, attr: "" })
                else update(i, { from: "route", value: undefined, param: "" })
              }}
              className="h-7 rounded border bg-background px-1 text-[11px]"
            >
              <option value="literal">nilai tetap</option>
              <option value="session">atribut sesi</option>
              <option value="route">parameter URL</option>
            </select>
            {source === "session" ? (
              <Input
                value={p.attr ?? ""}
                onChange={(e) => update(i, { attr: e.target.value })}
                placeholder="attr (mis. branch_id)"
                className="h-7 w-40 font-mono text-[11px]"
              />
            ) : source === "route" ? (
              <Input
                value={p.param ?? ""}
                onChange={(e) => update(i, { param: e.target.value })}
                placeholder="param (mis. token)"
                className="h-7 w-40 font-mono text-[11px]"
              />
            ) : (
              <Input
                value={p.value ?? ""}
                onChange={(e) => update(i, { value: e.target.value })}
                placeholder="value"
                className="h-7 w-40 font-mono text-[11px]"
              />
            )}
            <button
              type="button"
              onClick={() => remove(i)}
              className="text-[10px] text-muted-foreground underline-offset-2 hover:underline"
            >
              hapus
            </button>
          </div>
        )
      })}

      <div className="flex items-center gap-2">
        <button
          type="button"
          onClick={add}
          className="text-[10px] font-medium text-muted-foreground underline-offset-2 hover:underline"
        >
          + tambah batasan
        </button>
        {predicates.length > 0 && (
          <span className="font-mono text-[10px] text-muted-foreground">
            {prunePredicates(predicates).map(describePredicate).join(" · ") ||
              "belum ada batasan yang bisa disimpan"}
          </span>
        )}
      </div>
    </div>
  )
}
