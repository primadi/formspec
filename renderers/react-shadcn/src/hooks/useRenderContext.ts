// ─── useRenderContext — resolve `spec.context` declarations ───
//
// Render-context standard (docs_internal/plan/render-context-standard.md,
// Phase 2). Resolves a Page/Form's `context:` declarations into a flat
// context object merged over the standard slots (`user`, `route`, `fields`).
//
// Sources (closed set):
//   - session → the caller's identity (same as the `user` standard slot)
//   - const   → a literal value
//   - expr    → a FormSpecExpr evaluated against the current context
//   - entity  → a record fetched by id (permission-gated: module.entity.view)
//   - api     → a Service action invoked on the UI surface
//               (/{ws}/_ui/service/{module}/{service}/{action}),
//               permission-gated: module.service.action
//
// Async entries resolve in parallel; the hook reports `loading` until all
// settle, and per-entry `error` (fallback is used on error).

import { useEffect, useMemo, useState } from "react"
import { apiGet } from "@/lib/api"
import { evalFormSpecExpr } from "@/lib/formspec-expr"
import { subscribeRealtime } from "@/hooks/useRealtime"
import { useSessionStore } from "@/stores/session"
import type { ContextDecl } from "@/types/manifest"

export interface RenderContextState {
  /** Merged context: standard slots + resolved declarations. */
  context: Record<string, unknown>
  /** True while any async declaration is still resolving. */
  loading: boolean
  /** First error encountered (declarations fall back on error). */
  error?: string
}

/** Resolve a single declaration against the current context. */
async function resolveDecl(
  decl: ContextDecl,
  ctx: Record<string, unknown>,
  getClient: () => import("ky").KyInstance,
): Promise<unknown> {
  switch (decl.source) {
    case "session":
      return ctx.user
    case "const":
      return decl.value
    case "expr": {
      if (!decl.expr) return decl.fallback
      try {
        return evalFormSpecExpr(decl.expr, ctx as never)
      } catch {
        return decl.fallback
      }
    }
    case "entity": {
      if (!decl.entity) return decl.fallback
      // Permission ceiling: entity context requires view on the entity.
      const can = useSessionStore.getState().can
      if (!can(`${decl.entity}.view`)) return decl.fallback
      // Resolve `{token}` in the id against the current context.
      let id = decl.id ?? ""
      const tokenMatch = id.match(/^\{([\w.]+)\}$/)
      if (tokenMatch) {
        const resolved = tokenMatch[1].split(".").reduce<unknown>((acc, k) => {
          if (acc == null || typeof acc !== "object") return undefined
          return (acc as Record<string, unknown>)[k]
        }, ctx)
        id = resolved == null ? "" : String(resolved)
      }
      if (!id) return decl.fallback
      const [module, entity] = decl.entity.split(".")
      if (!module || !entity) return decl.fallback
      try {
        const client = getClient()
        return await apiGet<Record<string, unknown>>(
          client,
          `${module}/${entity}/${id}`,
        )
      } catch {
        return decl.fallback
      }
    }
    case "api": {
      if (!decl.call) return decl.fallback
      // Permission ceiling: api context requires the service action grant.
      const can = useSessionStore.getState().can
      if (!can(decl.call)) return decl.fallback
      const [module, service, action] = decl.call.split(".")
      if (!module || !service || !action) return decl.fallback
      try {
        const client = getClient()
        // Client prefix is /{ws}/_ui/entity — "../service/..." normalizes
        // to /{ws}/_ui/service/{module}/{service}/{action}.
        const response = await client.post(
          `../service/${module}/${service}/${action}`,
          { json: decl.params ?? {} },
        )
        const body = (await response.json()) as { data?: unknown }
        return body.data ?? body
      } catch {
        return decl.fallback
      }
    }
    default:
      return decl.fallback
  }
}

export function useRenderContext(
  decls: ContextDecl[] | undefined,
  base: Record<string, unknown>,
): RenderContextState {
  const getClient = useSessionStore((s) => s.getClient)
  const [extra, setExtra] = useState<Record<string, unknown>>({})
  const [loading, setLoading] = useState(false)
  const [error, setError] = useState<string | undefined>(undefined)

  const declKey = useMemo(
    () => (decls ?? []).map((d) => `${d.name}:${d.source}`).join("|"),
    [decls],
  )

  useEffect(() => {
    if (!decls?.length) {
      setExtra({})
      setLoading(false)
      setError(undefined)
      return
    }
    let cancelled = false
    setLoading(true)
    setError(undefined)

    // Sequential resolution so `expr` entries can reference earlier ones.
    const run = async () => {
      const out: Record<string, unknown> = {}
      for (const decl of decls) {
        const value = await resolveDecl(decl, { ...base, ...out }, getClient)
        if (cancelled) return
        out[decl.name] = value
      }
      if (!cancelled) {
        setExtra(out)
        setLoading(false)
      }
    }
    run().catch((e) => {
      if (cancelled) return
      setError(e instanceof Error ? e.message : String(e))
      setLoading(false)
    })

    // Phase 3 — reactivity: `source: entity` + `realtime: true` subscribes
    // to the entity's events and re-resolves that entry on change (and on
    // reconnect — realtime is non-durable, consumers must refetch).
    const unsubs = (decls ?? [])
      .filter((d) => d.source === "entity" && d.realtime && d.entity)
      .map((d) =>
        subscribeRealtime(d.entity!, () => {
          if (cancelled) return
          const refresh = async () => {
            const value = await resolveDecl(d, { ...base, ...extra }, getClient)
            if (cancelled) return
            setExtra((prev) => ({ ...prev, [d.name]: value }))
          }
          refresh().catch(() => {
            /* keep last value on transient failure */
          })
        }),
      )

    return () => {
      cancelled = true
      unsubs.forEach((u) => u())
    }
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [declKey, getClient])

  return { context: { ...base, ...extra }, loading, error }
}
