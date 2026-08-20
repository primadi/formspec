# Session Note — 2026-08-17 (Fase 4 completion: extension R/W + restore remap)

## Position

- Batch selesai: todo **4.3.1** (extension read), **4.3.2** (extension write),
  **4.8.3** (restore conflict remap + dry-run compatibility report) — semua ✅.
- **Fase 4 (4.1–4.10) kini 100% selesai** — tidak ada lagi item `⬜` di Fase 4.
- Changelog: `2026-08-17-035-extension-read-write.md`, `2026-08-17-036-restore-conflict-remap.md`.

## What was done

- **4.3.1/4.3.2 extension read/write** (`renderers/jsonb-persist/crud.go` +
  `internal/entity/registry.go`):
  - `validateKnownFields` menerima key namespace extension sebagai input write.
  - `Insert` & `Update` menulis payload namespace ke kolom `ext_{namespace}`
    (sebelumnya `splitExtensions` dipanggil tapi hasilnya tidak pernah ditulis).
  - `mergeExtensions` (baca) sudah ada, tetap dipakai di `hydrateAndCompute`.
  - Registry `GetEntityStore` me-wire `SetExtensions` dengan memindai semua
    entity `ExtendStorage` yang menarget entity ini.
  - Test baru: `renderers/jsonb-persist/extension_test.go`
    `TestEntityStore_ExtensionReadWrite`.
- **4.8.3 restore conflict remap** (`cmd/formspec/backup.go`):
  - `--conflict skip|overwrite|remap` (remap sebelumnya ditolak).
  - `restoreFrom` mengembalikan `RestoreReport` (total + per-entity).
  - `remapNaturalKey` menetapkan key baru (`<base>-r1`, `-r2`, …) dan insert
    sebagai record baru.
  - `--dry-run` mencetak compatibility report per-entity.
  - Test baru: `TestBackupRestoreRemap`.

## Verification

- `go test ./...`: **646 passed in 45 packages** (was 644).
- `go build ./...`: green. `go vet ./cmd/formspec/`: clean.

## Open questions / next

- **3.6.4 `formspec summary rebuild <entity>`** — ⏸️ **butuh design decision**:
  kontrak `sources`/`join_key`/`rebuild` untuk summary Entity belum
  dispesifikasikan di `pkg/spec` (tidak ada field tsb di `EntitySpec`), dan
  `docs/renderers/jsonb-persist/04-query-and-keys.md` §4 menyatakan detail
  populasi summary mengikuti "bagaimana Summary dipopulasikan dari event
  durable" yang belum ada (projection engine belum ada). Jangan invent
  contract — perlu keputusan desain dulu.
- Next tractable unchecked items (non-deferred, non-external):
  - Fase 1 `1.4.12` MoneyType FX — spec "Open", needs design decision → stop & report.
  - Fase 2 `2.9.4` ctx.db module-scoped — design-heavy.
  - Fase 3 `3.1.1a` validate honesty scan Starlark — butuh `internal/starlark` analyzer penuh.
  - Fase 5+ (frontend) — banyak item, sebagian butuh keputusan desain.
- Deferred by design: Control Plane, Marketplace, Cloud deploy, Fase 12 DNS/Resend.
