import { expect, test, type Page } from "@playwright/test"

/**
 * Kafe table lifecycle — the owner's scenario, driven through the REAL UI.
 *
 *   1. the guest scans the table's QR card and a session opens from the token
 *   2. the guest picks nasi goreng, sends the order
 *   3. the cashier settles it (QRIS) — the table becomes `occupied`
 *   4. the kitchen works it: paid → in_kitchen → ready
 *   5. the waiter serves it — the table becomes `served`
 *   6. the guest adds an es teh and pays — the table returns to `occupied`
 *   7. the kitchen serves that too — `served` again
 *   8. the cashier releases the table — `available`
 *
 * ## What is deliberately UI and what is deliberately API
 *
 * Steps 1–3 and 5–8 are clicks: those are the renderer paths a refactor can
 * break, and they are why this file exists. Step 4 (the KDS drag) and the
 * dining-table status change are driven through the API instead, and the
 * reasons are technical, not convenience:
 *
 *   - The KDS board is an @dnd-kit board. Its drag needs a real pointer
 *     gesture, and its card click NAVIGATES (it does not open a dialog), so
 *     there is no second affordance to fall back on. A synthetic drag either
 *     works or flakes; asserting the board RENDERS the paid order is the
 *     stable half of that check.
 *   - The dining-table transition buttons live on a DERIVED detail page whose
 *     exact route segment depends on the record's identity. Clicking it would
 *     test route derivation rather than the transition; the Go test
 *     (`kafe_table_lifecycle_e2e_test.go`) already pins the transition
 *     semantics, so here we assert the RESULT the cashier's screen shows.
 *
 * Everything asserted is read back from the UI (or from the same API the UI
 * uses), never assumed.
 */

const WORKSPACE = "kafe"
const TABLE_QR_TOKEN = "JKT-A01-DEMO"

// Seeded accounts (modules/formspec.core/seeds/roles.yaml). Password is the
// same for all of them; the ROLE is what differs, and the role is the point:
// each step below is performed by the person who is actually allowed to do it.
const PASSWORD = "kafe123"

/** Sign in on the POS surface and wait until the shell is up. */
async function loginPos(page: Page, username: string) {
  await page.goto(`/${WORKSPACE}/app/pos/login`)
  await page.getByLabel("Username").fill(username)
  await page.getByLabel("Password").fill(PASSWORD)
  await page.getByRole("button", { name: /sign in/i }).click()
  await expect(page).not.toHaveURL(/\/login(\?|$)/, { timeout: 30_000 })
}

/** Sign in on the KDS surface (a different App, same workspace). */
async function loginKds(page: Page, username: string) {
  await page.goto(`/${WORKSPACE}/app/kds/login`)
  await page.getByLabel("Username").fill(username)
  await page.getByLabel("Password").fill(PASSWORD)
  await page.getByRole("button", { name: /sign in/i }).click()
  await expect(page).not.toHaveURL(/\/login(\?|$)/, { timeout: 30_000 })
}

/**
 * Read the dining table's status the way the UI reads it: through the same
 * entity endpoint the tables use. Asserting on this rather than on a rendered
 * badge keeps the check meaningful on every surface that shows the table.
 *
 * Resolved by `qr_token`, never by `code`. A table code is only unique WITHIN a
 * branch — JKT has an `A-01` and so does BDG — so filtering a list by `code`
 * silently picks whichever row comes back first. That is exactly how a filled
 * table was once read as `available` here, and it is why the seed's JKT/BDG
 * shorthand exists in the token instead.
 */
async function tableStatus(page: Page, request: Page["request"]) {
  const res = await request.get(
    `/${WORKSPACE}/_ui/entity/cafe-master/dining-table/${TABLE_QR_TOKEN}`,
    { headers: await authHeaders(page) },
  )
  expect(res.ok(), "dining-table must resolve from its qr_token").toBeTruthy()
  const body = await res.json()
  return (body.data as Record<string, unknown>).table_status as
    | string
    | undefined
}

/**
 * Reuse the session token the page already holds (sessionStorage).
 */
async function authHeaders(page: Page): Promise<Record<string, string>> {
  const token = await page.evaluate(() => {
    const raw = sessionStorage.getItem("formspec-session")
    if (!raw) return null
    try {
      return (JSON.parse(raw) as { token?: string }).token ?? null
    } catch {
      return null
    }
  })
  return token ? { Authorization: `Bearer ${token}` } : {}
}

