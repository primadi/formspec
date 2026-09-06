// ─── Auth Page ───
//
// Renders one auth screen (login/setup/change-password/reset-password/oauth-
// callback) from the resolved auth page ref (plan auth-screens-spec-driven).
// The ref comes from bundle.app.auth.<slot> — the App's override
// (App.spec.auth) or the framework default (formspec.core/<slot>).
//
// - Framework default (or bundle not loaded yet) → the built-in shell
//   component for the slot (LoginPage/SetupScreen/...), which owns its
//   full-page layout and accepts slot-specific props (e.g. login mode).
// - Override page → the normal PageRenderer, which renders the App's own
//   Page kind (blocks/tabs/custom asset).

import { useMetaStore } from "@/stores/meta"
import PageRenderer from "@/kinds/page/PageRenderer"
import { BUILTIN_AUTH_ASSETS } from "./authAssets"
import { LoginPage } from "./LoginPage"

/** The auth page slots (chrome_auth is a component ref, handled separately). */
export type AuthSlot =
  | "login_page"
  | "setup_page"
  | "change_password_page"
  | "reset_password_page"
  | "oauth_callback_page"

/** Framework default auth page refs (module formspec.core). */
export const DEFAULT_AUTH_REFS: Record<AuthSlot, string> = {
  login_page: "formspec.core/login",
  setup_page: "formspec.core/setup",
  change_password_page: "formspec.core/change-password",
  reset_password_page: "formspec.core/reset-password",
  oauth_callback_page: "formspec.core/oauth-callback",
}

export function AuthPage({
  slot,
  mode,
}: {
  slot: AuthSlot
  /** Login mode — only meaningful for the login_page slot. */
  mode?: "login" | "register"
}) {
  const bundle = useMetaStore((s) => s.bundle)
  const ref = bundle?.app.auth?.[slot] ?? DEFAULT_AUTH_REFS[slot]
  const page = bundle?.pages?.find((p) => `${p.module}/${p.name}` === ref)

  // Framework default (or bundle not loaded yet) → built-in component.
  if (ref === DEFAULT_AUTH_REFS[slot] || !page) {
    const Builtin = BUILTIN_AUTH_ASSETS[DEFAULT_AUTH_REFS[slot]]
    if (!Builtin) return null
    if (slot === "login_page") return <LoginPage mode={mode} />
    return <Builtin />
  }

  // Override page → normal PageRenderer.
  return <PageRenderer entry={page} />
}
