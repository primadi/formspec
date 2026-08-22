// ─── Meta Store ───
//
// Zustand store for the Meta bundle (entity schemas + authored UI manifests).
// Loaded once at boot via fetchMetaBundle(). Provides selectors for quick
// lookup by entity name, form name, table name, etc.

import { create } from "zustand"
import { HTTPError } from "ky"

import {
  type MetaBundle,
  type EntitySchema,
  type Entry,
  type PageSpec,
  type FormSpec,
  type TableSpec,
  type DashboardSpec,
  type WidgetSpec,
  type ReportSpec,
  type WizardSpec,
  type KanbanSpec,
  type TimelineSpec,
  type PrintSpec,
  type ThemeSpec,
  type ListingSpec,
  type AppSummary,
} from "@/types/manifest"
import { fetchMetaBundle, fetchMetaApps } from "@/lib/api"
import { FormaApiError } from "@/types/manifest"

// Picks which resolved App (Core §4.4) the current URL belongs to: the
// longest root_url prefix match, after stripping the leading {workspace}
// path segment. Falls back to the first App when nothing matches (e.g. the
// _admin surface, which isn't scoped to any App's root_url).
function detectAppName(
  pathname: string,
  apps: AppSummary[],
): string | undefined {
  if (apps.length === 0) return undefined
  if (apps.length === 1) return apps[0].name

  const segments = pathname.split("/").filter(Boolean)
  const rest = "/" + segments.slice(1).join("/") // drop {workspace}

  let best: AppSummary | undefined
  for (const a of apps) {
    if (rest === a.root_url || rest.startsWith(a.root_url + "/")) {
      if (!best || a.root_url.length > best.root_url.length) best = a
    }
  }
  return (best ?? apps[0]).name
}

export interface MetaState {
  bundle: MetaBundle | null
  loading: boolean
  error: string | null
  // Set when the server rejected the request with 403 (e.g. `_admin` without
  // the `_admin.access` permission) — distinct from `error` so the UI can
  // show "Access Denied" instead of a generic connection-error screen.
  forbidden: boolean

  // ── Actions ──
  load: (
    workspace: string,
    surface: "admin" | "app",
    token?: string,
  ) => Promise<void>
  reset: () => void
  refresh: (
    workspace: string,
    surface: "admin" | "app",
    token?: string,
  ) => Promise<void>

  // ── Entity Lookups ──
  getEntity: (module: string, name: string) => EntitySchema | undefined
  getEntityByPlural: (
    module: string,
    plural: string,
  ) => EntitySchema | undefined

  // ── Manifest Lookups ──
  getPage: (name: string) => Entry<PageSpec> | undefined
  getForm: (name: string) => Entry<FormSpec> | undefined
  getTable: (name: string) => Entry<TableSpec> | undefined
  getDashboard: (name: string) => Entry<DashboardSpec> | undefined
  getWidget: (name: string) => Entry<WidgetSpec> | undefined
  getReport: (name: string) => Entry<ReportSpec> | undefined
  getWizard: (name: string) => Entry<WizardSpec> | undefined
  getKanban: (name: string) => Entry<KanbanSpec> | undefined
  getTimeline: (name: string) => Entry<TimelineSpec> | undefined
  getPrint: (name: string) => Entry<PrintSpec> | undefined
  getTheme: (name: string) => Entry<ThemeSpec> | undefined
  getListing: (name: string) => Entry<ListingSpec> | undefined

  // ── Derived Helpers ──
  /** Get all entities that need default UI derivation (not yet covered by authored pages/tables/forms) */
  getDerivedEntities: () => EntitySchema[]
  /** Get all entities grouped by module */
  getEntitiesByModule: () => Map<string, EntitySchema[]>
}

function createLookups(bundle: MetaBundle) {
  const byName = <T>(items: Entry<T>[]) => {
    const map = new Map<string, Entry<T>>()
    for (const item of items) {
      map.set(item.name, item)
    }
    return map
  }

  const entitiesByKey = new Map<string, EntitySchema>()
  const entitiesByPlural = new Map<string, EntitySchema>()
  for (const e of bundle.entities) {
    entitiesByKey.set(`${e.module}/${e.name}`, e)
    entitiesByPlural.set(`${e.module}/${e.plural}`, e)
  }

  const pages = byName(bundle.pages)
  const forms = byName(bundle.forms)
  const tables = byName(bundle.tables)
  const dashboards = byName(bundle.dashboards)
  const widgets = byName(bundle.widgets)
  const reports = byName(bundle.reports)
  const wizards = byName(bundle.wizards)
  const kanbans = byName(bundle.kanbans)
  const timelines = byName(bundle.timelines)
  const prints = byName(bundle.prints)
  const themes = byName(bundle.themes)
  const listings = byName(bundle.listings)

  return {
    entitiesByKey,
    entitiesByPlural,
    pages,
    forms,
    tables,
    dashboards,
    widgets,
    reports,
    wizards,
    kanbans,
    timelines,
    prints,
    themes,
    listings,
  }
}

