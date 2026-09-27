// @vitest-environment node
//
// Guard for the Vite dev proxy (renderers/react-shadcn/vite.config.ts).
//
// Why this test exists: the SPA is served by Vite directly in `--dev-ui` mode
// (via `npm run dev`, or `formspec dev --dev-ui` which spawns it), so Vite's
// own `server.proxy` — NOT Go's viteSPAProxy — answers the SPA's API calls.
// The keys used to be the literal "/default/api/v1" + "/default/_ui/", so any
// other workspace ("/kafe/_ui/_meta/apps") matched no proxy rule and Vite
// replied with index.html. The SPA then failed with
// `Unexpected token '<', "<!doctype "... is not valid JSON`.
//
// Go's predicate (cmd/formspec/dev.go isWorkspaceAPIPath) was already fixed for
// every workspace slug (todo 2.11.7); this test pins the Vite half of the same
// contract so the two cannot drift apart again.
//
// It runs the REAL config through a REAL Vite dev server with a stub backend
// rather than re-implementing Vite's key-matching rules here — a hand-rolled
// matcher would pass even if Vite stopped honouring the config shape.

import { createServer as createHttpServer, type Server } from "node:http"
import type { AddressInfo } from "node:net"
import path from "node:path"
import { fileURLToPath } from "node:url"

import { afterAll, beforeAll, describe, expect, it } from "vitest"
import {
  createServer as createViteServer,
  loadConfigFromFile,
  type ViteDevServer,
} from "vite"

const here = fileURLToPath(import.meta.url)
// src/lib/api → renderers/react-shadcn (three levels up). Computed through a
// variable: `new URL("literal", import.meta.url)` is rewritten by Vite as an
// asset URL in tests and then fails fileURLToPath.
const webDir = path.resolve(path.dirname(here), "../../..")
const configPath = path.join(webDir, "vite.config.ts")

const PROXIED = "proxied"
const SPA_HTML = "spa-html"

let vite: ViteDevServer
let stub: Server
let baseURL: string
/** Set when the stub backend received a WebSocket upgrade request. */
let sawUpgrade = false

/** Ask the running Vite dev server for `path` and classify the answer. */
async function classify(pathname: string): Promise<string> {
  const res = await fetch(`${baseURL}${pathname}`)
  const body = await res.text()
  if (body.includes(`"${PROXIED}"`)) return PROXIED
  if (
    body.includes("<html") ||
    res.headers.get("content-type")?.includes("html")
  ) {
    return SPA_HTML
  }
  return `other:${res.status}:${body.slice(0, 80)}`
}

beforeAll(async () => {
  // ── Stub backend: answers JSON, and records WS upgrades ──
  stub = createHttpServer((req, res) => {
    res.setHeader("content-type", "application/json")
    res.end(JSON.stringify({ proxied: true, url: req.url }))
  })
  stub.on("upgrade", (_req, socket) => {
    sawUpgrade = true
    socket.destroy()
  })
  await new Promise<void>((resolve) =>
    stub.listen(0, "127.0.0.1", () => resolve()),
  )
  const stubPort = (stub.address() as AddressInfo).port

  // ── Real config, targets repointed at the stub ──
  const loaded = await loadConfigFromFile(
    { command: "serve", mode: "development" },
    configPath,
    webDir,
  )
  if (!loaded?.config.server?.proxy) {
    throw new Error(
      `${configPath} has no server.proxy — dev API calls cannot work`,
    )
  }
  const proxy = loaded.config.server.proxy
  for (const key of Object.keys(proxy)) {
    ;(proxy[key] as { target: string }).target = `http://127.0.0.1:${stubPort}`
  }

  vite = await createViteServer({
    ...loaded.config,
    configFile: false,
    root: webDir,
    logLevel: "silent",
    server: { ...loaded.config.server, host: "127.0.0.1", port: 0, proxy },
  })
  await vite.listen()
  baseURL = `http://127.0.0.1:${(vite.httpServer!.address() as AddressInfo).port}`
}, 60_000)

afterAll(async () => {
  await vite?.close()
  await new Promise<void>((resolve) => stub?.close(() => resolve()))
})

describe("vite dev proxy — workspace API paths", () => {
  // The reported bug: a non-default workspace slug must reach the backend.
  it.each([
    "/kafe/_ui/_meta/apps",
    "/kafe/_ui/_meta/me",
    "/kafe/_ui/entity/cafe-master/menu-item",
    "/kafe/_ui/auth/login",
    "/kafe/api/v1/orders",
  ])("proxies %s to the backend", async (p) => {
    await expect(classify(p)).resolves.toBe(PROXIED)
  })

  // The default workspace keeps working (these were the only keys before).
  it("proxies the default workspace", async () => {
    await expect(classify("/default/_ui/_meta/apps")).resolves.toBe(PROXIED)
  })

  // Arbitrary slugs, not just the one in the bug report — the regex covers the
  // whole slug charset (`pkg/spec/workspace.go` workspaceSlugPattern).
  it("proxies any valid workspace slug", async () => {
    await expect(classify("/acme-2/_ui/_meta/apps")).resolves.toBe(PROXIED)
  })

  // The "^" anchor matters: `_ui` deeper in a path is a SPA route, not an API
  // call — same shape as Go's isWorkspaceAPIPath (first segment = workspace,
  // second = surface).
  it("does not proxy _ui when it is not the first segment after the slug", async () => {
    await expect(classify("/kafe/_admin/menu/_ui/x")).resolves.toBe(SPA_HTML)
    await expect(classify("/kafe/app/_ui/x")).resolves.toBe(SPA_HTML)
  })

  it("serves SPA routes from Vite", async () => {
    await expect(classify("/kafe/_admin")).resolves.toBe(SPA_HTML)
    await expect(classify("/kafe")).resolves.toBe(SPA_HTML)
  })

  // Realtime rides the same proxy: /{ws}/_ui/_ws needs ws: true, or the upgrade
  // hangs and no event ever arrives.
  it("forwards the WebSocket upgrade for /{ws}/_ui/_ws", async () => {
    const port = (vite.httpServer!.address() as AddressInfo).port
    const { request } = await import("node:http")
    const upgradeSeen = new Promise<void>((resolve) => {
      const req = request({
        host: "127.0.0.1",
        port,
        path: "/kafe/_ui/_ws",
        headers: {
          Connection: "Upgrade",
          Upgrade: "websocket",
          "Sec-WebSocket-Version": "13",
          "Sec-WebSocket-Key": "dGhlIHNhbXBsZSBub25jZQ==",
        },
      })
      req.on("upgrade", () => resolve())
      req.on("response", () => resolve()) // Vite's own ws server answered instead
      req.on("error", () => resolve())
      req.end()
      setTimeout(resolve, 3000)
    })
    await upgradeSeen
    expect(sawUpgrade).toBe(true)
  }, 15_000)
})
