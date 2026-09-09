// ─── Auth Form Renderer ───
//
// Renders a `kind: Form` whose spec declares `auth_action`
// (plan custom-screens-spec-driven Phase 2) — a pure-YAML custom auth form
// bound to the platform auth endpoints (/_ui/auth/*) via FormspecAuth.
//
// No entity: fields come from `sections` and map conventionally to the auth
// payload — username, password, current_password, new_password, email,
// display_name, token. Success/error codes from the backend surface as
// inline form messages. Login success boots the session (FormspecAuth.login)
// then redirects to `?returnTo` (same-origin guard, plan Phase 1 route slot).

import { useMemo, useState, type FormEvent } from "react"
import { useAppNavigate } from "@/lib/navigation"
import { useParams, useSearchParams } from "react-router-dom"
import { Loader2 } from "lucide-react"
import { Button } from "@/components/ui/button"
import { TextInput } from "@/widgets/TextInput"
import { PasswordInput } from "@/widgets/PasswordInput"
import { createAuth } from "@/lib/formspec-client"
import { useMetaStore } from "@/stores/meta"
import type { FormField, FormSpec } from "@/types/manifest"

/** Per-action field names that are NOT required. */
const OPTIONAL_FIELDS: Record<string, string[]> = {
  register: ["email", "display_name"],
  reset_password: ["token"], // falls back to ?reset_token (query, Phase 1 slot)
}

const DEFAULT_LABELS: Record<string, string> = {
  login: "Sign in",
  register: "Create account",
  change_password: "Change password",
  forgot_password: "Send reset link",
  reset_password: "Reset password",
}

const DEFAULT_SUCCESS: Record<string, string> = {
  change_password: "Password changed successfully",
  forgot_password: "If the email exists, a reset link has been sent",
  reset_password: "Password reset — you can sign in now",
}

function fieldWidget(field: FormField) {
  if (field.widget === "password" || field.name.includes("password")) {
    return "password" as const
  }
  return "text" as const
}

export default function AuthFormRenderer({ spec }: { spec: FormSpec }) {
  const { workspace = "default" } = useParams<{ workspace: string }>()
  const navigate = useAppNavigate()
  const [searchParams] = useSearchParams()
  const auth = useMemo(() => createAuth(workspace), [workspace])

  const required = useMemo(() => {
    const optional = new Set(OPTIONAL_FIELDS[spec.auth_action ?? ""] ?? [])
    return spec.sections
      .flatMap((s) => s.fields)
      .filter((f) => !optional.has(f.name))
  }, [spec])

  const [values, setValues] = useState<Record<string, string>>({})
  const [fieldErrors, setFieldErrors] = useState<Record<string, string>>({})
  const [error, setError] = useState<string | null>(null)
  const [success, setSuccess] = useState<string | null>(null)
  const [submitting, setSubmitting] = useState(false)

  const set = (name: string, value: string) =>
    setValues((prev) => ({ ...prev, [name]: value }))

  const finish = () => {
    setSuccess(spec.submit?.message ?? DEFAULT_SUCCESS[spec.auth_action ?? ""])
  }

  const redirectAfterLogin = () => {
    // Bundle may have been loaded anonymously while on the auth route —
    // reset so it reloads with the authenticated identity's permissions
    // (same as LoginPage).
    useMetaStore.getState().reset()
    const returnTo = searchParams.get("returnTo")
    // Same-origin guard: only accept a path starting with "/" that is not
    // "//" (protocol-relative) and not a bare "/" (which would loop).
    navigate(
      returnTo &&
        returnTo.startsWith("/") &&
        !returnTo.startsWith("//") &&
        returnTo !== "/"
        ? returnTo
        : `/${workspace}`,
      { replace: true },
    )
  }

  const onSubmit = async (e: FormEvent) => {
    e.preventDefault()
    setError(null)
    setSuccess(null)
    const errs: Record<string, string> = {}
    for (const f of required) {
      if (!values[f.name]?.trim()) {
        errs[f.name] = `${f.label ?? f.name} is required`
      }
    }
    if (Object.keys(errs).length > 0) {
      setFieldErrors(errs)
      return
    }
    setFieldErrors({})
    setSubmitting(true)
    try {
      switch (spec.auth_action) {
        case "login": {
          await auth.login(values.username.trim(), values.password)
          redirectAfterLogin()
          return
        }
        case "register": {
          await auth.register(values.username.trim(), values.password, {
            email: values.email?.trim(),
            displayName: values.display_name?.trim(),
          })
          // Self-service flow: auto-login with the just-registered
          // credentials (mirrors LoginScreen's register behavior).
          await auth.login(values.username.trim(), values.password)
          redirectAfterLogin()
          return
        }
        case "change_password": {
          await auth.changePassword(
            values.current_password,
            values.new_password,
          )
          break
        }
        case "forgot_password": {
          await auth.forgotPassword(values.email.trim())
          break
        }
        case "reset_password": {
          // Token from the form field, else from the query string
          // (?reset_token — the auth middleware's convention).
          const token =
            values.token ??
            searchParams.get("reset_token") ??
            searchParams.get("token") ??
            ""
          await auth.resetPassword(token, values.password)
          break
        }
        default:
          setError(`Unknown auth_action: ${spec.auth_action}`)
          return
      }
      finish()
    } catch (err) {
      setError(err instanceof Error ? err.message : "Authentication failed")
    } finally {
      setSubmitting(false)
    }
  }

  return (
    <form onSubmit={onSubmit} autoComplete="off" className="space-y-6">
      {spec.sections
        .filter(
          (section) => !section.visible_when || section.visible_when === "true",
        )
        .map((section, sIdx) => (
          <div key={sIdx} className="space-y-4">
            {section.title && (
              <h3 className="text-lg font-medium">{section.title}</h3>
            )}
            {section.description && (
              <p className="text-sm text-muted-foreground">
                {section.description}
              </p>
            )}
            <div
              className="grid gap-4"
              style={{
                gridTemplateColumns: `repeat(${section.columns || 1}, 1fr)`,
              }}
            >
              {section.fields.map((field) => {
                const widget = fieldWidget(field)
                return (
                  <div key={field.name} className="space-y-1">
                    <label className="text-sm font-medium" htmlFor={field.name}>
                      {field.label ?? field.name}
                    </label>
                    {widget === "password" ? (
                      <PasswordInput
                        value={values[field.name] ?? ""}
                        onChange={(v) => set(field.name, v)}
                        placeholder={field.placeholder}
                        error={fieldErrors[field.name]}
                      />
                    ) : (
                      <TextInput
                        value={values[field.name] ?? ""}
                        onChange={(v) => set(field.name, v)}
                        placeholder={field.placeholder}
                        error={fieldErrors[field.name]}
                      />
                    )}
                    {fieldErrors[field.name] && (
                      <p className="text-xs text-destructive">
                        {fieldErrors[field.name]}
                      </p>
                    )}
                    {field.help && (
                      <p className="text-xs text-muted-foreground">
                        {field.help}
                      </p>
                    )}
                  </div>
                )
              })}
            </div>
          </div>
        ))}

      {error && (
        <p className="text-sm text-destructive" role="alert">
          {error}
        </p>
      )}
      {success && (
        <p className="text-sm text-green-600 dark:text-green-400" role="status">
          {success}
        </p>
      )}

      <Button type="submit" className="w-full" disabled={submitting}>
        {submitting && <Loader2 className="size-4 animate-spin" />}
        {spec.submit?.label ?? DEFAULT_LABELS[spec.auth_action ?? ""]}
      </Button>
    </form>
  )
}
