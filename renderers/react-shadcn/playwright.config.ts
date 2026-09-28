import { defineConfig, devices } from "@playwright/test"

/**
 * Playwright config for the FormSpec shell (shadcn renderer).
 *
 * This is the repo's FIRST browser harness — every other test is either a Go
 * in-process HTTP test (`resource/*_e2e_test.go`) or a jsdom component test
 * (`vitest`). Those two cover a lot, but neither can answer "does the button a
 * human would click actually work", which is exactly the class of bug this
 * config exists to catch (see the kafe table lifecycle: the QR check-in page,
 * the picker, the KDS drag, and the table-status buttons are all UI paths).
 *
 * ## How the server is provided
 *
 * `webServer` boots the REAL dev server — the same one a developer runs:
 *
 *     go run ./cmd/formspec dev --spec examples/kafe/spec --workspace-id kafe ...
 *
 * `--dev-ui` is deliberately NOT used: it spawns a second Vite process that
 * races the one Playwright is about to use, and its port-sniffing adds a
 * failure mode that has nothing to do with the feature under test. Instead the
 * Go server owns :8080 (API + SPA) and Playwright owns Vite on :5173, which
 * proxies /{ws}/_ui and /{ws}/api/v1 back to :8080 (see vite.config.ts).
 *
 * ## Secrets / no network
 *
 * The kafe spec validates against the schema REGISTRY on first use, which fails
 * offline. `seed` and `dev` don't validate, so the harness avoids it — the
 * Go test suite is where `formspec validate` runs.
 */
const GO_SERVER_PORT = 8080
const VITE_PORT = 5173
// The browser path is overridable: local dev containers often have a
// root-owned ~/.cache, so the browsers are installed to a writable location
// and pointed at explicitly. CI can leave it unset and use the default.
const browserPath = process.env.PLAYWRIGHT_BROWSERS_PATH ?? ".pw-browsers"

export default defineConfig({
  testDir: "./e2e",
  // Browser tests are slower than unit tests and share one backend process;
  // running them serially keeps the DB state of each spec comprehensible.
  fullyParallel: false,
  workers: 1,
  forbidOnly: !!process.env.CI,
  retries: process.env.CI ? 1 : 0,
  reporter: process.env.CI ? [["list"], ["html", { open: "never" }]] : "list",
  timeout: 90_000,
  expect: { timeout: 15_000 },

  use: {
    baseURL: `http://localhost:${VITE_PORT}`,
    trace: "retain-on-failure",
    screenshot: "only-on-failure",
    video: "off",
  },

  projects: [
    {
      name: "chromium",
      use: {
        ...devices["Desktop Chrome"],
        launchOptions: {
          args: ["--no-sandbox", "--disable-dev-shm-usage"],
        },
      },
    },
  ],

  webServer: [
    {
      // The Go dev server: REST API + WebSocket + SPA fallback.
      command: "node e2e/scripts/start-backend.mjs",
      url: `http://localhost:${GO_SERVER_PORT}/kafe/_ui/_meta/version`,
      // NEVER reuse. The launcher seeds a fresh temp database per run, so
      // reusing a server that is already listening — a developer's own
      // `formspec dev`, or a previous run whose worker outlived the test —
      // silently points the suite at a DIFFERENT database. That is not a
      // theoretical hazard: it produced a table list containing two `A-01`
      // rows in one branch (one `available`, one `occupied`), so the assertion
      // read the untouched row and reported a table that had in fact been
      // filled. `reuseExistingServer: false` makes that impossible; the
      // dedicated port is what keeps it from fighting a running dev server.
      reuseExistingServer: false,
      timeout: 180_000,
      stdout: "pipe",
      stderr: "pipe",
    },
    {
      // Vite. Its dev-server proxy forwards the workspace-scoped API calls to
      // the Go server, which is why the browser can use :5173 exclusively.
      command: "npx vite --port 5173 --strictPort",
      url: `http://localhost:${VITE_PORT}`,
      // Same reasoning as the backend: a recycled Vite would be fine on its
      // own, but keeping both sides symmetric means one flag answers
      // "is this run hermetic?".
      reuseExistingServer: false,
      timeout: 180_000,
      env: { PLAYWRIGHT_BROWSERS_PATH: browserPath },
    },
  ],
})
