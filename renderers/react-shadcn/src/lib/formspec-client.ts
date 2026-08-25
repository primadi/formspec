// ─── formspec client — injected into asset components (todo 5.9.2) ───
//
// The `formspec` object passed to `mount(el, props, formspec)` of custom
// asset components (07-component-kinds.md §4). Provides typed API access,
// realtime subscribe, navigation, theme tokens, ui chrome, and the base
// widget registry.

import type { KyInstance } from "ky"
import type { AssetNeeds, EntitySchema } from "@/types/manifest"
import { useSessionStore } from "@/stores/session"
import { ui } from "@/lib/ui"
import { files } from "@/lib/files"
import {
  createHeadlessForm,
  type HeadlessForm,
  type HeadlessFormOptions,
} from "@/lib/headless-form"
import { subscribeRealtime } from "@/hooks/useRealtime"
import * as components from "@/widgets"

export interface FormspecClient {
  api: KyInstance
  subscribe: (entity: string, cb: (msg: unknown) => void) => () => void
  navigate: (page: string, params?: Record<string, string>) => void
  theme: Record<string, string>
  ui: typeof ui
  files: typeof files
  form: (entity: EntitySchema, opts: HeadlessFormOptions) => HeadlessForm
  components: typeof components
}

const THEME_TOKENS = [
  "--primary",
  "--primary-foreground",
  "--background",
  "--foreground",
  "--muted",
  "--muted-foreground",
  "--accent",
  "--accent-foreground",
  "--border",
  "--input",
  "--ring",
  "--radius",
  "--destructive",
] as const

function readThemeTokens(): Record<string, string> {
  const root = getComputedStyle(document.documentElement)
  const out: Record<string, string> = {}
  for (const t of THEME_TOKENS) out[t] = root.getPropertyValue(t).trim()
  return out
}

// ── needs enforcement (todo 5.9.6) ──
//
// `formspec.api` calls outside the asset's declared `needs` fail client-side
// (07-component-kinds.md §4). The ky client is wrapped with a beforeRequest
// hook that parses module/entity from the URL and checks it against
// needs.actions / needs.subscribe.

function isAllowed(module: string, entity: string, needs: AssetNeeds): boolean {
  const key = `${module}.${entity}`
  for (const a of needs.actions ?? []) {
    if (a === key || a === `${key}.*` || a.startsWith(`${key}.`)) return true
  }
  for (const s of needs.subscribe ?? []) {
    if (s === key) return true
  }
  return false
}

function withNeeds(api: KyInstance, needs?: AssetNeeds): KyInstance {
  if (!needs) return api
  return api.extend({
    hooks: {
      beforeRequest: [
        (request) => {
          const url = new URL(request.url)
          const parts = url.pathname.split("/").filter(Boolean)
          const idx = parts.indexOf("entity")
          if (idx === -1 || idx + 2 >= parts.length) return
          const module = parts[idx + 1]
          const entity = parts[idx + 2]
          if (!isAllowed(module, entity, needs)) {
            throw new Error(
              `formspec.api: access to ${module}.${entity} not declared in needs`,
            )
          }
        },
      ],
    },
  })
}

export function createFormspecClient(opts: {
  navigate: (page: string, params?: Record<string, string>) => void
  needs?: AssetNeeds
}): FormspecClient {
  return {
    api: withNeeds(useSessionStore.getState().getClient(), opts.needs),
    subscribe: (entity, cb) => subscribeRealtime(entity, cb),
    navigate: opts.navigate,
    theme: readThemeTokens(),
    ui,
    files,
    form: (entity, formOpts) => createHeadlessForm(entity, formOpts),
    components,
  }
}
