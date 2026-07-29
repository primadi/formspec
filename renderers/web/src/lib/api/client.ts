// ─── Forma API Client ───
//
// ky-based HTTP client with typed envelope unwrapping, auth, CAS,
// and list parameter building. Designed for the renderer's Meta API
// and CRUD endpoints.

import ky, { type KyInstance } from "ky"

import {
  type SingleResponse,
  type ListResponse,
  type ListParams,
  type ErrorResponse,
  FormaApiError,
} from "@/types/manifest"

// ── Factory ──

export interface ApiClientConfig {
  workspace: string
  token?: string
  baseUrl?: string // defaults to window.location.origin
}

/**
 * Creates a ky-based API client for entity CRUD operations on the UI surface.
 *
 * - Prefixes all URLs with `/{workspace}/_ui/entity`
 * - Injects `Authorization: Bearer` from stored token
 * - Unwraps envelope responses (data from SingleResponse/ListResponse)
 * - Maps ErrorResponse into a typed FormaApiError
 */
export function createApiClient(config: ApiClientConfig): KyInstance {
  const prefix = `/${config.workspace}/_ui/entity`

  const api = ky.create({
    prefix,
    hooks: {
      beforeRequest: [
        ({ request }: { request: Request }) => {
          if (config.token) {
            request.headers.set("Authorization", `Bearer ${config.token}`)
          }
        },
      ],
      afterResponse: [
        async ({ response }: { response: Response }) => {
          if (!response.ok) {
            const body = (await response.clone().json().catch(() => ({}))) as ErrorResponse
            const err = body?.error
            throw new FormaApiError(
              response.status,
              err?.code ?? "UNKNOWN",
              err?.message ?? response.statusText,
              err?.details,
            )
          }
        },
      ],
    },
    retry: 0,
  })
  return api
}

// ── Response unwrapping helpers ──

/**
 * Unwrap a single-resource response envelope.
 * Returns `response.data`.
 */
export async function unwrapSingle<T>(
  response: Response,
): Promise<T> {
  const body = (await response.json()) as SingleResponse<T>
  return body.data
}

/**
 * Unwrap a list-resource response envelope.
 * Returns `{ items: T[], meta: ListResponseMeta }`.
 */
export async function unwrapList<T>(
  response: Response,
): Promise<{ items: T[]; meta: import("@/types/manifest").ListResponseMeta }> {
  const body = (await response.json()) as ListResponse<T>
  return {
    items: body.data ?? [],
    meta: body.meta ?? { page: 1, per_page: 20, total: 0, total_pages: 0 },
  }
}

// ── List Parameter Builder ──

/**
 * Builds a URLSearchParams-compatible record from ListParams.
 *
 * Handles:
 * - `sort`: "field" or "-field"
 * - `filters`: simple ("status=active") or operator syntax ("total[gte]=100")
 * - `search`, `page`, `per_page`
 */
export function buildListParams(params: ListParams): Record<string, string> {
  const q: Record<string, string> = {}

  if (params.page != null && params.page > 0) q.page = String(params.page)
  if (params.per_page != null && params.per_page > 0) q.per_page = String(params.per_page)
  if (params.search) q.search = params.search
  if (params.sort) q.sort = params.sort

  if (params.filters) {
    for (const [field, value] of Object.entries(params.filters)) {
      if (typeof value === "string") {
        q[field] = value
      } else {
        // Operator syntax: field[op]=value
        q[`${field}[${value.op}]`] = value.value
      }
    }
  }

  return q
}

// ── Helper: typed getter on KyInstance ──

/**
 * Type-safe GET that returns unwrapped data from a SingleResponse.
 */
export async function apiGet<T>(
  client: KyInstance,
  path: string,
  searchParams?: Record<string, string | undefined>,
): Promise<T> {
  const response = await client.get(path, { searchParams })
  const body = (await response.json()) as SingleResponse<T>
  return body.data
}

/**
 * Type-safe list GET that returns unwrapped items + meta.
 */
export async function apiList<T>(
  client: KyInstance,
  path: string,
  searchParams?: Record<string, string | undefined>,
): Promise<{ items: T[]; meta: import("@/types/manifest").ListResponseMeta }> {
  const response = await client.get(path, { searchParams })
  const body = (await response.json()) as ListResponse<T>
  return {
    items: body.data ?? [],
    meta: body.meta ?? { page: 1, per_page: 20, total: 0, total_pages: 0 },
  }
}

/**
 * Type-safe POST with optional CAS version and Idempotency-Key headers.
 */
export async function apiPost<T>(
  client: KyInstance,
  path: string,
  json?: unknown,
  options?: { version?: number; idempotencyKey?: string },
): Promise<T> {
  const headers: Record<string, string> = {}
  if (options?.version != null) headers["If-Match"] = `version=${options.version}`
  if (options?.idempotencyKey) headers["Idempotency-Key"] = options.idempotencyKey

  const response = await client.post(path, { json, headers })
  const body = (await response.json()) as SingleResponse<T>
  return body.data
}

/**
 * Type-safe PUT with optional CAS version.
 */
export async function apiPut<T>(
  client: KyInstance,
  path: string,
  json?: unknown,
  version?: number,
): Promise<T> {
  const headers: Record<string, string> = {}
  if (version != null) headers["If-Match"] = `version=${version}`

  const response = await client.put(path, { json, headers })
  const body = (await response.json()) as SingleResponse<T>
  return body.data
}

/**
 * Type-safe PATCH with optional CAS version.
 */
export async function apiPatch<T>(
  client: KyInstance,
  path: string,
  json?: unknown,
  version?: number,
): Promise<T> {
  const headers: Record<string, string> = {}
  if (version != null) headers["If-Match"] = `version=${version}`

  const response = await client.patch(path, { json, headers })
  const body = (await response.json()) as SingleResponse<T>
  return body.data
}

/**
 * Type-safe DELETE (may return empty or SingleResponse).
 */
export async function apiDelete<T = void>(
  client: KyInstance,
  path: string,
  version?: number,
): Promise<T | void> {
  const headers: Record<string, string> = {}
  if (version != null) headers["If-Match"] = `version=${version}`

  const response = await client.delete(path, { headers })
  if (response.status === 204) return
  const body = (await response.json()) as SingleResponse<T>
  return body.data
}
