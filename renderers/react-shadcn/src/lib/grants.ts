// ─── Role grant serialization ───
//
// The admin-facing grant tree is a flat set of checkboxes (`page::tab::action`)
// plus, per action, an optional ROW SCOPE. Keeping the conversion in one pure
// module is deliberate: the shape written here is what the backend
// `Materializer` reads, and it is NOT schema-validated — `grants` is free JSON on
// the role entity. A key spelled wrong (or nested one level off) produces a role
// that looks configured and enforces nothing, which is the kafe 10.47/10.10
// failure class. So the mapping gets a test instead of living inside a component.
//
// Backend counterpart: internal/auth/grant.go (Grant / ActionGrant.RowScope).

import type { FilterSpec } from "@/types/manifest"

export interface GrantAction {
  name: string
  /**
   * Restricts which ROWS the granted action applies to. Enforced server-side by
   * the storage layer, together with the entity's own `row_scope` — a caller
   * cannot widen or drop it (kafe 10.67: only paid orders reach the kitchen).
   */
  row_scope?: FilterSpec[]
}

export interface GrantTab {
  tab: string
  actions: GrantAction[]
}

export interface Grant {
  page: string
  actions?: GrantAction[]
  tabs?: GrantTab[]
}

/** One checkbox key: `page` / `page::action` / `page::tab::action`. */
export type GrantKey = string

export function grantKey(page: string, tab?: string, action?: string): GrantKey {
  if (tab && action) return `${page}::${tab}::${action}`
  if (action) return `${page}::${action}`
  return page
}

/** The selection a Role's `grants` describes: checked keys + their row scopes. */
export interface GrantSelection {
  keys: Set<GrantKey>
  /** key → row scope. Absent (or empty) means "every row the permission reaches". */
  scopes: Record<GrantKey, FilterSpec[]>
}

/**
 * The only predicate shape the editor produces: a field, an operator, and one
 * value source. `from` empty means the literal `value`.
 */
export interface ScopePredicate {
  field: string
  op: string
  from?: "session" | "route"
  value?: string
  attr?: string
  param?: string
}

/** Operators a row scope may use. Mirrors the backend list (a superset of the
 *  useful ones — `in`/`nin` are what a status list needs). */
export const SCOPE_OPERATORS = [
  "eq",
  "neq",
  "in",
  "nin",
  "gt",
  "gte",
  "lt",
  "lte",
  "like",
  "notnull",
  "null",
] as const

/**
 * Converts a raw `grants` value into the editor's selection. Tolerates the JSON
 * round-trip shapes and drops nothing silently: an unknown key is kept so the
 * operator sees it rather than losing a grant by opening the form.
 */
export function grantsToSelection(grants: Grant[] | undefined): GrantSelection {
  const keys = new Set<GrantKey>()
  const scopes: Record<GrantKey, FilterSpec[]> = {}
  const record = (k: GrantKey, a: GrantAction) => {
    keys.add(k)
    if (a.row_scope && a.row_scope.length) scopes[k] = a.row_scope
  }
  for (const g of grants ?? []) {
    if (g.tabs?.length) {
      for (const t of g.tabs) {
        for (const a of t.actions) record(grantKey(g.page, t.tab, a.name), a)
      }
    } else {
      for (const a of g.actions ?? []) record(grantKey(g.page, undefined, a.name), a)
    }
  }
  return { keys, scopes }
}

/**
 * Drops predicates that would be inert. A row scope that names no field, or
 * declares neither a value nor a source, is dropped rather than written: the
 * backend would refuse it (fail-closed 403 forever), and a role that cannot be
 * used is not better than a role with one fewer line. `value` may legitimately
 * be "0", so emptiness is checked on the string, not on truthiness.
 */
export function prunePredicates(preds: ScopePredicate[]): FilterSpec[] {
  const out: FilterSpec[] = []
  for (const p of preds) {
    const field = p.field.trim()
    if (!field) continue
    const spec: FilterSpec = { field, op: p.op || "eq" }
    if (p.from === "session") {
      const attr = (p.attr ?? "").trim()
      if (attr) spec.attr = attr
      spec.from = "session"
    } else if (p.from === "route") {
      const param = (p.param ?? "").trim()
      if (param) spec.param = param
      spec.from = "route"
    } else {
      const value = (p.value ?? "").trim()
      // A literal-less predicate is the shape that silently filters nothing, so
      // it never reaches the manifest.
      if (!value) continue
      spec.value = value
    }
    out.push(spec)
  }
  return out
}

/**
 * Builds the `grants` value from a selection. Actions are emitted in the page's
 * declared order so the YAML diff stays stable, and an action with no usable
 * predicate gets NO `row_scope` key at all (rather than an empty array, which
 * would suggest a restriction that is not there).
 */
export function selectionToGrants(
  pages: { page: string; actions: { name: string }[]; tabs: { label: string; actions: { name: string }[] }[] }[],
  selection: GrantSelection,
  predicates?: Record<GrantKey, ScopePredicate[]>,
): Grant[] {
  const scopeFor = (k: GrantKey): FilterSpec[] | undefined => {
    // When the caller supplies predicate rows for this key, they are the
    // authority — an EMPTY list means "remove the restriction", not "nothing
    // was provided". Falling back to the stored scope here would make a
    // restriction impossible to delete in the editor.
    if (predicates && k in predicates) {
      const pruned = prunePredicates(predicates[k] ?? [])
      return pruned.length ? pruned : undefined
    }
    const stored = selection.scopes[k]
    return stored && stored.length ? stored : undefined
  }

  const grants: Grant[] = []
  for (const m of pages) {
    if (m.tabs.length) {
      const tabs: GrantTab[] = []
      for (const t of m.tabs) {
        const actions: GrantAction[] = []
        for (const a of t.actions) {
          const k = grantKey(m.page, t.label, a.name)
          if (!selection.keys.has(k)) continue
          const scope = scopeFor(k)
          actions.push(scope ? { name: a.name, row_scope: scope } : { name: a.name })
        }
        if (actions.length) tabs.push({ tab: t.label, actions })
      }
      if (tabs.length) grants.push({ page: m.page, tabs })
      continue
    }
    const actions: GrantAction[] = []
    for (const a of m.actions) {
      const k = grantKey(m.page, undefined, a.name)
      if (!selection.keys.has(k)) continue
      const scope = scopeFor(k)
      actions.push(scope ? { name: a.name, row_scope: scope } : { name: a.name })
    }
    if (actions.length) grants.push({ page: m.page, actions })
  }
  return grants
}

/** The single string a predicate is shown as, so the row is readable at a glance. */
export function describePredicate(spec: FilterSpec): string {
  const where = spec.from === "session" ? `session:${spec.attr ?? "?"}` : spec.from === "route" ? `route:${spec.param ?? "?"}` : `"${spec.value ?? ""}"`
  return `${spec.field} ${spec.op ?? "eq"} ${where}`
}
