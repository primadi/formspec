# 2026-08-17-001 — Idempotency prepare flow + spec.expose enforcement (todo 2.7, 2.8)

**Apa:** Fase 2 Engine Core dituntaskan untuk item 2.7 (idempotency) dan 2.8
(`spec.expose`).

**2.7 — Idempotency prepare flow.** `IdempotencyStore` sudah ada di layer store
sejak 2.1.4, tapi belum dipakai di jalur HTTP. Sekarang:

- `HandlerFactory.SetIdempotencyStore` + `RouterBuilder.SetIdempotencyStore`,
  di-wire di `resource/formspec.go` pada `New()` **dan** `ReloadSpec()` (store
  yang sama, key in-flight survive reload).
- `HandlePrepare` — endpoint dua-langkah `POST /{resource}/{action}/prepare`
  untuk action `idempotent: true` + `idempotency_key.from: server`; mengeluarkan
  key UUID v7. Route di-generate `generatePrepareRoutes` untuk kedua surface
  (external `/api/v1/...` + UI `/_ui/entity/...`): `create/prepare` dan
  `{action}/prepare`. Action `from: header`/`param` tidak punya prepare (klien
  suplai key sendiri) → 404.
- Enforcement di `HandleCreate` + `HandleCustomAction` via `beginIdempotent`:
  key `completed` → replay response asli (status + body tersimpan dalam
  envelope); key `pending` (in-flight) → `409 CONFLICT`; key `failed` → retry
  diizinkan; key baru/kedaluwarsa → klaim + eksekusi. Action idempotent tanpa
  key → `422 VALIDATION_ERROR`. `IdempotencyStore.Lookup` ditambahkan untuk
  membedakan pending vs failed (`TryClaim` menggabungkan keduanya).
- Frontend (`apiPost` dengan `Idempotency-Key` header) kini dihormati server.

**2.8 — `spec.expose` enforcement.** Sudah ter-implement di `generator.go`
(`GenerateRoutes` skip entity tanpa expose → 404; `generateRESTRoutes` filter
`exp.Actions`) + test, tapi belum ditandai di todo. Ditandai selesai.

**Kenapa:** melengkapi kontrak idempotensi `01-core-basic.md` §5 (double-submit
browser protection) dan deny-by-default exposure (D49) yang sudah jalan.

**File terdampak:**

- `internal/api/handler.go` — `SetIdempotencyStore`, `HandlePrepare`,
  `beginIdempotent`/`completeIdempotent`/`failIdempotent`, enforcement di
  `HandleCreate`/`HandleCustomAction`
- `internal/api/router.go` — forward `SetIdempotencyStore`, registrasi handler
  `prepare` (kedua register path)
- `internal/api/generator.go` — `generatePrepareRoutes` (external + UI)
- `renderers/jsonb-persist/idempotency.go` — `Lookup`
- `resource/formspec.go` — wire store di `New()` + `ReloadSpec()`
- `internal/api/idempotency_test.go` — 7 test baru (prepare, replay, 422, 409,
  custom action replay, route generation)
- `docs/plan/idempotency-prepare-flow.md` — plan baru
- `docs/renderers/jsonb-persist/04-query-and-keys.md` — §3 update (jalur HTTP aktif)
- `docs/plan/todo.md` — 2.7.1, 2.7.2, 2.8 selesai

**Verifikasi:** `go test ./...` → 549 pass, 9 gagal pre-existing
(Clinic-UI-Showcase e2e, sudah gagal di HEAD bersih sebelum perubahan ini).
`go build ./...` hijau.

**Referensi:** `docs/plan/idempotency-prepare-flow.md`, todo 2.7/2.8.
