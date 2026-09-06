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
import { loginWithPassword, type LoginResult } from "@/lib/api/auth"
import {
  createHeadlessForm,
  type HeadlessForm,
  type HeadlessFormOptions,
} from "@/lib/headless-form"
import { subscribeRealtime } from "@/hooks/useRealtime"
import * as components from "@/widgets"

/**
 * Auth actions available to custom auth pages (plan auth-screens-spec-driven).
 * A custom login/setup/change-password page (App.spec.auth.<slot> override)
 * calls these instead of the built-in shell screens. login() boots the
 * session; logout() clears it; the rest call the /_ui/auth/* endpoints.
 */
export interface FormspecAuth {
  login: (
    username: string,
    password: string,
    opts?: { app?: string },
  ) => Promise<LoginResult>
  register: (
    username: string,
    password: string,
    opts?: { email?: string; displayName?: string },
  ) => Promise<void>
  logout: () => void
  changePassword: (
    currentPassword: string,
    newPassword: string,
  ) => Promise<void>
  resetPassword: (token: string, newPassword: string) => Promise<void>
  forgotPassword: (email: string) => Promise<void>
}

export interface FormspecClient {
  api: KyInstance
  subscribe: (entity: string, cb: (msg: unknown) => void) => () => void
  navigate: (page: string, params?: Record<string, string>) => void
  theme: Record<string, string>
  ui: typeof ui
  files: typeof files
  form: (entity: EntitySchema, opts: HeadlessFormOptions) => HeadlessForm
  components: typeof components
  auth: FormspecAuth
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
        (state) => {
          const url = new URL(state.request.url)
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
  workspace?: string
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
    auth: createAuth(opts.workspace ?? "default"),
  }
}

// ── auth actions (plan auth-screens-spec-driven) ──
//
// Backed by the same /_ui/auth/* endpoints the built-in shell screens call.
// login() boots the session store (like LoginPage); logout() clears it.
// Exported for AuthFormRenderer (plan custom-screens-spec-driven Phase 2) —
// the declarative `auth_action` form dispatches to these same actions.

export function createAuth(workspace: string): FormspecAuth {
  const post = async (path: string, body: unknown, token?: string) => {
    const res = await fetch(`/${workspace}${path}`, {
      method: "POST",
      headers: {
        "Content-Type": "application/json",
        ...(token ? { Authorization: `Bearer ${token}` } : {}),
      },
      body: JSON.stringify(body),
    })
    if (!res.ok) {
      const parsed = (await res.json().catch(() => null)) as {
        error?: { message?: string }
      } | null
      throw new Error(
        parsed?.error?.message ?? `Request failed (${res.status})`,
      )
    }
    return res
  }

  return {
    async login(username, password, opts) {
      const { accessToken, refreshToken } = await loginWithPassword(
        workspace,
        username,
        password,
        opts?.app,
      )
      await useSessionStore
        .getState()
        .boot(workspace, accessToken, refreshToken, opts?.app)
      return { accessToken, refreshToken }
    },
    async register(username, password, opts) {
      await post("/_ui/auth/register", {
        username,
        email: opts?.email || undefined,
        password,
        display_name: opts?.displayName || username,
      })
    },
    logout() {
      useSessionStore.getState().clearSession()
    },
    async changePassword(currentPassword, newPassword) {
      const token = useSessionStore.getState().token
      await post(
        "/_ui/auth/change-password",
        { current_password: currentPassword, new_password: newPassword },
        token ?? undefined,
      )
    },
    async resetPassword(token, newPassword) {
      await post("/_ui/auth/reset-password", { token, password: newPassword })
    },
    async forgotPassword(email) {
      // Always 200 (no address-existence leak) — the response does not
      // distinguish known/unknown emails by design.
      await post("/_ui/auth/forgot-password", { email })
    },
  }
}
