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
} from "@/types/manifest"

/**
 * Fetch the full Meta bundle: entity schemas + all authored UI manifests.
 * This is the primary payload the renderer uses to bootstrap (one round-trip).
 */
export async function fetchMetaBundle(client: KyInstance): Promise<MetaBundle> {
  const response = await client.get("_meta/ui")
  const body = (await response.json()) as { data: MetaBundle }
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
