# 2026-09-28-001 — Pelanggaran constraint dijawab 409, dan subscription bisa `uses`

**Apa:** menindaklanjuti "perbaiki bug yang timbul" dari gelombang alur sesi meja.
Dua bug platform diperbaiki, satu butuh keputusan pemilik, satu tidak dikejar
karena tidak terbukti. Rencana: `docs_internal/plan/perbaikan-bug-timbul-2026-09-28.md`.

## B1 — Pelanggaran constraint dijawab **500 INTERNAL_ERROR** → **409 CONFLICT**

**Diukur, bukan disimpulkan.** Probe tiga bentuk pelanggaran unik terhadap server
in-process menunjukkan **ketiganya 500**:

| Bentuk                                                              | Sebelum              | Sesudah                                   |
| ------------------------------------------------------------------- | -------------------- | ----------------------------------------- |
| Sesi TERBUKA kedua di satu meja (partial unique index, kafe 10.34c) | 500 `INTERNAL_ERROR` | **409** `CONFLICT` — `…: dining_table_id` |
| `dining-table` `(branch_id, code)` duplikat (composite unique)      | 500 `INTERNAL_ERROR` | **409** `CONFLICT` — `…: branch_id, code` |
| `qr_token` duplikat (`unique: true` + `natural_key`)                | 500 `INTERNAL_ERROR` | **409** `CONFLICT` — `…: qr_token`        |

**Tiga mata rantai, semuanya harus dibuka:**

1. `renderers/jsonb-persist` **tidak punya sentinel** untuk pelanggaran
   constraint — error driver naik apa adanya sebagai string.
2. `internal/api` `isConflictError` hanya mencocokkan substring `"version
conflict"`/`"not found"`; tidak ada cabang untuk `UNIQUE constraint failed`.
3. `writeStoreError` jatuh ke `default:` → 500.

**Biayanya bukan kosmetik.** 500 adalah kelas yang memicu page operator, jadi
setiap nilai duplikat di produksi mengangkat alarm palsu **sekaligus**
mengubur kerusakan nyata di antaranya; dan klien tidak bisa membedakan "kirim
nilai lain" dari "coba lagi nanti", sehingga tidak ada pesan yang bisa
ditampilkan. Di jalur QR ini user-facing: halaman masuk tamu menampilkan 500.

**Yang dikerjakan:** `renderers/jsonb-persist/constraint.go` (baru) —
`ErrUniqueViolation`, `UniqueViolationError{Detail, Err}`, `classifyConstraintError`.
Frase yang dikenali: `UNIQUE constraint failed:` (SQLite, termasuk suffix kode
perluasan `(2067)`), `duplicate key value violates unique constraint` + `DETAIL:
Key (...)` (PostgreSQL/pgx), dan SQLSTATE `23505` telanjang. Klasifikasi dipasang
di **titik tulis** (`Insert` baris induk, insert children, update children),
bukan di lapisan HTTP — supaya script, seed, dan operator script mendapat kelas
error yang sama.

Dua keputusan kecil yang keduanya soal pesan: detail dinormalisasi ke **nama
field logis** (prefix tabel dibuang; `cafe_order_table_sessions._dining_table_id`
→ `dining_table_id`) dan **kolom scope milik framework disaring** (`tenant_id`)
— kalau tidak, pesannya menunjuk field yang tidak pernah dikirim pemanggil.

**409, bukan 422:** nilainya **valid**; yang salah keadaannya (sudah ada). Sama
kelas dengan version conflict, dan memberi tahu klien untuk mengganti nilai, bukan
memperbaiki bentuk request atau mengulang buta.

**Test:** 8 bentuk teks driver di `constraint_test.go` — termasuk **negatif**
(FK, NOT NULL, dan error lain tidak boleh diklasifikasi; error yang tidak cocok
harus dilewatkan **utuh**, karena mewrapping semuanya akan membuat setiap error
storage tampak konflik), idempotensi, dan nil. Plus 3 bentuk nyata lewat HTTP di
`constraint_status_e2e_test.go`, yang menegaskan **status dan nama field**, serta
satu test negatif bahwa relasi rusak tetap **bukan** 409.

## B3 — Handler subscription tidak bisa mendeklarasikan `uses`

