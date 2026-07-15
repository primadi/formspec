// ─── Meta API Fetchers ───
//
// Functions to fetch the Meta API endpoints:
//   - GET /_meta/ui      → full bundle (entities + UI manifests)
//   - GET /_meta/me      → caller identity + permissions
//   - GET /_meta/entities/{module}/{name} → single entity schema
//
// These are the first calls the renderer makes at boot (design doc §5.2).

import type { KyInstance } from "ky"

import {
  type MetaBundle,
  type EntitySchema,
  type MeResponse,
  type AppSummary,
} from "@/types/manifest"

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
  client: KyInstance,
  opts?: { appName?: string; admin?: boolean },
): Promise<MetaBundle> {
  const searchParams = opts?.admin ? { admin: "true" } : opts?.appName ? { app: opts.appName } : undefined
  const response = await client.get("_meta/ui", searchParams ? { searchParams } : undefined)
  const body = (await response.json()) as { data: MetaBundle }
  return body.data
}

/**
 * List every resolved App in this workspace (name + root_url) — Core §4.4.
 * Fetched once at boot to figure out which App the current URL belongs to.
 */
export async function fetchMetaApps(client: KyInstance): Promise<AppSummary[]> {
  const response = await client.get("_meta/apps")
  const body = (await response.json()) as { data: AppSummary[] }
  return body.data
}

/**
 * Fetch a single entity schema by module and name.
 * Used for lazy-loading form-heavy entity schemas.
 */
export async function fetchEntitySchema(
  client: KyInstance,
  module: string,
  name: string,
): Promise<EntitySchema> {
  const response = await client.get(`_meta/entities/${module}/${name}`)
  const body = (await response.json()) as { data: EntitySchema }
  return body.data
}

/**
 * Fetch the caller's identity, roles, and effective permissions.
 * Returns null if the request fails (e.g. 401 in dev mode).
 */
export async function fetchMe(client: KyInstance): Promise<MeResponse | null> {
  try {
    const response = await client.get("_meta/me")
    const body = (await response.json()) as { data: MeResponse }
    return body.data
  } catch {
    return null
  }
}
