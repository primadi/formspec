# GAP #30–#34 — Lapisan Script & Hook

Muncul saat menulis **guard script eksplisit** untuk tiga aturan keunikan yang
tidak bisa ditegakkan database (GAP-22):

| Aturan | Entity | Guard |
| --- | --- | --- |
| Satu harga per menu per cabang (D1) | `menu-item-price` | `cafe-master/scripts/guard_menu_item_price_unique.star` |
| Satu shift terbuka per (cabang, kasir) | `shift` | `cafe-order/scripts/guard_shift_open_unique.star` |
| Satu baris stok per (cabang, bahan) | `stock-level` | `cafe-stock/scripts/guard_stock_level_unique.star` (sengaja **tidak** dipasang — lihat GAP-33) |

Menulisnya membuka satu lapisan masalah yang belum pernah tersentuh: **lapisan
script & hook**. Semua yang di bawah berasal dari pekerjaan itu.

---

## Gap #30 — `ctx.db()` query di dalam transaksi aksi deadlock di SQLite ⚠️ Kuat

### Bukti

Dokumentasi paling jujur soal ini justru ada sebagai komentar di script contoh
milik FormSpec sendiri — `examples/arisan/spec/modules/arisan-field/transaction/contribution/scripts/validate.star`:

> `# aksi). Di SQLite (dev) ini DEADLOCK karena koneksi tunggal sedang dipegang`
> `# transaksi aksi. Di PostgreSQL (produksi) tidak deadlock. Fix upstream:`
> `# resolveRelations harus memakai txReadDB(ctx, s.db), bukan s.db.`

Jadi bila sebuah script melakukan query DB sementara transaksi aksi masih
terbuka, pada **SQLite (driver dev)** koneksi tunggal itu sudah dipegang →
deadlock.

### Dampak ke aplikasi kafe: **HIGH** untuk dev

Ketiga guard keunikan **harus** membaca dulu sebelum menulis. Artinya ketiganya
memakai jalur yang deadlock di SQLite:

- Guard tidak bisa diuji di dev.
- Digabung **GAP-24** (`formspec dev` mati di Windows), ini berarti lapisan yang
  paling butuh pengujian — integritas data — adalah lapisan yang **paling tidak
  bisa diuji**.
- Produksi (PostgreSQL) aman, jadi masalahnya spesifik dev. Tapi dev adalah
  tempat bug ditangkap.

### Usulan

Tutup ini sebelum menyarankan guard script sebagai pola umum. Kalau jalur baca
di dalam transaksi aksi memakai `txReadDB`, guard akan jalan di kedua driver —
dan pola "guard di script" jadi masuk akal. Selama belum, **constraint database
(GAP-22) tetap satu-satunya penutup yang benar** untuk keunikan.

---

## Gap #31 — Tidak ada API Starlark "find by field value" ✅ Terverifikasi (API absen)

### Bukti

Permukaan API Starlark yang terverifikasi ada:

| API | Berguna untuk cek keunikan? |
| --- | --- |
| `resource.field.X`, `resource.id` | ✅ baca nilai baris ini |
| `resource.set(k, v).save()` | ✅ tulis |
| `resource.fetch("entity", id)` | ❌ butuh **ID**, bukan filter |
| `ctx.next_key(field)`, `ctx.log.info(...)`, `ctx.lock`, `ctx.storage` | — |
| `ok(...)`, `fail(...)` | ✅ hasil guard |
| `ctx.db().query(sql, params)` | ✅ tapi **raw SQL** |

Tidak ada cara mencari record **berdasarkan nilai field** tanpa SQL mentah.
Yang dibutuhkan kira-kira:

```python
resource.find("cafe-master/menu-item-price",
              {"branch_id": branch_id, "menu_item_id": menu_item_id})
```

sehingga cukup dideklarasikan `uses.resources: [menu-item-price.find]`.

### Dampak ke aplikasi kafe: **HIGH**

Konvensi proyek (`AGENTS.md`), yang juga diulang di dokumentasi FormSpec:

