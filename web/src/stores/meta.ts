// ─── Meta Store ───
//
// Zustand store for the Meta bundle (entity schemas + authored UI manifests).
// Loaded once at boot via fetchMetaBundle(). Provides selectors for quick
// lookup by entity name, form name, table name, etc.

import { create } from "zustand"
import type { KyInstance } from "ky"

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
  type MenuSpec,
  type PrintSpec,
  type ThemeSpec,
} from "@/types/manifest"
import { fetchMetaBundle } from "@/lib/api"

export interface MetaState {
  bundle: MetaBundle | null
  loading: boolean
  error: string | null

  // ── Actions ──
  load: (client: KyInstance) => Promise<void>
  reset: () => void

  // ── Entity Lookups ──
  getEntity: (module: string, name: string) => EntitySchema | undefined
  getEntityByPlural: (module: string, plural: string) => EntitySchema | undefined

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
  getMenu: (name: string) => Entry<MenuSpec> | undefined
  getPrint: (name: string) => Entry<PrintSpec> | undefined
  getTheme: (name: string) => Entry<ThemeSpec> | undefined

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
  const menus = byName(bundle.menus)
  const prints = byName(bundle.prints)
  const themes = byName(bundle.themes)

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
    menus,
    prints,
    themes,
  }
}

// Build lookup maps lazily — they're derived from `bundle`.
function getOrBuildLookups(bundle: MetaBundle | null) {
  if (!bundle) return null
  // Cache on the bundle reference to avoid rebuilding on every selector call
  if ((bundle as any).__lookups) return (bundle as any).__lookups as ReturnType<typeof createLookups>
  const lookups = createLookups(bundle)
  ;(bundle as any).__lookups = lookups
  return lookups
}

export const useMetaStore = create<MetaState>((set, get) => ({
  bundle: null,
  loading: false,
  error: null,

  load: async (client: KyInstance) => {
    set({ loading: true, error: null })
    try {
      const bundle = await fetchMetaBundle(client)
      set({ bundle, loading: false, error: null })
    } catch (err) {
      const message = err instanceof Error ? err.message : "Failed to load meta bundle"
      set({ loading: false, error: message })
    }
  },

  reset: () => {
    set({ bundle: null, loading: false, error: null })
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
  getDashboard: (name: string) => getOrBuildLookups(get().bundle)?.dashboards.get(name),
  getWidget: (name: string) => getOrBuildLookups(get().bundle)?.widgets.get(name),
  getReport: (name: string) => getOrBuildLookups(get().bundle)?.reports.get(name),
  getWizard: (name: string) => getOrBuildLookups(get().bundle)?.wizards.get(name),
  getKanban: (name: string) => getOrBuildLookups(get().bundle)?.kanbans.get(name),
  getTimeline: (name: string) => getOrBuildLookups(get().bundle)?.timelines.get(name),
  getMenu: (name: string) => getOrBuildLookups(get().bundle)?.menus.get(name),
  getPrint: (name: string) => getOrBuildLookups(get().bundle)?.prints.get(name),
  getTheme: (name: string) => getOrBuildLookups(get().bundle)?.themes.get(name),

  getDerivedEntities: () => {
    const bundle = get().bundle
    if (!bundle) return []

    // An entity needs derivation if there's no authored page/form/table with its name
    const authoredNames = new Set<string>()
    for (const p of bundle.pages) authoredNames.add(p.name)
    for (const f of bundle.forms) authoredNames.add(f.name)
    for (const t of bundle.tables) authoredNames.add(t.name)

    return bundle.entities.filter(
      (e) => !authoredNames.has(e.name),
    )
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
