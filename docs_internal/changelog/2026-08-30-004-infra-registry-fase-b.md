# 2026-08-30-004 — Infra Registry fase B: App Registry selection + uses.datastores enforcement

**Plan:** `docs/plan/infra-registry-3-level.md` (fase B)

## Apa yang diubah

**App Registry selection (level 2 dari arsitektur 3-level):**

- `pkg/spec/resources.go`: `AppSpec.Datastores` dan `ModuleSpec.Datastores`
  (map `primitive → service`, key `primitive/alias` untuk named logical
  primitive — fase C). `ModuleSpec.Datastore` (single string) tetap ada
  sebagai legacy shorthand `db`-only.
- `resource/datastoreregistry.go`: `LoadManifests` kini memproses
  `kind: App` (selection + module→App ownership) dan `kind: Module`
  `datastores` (override per primitive). Chain resolusi plain call:
  **module binding (legacy) → module `datastores` → App selection →
  per-primitive registry default** (`chainTarget`). Validasi fail-loud:
  key primitive tidak dikenal, service tidak teregistrasi, service tidak
  serve primitive, named key di module level (ditolak sampai fase C).
- `internal/starlark/context.go`: gate baru `checkDatastoreAccess` —
  action yang mendeklarasikan `uses.datastores` hanya boleh menyentuh
  primitive yang tercantum; pelanggaran → `DATASTORE_ACCESS_DENIED`
  (spec `platform/06-datastore.md` §6). Action tanpa deklarasi
  `datastores` tidak dibatasi oleh gate ini (`uses.primitives` tetap
  menjadi gate kasar di strict mode).

## Kenapa

Model 2-level yang disepakati: Infra Registry (cloud control) meregistrasi
service fisik; App Registry (app builder) memilih logical primitive —
default per App bisa berbeda antar App (app1 → pg-main, app2 →
pg-analytics) dan dioverride per module. Deklarasi `uses.datastores`
menjadi enforceable, bukan sekadar dokumentasi.

## File terdampak

- `pkg/spec/resources.go` — `AppSpec.Datastores`, `ModuleSpec.Datastores`
- `resource/datastoreregistry.go` — `moduleApp`/`moduleSel`/`appSel`/
  `appNamed`, `parseDatastoreKey`, `validateSelection`, `chainTarget`,
  `LoadManifests` App/Module processing, `Resolve` chain
- `internal/starlark/context.go` — `checkDatastoreAccess` + wiring di 7
  case primitive
- Test: `resource/datastoreregistry_test.go` (+3: AppSelection,
  ModuleOverride, LoadErrors_AppSelection),
  `internal/starlark/primitive_test.go` (+1: DatastoreAccessDenied)

Verifikasi: 974 test Go lulus tanpa regresi.
