// ─── Meta API Fetchers ───
//
// Functions to fetch the Meta API endpoints:
//   - GET /_meta/ui      → full bundle (entities + UI manifests)
//   - GET /_meta/me      → caller identity + permissions
//   - GET /_meta/entities/{module}/{name} → single entity schema
//
// These are the first calls the renderer makes at boot (design doc §5.2).
// Meta API lives under /_ui/_meta/..., separate from the entity CRUD
// surface at /_ui/entity/.

import ky, { HTTPError, type KyInstance } from "ky"

import { createAuthHooks } from "./authHooks"
import { SessionExpiredError } from "./sessionEvents"
import {
  type MetaBundle,
  type EntitySchema,
  type MeResponse,
  type AppSummary,
  FormaApiError,
} from "@/types/manifest"

/** Optional live auth callbacks for the refresh-token flow. */
interface MetaClientOpts {
  getToken?: () => string
  onUnauthorized?: () => Promise<boolean>
}

/** Create a ky client scoped to /_ui for Meta API calls. */
function createMetaClient(
  workspace: string,
  token?: string,
  opts?: MetaClientOpts,
): KyInstance {
  const auth = createAuthHooks({
    getToken: opts?.getToken ?? (() => token ?? ""),
    onUnauthorized: opts?.onUnauthorized ?? (async () => false),
  })
  return ky.create({
    prefix: `/${workspace}/_ui`,
    ...auth,
  })
}

/**
 * Fetch the full Meta bundle: entity schemas + all authored UI manifests,
 * scoped to one resolved App (Core §4.4). `appName` is required whenever the
 * workspace resolves to more than one App — see fetchMetaApps/detectAppName.
 *
 * Pass `{ admin: true }` instead for the `_admin` surface: an unscoped bundle
 * (every module's entities, no App concept) gated by a single binary
 * permission (`_admin.access`) rather than the App's `?app=` scoping.
 */
export async function fetchMetaBundle(
  workspace: string,
  opts?: {
    appName?: string
    admin?: boolean
    token?: string
    getToken?: () => string
    onUnauthorized?: () => Promise<boolean>
  },
): Promise<MetaBundle> {
  const client = createMetaClient(workspace, opts?.token, opts)
  const searchParams = opts?.admin
    ? { admin: "true" }
    : opts?.appName
      ? { app: opts.appName }
      : undefined
  const response = await client.get(
    "_meta/ui",
    searchParams ? { searchParams } : undefined,
  )
  const body = (await response.json()) as { data: MetaBundle }
  return body.data
}

/**
 * List every resolved App in this workspace (name + root_url) — Core §4.4.
 * Fetched once at boot to figure out which App the current URL belongs to.
 */
export async function fetchMetaApps(
  workspace: string,
  token?: string,
  opts?: MetaClientOpts,
): Promise<AppSummary[]> {
  const client = createMetaClient(workspace, token, opts)
  const response = await client.get("_meta/apps")
  const body = (await response.json()) as { data: AppSummary[] }
  return body.data
}

/**
 * Fetch a single entity schema by module and name.
 * Used for lazy-loading form-heavy entity schemas.
 */
export async function fetchEntitySchema(
  workspace: string,
  module: string,
  name: string,
  token?: string,
  opts?: MetaClientOpts,
): Promise<EntitySchema> {
  const client = createMetaClient(workspace, token, opts)
  const response = await client.get(`_meta/entities/${module}/${name}`)
  const body = (await response.json()) as { data: EntitySchema }
  return body.data
}

/**
 * Fetch the caller's identity, roles, and effective permissions.
 * Returns null when the server rejects the session with 401 (invalid /
 * expired token) — the session store turns that into a login redirect.
 * Other failures (network, 5xx) propagate so the caller can show a
 * connection error. When not authenticated the server returns an identity
 * with user_id "anonymous" and empty permissions.
 */
export async function fetchMe(
  workspace: string,
  token?: string,
  opts?: MetaClientOpts,
): Promise<MeResponse | null> {
  try {
    const client = createMetaClient(workspace, token, opts)
    const response = await client.get("_meta/me")
    const body = (await response.json()) as { data: MeResponse }
    return body.data
  } catch (err) {
    if (err instanceof HTTPError && err.response.status === 401) return null
    if (err instanceof FormaApiError && err.status === 401) return null
    if (err instanceof SessionExpiredError) return null
    throw err
  }
}