> **Use ctx.\* primitives** — ctx.db, ctx.cache, ctx.lock … — **never raw SQL**

Ketiga guard terpaksa **melanggar konvensi** itu, karena alternatifnya adalah
membiarkan aturan bisnis tidak dijaga sama sekali. Jadi posisinya: pilih antara
melanggar konvensi, atau melanggar aturan bisnis.

Dan raw SQL di guard punya konsekuensi lanjutan: ia mengikat ke **nama tabel
fisik** (`cafe_master_menu_item_prices`) + kolom `deleted_at`, yang merupakan
detail internal persist backend. Ganti persist backend, guard rusak.

### Usulan

1. Tambah `resource.find(entityRef, filter)` — ini yang paling sering
   dibutuhkan script bisnis, bukan `fetch(id)`.
2. Kalau tidak, dokumentasikan secara eksplisit bahwa `ctx.db().query` adalah
   jalur **yang disahkan** untuk cek keunikan, beserta pola amannya.

---

## Gap #32 — Guard keunikan di script tidak atomik; butuh `ctx.lock` ✅ Terverifikasi (secara desain)

### Bukti

Pola guard yang mungkin hari ini:

```
SELECT ... LIMIT 1   →  kalau ada: fail()   →  kalau tidak ada: lanjut INSERT
```

Dua permintaan bersamaan bisa **sama-sama lolos SELECT** lalu **sama-sama
INSERT**. `ctx.lock` tersedia, jadi ini bisa ditutup — tapi lihat implikasinya.

### Dampak ke aplikasi kafe: **HIGH**

Konsekuensinya bersifat konseptual dan penting:

> Untuk menegakkan **satu** UNIQUE constraint, kita menulis
> lock + query + insert/update manual — **reimplementasi yang lebih rapuh
> daripada constraint yang tidak dihormati engine.**

Constraint database berlaku untuk **semua** jalur tulis (API, script, seed,
migrasi, SQL manual). Guard aplikasi hanya berlaku pada jalur yang melewatinya —
dan GAP-33 menunjukkan bahkan ada jalur yang **tidak** melewatinya.

Verifikasi negatifnya juga tidak ada: kalau ada script lain (atau operator) yang
menulis langsung, guard tidak tahu.

### Usulan

Ini bukan masalah yang seharusnya diselesaikan di aplikasi. **Prioritas:
perbaiki GAP-22** (IndexDecl dihormati + relation bisa diindeks). Guard script
hanya penjaga sementara yang harus ditandai `# TODO: hapus setelah GAP-22
ditutup` — sudah ditulis begitu di ketiga file guard.

---

## Gap #33 — `hooks:` dan `conditions:` tidak berlaku pada entity `summary` ⚠️ Kesimpulan desain

### Bukti

`stock-level` berkarakteristik `summary`. FormSpec menonaktifkan
create/update/delete permanen untuk summary (`derive.ts` → `hasDelete/hasCreate`,
dan komentar di `internal/api/generator.go` menyatakan backend tidak pernah
membuat route delete untuk `reference`/`summary`). Entity jenis ini **hanya**
ditulis oleh script.

Karena penulisan itu tidak melewati action pipeline, `hooks:`/`conditions:` yang
terpasang pada entity `summary` **tidak akan pernah dipanggil**.

> **Status: kesimpulan dari desain, belum diuji runtime.** Mengujinya butuh
> runtime → terblokir GAP-24. Kalau ternyata salah, GAP ini gugur — dan itu
> justru alasan catatan ini ditulis: supaya bisa diuji dan dibatalkan.

### Dampak ke aplikasi kafe: **MEDIUM–HIGH**

Bahayanya bukan pada absennya guard, tapi pada **penampakannya**:

1. Developer menulis `hooks:` pada entity `summary` → validasi **hijau**,
   `formspec check` **0 error** → terlihat terlindungi.
2. Guard tidak pernah jalan.
3. Invarian tidak dijaga, tanpa gejala.

