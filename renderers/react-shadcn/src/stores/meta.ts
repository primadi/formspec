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
  type Settings,
} from "@/types/manifest"
import { fetchMetaBundle, fetchMetaApps } from "@/lib/api"
import { notifySessionExpired } from "@/lib/api/sessionEvents"
import { useSessionStore } from "@/stores/session"
import { FormaApiError } from "@/types/manifest"

// Picks which resolved App (Core §4.4) the current URL belongs to: the
// longest root_url prefix match, after stripping the leading {workspace}
// path segment. A `root_url` of "/" is the WORKSPACE ROOT — the shortest
// possible mount, so it is the fallback rather than a prefix to exclude.
//
// Exported because the login screen needs the SAME resolution: the session must
// be scoped by the App's NAME (`kafe-pos`), which is what roles are declared
// against — not by the URL segment (`pos`), and the post-login redirect must
// land on the App's `root_url` rather than on `/{workspace}` (which would drop
// the `/app/pos` prefix and silently re-scope the session to whatever App owns
// the root).
export function detectApp(
  pathname: string,
  apps: AppSummary[],
): AppSummary | undefined {
  if (apps.length === 0) return undefined
  if (apps.length === 1) return apps[0]

  const segments = pathname.split("/").filter(Boolean)
  const rest = "/" + segments.slice(1).join("/") // drop {workspace}

  const normalized = (root: string) =>
    root === "" || root === "/" ? "/" : root.replace(/\/+$/, "")

  let best: AppSummary | undefined
  let bestLen = -1
  for (const a of apps) {
    const root = normalized(a.root_url)
    const matches =
      root === "/" ? true : rest === root || rest.startsWith(root + "/")
    if (matches && root.length > bestLen) {
      best = a
      bestLen = root.length
    }
  }
  // Nothing matched an App's mount: the path belongs to no App, so there is no
  // App to scope to. Returning `apps[0]` here is what made `/{ws}/menu` (the
  // PUBLIC `kafe-qr`) resolve to `kafe-kds`, whose root_url matched nothing —
  // the request was then scoped to the wrong App and rendered "Page not found"
  // inside another App's chrome. Only the doc's fallback case (the `_admin`
  // surface, which is not App-scoped at all) still wants an arbitrary App.
  return best
}

export function detectAppName(
  pathname: string,
  apps: AppSummary[],
): string | undefined {
  return detectApp(pathname, apps)?.name
}

export interface MetaState {
  bundle: MetaBundle | null
  // Which surface the loaded bundle belongs to ("admin" | "app"). The store
  // holds ONE bundle — navigating between surfaces must ignore (and reload)
  // a bundle fetched for the other surface, otherwise guards read stale data
  // (e.g. setup_required=true from the app bundle blocking the admin login
  // route → setup↔login redirect loop after first-run setup).
  loadedSurface: "admin" | "app" | null
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
  /** Get the resolved global settings namespace (spec §10). Never null once loaded. */
  getSettings: () => Settings | undefined
}

