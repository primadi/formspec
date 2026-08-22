# Fase 6 Dogfooding — Field-Level Security (Fase H)

**Tanggal**: 2026-08-20 · **Sequence**: 010
**Plan**: `docs/plan/fase6-dogfooding-auth-module.md` (Fase H)

## Apa yang diubah

Menegakkan field-level security di layer response API (todo 6.7). Struct field
sudah ada di Fase 1; Fase H = enforcement.

### Fase H — selesai (sebagian)

- **H2** (6.7.2) `required_permission` field-level — `sanitizeData` menghapus
  field dari response bila caller tidak punya permission.
- **H3** (6.7.3) `exclude` per-surface — field dengan `exclude: [public_api]`
  disembunyikan di surface external, tetap tampil di UI.
- **H5** (6.7.5) `masked` — auto-mask di response (mis. `password_hash` →
  `ab****cd`).
- **H6** (6.7.6) `computed` — sudah dievaluasi di persist layer (`evaluateComputed`).
- **H1** (6.7.1) `classification` — struct ada; tagging di log/export di-defer
  (butuh kebijakan log/export terpusat).
- **H4** (6.7.4) `encrypted` AES-256-GCM at-rest — **di-defer** (butuh master
  key/keystore; keputusan `FORMSPEC_MASTER_KEY` vs vault).

### Implementasi

- `internal/api/fieldsec.go` (baru) — `sanitizeData` (masked + required_permission
  - exclude per-surface), `surfaceFromPath`, `maskValue`, `HandlerFactory.sanitize`
    (via `specLookup` + identity + surface), `sanitizeList`.
- Diterapkan di semua handler response entity: List, Find, Create, Update, Delete,
  Cancel, Amend, Deactivate/Reactivate, Custom Action.
- Fix test time-dependent: `uses_enforcement_e2e_test.go` hardcoded `2026-08-17`
  → `recentDate()` (pre-existing, bukan dari perubahan ini).

## Kenapa

Field-level security mencegah kebocoran data sensitif (masked, permission-gated,
surface-excluded) di response API — prasyarat keamanan produksi.

## File yang terkena dampak

- `internal/api/fieldsec.go` + `fieldsec_test.go` (baru)
- `internal/api/handler.go` — sanitize di semua handler response
- `resource/uses_enforcement_e2e_test.go` — fix tanggal hardcoded

## Verifikasi

- `go build ./...` + `go test ./...` hijau.
- Test `sanitizeData`: masked, required_permission (ada/tidak), exclude per-surface,
  tidak memutasi input — hijau.
