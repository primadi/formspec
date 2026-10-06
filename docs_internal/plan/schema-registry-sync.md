# Schema registry sync plan

## Context

`formspec validate` against the default registry fails on `App.spec.public_entities`, while the local schema override (`--schema schemas`) succeeds. The root cause is schema drift: generated files under `schemas/` are stale relative to the current Go contract in `pkg/spec`.

## Scope

- Rebuild generated JSON Schema from `pkg/spec`.
- Verify default validator behavior with the cached registry-backed path.
- Confirm the Kafe public app validates without requiring the local override.

## Files involved

- `pkg/spec/resources.go`
- `cmd/formspec-gen-schema/main.go`
- `schemas/**`
- `examples/kafe/spec/apps/kafe-qr.yaml`

## Effort

Small. This is a deterministic regeneration and verification pass.

## Validation

- `make generate-schema`
- `./formspec validate --spec examples/kafe/spec`

## 2026-10-06 — Akar masalah `Seed` 404: `dist/` di `.gitignore`

### Context

Item todo **3.6.7** mencatat registry online ketinggalan kontrak + kind `Seed`
404 (`formspec validate` mode registry, exit 2). Regenerasi sudah lama hijau dan
`schemas/kinds/Seed.schema.json` sudah ada sejak 2026-09-25, jadi teka-tekinya
bukan generator: **file-nya ada di disk, tetapi tidak pernah ter-commit.**

### Akar masalah (terukur)

`.gitignore` baris 45 memuat pola telanjang `dist/` (ditambahkan 2026-09-10 untuk
artefak build Go). Pola itu **juga mencocoki `schemas/dist/`**:

```
$ git check-ignore -v schemas/dist/v1/kinds/Seed.schema.json
.gitignore:45:dist/     schemas/dist/v1/kinds/Seed.schema.json
```

Karena file `schemas/dist/` yang lama sudah **tracked**, ignore tidak berlaku
untuk mereka — sehingga `git add schemas/dist` (persis instruksi di
`schemas/README.md`) terlihat berhasil, padahal **setiap file kind BARU dilewati
diam-diam**. Bukti:

| Pemeriksaan | Nilai |
| --- | --- |
| file di `schemas/dist/v1/kinds/` (disk) | 34 |
| file di sana yang tracked git | 33 |
| selisih | `Seed.schema.json` (v1 **dan** latest) |
| `git ls-tree -r HEAD \| grep -i seed.schema.json` | hanya `schemas/kinds/...` |
| `git add --dry-run schemas/dist/v1/kinds/Seed.schema.json` | "paths are ignored" |
| commit terakhir yang menyentuh `dist` | hanya root schema + `index.json`, tanpa `kinds/` |

Akibatnya `index.json` (tracked) menyebut `Seed` sementara request
`/v1/kinds/Seed.schema.json` → 404 untuk **setiap** proyek ber-`kind: Seed`.

### Perubahan

- `.gitignore`: tambah `!schemas/dist/` (negasi) + komentar alasan.
- `schemas/dist/{v1,latest}/kinds/Seed.schema.json`: di-stage (sebelumnya
  terlewat karena ignore).
- `scripts/publish-schemas.sh`: **guard baru** — setiap file `schemas/dist/`
  dicek `git check-ignore --no-index`; ada yang ter-ignore → `exit 1` dengan
  pesan sebab + saran negasi. Kelas kegagalan ini jadi berisik di titik
  kejadiannya, bukan 10 hari kemudian lewat 404.
- `schemas/README.md`: catatan `dist/` + langkah verifikasi `git status --short
  schemas/dist`; catatan bahwa per-kind schema **bukan** schema mandiri
  (`$ref`-nya di-resolve dari `$defs` root `formspec.schema.json` — temuan
  sampingan, bukan cacat generator).

### Validation

- `git check-ignore --no-index schemas/dist/v1/kinds/Seed.schema.json` → no match;
  delapan `dist/` lain (`./dist`, `renderers/react-shadcn/dist`, …) tetap IGNORED.
- Registry-mirror lokal (`python3 -m http.server` di `schemas/dist`, tanpa
  `--schema`): `service-demo` **13 manifest / 0 problem**, `kafe` **88 / 0**
  (sebelumnya `404 .../kinds/Seed.schema.json`, exit 2).
- `make publish-schemas` → guard hijau; dengan negasi dihapus sementara → guard
  merah, `exit 1`, 74 file terdeteksi.
- Sisa (butuh push, tidak bisa diverifikasi lokal): `curl` live → 200 dan
  `formspec validate` tanpa `--schema` di kafe → 0 problem.

### Effort

Small — satu baris ignore + dua file ter-stage + guard shell.

### Files

- `.gitignore`, `schemas/dist/{v1,latest}/kinds/Seed.schema.json`,
  `schemas/dist/{v1,latest}/index.json`, `scripts/publish-schemas.sh`,
  `schemas/README.md`
