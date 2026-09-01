# Plan: Render Context Standard (Page/Form)

**Tanggal**: 2026-08-31
**Status**: ✅ Phase 1 · ✅ Phase 2 (session/const/expr/entity/api) · ✅ Phase 3 (reaktivitas)
**Scope**: Standarisasi context render untuk kind Page & Form + mekanisme
inject variabel dari banyak sumber (session, entity, api, const, expr).

## Tujuan

Membuat context render Page/Form menjadi **kontrak terstandarisasi** yang
bisa di-inject dari banyak sumber, sehingga halaman seperti profile bisa
dibuat pure YAML (tanpa custom asset) dan Page/Form fleksibel tanpa membuka
pintu custom code.

## Latar Belakang (temuan riset)

- Context hari ini terfragmentasi per renderer:
  - Form: `fields` + `user` (hardcoded di `FormRenderer.tsx:306,409,443`,
    `headless-form.ts:55`).
  - Page title: ad-hoc `titleCtx` dari record entity pertama yang match
    (`PageRenderer.tsx:184`).
  - Page blocks: route params + master-detail bind, implisit.
  - Custom asset: kontrak `mount(el, props, formspec)`.
- `EvalContext` (`renderers/react-shadcn/src/lib/formspec-expr/eval.ts`)
  sudah punya slot `user` — engine context-agnostic, injector yang
  menentukan isinya.
- `interpolate()` (`renderers/react-shadcn/src/lib/interpolate.ts`) sudah
  mendukung token `{dotted.path}` untuk title/print — tinggal diperluas ke
  section items & block params.
- Halaman profile registry (`registry/spec/modules/portal/pages/profile.yaml`)
  terpaksa `mode: custom` karena blocks tidak bisa render identitas session.

## Keputusan Desain (disepakati 2026-08-31)

1. **Standard slots (hardcoded, didokumentasikan resmi di kontrak kind):**
   - `fields` — nilai form saat ini (khusus Form)
   - `route` — path params (Page & Form)
   - `user` — identitas session pemanggil dari `/_meta/me` (Page & Form).
     Nama kanonik = `user` (kontinuitas dengan form existing). Alasan
     hardcode: zero-cost (sudah di-fetch saat boot), data caller sendiri
     (bukan eksfiltrasi), backward compat.
2. **Context tambahan: wajib deklarasi eksplisit** via `spec.context` dengan
   **closed source set**: `session` | `entity` | `api` | `const` | `expr`.
   - `entity`/`api` tunduk pada permission ceiling yang sama dengan entity
     CRUD (user tanpa grant → entry = error/null, bukan data bocor).
   - `api` hanya boleh memanggil Service/action yang dideklarasikan, bukan
     endpoint bebas.
3. **Async support**: nilai context bisa `T | Promise<T> | () => T |
Promise<T>`. Wajib ada state render eksplisit: `loading`, `error`,
   `fallback` per entry (atau page-level). Function = lazy, promise = eager.
4. **Interpolasi seragam**: token `{var.path}` berlaku di page title, section
   items, block params, dan `visible_when`/`compute` (via EvalContext).
5. **Engine tetap context-agnostic** — `EvalContext` tidak berubah; yang
   berubah adalah wiring renderer + deklarasi spec.

## Rencana Implementasi

### Phase 1 — Standard slot `user` + interpolasi di blocks (small) ✅ 2026-08-31

- `PageBlocks` (`PageRenderer.tsx`): inject `user` (dari session store) ke
  context yang sama dengan `titleCtx`; perluas `interpolate()` ke section
  items (title/text) dan block params.
- Dokumentasikan standard slots di spec kind Page/Form
  (`docs/spec/frontend/` + `docs/kind/` regenerated).
- Migrasi contoh: `profile.yaml` → pure YAML blocks memakai `{user.*}`;
  `assets/profile.js` dipertahankan sebagai contoh escape hatch.
- Verifikasi: `vitest` (interpolate + PageRenderer), browser profile page.

**Implementasi (changelog 015):**

- `SectionBlocks.tsx`: helper `t()` + prop `context` di semua block
  (hero/feature_grid/card/carousel/cta/alert) — interpolasi `{user.*}`.
- `PageRenderer.tsx`: `userCtx = { user: me }` dari session store,
  di-pass ke `PageBlockRenderer` → `SectionBlockRenderer`; title juga
  di-interpolasi dengan `userCtx`.
- `profile.yaml`: migrasi dari `mode: custom` asset ke pure YAML blocks
  (`{user.username}`, `{user.user_id}`, `{user.workspace}`, `{user.roles}`,
  `{user.permissions}`).
- Verifikasi: `tsc` bersih, `vitest` 166 pass, `formspec validate` 0
  problem, browser profile page render penuh.

### Phase 2 — `spec.context` declaration (medium) ✅ 2026-09-01

- `pkg/spec/frontend.go`: tipe `ContextDecl` + field `Context` di `PageSpec`
  & `FormSpec`; closed source set + validasi (unknown source → error).