Ini pola gagal-senyap yang sama persis dengan GAP-18/GAP-21/GAP-25, tapi di
tempat baru. Karena itu ketiga entity `summary` di aplikasi ini
(`stock-level`, `menu-cost`, `member-point`) **tidak** diberi hook, dan sebagai
gantinya invarian ditulis sebagai rujukan di dalam script penulisnya.

### Usulan

`formspec validate` harus **menolak atau memperingatkan** `hooks:`/`conditions:`
pada entity yang tidak punya action create/update (summary) — "hook ini tidak
akan pernah dipanggil". Murah, dan menutup satu kelas kesalahan senyap.

---

## Gap #34 — `HookDecl` tidak punya `uses`, sehingga akses script tidak terlihat di footprint ✅ Terverifikasi

### Bukti

`$defs.HookDecl` di schema:

```json
"HookDecl": {
  "properties": {
    "action":   { "type": "string" },
    "event":    { "type": "string" },
    "impl":     { "$ref": "#/$defs/ImplDecl" },
    "on":       { "$ref": "#/$defs/HookTiming" },
    "priority": { "type": "integer" }
  }
}
```

**Tidak ada `uses`.** Sementara `Action` punya `UsesDecl` (`{config, datastores,
db, kvstore, primitives, resources}`) — dan `UsesDbDecl` menyatakan tujuannya:
*"raw database access per category/module … cross-module write is high-risk
consent (D46)"*.

Diperkuat oleh keluaran nyata `formspec check -f spec -footprint` pada spec ini:

```
Module: cafe-order
  Required permissions: 15
  Uses declarations:    0
...
Module: cafe-stock
  Required permissions: 7
  Uses declarations:    0
```

Guard hook yang membaca DB lewat `ctx.db().query` **tidak muncul sama sekali**.
Module `cafe-master` — tempat guard `menu-item-price` berada — bahkan **tidak
terdaftar** di footprint, karena modul hanya muncul bila punya permission/uses.

### Dampak ke aplikasi kafe: **MEDIUM** (governance)

`uses` adalah mekanisme consent (D20/D46): "script ini boleh menyentuh apa".
Karena `HookDecl` tidak punya `uses`:

- Hook menjadi **jalur bypass** governance — ia bisa membaca/menulis tanpa
  deklarasi dan tanpa terlihat di footprint.
- Konsumen `kind: Subscription` punya masalah serupa bila implementasinya script.
- Untuk marketplace pihak ketiga (modul ter-`sign`, trust tier), ini berarti
  **pihak ketiga bisa menempelkan hook ke action milik module lain** dan akses
  datanya tidak akan terlihat saat audit consent.

### Usulan

1. Tambah `uses` ke `HookDecl`, dan masukkan hook ke perhitungan footprint.
2. `formspec check -footprint` sebaiknya menampilkan hook-nya juga — minimal
   sebagai baris `- hook <action> → <script ref>`.
3. Idealnya: `HookDecl` tanpa `uses` yang implementasinya script dan menyentuh
   DB → **peringatan validasi**.

---

## Yang Terverifikasi **Bagus**

| Aspek | Bukti |
| --- | --- |
| `hooks:` dengan `{on, action, impl}` diterima | 47 manifest, 0 problem |
| `impl: { type: script_ref, ref: module/nama }` valid | 3 hook terpasang, tanpa error meski file `.star` tidak diparse validator |
| **Auto-prefix `required_permission` bekerja** | Ditulis `orders.submit-order` → footprint menampilkan `cafe-order.orders.submit-order` |
| `contains`-style guard `conditions` sudah dipakai di action | `orders.void-order` dengan kondisi alasan void |
| Footprint per module berjalan | `formspec check -footprint` menampilkan permission + uses per module |

> Catatan: validator **tidak** memeriksa keberadaan file `.star` yang dirujuk
> `impl.ref` — tiga hook lolos tanpa error. Ini masuk keluarga GAP-21
> (referensi menggantung tidak divalidasi).