// Build lookup maps lazily — they're derived from `bundle`.
function getOrBuildLookups(bundle: MetaBundle | null) {
  if (!bundle) return null
  // Cache on the bundle reference to avoid rebuilding on every selector call
  if ((bundle as any).__lookups)
    return (bundle as any).__lookups as ReturnType<typeof createLookups>
  const lookups = createLookups(bundle)
  ;(bundle as any).__lookups = lookups
  return lookups
}

export const useMetaStore = create<MetaState>((set, get) => ({
  bundle: null,
  loading: false,
  error: null,
  forbidden: false,

  load: async (workspace: string, surface: "admin" | "app", token?: string) => {
    set({ loading: true, error: null, forbidden: false })
    try {
      // `_admin` isn't scoped to any App (Core §4.4) — skip App detection
      // entirely and fetch the unscoped, binary-gated bundle.
      let bundle: MetaBundle
      if (surface === "admin") {
        bundle = await fetchMetaBundle(workspace, { admin: true, token })
      } else {
        const apps = await fetchMetaApps(workspace, token)
        const appName = detectAppName(window.location.pathname, apps)
        bundle = await fetchMetaBundle(workspace, { appName, token })
      }
      set({ bundle, loading: false, error: null, forbidden: false })
    } catch (err) {
      // 403 → forbidden (distinct from a connection error): the `_admin`
      // surface without `_admin.access`, or an app the caller can't see.
      // fetchMetaBundle throws a ky HTTPError (not FormaApiError), so check
      // both.
      const status =
        err instanceof FormaApiError
          ? err.status
          : err instanceof HTTPError
            ? err.response.status
            : undefined
      if (status === 403) {
        set({ loading: false, error: null, forbidden: true })
        return
      }
      const message =
        err instanceof Error ? err.message : "Failed to load meta bundle"
      set({ loading: false, error: message })
    }
  },

  refresh: async (
    workspace: string,
    surface: "admin" | "app",
    token?: string,
  ) => {
    try {
      let bundle: MetaBundle
      if (surface === "admin") {
        bundle = await fetchMetaBundle(workspace, { admin: true, token })
      } else {
        const apps = await fetchMetaApps(workspace, token)
        const appName = detectAppName(window.location.pathname, apps)
        bundle = await fetchMetaBundle(workspace, { appName, token })
      }
      set({ bundle, error: null })
    } catch {
      // Silently ignore refresh errors — keep the old bundle.
    }
  },

  reset: () => {
    set({ bundle: null, loading: false, error: null, forbidden: false })
  },

  getEntity: (module: string, name: string) => {
    const lookups = getOrBuildLookups(get().bundle)
    return lookups?.entitiesByKey.get(`${module}/${name}`)
  },

  getEntityByPlural: (module: string, plural: string) => {
    const lookups = getOrBuildLookups(get().bundle)
    return lookups?.entitiesByPlural.get(`${module}/${plural}`)
  },

  getPage: (name: string) => getOrBuildLookups(get().bundle)?.pages.get(name),
  getForm: (name: string) => getOrBuildLookups(get().bundle)?.forms.get(name),
  getTable: (name: string) => getOrBuildLookups(get().bundle)?.tables.get(name),
  getDashboard: (name: string) =>
    getOrBuildLookups(get().bundle)?.dashboards.get(name),
  getWidget: (name: string) =>
    getOrBuildLookups(get().bundle)?.widgets.get(name),
  getReport: (name: string) =>
    getOrBuildLookups(get().bundle)?.reports.get(name),
  getWizard: (name: string) =>
    getOrBuildLookups(get().bundle)?.wizards.get(name),
  getKanban: (name: string) =>
    getOrBuildLookups(get().bundle)?.kanbans.get(name),
  getTimeline: (name: string) =>
    getOrBuildLookups(get().bundle)?.timelines.get(name),
  getPrint: (name: string) => getOrBuildLookups(get().bundle)?.prints.get(name),
  getTheme: (name: string) => getOrBuildLookups(get().bundle)?.themes.get(name),
  getListing: (name: string) =>
    getOrBuildLookups(get().bundle)?.listings.get(name),

  getDerivedEntities: () => {
    const bundle = get().bundle
    if (!bundle) return []

    // An entity needs derivation if there's no authored page/form/table with its name
    const authoredNames = new Set<string>()
    for (const p of bundle.pages) authoredNames.add(p.name)
    for (const f of bundle.forms) authoredNames.add(f.name)
    for (const t of bundle.tables) authoredNames.add(t.name)

    return bundle.entities.filter((e) => !authoredNames.has(e.name))
  },

  getEntitiesByModule: () => {
    const bundle = get().bundle
    if (!bundle) return new Map()

    const map = new Map<string, EntitySchema[]>()
    for (const e of bundle.entities) {
      const list = map.get(e.module) ?? []
      list.push(e)
      map.set(e.module, list)
    }
    return map
  },
}))