`SubscriptionSpec` tidak punya field `Uses`, sementara dispatcher memodelkan
handler sebagai action sintetis yang hanya membawa name + impl. Di
`ProdMode`/`StrictMode`, `ctx.db` lalu gagal dengan
_"USES_VIOLATION: ctx.db used but the action declares no uses block — add
uses.primitives: [db]"_ — **instruksi yang manifest-nya tidak punya tempat untuk
dituruti**. Instruksi mustahil lebih buruk daripada tidak ada instruksi.

**Yang dikerjakan:** `Uses` pada `SubscriptionSpec` (bentuk dan alasan yang sama
dengan `Action` dan `HookDecl`, yang sudah lebih dulu punya), diteruskan ke
`actionSpec` di `dispatchOne`. Test: `TestDispatcher_CarriesUsesToTheHandler`
menegaskan `uses` sampai ke executor **lewat action**, bukan lewat params.

**Koreksi saat implementasi:** `uses` adalah **saudara** `handler`, bukan anaknya
— percobaan pertama menaruhnya di dalam blok `handler` dan langsung tertangkap
validator (`schema: /spec/handler: validation failed`), karena `handler` bertipe
`ImplDecl` (type/ref saja).

**Satu hal yang sengaja TIDAK dibangun.** Setelah diperiksa: skema `Subscription`
yang digenerate sudah membawa `uses` bertipe (`$ref: #/$defs/UsesDecl`), jadi
struktur tervalidasi. Yang tidak bisa ditangkap schema adalah typo **nama
primitive** (`uses.primitives: [dbb]`) — dan itu tetap tertangkap di runtime oleh
`checkPrimitive` yang sama dengan action, karena ia membandingkan dengan daftar
yang dideklarasikan. Menambah whitelist di `formspec check` berarti
**menduplikasi set tertutup** yang bisa menyimpang dari `context.go`.

**Asimetri didokumentasikan, bukan diubah.** Terverifikasi dari kode:
`checkPrimitive` dipanggil untuk **tepat 7** primitive datastore (`db`, `cache`,
`lock`, `queue`, `pubsub`, `storage`, `kvstore`); `config`, `log`, `now`,
`today`, `next_key`, `unit` tidak melewatinya. `docs/reference/primitives.md`
kini memuat tabelnya + alasannya (yang diperiksa adalah yang menyentuh
infrastruktur ber-batas) dan tabel "di mana `uses` boleh ditulis", plus `uses`
deklaratif pada `cafe-master/table-occupancy` sebagai contoh nyata. Ini juga
menjawab kenapa `ctx.now()` di `close_session_on_clear.star` **lolos**: bukan
karena aman dari pemeriksaan, melainkan karena `now` memang tidak diperiksa.

## B2 — Sesi ditinggalkan mengunci meja: **butuh keputusan pemilik**

Tidak dikerjakan. Setiap opsi punya konsekuensi produk, dan tiga di antaranya
terhalang gap lain (rekomendasi: tamu masuk ke sesi terbuka meja itu, yang
membutuhkan `kind: Service` publik = 10.39 lebih dulu). **Efek B1 pada bug ini:**
tamu kedua kini menerima **409 dengan pesan yang benar** ("meja ini sedang punya
kunjungan terbuka") alih-alih 500 — jujur dan tidak menyesatkan, tetapi tetap
berarti tamu tidak bisa memesan. **B2 tidak ditutup oleh B1.** Opsi lengkapnya di
plan.

## B4 — vitest flaky: tidak dikejar tanpa bukti

3 test gagal pada satu run, lalu 536/536 sembilan kali berturut-turut. Test mana
yang gagal tidak tertangkap, jadi tidak ada perubahan kode. Suite ini **tidak
bisa** diklaim hijau secara deterministik; yang bisa diklaim: sembilan run
terakhir hijau, penyebab run yang gagal tidak diketahui.

## Verifikasi

`go test ./...` hijau · `formspec validate` kafe 89 manifest/0 problem ·
`make e2e-kafe` 1 passed · `gofmt -l` bersih · `make generate-schema` dijalankan
(`schemas/kinds/Subscription.schema.json` ikut berubah).

## File

`renderers/jsonb-persist/{constraint.go, constraint_test.go, crud.go}`,
`internal/api/handler.go`, `internal/subscription/{dispatch.go, registry_test.go}`,
`pkg/spec/resources.go`, `resource/constraint_status_e2e_test.go`,
`examples/kafe/spec/modules/cafe-master/subscriptions/table-occupancy.yaml`,
`docs/reference/primitives.md`, `schemas/`