// createLookups builds the name→entry maps for every kind. Exported for tests
// (the widget map is module-qualified — gap #16 / item 7.3).
export function createLookups(bundle: MetaBundle) {
  const byName = <T>(items: Entry<T>[]) => {
    const map = new Map<string, Entry<T>>()
    for (const item of items) {
      map.set(item.name, item)
    }
    return map
  }

  // byQualified keys entries by BOTH the bare name and the module-qualified
  // form (`module/name` and `module.name`), so a reference like
  // `cafe-report/omzet-hari-ini` resolves to the right widget even when two
  // modules declare a widget with the same bare name (gap #16 / item 7.3).
  // The bare name is kept as a fallback for existing manifests.
  const byQualified = <T>(items: Entry<T>[]) => {
    const map = new Map<string, Entry<T>>()
    for (const item of items) {
      map.set(item.name, item)
      if (item.module) {
        map.set(`${item.module}/${item.name}`, item)
        map.set(`${item.module}.${item.name}`, item)
      }
    }
    return map
  }

  const entitiesByKey = new Map<string, EntitySchema>()
  const entitiesByPlural = new Map<string, EntitySchema>()
  // `?? []`: a bundle whose caller can see no entity used to arrive with
  // `entities: null`, and iterating it threw "e.entities is not iterable" —
  // killing the whole panel through the ErrorBoundary instead of rendering an
  // empty one. The server now always sends `[]`, but the renderer must not
  // depend on that to stay alive.
  for (const e of bundle.entities ?? []) {
    entitiesByKey.set(`${e.module}/${e.name}`, e)
    entitiesByPlural.set(`${e.module}/${e.plural}`, e)
  }

  const pages = byName(bundle.pages)
  const forms = byName(bundle.forms)
  const tables = byName(bundle.tables)
  const dashboards = byName(bundle.dashboards)
  const widgets = byQualified(bundle.widgets)
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
  loadedSurface: null,
  loading: false,
  error: null,
  forbidden: false,

  load: async (workspace: string, surface: "admin" | "app", token?: string) => {
    set({ loading: true, error: null, forbidden: false })
    try {
      // Live auth callbacks so a 401 during meta load can refresh the token.
      const getToken = () => useSessionStore.getState().token
      const onUnauthorized = () => useSessionStore.getState().refreshSession()
      // A refresh 409 is "pick a context", not "session expired" — without
      // this the auth hook would expire a still-valid session.
      const needsContext = () =>
        useSessionStore.getState().pendingContext !== null
      // `_admin` isn't scoped to any App (Core §4.4) — skip App detection
      // entirely and fetch the unscoped, binary-gated bundle.
      let bundle: MetaBundle
      if (surface === "admin") {
        bundle = await fetchMetaBundle(workspace, {
          admin: true,
          token,
          getToken,
          onUnauthorized,
          needsContext,
        })
      } else {
        const apps = await fetchMetaApps(workspace, token, {
          getToken,
          onUnauthorized,
        })
        const appName = detectAppName(window.location.pathname, apps)
        bundle = await fetchMetaBundle(workspace, {
          appName,
          token,
          getToken,
          onUnauthorized,
        })
      }
      set({
        bundle,
        loading: false,
        error: null,
        forbidden: false,
        loadedSurface: surface,
      })
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
        // Drop any previously loaded bundle — a failed load must not leave a
        // wrong-surface bundle behind for the guards to misread.
        set({
          loading: false,
          error: null,
          forbidden: true,
          bundle: null,
          loadedSurface: null,
        })
        return
      }
      if (status === 401) {
        // Invalid / expired token — mark the session unauthenticated so the
        // auth guard redirects to login instead of showing a connection
        // error. Keep the meta state clear so the loading gate exits.
        notifySessionExpired()
        set({
          loading: false,
          error: null,
          forbidden: false,
          bundle: null,
          loadedSurface: null,
        })
        return
      }
      const message =
        err instanceof Error ? err.message : "Failed to load meta bundle"
      set({
        loading: false,
        error: message,
        bundle: null,
        loadedSurface: null,
      })
    }
  },

  refresh: async (
    workspace: string,
    surface: "admin" | "app",
    token?: string,
  ) => {
    try {
      const getToken = () => useSessionStore.getState().token
      const onUnauthorized = () => useSessionStore.getState().refreshSession()
      const needsContext = () =>
        useSessionStore.getState().pendingContext !== null
      let bundle: MetaBundle
      if (surface === "admin") {
        bundle = await fetchMetaBundle(workspace, {
          admin: true,
          token,
          getToken,
          onUnauthorized,
          needsContext,
        })
      } else {
        const apps = await fetchMetaApps(workspace, token, {
          getToken,
          onUnauthorized,
          needsContext,
        })
        const appName = detectAppName(window.location.pathname, apps)
        bundle = await fetchMetaBundle(workspace, {
          appName,
          token,
          getToken,
          onUnauthorized,
          needsContext,
        })
      }
      set({ bundle, error: null, loadedSurface: surface })
    } catch (err) {
      // A 401 means the token expired — expire the session (login redirect).
      // Other refresh errors are silently ignored — keep the old bundle.
      const status =
        err instanceof FormaApiError
          ? err.status
          : err instanceof HTTPError
            ? err.response.status
            : undefined
      if (status === 401) {
        notifySessionExpired()
      }
    }
  },

  reset: () => {
    set({
      bundle: null,
      loadedSurface: null,
      loading: false,
      error: null,
      forbidden: false,
    })
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

  getSettings: () => get().bundle?.settings,

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
