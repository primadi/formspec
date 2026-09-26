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
import { ContextPicker } from "@/shell/ContextPicker"
import {
  defaultContextChoice,
  readContextPreference,
} from "@/lib/session-context"
import {
  FormaApiError,
  type ContextChoice,
  type FormField,
  type FormSpec,
} from "@/types/manifest"

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

// Chromium "Create Amazing Password Forms": password managers need explicit
// autocomplete tokens to know whether a field holds an existing credential
// (current-password) or a new one (new-password). Like the auth field names,
// this mapping is conventional rather than declared in YAML — so a custom
// `auth_action` form gets correct autofill for free. Fields we don't know
// default to "off" so Chrome stops guessing (address/payment dropdowns).
const AUTOCOMPLETE_BY_ACTION: Record<string, Record<string, string>> = {
  login: {
    username: "username",
    email: "username",
    password: "current-password",
  },
  register: {
    username: "username",
    email: "email",
    display_name: "name",
    password: "new-password",
  },
  change_password: {
    username: "username",
    current_password: "current-password",
    new_password: "new-password",
    password: "new-password",
  },
  forgot_password: { username: "username", email: "email" },
  reset_password: { password: "new-password", new_password: "new-password" },
}

function autocompleteFor(action: string | undefined, field: string) {
  return AUTOCOMPLETE_BY_ACTION[action ?? ""]?.[field] ?? "off"
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
  // Session context (role × branch) — set when login answers 409
  // CONTEXT_REQUIRED with the choices the caller must pick from (backend §8.7).
  const [contextChoices, setContextChoices] = useState<
    ContextChoice[] | null
  >(null)
  const [assignment, setAssignment] = useState<string | null>(null)

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
          await auth.login(values.username.trim(), values.password, {
            assignment: assignment ?? undefined,
          })
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
      // Several session contexts and no explicit choice → the server refused
      // to pick one (backend §8.7). Not a failure: present the picker.
      if (
        err instanceof FormaApiError &&
        err.status === 409 &&
        err.code === "CONTEXT_REQUIRED" &&
        err.choices?.length
      ) {
        setContextChoices(err.choices)
        return
      }
      setError(err instanceof Error ? err.message : "Authentication failed")
    } finally {
      setSubmitting(false)
    }
  }

  /** Retry the login action with the context the caller picked. */
  const onPickContext = async (chosen: string) => {
    setAssignment(chosen)
    setSubmitting(true)
    try {
      await auth.login(values.username.trim(), values.password, {
        assignment: chosen,
      })
      redirectAfterLogin()
    } catch (err) {
      if (
        err instanceof FormaApiError &&
        err.status === 409 &&
        err.code === "CONTEXT_REQUIRED" &&
        err.choices?.length
      ) {
        setContextChoices(err.choices)
        setError(err.message)
        return
      }
      setError(err instanceof Error ? err.message : "Authentication failed")
    } finally {
      setSubmitting(false)
    }
  }

  // The credentials were accepted but the principal holds several session
  // contexts and the server refused to pick one (backend §8.7). Replace the
  // credential form entirely — a nested <form> would be invalid HTML — and
  // keep the password in state so the retry does not ask for it again.
  if (contextChoices) {
    // No App scope here: a custom auth_action form renders before any bundle
    // is loaded, so only the workspace is known. The preference key falls back
    // to the workspace-level one (LoginScreen uses the resolved App).
    return (
      <div className="space-y-6">
        <ContextPicker
          choices={contextChoices}
          defaultId={defaultContextChoice(
            contextChoices,
            readContextPreference(workspace),
          )}
          onSubmit={onPickContext}
          busy={submitting}
        />
        {error && (
          <p className="text-sm text-destructive" role="alert">
            {error}
          </p>
        )}
      </div>
    )
  }

  return (
    <form onSubmit={onSubmit} autoComplete="on" className="space-y-6">
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
                const autoComplete = autocompleteFor(
                  spec.auth_action,
                  field.name,
                )
                return (
                  <div key={field.name} className="space-y-1">
                    <label className="text-sm font-medium" htmlFor={field.name}>
                      {field.label ?? field.name}
                    </label>
                    {widget === "password" ? (
                      <PasswordInput
                        id={field.name}
                        name={field.name}
                        autoComplete={autoComplete}
                        value={values[field.name] ?? ""}
                        onChange={(v) => set(field.name, v)}
                        placeholder={field.placeholder}
                        error={fieldErrors[field.name]}
                      />
                    ) : (
                      <TextInput
                        id={field.name}
                        name={field.name}
                        autoComplete={autoComplete}
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