- `internal/ui/meta.go`: bundle membawa context decls (Entry sudah membawa
  Spec, kemungkinan otomatis — verifikasi).
- Renderer: resolver context (parallel resolve, loading/error/fallback
  states), permission ceiling untuk `entity`/`api` sources (reuse
  `checkPermission` + `binds` pattern).
- `schemas/` regenerated; `docs/kind/` regenerated.
- Verifikasi: `go test`, `formspec validate`, vitest, contoh page dengan
  `source: entity` + `source: api`.

**Implementasi (changelog 016):**

- `pkg/spec/frontend.go`: `ContextDecl` (name/source/entity/id/call/params/
  value/expr/fallback), `ContextSourceSet`, `ValidateContextDecls`;
  `PageSpec.Context` + `FormSpec.Context`; validasi di `ValidatePageSpec`.
- `internal/genjsonschema/generator.go`: `ContextDecl` masuk sharedTypes
  allowlist → schema valid.
- `renderers/react-shadcn/src/hooks/useRenderContext.ts` (baru): resolver
  sequential (agar `expr` bisa refer entry sebelumnya), loading/error/
  fallback; `session`→user, `const`→value, `expr`→evalFormSpecExpr,
  `entity`→apiGet + permission ceiling `{entity}.view` + id token `{user.x}`.
- `PageRenderer.tsx`: `useRenderContext` di-wire ke PageBlocks; loading gate
  (skeleton) saat context async masih resolve.
- `profile.yaml`: demo `context:` const (`greeting`) + expr (`is_admin`).
- Verifikasi: `go test` 43 pass, `vitest` 166 pass, `formspec validate` 0
  problem, browser "Selamat datang, TestUser".

**Deferred — `source: api`:** service actions hanya di-route di surface
`/api/v1` (deny-by-default, API-key auth), tidak ada route UI-surface.
`api` source butuh task terpisah: UI-surface service action route
(`/{ws}/_ui/service/...`) sebelum bisa dipakai context. Saat ini deklarasi
`api` resolve ke fallback.

**SELESAI (changelog 002):** UI-surface service routes ditambahkan —
`GenerateUIServiceRoutes` (`/_ui/service/{module}/{service}/{action}`),
ter-register di router + `registerRouteWithPattern` handle `service`;
resolver `api` source memanggil route tsb dengan permission ceiling
`can(call)`. Verifikasi: 403 tanpa permission, 404 service tak ada,
200/validation dengan admin (permission `*`).

### Phase 3 — Reaktivitas (deferred, medium) ✅ 2026-09-01

- `source: entity` + `realtime: true` memakai `useRealtime` yang sudah ada
  untuk context yang auto-update (dashboard).
- Verifikasi: dashboard live-update test.

**Implementasi (changelog 003):**

- `ContextDecl.Realtime` (Go + TS) — flag `realtime: true`.
- `useRenderContext.ts`: untuk entity decl dengan `realtime: true`,
  subscribe via `subscribeRealtime(entity)` dan re-resolve entry saat
  event/reconnect (realtime non-durable → refetch wajib).
- `profile.yaml`: demo `module_status` (entity registry.module,
  realtime: true, fallback "—").
- Verifikasi: `go test` 43 pass, `vitest` 166 pass, `formspec validate` 0
  problem, browser render normal (fallback untuk user tanpa grant).

**Catatan demo live-update:** verifikasi penuh butuh user ber-permission +
producer event (mis. update record module). Mekanisme subscribe+refetch
ter-wire dan teruji via type-check + test.

## File Terdampak

- `pkg/spec/frontend.go` (ContextDecl, PageSpec.Context, FormSpec.Context)
- `internal/ui/meta.go` (bundle serialization — verifikasi)
- `renderers/react-shadcn/src/kinds/page/PageRenderer.tsx` (PageBlocks wiring)
- `renderers/react-shadcn/src/kinds/form/FormRenderer.tsx` (context resolver)
- `renderers/react-shadcn/src/lib/interpolate.ts` (perluasan token)
- `renderers/react-shadcn/src/lib/formspec-expr/eval.ts` (tidak berubah —
  hanya verifikasi)
- `registry/spec/modules/portal/pages/profile.yaml` (migrasi contoh)
- `schemas/`, `docs/kind/` (regenerated)

## Verifikasi

1. `rtk go test ./...` hijau setelah Phase 2 (spec validation).
2. `cd renderers/react-shadcn && rtk vitest run` hijau (interpolate,
   PageRenderer, FormRenderer).
3. `formspec validate -spec registry/spec -schema schemas` — 0 problem.
4. Browser: halaman profile pure YAML menampilkan `{user.username}` dll.
5. Contoh page dengan `source: entity`/`api` menampilkan loading → data,
   dan error state saat user tanpa grant.

## Referensi

- `docs/spec/frontend/` (kind Page/Form contract)
- `docs_internal/changelog/2026-08-31-013-profile-page.md` (latar custom asset)
- `docs_internal/changelog/2026-08-31-014-asset-path-spec-root-relative.md`