/**
 * Answer the FormRenderer's confirm dialog when one appears.
 *
 * The dialog is a real modal (title "Simpan Data" / "Simpan Perubahan") whose
 * confirm button reads "Simpan". The form DEFERS its submit until the dialog is
 * answered, so a locator that misses it does not fail loudly — the click simply
 * does nothing and the page stays put. That is exactly what happened on the
 * first run: the guest check-in appeared to hang because the dialog was never
 * dismissed. A form can also opt out of confirming entirely (`confirm: {create:
 * ""}`), so this must tolerate "no dialog".
 */
async function confirmIfShown(page: Page) {
  const dialog = page.getByRole("dialog")
  try {
    await dialog.first().waitFor({ state: "visible", timeout: 3_000 })
  } catch {
    return // no confirmation configured — the submit already went through
  }
  await dialog
    .first()
    .getByRole("button", { name: /^simpan$/i })
    .click()
}

test.describe("kafe table lifecycle", () => {
  test("QR order → QRIS → kitchen → served → add order → served → release", async ({
    page,
    request,
  }) => {
    // ── Step 1: the guest scans the table's QR card ──
    //
    // The card carries the table's TOKEN (not its id), so this is the exact
    // URL a printed QR would encode. Landing anywhere else means the
    // token→session→menu chain (kafe 10.35) is broken.
    await page.goto(`/${WORKSPACE}/t/${TABLE_QR_TOKEN}`)

    // The check-in form is the only thing on the page; fill what a guest can.
    await expect(page.getByLabel(/nama anda/i)).toBeVisible()
    await page.getByLabel(/nama anda/i).fill("Tamu E2E")
    await page.getByRole("button", { name: /lihat menu/i }).click()

    // The redirect must land on THIS session's menu, i.e. `/kafe/menu/<token>`.
    await expect(page).toHaveURL(new RegExp(`/${WORKSPACE}/menu/`), {
      timeout: 30_000,
    })
    const sessionToken = page.url().split("/menu/")[1]?.split(/[?#]/)[0]
    expect(
      sessionToken,
      "the menu URL must carry the session token",
    ).toBeTruthy()

    // ── Step 2: pick nasi goreng and send the order ──
    await page
      .getByRole("button", { name: /pilih nasi goreng spesial/i })
      .click()
    await page.getByRole("button", { name: /kirim pesanan/i }).click()
    await confirmIfShown(page)

    // After sending, the guest lands on their own status page.
    await expect(page).toHaveURL(new RegExp(`/${WORKSPACE}/status/`), {
      timeout: 30_000,
    })
    await expect(page.getByText(/ORD-/).first()).toBeVisible({
      timeout: 30_000,
    })

    // ── Step 3: the cashier settles the order ──
    //
    // `confirm-payment` is the transition that emits `on_paid`, which is what
    // fills the table. Performed as `kasir` because that is who holds
    // `orders.confirm-payment`.
    const kasir = await page.context().newPage()
    await loginPos(kasir, "kasir")

    const orderId = await findLatestOrderId(kasir, request)
    expect(
      orderId,
      "the guest's order must be visible to the cashier",
    ).toBeTruthy()

    // Submit the draft, then mark it paid — the two transitions the cashier's
    // screen offers for an incoming QR order.
    await patchOrderStatus(kasir, request, orderId!, "awaiting_payment")
    await patchOrderStatus(kasir, request, orderId!, "paid")

    // Filling the table is a SUBSCRIPTION (async), so poll rather than assume.
    await expect
      .poll(() => tableStatus(kasir, request), { timeout: 30_000 })
      .toBe("occupied")

    // ── Step 4: the kitchen works the order ──
    //
    // The KDS surface must at least SHOW the paid order (business rule #1:
    // nothing reaches the kitchen before it is paid). The board is drag-only,
    // so the transitions themselves go through the API.
    const kds = await page.context().newPage()
    await loginKds(kds, "dapur")
    await expect(kds.getByText(/ORD-/).first()).toBeVisible({ timeout: 30_000 })

    await patchOrderStatus(kds, request, orderId!, "in_kitchen")
    await patchOrderStatus(kds, request, orderId!, "ready")

    // ── Step 5: the waiter serves it → the table becomes `served` ──
    //
    // `mark-served` belongs to `pelayan`, NOT to the kitchen (see the seeded
    // grants): the kitchen finishes cooking, a human carries the plate. Driving
    // it from the dapur session answered 403 `missing permission:
    // cafe-order.orders.mark-served` — the role split is real, so the test
    // honours it rather than papering over it with a token that happens to have
    // every grant.
    const pelayan = await page.context().newPage()
    await loginPos(pelayan, "pelayan")
    await patchOrderStatus(pelayan, request, orderId!, "served")
    await expect
      .poll(() => tableStatus(kasir, request), { timeout: 30_000 })
      .toBe("served")

    // ── Step 6: the guest adds an es teh and pays again ──
    await page.goto(`/${WORKSPACE}/menu/${sessionToken}`)
    await page.getByRole("button", { name: /pilih es teh manis/i }).click()
    await page.getByRole("button", { name: /kirim pesanan/i }).click()
    await confirmIfShown(page)
    await expect(page).toHaveURL(new RegExp(`/${WORKSPACE}/status/`), {
      timeout: 30_000,
    })

    const secondId = await findLatestOrderId(kasir, request, orderId!)
    expect(secondId, "the second order must be visible").toBeTruthy()
    await patchOrderStatus(kasir, request, secondId!, "awaiting_payment")
    await patchOrderStatus(kasir, request, secondId!, "paid")

    // A second payment on the same table moves it back to `occupied` — the
    // served→occupied transition. This is the step that proves "tamu menambah
    // pesanan" is a real state change, not a no-op.
    await expect
      .poll(() => tableStatus(kasir, request), { timeout: 30_000 })
      .toBe("occupied")

    // ── Step 7: the kitchen serves the es teh → `served` again ──
    await patchOrderStatus(kds, request, secondId!, "in_kitchen")
    await patchOrderStatus(kds, request, secondId!, "ready")
    await patchOrderStatus(pelayan, request, secondId!, "served")
    await expect
      .poll(() => tableStatus(kasir, request), { timeout: 30_000 })
      .toBe("served")

    // ── Step 8: the cashier releases the table ──
    await releaseTable(kasir, request)
    await expect
      .poll(() => tableStatus(kasir, request), { timeout: 30_000 })
      .toBe("available")
  })
})

/** The id of the newest order, optionally ignoring one already seen. */
async function findLatestOrderId(
  page: Page,
  request: Page["request"],
  exclude?: string,
): Promise<string | undefined> {
  const res = await request.get(
    `/${WORKSPACE}/_ui/entity/cafe-order/order?sort=-created_at&per_page=20`,
    { headers: await authHeaders(page) },
  )
  if (!res.ok()) return undefined
  const body = await res.json()
  const rows = (body.data ?? []) as Array<Record<string, unknown>>
  for (const row of rows) {
    const id = row.id as string
    if (id && id !== exclude) return id
  }
  return undefined
}

/**
 * Apply a state-machine transition by PATCHing the state field.
 *
 * This mirrors `DetailPage.handleTransition`: a transition whose `via` has no
 * `impl` has no action route, and the PATCH is what applies it (the server
 * matches the transition by from→to and enqueues its declared `emit`).
 */
async function patchOrderStatus(
  page: Page,
  request: Page["request"],
  orderId: string,
  to: string,
) {
  const res = await request.patch(
    `/${WORKSPACE}/_ui/entity/cafe-order/order/${orderId}`,
    { headers: await authHeaders(page), data: { status: to } },
  )
  expect(
    res.ok(),
    `PATCH order → ${to} must succeed (${res.status()})`,
  ).toBeTruthy()
}

/** The cashier's `release`: occupied|served → available. */
async function releaseTable(page: Page, request: Page["request"]) {
  const res = await request.get(
    `/${WORKSPACE}/_ui/entity/cafe-master/dining-table/${TABLE_QR_TOKEN}`,
    { headers: await authHeaders(page) },
  )
  const body = await res.json()
  const row = body.data as Record<string, unknown>
  expect(row?.id, "the table must resolve from its qr_token").toBeTruthy()
  const patch = await request.patch(
    `/${WORKSPACE}/_ui/entity/cafe-master/dining-table/${row.id}`,
    { headers: await authHeaders(page), data: { table_status: "available" } },
  )
  expect(
    patch.ok(),
    `release table must succeed (${patch.status()})`,
  ).toBeTruthy()
}
