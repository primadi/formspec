# Fase 6 Dogfooding — Demo Merge + Docs + Regresi (Fase L)

**Tanggal**: 2026-08-21 · **Sequence**: 014
**Plan**: `docs/plan/fase6-dogfooding-auth-module.md` (Fase L)

## Apa yang diubah

Penutup Fase 6: verifikasi demo merge, update docs, dan regresi penuh.

### Fase L — selesai

- **L1/L2** Demo merge — auth module (bundled + embedded) diverifikasi bekerja
  di project nyata (`verticals/billing/spec`): login `admin/admin` → token,
  meta bundle berisi 5 entity auth UIExposed (`api-key`, `app-membership`,
  `role`, `role-assignment`, `user`); `session`/`workspace` tersembunyi.
  `formspec generate auth` menyalin module ke `external/auth` untuk
  dikustomisasi (merge via copy folder). Catatan: `verticals/reference-app`
  (komposisi) gagal boot karena issue pre-existing (report `trial-balance` gl
  malformed) — bukan dari perubahan ini.
- **L3** Docs — `docs/spec/platform/08-project-layout.md` ditambah catatan
  `formspec.core` sebagai bundled module (dogfooding). Spec docs lain bersifat
  normatif (kontrak) — tidak diubah.
- **L4** Regresi — `go test ./...` hijau; `formspec validate` pada billing
  (problem pre-existing, bukan auth); `make lint` (74 issue pre-existing, tidak
  ada di file baru Fase 6).

## Kenapa

Menutup Fase 6: auth framework kini dibangun sebagai modul FormSpec (dogfooding)
yang bekerja di project nyata, mergeable via `external/`/`spec/modules/`.

## File yang terkena dampak

- `docs/spec/platform/08-project-layout.md` — catatan bundled auth module

## Verifikasi

- `go build ./...` + `go test ./...` hijau.
- Boot billing: login OK + meta bundle berisi 5 entity auth.
- `formspec validate` + `make lint` — hanya issue pre-existing.
