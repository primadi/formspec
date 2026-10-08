// ─── View targets (`<kind>:<name>`) ───
//
// A TableAction can carry `view` — a NAVIGATION to a view resource instead of a
// call to the entity action. This is how renderer builtins such as `print` (and
// `export`) get a UI trigger: they have no backing entity action
// (`internal/ui/validate.go` builtinRowActions lists them), so
// `canDoEntityAction` would reject them and the button would never render.
//
// The vocabulary is the same `{kind}:{name}` used by MenuItem.view /
// registered_views / grants, and the kinds mirror the client's buildRoutes
// (`shell/router.tsx`) — a target that resolves here has a registered route, so
// a click never lands on the SPA's 404.
//
// Plan: docs_internal/plan/print-row-action.md

import type { EntitySchema, Entry, MetaBundle } from "@/types/manifest"
import { canDoEntityAction } from "@/engine/permissions"
import { resolveEntityRef } from "@/engine/entityRef"

/** The shape every view kind's spec shares for our purposes. */
interface ViewSpecLike {
  entity?: string
}

export interface ViewTarget {
  kind: string
  name: string
  module: string
  /** The entity the view reads from, when it names one. Undefined for kinds
   *  with no single entity (dashboard, page, approval-inbox). */
  entityRef?: string
}

type AnyEntry = Entry<ViewSpecLike>

const asEntries = (x: unknown): AnyEntry[] | undefined =>
  x as AnyEntry[] | undefined

/** Parse a `<kind>:<name>` reference; undefined when malformed. */
export function parseViewTarget(
  view: string | undefined,
): { kind: string; name: string } | undefined {
  if (!view) return undefined
  const i = view.indexOf(":")
  if (i <= 0 || i === view.length - 1) return undefined
  return { kind: view.slice(0, i), name: view.slice(i + 1) }
}

/** The bundle collection that holds a kind's entries. */
function collectionFor(
  bundle: MetaBundle,
  kind: string,
): AnyEntry[] | undefined {
  switch (kind) {
    case "page":
      return asEntries(bundle.pages)
    case "dashboard":
      return asEntries(bundle.dashboards)
    case "widget":
      return asEntries(bundle.widgets)
    case "report":
      return asEntries(bundle.reports)
    case "wizard":
      return asEntries(bundle.wizards)
    case "kanban":
      return asEntries(bundle.kanbans)
    case "timeline":
      return asEntries(bundle.timelines)
    case "calendar":
      return asEntries(bundle.calendars)
    case "listing":
      return asEntries(bundle.listings)
    case "print":
      return asEntries(bundle.prints)
    case "form":
      return asEntries(bundle.forms)
    case "table":
      return asEntries(bundle.tables)
    case "approval-inbox":
      return asEntries(bundle.approval_inboxes)
    case "notification-center":
      return asEntries(bundle.notification_centers)
    default:
      return undefined
  }
}

// Kinds whose route has an `:id` suffix (`shell/router.tsx`): the record id is
// appended so the action opens the document for THAT record. Only `print`
// registers `.../print/{name}/:id`.
const VIEW_KINDS_WITH_ID = new Set(["print"])

export function viewTargetTakesId(kind: string): boolean {
  return VIEW_KINDS_WITH_ID.has(kind)
}

/** Resolve a `<kind>:<name>` reference against the loaded bundle. */
export function resolveViewTarget(
  bundle: MetaBundle | null | undefined,
  view: string | undefined,
): ViewTarget | undefined {
  const parsed = parseViewTarget(view)
  if (!parsed || !bundle) return undefined
  const entries = collectionFor(bundle, parsed.kind)
  if (!entries) return undefined
  const entry = entries.find(
    (e) => e.name === parsed.name || `${e.module}/${e.name}` === parsed.name,
  )
  if (!entry) return undefined
  return {
    kind: parsed.kind,
    name: entry.name,
    module: entry.module,
    entityRef: entry.spec?.entity,
  }
}

/**
 * Whether the caller may use a view-target action. UX-only — the server
 * enforces authoritatively. A kind that names an entity is gated by that
 * entity's `view` (the same check the "Lihat" button uses); a kind with no
 * single entity is allowed through, and the server decides.
 *
 * A reference that does NOT resolve returns false: a button that navigates
 * nowhere is worse than an absent one.
 */
export function canDoViewTarget(
  me: { permissions: string[] } | null | undefined,
  bundle: MetaBundle | null | undefined,
  view: string | undefined,
): boolean {
  const target = resolveViewTarget(bundle, view)
  if (!target) return false
  if (!target.entityRef) return true
  const [mod, name] = resolveEntityRef(target.entityRef, target.module)
  const entity = bundle?.entities?.find(
    (e) => e.module === mod && e.name === name,
  )
  if (!entity) return true // unknown entity — let the server decide
  return canDoEntityAction(me, entity, "view")
}

/**
 * Gate a TableAction: a `view` action is gated by its target, everything else
 * by the entity action of the same name. Used both to decide whether the
 * button renders and to guard the click, so the two can never disagree.
 */
export function canDoTableAction(
  me: { permissions: string[] } | null | undefined,
  bundle: MetaBundle | null | undefined,
  entity: EntitySchema,
  action: { action: string; view?: string },
): boolean {
  if (action.view) return canDoViewTarget(me, bundle, action.view)
  return canDoEntityAction(me, entity, action.action)
}
