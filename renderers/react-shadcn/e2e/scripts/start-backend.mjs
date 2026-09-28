#!/usr/bin/env node
/**
 * Boots the kafe FormSpec dev server for the Playwright suite.
 *
 * Playwright's `webServer` expects a long-running process, so this script:
 *   1. seeds a FRESH temp SQLite database (so a run never inherits another
 *      run's orders/table state — the consequence of the module chosen for
 *      this harness: the kafe dev DB is a mutable fixture),
 *   2. starts `formspec dev` against it and forwards its stdout/stderr,
 *   3. exits when the server does, and passes SIGTERM/SIGINT through so
 *      Playwright can shut it down cleanly.
 *
 * ## Why not the checked-in dev DB
 *
 * The dev database (`examples/kafe/.formspec/kafe.db`) is a mutable fixture: it
 * accumulates orders and table sessions, and `table-session` now has a partial
 * unique index on "one OPEN session per table". Re-running the suite against it
 * would fail on the second run for a reason that has nothing to do with the
 * code under test. A temp DB per run makes the suite repeatable, which is the
 * property that makes it worth having.
 *
 * ## Why not `--dev-ui`
 *
 * `--dev-ui` makes the Go server spawn its own Vite and reverse-proxy the SPA
 * to it. Playwright already starts Vite (with a proxy back to the Go server),
 * so enabling it here would create two Vite processes racing for :5173. The Go
 * server's job in this harness is the API + WebSocket; Vite's is the UI.
 */
import { spawn, spawnSync } from "node:child_process"
import { mkdtempSync } from "node:fs"
import { tmpdir } from "node:os"
import { dirname, join, resolve } from "node:path"
import { fileURLToPath } from "node:url"

const here = dirname(fileURLToPath(import.meta.url))
// e2e/scripts → react-shadcn → renderers → repo root
const repoRoot = resolve(here, "..", "..", "..", "..")

const specDir =
  process.env.KAFE_E2E_SPEC ?? join(repoRoot, "examples", "kafe", "spec")
const dsn =
  process.env.KAFE_E2E_DSN ??
  `sqlite:${join(mkdtempSync(join(tmpdir(), "kafe-e2e-")), "kafe.db")}`
const addr = process.env.KAFE_E2E_ADDR ?? ":8080"
const workspace = process.env.KAFE_E2E_WORKSPACE ?? "kafe"

// `os.UserCacheDir()` is ~/.cache on Linux, and in some dev containers that is
// root-owned — `formspec` then fails with "mkdir cache: permission denied"
// before it ever reaches the spec. Point it somewhere writable unless the
// caller already did.
const env = {
  ...process.env,
  XDG_CACHE_HOME:
    process.env.XDG_CACHE_HOME ?? join(tmpdir(), "formspec-e2e-cache"),
}

const go = (args) =>
  spawnSync("go", args, { cwd: repoRoot, env, stdio: "inherit", shell: false })

console.log(`[e2e] spec:      ${specDir}`)
console.log(`[e2e] dsn:       ${dsn}`)
console.log(`[e2e] workspace: ${workspace}`)

// Seed first: the suite needs master data (branch, tables, menu, roles) and
// seeding is idempotent, so a re-run against a kept DSN stays correct.
console.log("[e2e] seeding…")
const seed = go([
  "run",
  "./cmd/formspec",
  "seed",
  "--spec",
  specDir,
  "--dsn",
  dsn,
  "--workspace",
  workspace,
])
if (seed.status !== 0) {
  console.error(`[e2e] seed failed with status ${seed.status}`)
  process.exit(seed.status ?? 1)
}

console.log(`[e2e] starting formspec dev on ${addr}`)
const server = spawn(
  "go",
  [
    "run",
    "./cmd/formspec",
    "dev",
    "--spec",
    specDir,
    "--dsn",
    dsn,
    "--addr",
    addr,
    "--workspace-id",
    workspace,
  ],
  { cwd: repoRoot, env, stdio: "inherit", shell: false },
)

const shutdown = (signal) => () => server.kill(signal)
process.on("SIGTERM", shutdown("SIGTERM"))
process.on("SIGINT", shutdown("SIGINT"))

server.on("exit", (code) => process.exit(code ?? 0))
