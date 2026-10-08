// The OAuth context-required resume (kafe 6.5.10 / backend §8.7).
//
// The fragment is input from a redirect: it can be truncated, empty, or carry
// parameters meant for another flow (error / email_unverified / link_required).
// Every one of those must degrade to "no picker" — a picker with nothing in it
// would replace an explainable mistake with a blank screen.

import { describe, it, expect } from "vitest"
import { parseOAuthContextHash, oauthAuthorizeURL } from "./oauthContext"

describe("parseOAuthContextHash", () => {
  it("reads the repeated c parameters as choices", () => {
    const got = parseOAuthContextHash(
      "#app=kafe-pos&c=sales%40KFE-JKT-01&c=admin%40KFE-BDG-01&oauth=context_required&provider=google",
    )
    expect(got?.provider).toBe("google")
    expect(got?.choices.map((c) => c.id)).toEqual([
      "sales@KFE-JKT-01",
      "admin@KFE-BDG-01",
    ])
    // The label the picker renders; the dimension is re-checked by the server.
    expect(got?.choices[0].role).toBe("sales")
    expect(got?.choices[0].value).toBe("KFE-JKT-01")
  })

  it("splits the id on the FIRST @ so a value may contain one", () => {
    const got = parseOAuthContextHash(
      "#oauth=context_required&provider=google&c=sales%40outlet%40north",
    )
    expect(got?.choices[0].role).toBe("sales")
    expect(got?.choices[0].value).toBe("outlet@north")
  })

  // Every non-matching input has its own handling elsewhere; returning a picker
  // here would hijack that screen.
  it.each([
    ["empty hash", ""],
    ["another oauth value", "#oauth=error"],
    ["another oauth value", "#oauth=email_unverified"],
    ["another oauth value", "#oauth=link_required"],
    ["no choices", "#oauth=context_required&provider=google"],
    ["no provider", "#oauth=context_required&c=sales%40B1"],
    ["a token handoff", "#token=abc&refresh_token=def&app=kafe-pos"],
    ["a malformed id", "#oauth=context_required&provider=google&c=no-at-sign"],
  ])("returns null for %s", (_name, hash) => {
    expect(parseOAuthContextHash(hash)).toBeNull()
  })
})

describe("oauthAuthorizeURL", () => {
  it("carries the choice so the backend can keep it in state", () => {
    const url = oauthAuthorizeURL("kafe", "kafe-pos", "google", "sales@B1")
    expect(url).toBe(
      "/kafe/_ui/auth/oauth/google/authorize?app=kafe-pos&assignment=sales%40B1",
    )
  })

  it("omits empty parameters rather than sending blanks", () => {
    // A blank `app` would be rejected as APP_REQUIRED by the authorize endpoint,
    // so an empty value must not be sent at all.
    expect(oauthAuthorizeURL("kafe", "", "google", "")).toBe(
      "/kafe/_ui/auth/oauth/google/authorize",
    )
  })
})
