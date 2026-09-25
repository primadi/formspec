// The login session must be scoped by the App's NAME, not by the URL segment.
//
// Kafe regression (2026-09-22): the URL is `/{ws}/app/pos/...` (segment `pos`)
// while the App is named `kafe-pos`. Scoping the session to `pos` matched no
// role — `PermissionResolver` skips every role whose `app` differs from the
// session's — so the user got zero permissions. For an `access: private` App the
// bundle then ships an EMPTY entity list while the menu still renders (it is
// built from the App manifest, not from permissions): a sidebar full of items
// over a blank page.
import { describe, expect, it } from "vitest"

import { detectAppName } from "./meta"
import type { AppSummary } from "@/types/manifest"

const KAFE: AppSummary[] = [
  { name: "kafe-pos", root_url: "/app/pos" },
  { name: "kafe-kds", root_url: "/app/kds" },
  { name: "kafe-qr", root_url: "/" },
] as AppSummary[]

describe("detectAppName", () => {
  it("returns the App NAME, not the URL segment", () => {
    // The whole point: segment is "pos", name is "kafe-pos".
    expect(detectAppName("/kafe/app/pos/cafe-order/orders", KAFE)).toBe("kafe-pos")
  })

  it("matches the longest root_url prefix", () => {
    expect(detectAppName("/kafe/app/kds/kanban/order-board-kds", KAFE)).toBe("kafe-kds")
  })

  it("resolves the App mounted at the workspace root", () => {
    // `kafe-qr` has root_url "/" — the SHORTEST mount, so it is the fallback,
    // not a prefix to exclude. Measured before the fix: `/{ws}/menu` (the
    // public catalog) was served by `kafe-kds` and rendered "Page not found"
    // inside that App's chrome, because "/" scored as a full-length match and
    // won the longest-match comparison against every other App.
    expect(detectAppName("/kafe/menu/abc", KAFE)).toBe("kafe-qr")
  })

  it("prefers a specific mount over the workspace root", () => {
    expect(detectAppName("/kafe/app/kds/kanban/x", KAFE)).toBe("kafe-kds")
  })

  it("leaves the workspace root's leftovers to the root-mounted App", () => {
    // A `root_url: "/"` App owns everything no longer mount claims — that is
    // what mounting at the root means. So an unmatched path (a deep link, a
    // typo) belongs to `kafe-qr`, and its own 404 is the honest answer.
    expect(detectAppName("/kafe/nowhere/at/all", KAFE)).toBe("kafe-qr")
  })

  it("is undefined only when nothing matches at all", () => {
    // No root-mounted App present: then an unmatched path has no owner, and
    // returning an arbitrary App would scope the request to the wrong one.
    const noRoot = KAFE.filter((a) => a.root_url !== "/")
    expect(detectAppName("/kafe/nowhere/at/all", noRoot)).toBeUndefined()
  })

  it("matches the app root itself", () => {
    expect(detectAppName("/kafe/app/pos", KAFE)).toBe("kafe-pos")
  })

  it("is undefined when no apps are loaded", () => {
    expect(detectAppName("/kafe/app/pos/x", [])).toBeUndefined()
  })
})
