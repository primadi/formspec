# 2026-09-25-008 — Publish schema: direktori versi dibersihkan; artefak `Migration` basi dihapus

**Konteks.** Pertanyaan pemakaian: "todos 3–9 sudah?" Memverifikasi item 5 (kafe
10.5) berarti menjalankan `formspec validate`, dan di sanalah dua masalah sembuh
muncul secara berdampingan.

## 1. `validate` mode registry gagal pada kind `Seed` (404)

```
formspec validate
→ schema v1: fetch https://schemas.formspec.dev/v1/kinds/Seed.schema.json: 404
```

Tidak ada kind schema yang punya blok `$defs` sendiri — tiap `$ref` diselesaikan
dari **root** schema — jadi saya periksa root online:

|                      | kind di root | `Seed`    | `SeedEntity` di `$defs` |
| -------------------- | ------------ | --------- | ----------------------- |
| online               | 35           | **tidak** | **tidak**               |
| hasil generate lokal | 35           | ya        | ya                      |

Root online memuat `Migration` sebagai gantinya. Jadi **bukan cacat generator**:
artefak lokal sudah benar (`schemas/kinds/Seed.schema.json` memuat
`$ref: #/$defs/SeedEntity`, dan root hasil generate mendefinisikannya). Yang
tertinggal adalah **publish** — schema yang dilayani lebih tua dari generator,
sehingga `formspec validate` gagal dengan **exit 2** pada setiap proyek yang
memakai `kind: Seed` (kafe: 4 manifest).

Ini kelas yang sudah terlacak: `2.11.10` (kind `Workspace`) dan `3.6.7`
(registry basi vs field kontrak). Tiket yang sama — `make publish-schemas` +
push menutup semuanya sekaligus. **Tidak dikerjakan di sini**: langkah terakhirnya
deploy ke R2 dan kredensialnya tidak ada di lingkungan ini. Sementara itu
`--schema schemas` → **85 manifest, 0 problem**.

## 2. `Migration.schema.json` masih di-publish padahal kind-nya sudah ditolak

Drift lebih awal terlihat dari jumlah file: `schemas/dist/v1/kinds/` berisi 36
file untuk **35** kind. Penyebabnya ada di `scripts/publish-schemas.sh`:

```bash
mkdir -p "$DIST_DIR/$VERSION/kinds"   # tidak pernah menghapus
cp "$OUT_DIR"/kinds/*.schema.json ...
```

`mkdir -p` + `cp` tidak pernah membuang apa yang **berhenti** di-generate, jadi
kind yang di pensiun tetap menyimpan schema-nya di set terbitan selamanya.
Terukur: `Migration.schema.json` masih ter-commit dan ter-serve, sedangkan engine
menolaknya — `formspec validate --no-schema` → `unknown kind "Migration" for spec
version v1`. File basi di sini **tidak inert**: ia memberi tahu setiap konsumen
(dan daftar `kinds` di `index.json`) bahwa kind itu sah.

**Perbaikan** (3 baris): `rm -rf "$DIST_DIR/$VERSION"` sebelum `mkdir -p`.
Sesudah staging ulang: `kinds/` 36 → **35**, `Migration` hilang dari root dan dari
`index.json`, dan ketiga daftar (root, `index.json`, file) kini sepakat.

## Catatan: schema yang ter-commit basi terhadap generator

Staging ulang juga menunjukkan `schemas/` yang ter-commit tertinggal dari
generator saat ini — perbedaannya adalah field `options.multiple` (pekerjaan
`options`/5.10.19 yang sedang berjalan, changelog `2026-09-25-007`). Jadi
`schemas/` memang di-commit dan memang perlu regenerasi berkala; `make
generate-schema` + `publish-schemas.sh` adalah jalurnya.

## Dampak & status

- `scripts/publish-schemas.sh` — bersihkan direktori versi sebelum menyusun ulang.
- `schemas/**` hasil staging ulang ada di working tree, **belum di-commit**: isinya
  bercampur dengan regenerasi milik pekerjaan `options` yang sedang berjalan.
  Commit-nya harus bersama `options`, atau sesudahnya — bukan sendiri-sendiri.
- Gate: `bash -n scripts/publish-schemas.sh` ok; `go test ./cmd/formspec/
./internal/genjsonschema/` lulus.

Referensi plan: `docs_internal/plan/todo.md` 2.11.10 / 3.6.7;
`docs_internal/plan/schema-registry-sync.md`.
