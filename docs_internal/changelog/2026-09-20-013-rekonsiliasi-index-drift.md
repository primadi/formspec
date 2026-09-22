# 3.9 (sebagian) — Rekonsiliasi index yang definisinya berubah

**Tanggal:** 2026-09-20 · **TODO:** `examples/kafe/gaps_found/TODO.md` 3.9 · **Plan:** `docs_internal/plan/kafe-sisa-gap.md`

## Masalah

Ditemukan saat verifikasi 3.8: membuat order di cabang kedua pada DB kafe yang
sudah ada gagal `UNIQUE constraint failed: cafe_order_orders.tenant_id,
cafe_order_orders._number`. Dibuktikan dengan membandingkan dua database:

| Database | Index natural key `number` |
| --- | --- |
| segar (`migrate apply` pada DB baru) | `(tenant_id, _branch_id, _number)` |
| lama (`.formspec/kafe.db`) | `(tenant_id, _number)` |

Spec kafe memang mendeklarasikan `natural_key_rule.scope_field: branch_id`, jadi
DB lama **tertinggal** — bukan bug loader/spec. Diff migrasi sebelumnya hanya
membandingkan **nama** index ("sudah ada → lewati"), sehingga definisi yang
berubah tidak pernah dibangun ulang.

## Yang diubah (`renderers/jsonb-persist/migrate.go`)

1. **Perbandingan bentuk, bukan nama.** `indexShape` (unique + daftar kolom +
   predikat parsial) di-parse dari SQL yang ada dan dari DDL yang diinginkan;
   bila berbeda → `DROP INDEX` + `CREATE` ulang. Normalisasi menoleransi
   perbedaan kosmetik (`quoting`, `ASC/DESC`, `USING btree`, predikat dalam
   tanda kurung, cast `::text`) supaya plan tetap konvergen — plan yang tidak
   pernah kosong tidak bisa dipakai sebagai gerbang.
2. **`driftedIndexes` juga berjalan di jalur "checksum sama".** Checksum
   mem-fingerprint *manifest*, bukan storage; tanpa pemeriksaan ini, DB yang
   tertinggal tidak akan pernah diperbaiki karena `migrate plan` menjawab
   "tidak ada yang perlu dilakukan".
3. `existingIndexes` (set nama) → `existingIndexDefs` (nama → SQL), plus
   `dropIndexSQL` (PostgreSQL butuh nama ter-schema).

## Bukti

| Perintah | Hasil |
| --- | --- |
| `go test ./renderers/jsonb-persist/ -run "ChangedIndexDefinition\|Drifted"` | **PASS** — (a) nama sama/kolom berbeda → DROP+CREATE → konvergen; (b) checksum sama, storage drift → repair → konvergen |
| `go test ./...` | hijau |
| `make lint` | 0 issues |

## Status akhir: selesai (3.9) + satu jebakan DX & satu gap baru (3.10)

Empat sub-masalah ditutup, semuanya dengan test pengunci:

| # | Sub-masalah | Perbaikan |
| --- | --- | --- |
| i | Diff index membandingkan **nama** saja | `indexShape` (unique + kolom + predikat parsial, dinormalisasi) |
| ii | Pemeriksaan drift tidak jalan di jalur "checksum sama" | `driftedIndexes` di cabang itu |
| iii | DDL dibangun **hanya** dari `DiffShapes(snapshot, manifest)` — snapshot merekam *niat*, bukan storage | drift storage ikut diperiksa di `planEntityChange` |
| iv | **Kolom turunan scope field natural key** tidak dibuat di jalur alter | `derivedColumnFields` menandai scope field juga |

**Jebakan DX yang menyesatkan diagnosis:** `formspec migrate` default DSN-nya
`.formspec/data.db`, sedangkan `formspec dev` memakai `dsn:` dari
`formspec-app.yaml` (`.formspec/kafe.db`). Semua pemeriksaan awal menunjuk **DB
yang salah** — plan tampak "senyap" padahal DB dev memang tertinggal.

**Bukti E2E (DB dev `kafe.db`):**

| Langkah | Hasil |
| --- | --- |
| `migrate plan --dsn sqlite:.formspec/kafe.db` | `[derived] index_changed: declared index definition drifted from storage — rebuilt` + `field_projection_changed branch_id` |
| hapus snapshot (bootstrap, karena 3.10) → `migrate apply` | `Applied 24 migration(s)`; index jadi `(tenant_id, _branch_id, _number)`, kolom `_branch_id` ada |
| `POST /_ui/entity/cafe-order/order` cabang **B** | **201** `ORD-2026-00002` (sebelumnya 500 `UNIQUE constraint failed`) |
| …cabang **A** | **201** `ORD-2026-00004` (deret sendiri) |

**Gap baru dicatat sebagai 3.10:** pada DB lama, `migrate apply` menolak seluruh
run dengan 18 perubahan `[lossy] field_removed is_active` — `is_active` bukan
field manifest (engine menambahkannya dari `soft_deactivate`), jadi perubahannya
tidak bisa dideklarasikan dan DB seperti itu tidak bisa dimigrasi tanpa membuang
snapshot. Akar yang dicurigai: snapshot lama merekam field turunan engine sebagai
field yang dideklarasikan.

**Status repo:** `go test ./...` hijau · `make lint` 0 issues · kafe `validate`
0 problem.
