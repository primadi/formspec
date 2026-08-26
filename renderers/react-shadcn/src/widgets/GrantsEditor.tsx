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

interface GrantAction {
  name: string
}

interface GrantTab {
  tab: string
  actions: GrantAction[]
}

interface Grant {
  page: string
  actions?: GrantAction[]
  tabs?: GrantTab[]
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
      const mode = block.form?.mode ?? form.spec.mode ?? "create"
      push(
        mode === "edit" || mode === "view" ? "update" : "create",
        form.spec.entity,
      )
      form.spec.actions?.forEach((a) => push(a.action, form.spec.entity))
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
function grantKey(page: string, tab?: string, action?: string): string {
  if (tab && action) return `${page}::${tab}::${action}`
  if (action) return `${page}::${action}`
  return page
}

function grantsToKeys(grants: Grant[] | undefined): Set<string> {
  const keys = new Set<string>()
  for (const g of grants ?? []) {
    if (g.tabs?.length) {
      for (const t of g.tabs) {
        for (const a of t.actions) keys.add(grantKey(g.page, t.tab, a.name))
      }
    } else {
      for (const a of g.actions ?? [])
        keys.add(grantKey(g.page, undefined, a.name))
    }
  }
  return keys
}

function keysToGrants(models: PageModel[], keys: Set<string>): Grant[] {
  const grants: Grant[] = []
  for (const m of models) {
    if (m.tabs.length) {
      const tabs: GrantTab[] = []
      for (const t of m.tabs) {
        const actions = t.actions
          .filter((a) => keys.has(grantKey(m.page, t.label, a.name)))
          .map((a) => ({ name: a.name }))
        if (actions.length) tabs.push({ tab: t.label, actions })
      }
      if (tabs.length) grants.push({ page: m.page, tabs })
    } else {
      const actions = m.actions
        .filter((a) => keys.has(grantKey(m.page, undefined, a.name)))
        .map((a) => ({ name: a.name }))
      if (actions.length) grants.push({ page: m.page, actions })
    }
  }
  return grants
}

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
  const [keys, setKeys] = useState<Set<string>>(() =>
    grantsToKeys(value as Grant[] | undefined),
  )
  const [query, setQuery] = useState("")

  // Sync internal selection when the external value changes (e.g. role data
  // loads after the form mounts).
  useEffect(() => {
    setKeys(grantsToKeys(value as Grant[] | undefined))
  }, [value])

  const visibleModels = useMemo(
    () => filterModels(models, query),
    [models, query],
  )
  const preview = useMemo(
    () => selectedPermissions(models, keys),
    [models, keys],
  )

  const commit = (next: Set<string>) => {
    setKeys(next)
    onChange?.(keysToGrants(models, next))
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
    const grants = keysToGrants(models, keys)
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
                        {t.actions.map((a) => (
                          <ActionRow
                            key={a.name}
                            action={a}
                            checked={keys.has(
                              grantKey(m.page, t.label, a.name),
                            )}
                            onToggle={(c) =>
                              toggle(grantKey(m.page, t.label, a.name), c)
                            }
                            readonly={readonly}
                          />
                        ))}
                      </div>
                    </div>
                  ))}
                </div>
              ) : (
                <div className="ml-6 space-y-1">
                  {m.actions.map((a) => (
                    <ActionRow
                      key={a.name}
                      action={a}
                      checked={keys.has(grantKey(m.page, undefined, a.name))}
                      onToggle={(c) =>
                        toggle(grantKey(m.page, undefined, a.name), c)
                      }
                      readonly={readonly}
                    />
                  ))}
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
}: {
  action: ActionModel
  checked: boolean
  onToggle: (checked: boolean) => void
  readonly: boolean
}) {
  return (
    <label className="flex cursor-pointer items-start gap-1.5 text-xs">
      <Checkbox
        checked={checked}
        onCheckedChange={(c: boolean | "indeterminate") => onToggle(!!c)}
        disabled={readonly}
        className="mt-0.5"
      />
      <span className="min-w-0">
        <span className="block font-medium">{action.label}</span>
        <span className="block font-mono text-[10px] text-muted-foreground">
          {action.permissions.join(", ")}
        </span>
      </span>
    </label>
  )
}
