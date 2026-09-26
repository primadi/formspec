// @vitest-environment jsdom
//
// ─── Auth API — error envelope contract ───
//
// Regression tests for the login form showing
//
//   TypeError: Failed to execute 'clone' on 'Response': Response body is
//   already used
//
// instead of the server's message ("invalid username or password"). The cause
// was `err.response.clone().json()` in `loginWithPassword`: ky v2 populates
// `HTTPError.data` by reading the response stream BEFORE throwing, so the body
// is already consumed and `clone()` fails — the TypeError then replaced the
// real message at the call site.
//
// These tests stub `globalThis.fetch` with a real `Response` (a real stream, so
// the consumption is genuine) and assert on what the caller sees.
//
// jsdom (not node) is the environment on purpose: it provides a `Request` that
// resolves the RELATIVE url `/kafe/_ui/auth/login`, matching the browser where
// this code really runs. Plain node fails those with "Failed to parse URL".

import { afterEach, beforeEach, describe, expect, it, vi } from "vitest"

import { loginWithPassword } from "./auth"
import { FormaApiError } from "@/types/manifest"

/**
 * jsdom does not implement `fetch`/`Request`, so `globalThis.Request` here is
 * Node's undici — which rejects the RELATIVE url `loginWithPassword` builds
 * ("Failed to parse URL from /kafe/_ui/auth/login"). A browser resolves it
 * against the document base URL. Extend rather than replace, so ky's
 * `input instanceof globalThis.Request` checks keep working.
 */
const NodeRequest = globalThis.Request

class BaseUrlRequest extends NodeRequest {
  constructor(input: RequestInfo | URL, init?: RequestInit) {
    super(
      typeof input === "string"
        ? new URL(input, "http://localhost:3000")
        : input,
      init,
    )
  }
}

beforeEach(() => {
  vi.stubGlobal("Request", BaseUrlRequest)
})

afterEach(() => {
  vi.unstubAllGlobals()
})

/** Stub fetch with one JSON response. */
function stubJson(status: number, body: unknown, statusText = "") {
  vi.stubGlobal(
    "fetch",
    vi.fn(
      async () =>
        new Response(JSON.stringify(body), {
          status,
          statusText,
          headers: { "content-type": "application/json" },
        }),
    ),
  )
}

describe("loginWithPassword — error envelope", () => {
  it("surfaces the server message on invalid credentials", async () => {
    stubJson(401, {
      error: { code: "UNAUTHORIZED", message: "invalid username or password" },
      meta: { timestamp: "2026-09-24T00:00:00Z" },
    })

    await expect(loginWithPassword("kafe", "manajer", "salah")).rejects.toThrow(
      "invalid username or password",
    )
  })

  it("never leaks the response-body TypeError", async () => {
    stubJson(401, {
      error: { code: "UNAUTHORIZED", message: "invalid username or password" },
    })

    // The old code produced exactly this string; keep it out of the UI.
    await expect(
      loginWithPassword("kafe", "manajer", "salah"),
    ).rejects.not.toThrow(/already (used|been consumed)/)
  })

  it("returns a typed FormaApiError with code and status", async () => {
    stubJson(401, {
      error: { code: "UNAUTHORIZED", message: "invalid username or password" },
    })

    const err = await loginWithPassword("kafe", "manajer", "salah").catch(
      (e: unknown) => e,
    )
    expect(err).toBeInstanceOf(FormaApiError)
    expect((err as FormaApiError).status).toBe(401)
    expect((err as FormaApiError).code).toBe("UNAUTHORIZED")
  })

  it("carries the choices on 409 CONTEXT_REQUIRED", async () => {
    const choices = [
      { id: "sales@B1", role: "sales", dimension: "branch_id", value: "B1" },
      { id: "admin@B2", role: "admin", dimension: "branch_id", value: "B2" },
    ]
    stubJson(409, {
      error: {
        code: "CONTEXT_REQUIRED",
        message: "choose a session context (role + branch) to continue",
        choices,
      },
    })

    const err = await loginWithPassword("kafe", "kasir", "kafe123").catch(
      (e: unknown) => e,
    )
    expect(err).toBeInstanceOf(FormaApiError)
    expect((err as FormaApiError).status).toBe(409)
    expect((err as FormaApiError).choices).toEqual(choices)
  })

  it("sends the chosen assignment in the login body", async () => {
    let body: unknown
    vi.stubGlobal(
      "fetch",
      vi.fn(async (input: Request) => {
        body = await input.clone().json()
        return new Response(
          JSON.stringify({
            data: { access_token: "a", refresh_token: "r" },
          }),
          { status: 200, headers: { "content-type": "application/json" } },
        )
      }),
    )

    await loginWithPassword("kafe", "kasir", "kafe123", "pos", "sales@B1")
    expect(body).toMatchObject({
      username: "kasir",
      password: "kafe123",
      app: "pos",
      assignment: "sales@B1",
    })
  })

  it("omits assignment when the caller has no remembered choice", async () => {
    let body: Record<string, unknown> = {}
    vi.stubGlobal(
      "fetch",
      vi.fn(async (input: Request) => {
        body = (await input.clone().json()) as Record<string, unknown>
        return new Response(
          JSON.stringify({ data: { access_token: "a", refresh_token: "r" } }),
          { status: 200, headers: { "content-type": "application/json" } },
        )
      }),
    )

    await loginWithPassword("kafe", "kasir", "kafe123")
    expect(body).not.toHaveProperty("assignment")
    expect(body).not.toHaveProperty("app")
  })

  it("falls back to the status text when the body is not an envelope", async () => {
    vi.stubGlobal(
      "fetch",
      vi.fn(
        async () =>
          // Plain-text body (bad gateway from a proxy, say) — ky puts the raw
          // text in `error.data`, so the helper must not assume an object.
          new Response("upstream error", {
            status: 502,
            statusText: "Bad Gateway",
            headers: { "content-type": "text/plain" },
          }),
      ),
    )

    const err = await loginWithPassword("kafe", "manajer", "kafe123").catch(
      (e: unknown) => e,
    )
    expect(err).toBeInstanceOf(FormaApiError)
    expect((err as FormaApiError).code).toBe("UNKNOWN")
    expect((err as FormaApiError).message).toBe("Bad Gateway")
  })
})

describe("ky v2 body-consumption behaviour (root cause pin)", () => {
  it("locks HTTPError.response so clone() throws — hence error.data", async () => {
    stubJson(401, { error: { code: "UNAUTHORIZED", message: "nope" } })

    // Reproduce the raw ky call to document WHY the API layer reads
    // `error.data`: cloning the consumed response is what used to explode.
    const ky = (await import("ky")).default
    const err = await ky
      .post("http://localhost:8099/kafe/_ui/auth/login", {
        json: { username: "x", password: "y" },
        retry: 0,
      })
      .catch((e: any) => e)

    expect(err.response.bodyUsed).toBe(true)
    // Message differs per runtime ("...body is already used" in Chromium,
    // "...Body has already been consumed" in undici/jsdom) — match both.
    expect(() => err.response.clone()).toThrow(/already (used|been consumed)/)
    // The pre-parsed body is still available on `data` — the supported path.
    expect(err.data).toMatchObject({ error: { code: "UNAUTHORIZED" } })
  })
})
