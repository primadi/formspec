// ─── Built-in Auth Assets ───
//
// Maps the framework's default auth page asset refs (module formspec.core,
// plan auth-screens-spec-driven) to their built-in shell renderers. The
// default auth pages are `kind: Page` specs with `mode: custom` +
// `asset: formspec-core/auth/<slot>`; PageRenderer's CustomPage resolves
// these refs here instead of loading an external ES module. An App override
// (App.spec.auth.<slot> → its own Page) renders through the normal
// PageRenderer path — these built-ins are only the framework defaults.

import type { ComponentType } from "react"
import { LoginPage } from "./LoginPage"
import { SetupScreen } from "./SetupScreen"
import { ChangePasswordPage } from "./ChangePasswordPage"
import { ResetPasswordScreen } from "./ResetPasswordScreen"
import { OAuthCallback } from "./OAuthCallback"

export const BUILTIN_AUTH_ASSETS: Record<string, ComponentType> = {
  "formspec-core/auth/login": LoginPage,
  "formspec-core/auth/setup": SetupScreen,
  "formspec-core/auth/change-password": ChangePasswordPage,
  "formspec-core/auth/reset-password": ResetPasswordScreen,
  "formspec-core/auth/oauth-callback": OAuthCallback,
}

/** True when an asset ref is a framework built-in (not an external module). */
export function isBuiltinAuthAsset(asset: string): boolean {
  return asset in BUILTIN_AUTH_ASSETS
}
