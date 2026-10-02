# Migrasi ditolak → permukaan repair harus bisa dibuka

**Tanggal**: 2026-09-28
**Status**: implementasi
**Kontrak terkait**: `docs/spec/backend/01-core-basic.md` §4.4, `docs/cli-tools/02-formspec-cli.md` §4
**Effort**: small

---

## 1. Masalah (terukur)

Dev server kafe gagal boot:

```
[formspec] engine boot: sync schema: apply migrations: apply migrations: 1 destructive change(s) refused
  - [lossy] index_added idx_cafe_order_table_sessions_dining_table_id: new unique index (1 row(s) affected)
    repair the duplicates first — run the repair once via `formspec repl`, then apply again
exit status 1
```

Penolakannya **benar**: entity `cafe-order.table-session` mendeklarasikan partial
unique index `(dining_table_id) WHERE status = 'open'` (kafe 10.34c), dan DB dev
memuat **19 sesi `open` pada satu meja** (data uji yang menumpuk **sebelum**
10.34c + subscription penutup sesi mendarat). Migrasi yang menambahkan constraint
tidak boleh dijalankan di atas data yang melanggarnya — itu keputusan yang
disengaja (`01-core-basic.md` §4.4).

Yang **tidak benar** adalah jalur perbaikannya:

> §4.4: _"operator merapikan datanya sekali lewat `formspec repl -f repair.star`,
> lalu apply dijalankan lagi."_

`formspec repl` memanggil `formspec.New` → `reg.SyncSchema` → **penolakan yang
sama**, jadi operator tidak bisa mengikuti alur yang didokumentasikan. Terukur
(2026-09-28, spec + DB kafe):

```
$ formspec repl --spec examples/kafe/spec --dsn sqlite:.formspec/kafe.db -e 'print("alive")'
Error: sync schema: apply migrations: apply migrations: 1 destructive change(s) refused
exit status 1
```

Pesan `Remedy` pada penolakan itu **menunjuk ke alat yang tidak bisa start** —
deadlock, bukan sekadar pesan kurang jelas. Ini juga membatalkan klaim prosa di
todo 4.2.5/4.2.6 ("operator merapikannya lewat `formspec repl -f <script>`, verb
nyata") — verb-nya ada, tetapi tidak dapat membuka DB bersangkutan.

## 2. Prinsip yang dipakai

- Penolakan migrasi **tetap fatal** untuk boot normal (dev/serve) — itu gerbangnya.
- Yang ditambahkan bukan "lewati gerbang", melainkan **permukaan yang secara
  eksplisit tidak menyentuh schema**: boot tanpa sync. Perbaikan data adalah
  tindakan operasional sekali jalan, dan ia harus bisa dijalankan justru ketika
  schema belum bisa diselaraskan.
- Tidak ada tempat untuk DML di spec; repair tetap script sekali jalan (tidak
  berubah).

## 3. Perubahan

| #   | File                                          | Perubahan                                                                                                                                                                                                                                                                                                                      |
| --- | --------------------------------------------- | ------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------ |
| 1   | `resource/formspec.go`                        | `Config.SkipSchemaSync bool`; `New` melewati `reg.SyncSchema` bila true. Sisa wiring boot tidak bergantung pada sync (registry/router in-memory), jadi aman untuk REPL.                                                                                                                                                        |
| 2   | `cmd/formspec/repl.go`                        | flag `--no-sync` → `SkipSchemaSync: true`; masuk usage/help + komentar paket.                                                                                                                                                                                                                                                  |
| 3   | `renderers/jsonb-persist/diff.go`, `alter.go` | `Remedy` menyebut perintah yang **bisa dijalankan**: `formspec repl --no-sync -f repair.star` (+ menyebut bahwa repair menulis lewat `ctx.db()`, karena `resource.*` di console tidak ter-wire).                                                                                                                               |
| 4   | `resource/skip_schema_sync_test.go` (baru)    | Kunci kelas bugnya: spec v1 (index non-unik) → data duplikat → spec v2 (index unik): `New` default **gagal** dengan "destructive change(s) refused"; `New` dengan `SkipSchemaSync` **berhasil**; setelah baris duplikat dirapikan lewat koneksi itu, `New` default **berhasil** dan index-nya ada. Dibuktikan gagal tanpa fix. |
| 5   | `docs/cli-tools/02-formspec-cli.md`           | `repl`: dokumentasikan `--no-sync` + koreksi contoh yang salah (`invoice.load(...)` — dan `load` reserved word di Starlark); `migrate`: sebut flag pada alur §4.4.                                                                                                                                                             |
| 6   | `docs_internal/plan/todo.md`                  | Item baru di Fase 4.2 + koreksi prosa 4.2.5/4.2.6.                                                                                                                                                                                                                                                                             |
| 7   | `docs_internal/changelog/2026-09-28-00N-*.md` | Catatan perubahan.                                                                                                                                                                                                                                                                                                             |
| 8   | DB dev kafe                                   | Perbaikan data sekali jalan (tutup sesi `open` ganda) supaya dev boot; bukan perubahan code.                                                                                                                                                                                                                                   |

## 4. Dependensi & urutan

1 → 2 → 3 (3 butuh nama flag final) → 4 (test butuh 1) → 5 → 8 → 7.

## 5. Verifikasi

- `go test ./resource/ -run SkipSchemaSync -v` (gagal sebelum fix, hijau sesudah).
- `go test ./renderers/jsonb-persist/` (pesan Remedy tidak menyentuh logika).
- `go test ./cmd/formspec/`.
- E2E: `formspec repl --spec examples/kafe/spec --dsn sqlite:.formspec/kafe.db --no-sync -e ...`
  menjalankan repair, lalu `formspec dev` boot tanpa penolakan.
- `gofmt -l` + `go build ./...`.

## 6. Di luar scope (tidak dikerjakan di sini)

- Subcommand `formspec migrate repair` yang men-generate dedupe otomatis:
  keputusan produk baru (bagaimana cara "merapikan" berbeda per entitas) —
  hanya dicatat sebagai item `[⏸️]` bila terbukti perlu.
- Menghidupkan `resource.*` di console (`loadFn`/`findFn`): perubahan lebih besar,
  tidak dibutuhkan untuk membuka deadlock-nya.
