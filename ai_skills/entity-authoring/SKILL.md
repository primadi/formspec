---
name: entity-authoring
description: >
  Gunakan skill ini saat membuat atau mengubah Entity kind — pemilihan field
  type, characteristic (master/transaction/reference/summary), natural_key,
  state machine, actions, events, permissions, dan expose. Trigger: percakapan
  menyebut "buat entity", "tambah field", "state machine", "transisi status",
  "natural key", atau mendesain data model aplikasi.
applies_to_kind: [Entity]
min_core_spec_version: "0.2.0"
metadata:
  version: "1.0"
  source: docs/spec/backend/01-core-basic.md + docs/spec/backend/02-core-extended.md
---

# Entity Authoring

## Urutan Kerja

1. **Discovery dulu** — pahami alur bisnis sebelum menulis YAML. Jangan lompat
   ke draft sebelum kebutuhan dikonfirmasi.
2. **Pilih characteristic** — menentukan perilaku data:
   - `master` — data stabil (produk, pelanggan, kategori). Boleh di-reference
     snapshot oleh transaksi.
   - `transaction` — append-heavy, berbasis waktu (order, invoice, journal).
     Wajib `transaction_date`.
   - `reference` — read-only seed data (provinsi, pajak, COA). Diisi via seeder.
   - `summary` — proyeksi system-managed; tidak ada CUD via API.
3. **Rancang fields** — lihat tabel tipe di bawah. Setiap field butuh `name`
   - `type`; `required`, `unique`, `title`, `description` sesuai kebutuhan.
4. **Lifecycle** — `plain_crud` untuk CRUD murni; state machine kalau ada
   alur status (draft → submitted → approved). State machine butuh `states`,
   `initial`, `transitions` (dengan optional `guard`), dan action `submit`.
5. **Actions** — custom action butuh `uses:` (primitives/resources/secrets)
   yang JUJUR: hanya deklarasikan yang benar-benar dipakai script.
   `formspec validate` men-scan honesty (undeclared usage → error).
6. **Expose** — API tidak terbuka by default; deklarasikan
   `expose: [{type: rest, actions: [list, find, ...]}]`.

## Tipe Field yang Tersedia

| Tipe                 | Catatan                                                                                                                        |
| -------------------- | ------------------------------------------------------------------------------------------------------------------------------ |
| `string`, `text`     | text = panjang, multiline                                                                                                      |
| `integer`, `decimal` | decimal punya `precision` + `scale` (mis. scale: 2 untuk uang desimal)                                                         |
| `money`              | selalu untuk nilai uang — jangan pakai decimal/float                                                                           |
| `boolean`            |                                                                                                                                |
| `date`, `datetime`   |                                                                                                                                |
| `enum`               | wajib `enum_values: [...]`                                                                                                     |
| `relation`           | referensi entity lain — `relation: {type: belongs_to, resource: "<module>.<entity>"}` (key-nya **`resource`**, bukan `target`) |

## Aturan Penting

- **Baris child diisi dengan memilih, bukan mengetik.** Kalau barisnya
  mereferensi master data (menu, bahan, produk, akun), deklarasikan `picker:`
  pada child field — bukan bikin kind/halaman sendiri:

  ```yaml
  - name: lines
    type: child
    child:
      picker:
        entity: cafe-master.menu-item
        filter: { is_available: "true" }
        display:
          { name_field: name, image_field: photo, columns: 3, search: true }
        map:
          ref_field: menu_item_id # WAJIB
          name_field: name_snapshot # snapshot
          price_field: unit_price_snapshot # snapshot
          quantity_field: quantity
          max_quantity: 20 # WAJIB bila ada quantity_field
  ```

  Field di `map` harus ada di `child.fields`. Tanpa `quantity_field`, satu pilih
  = satu baris (daftar/jurnal). Harga boleh dari entity lain via
  `display.price_entity` + `price_match_field` + `price_field`. Form menaruhnya
  dengan `render: { picker_panel: inline | aside }`.

- **Nilai yang tidak diisi user → `default_from` + `widget: hidden`**, bukan
  field yang tampil terisi. Contoh: `{ field: branch_id, widget: hidden,
default_from: "{session.branch_id}" }`; token `{now}`/`{today}` juga tersedia.
- **Uang selalu `money`** — bukan decimal/integer.
- **`money` berhitung langsung.** Nilainya objek `{amount, currency}`, tetapi
  `computed`/guard menulisnya sebagai operand biasa; jangan bongkar objeknya:

  ```yaml
  # BENAR
  - { name: change, type: money, computed: { formula: "tendered - amount" } }
  - {
      name: line_total,
      type: money,
      computed: { formula: "quantity * unit_price" },
    }
  - {
      name: total,
      type: money,
      computed: { formula: 'sum([i["line_total"] for i in lines])' },
    }
  ```

  | Aturan                                            | Konsekuensi                             |
  | ------------------------------------------------- | --------------------------------------- |
  | `money ± money`                                   | boleh; mata uang harus sama             |
  | `money × / number`                                | boleh (`money / money` → rasio angka)   |
  | `money` vs angka mentah (`total > 100`)           | **error** — pakai `amount(total) > 100` |
  | Operand bukan angka (objek non-money, list, teks) | **error**, bukan `0`                    |

  Untuk agregasi (`columns[].aggregate`, `totals[].fn`, widget
  `config.aggregate`): `sum`/`avg`/`min`/`max` atas field `money` menjumlahkan
  komponen `.amount`-nya; field non-numerik ditolak `formspec check` — bukan
  total `0`. `count` bebas.

- **`transaction_date` wajib** untuk characteristic `transaction`.
- **Natural key** (`natural_key: [field]`) untuk kode bisnis unik
  (mis. `INV-2026-001`) — dipakai next_key, bukan ID teknis.
- **Permission = resource + action** — jangan hardcode nama role di YAML.
- **Cross-module reference** harus dideklarasikan di `depends` module.
- **Reserved fields** tidak boleh dipakai sebagai nama field (id, created_at,
  updated_at, version, dst. — dikelola framework).

## Starlark — Batasan Dialek (script & hook)

Script `impl: {type: script_ref, ref: <module>/<name>}` dan `hooks:` ditulis
dalam **Starlark**, bukan Python penuh. Dua batasan yang paling sering membuat
script gagal:

- **Tidak ada implicit string concatenation.** `"a" "b"` adalah _syntax error_
  (`got string literal, want ','`) — gabungkan dengan `+`.
- **Query ber-parameter memakai satu argumen list**: `ctx.db().query(sql, [a, b])`,
  bukan varargs `ctx.db().query(sql, a, b)`.

Jalankan `formspec validate` — script yang dirujuk `impl.ref`/`hooks:`
dikompilasi, dan script yang gagal kompilasi atau tidak ditemukan dilaporkan
sebagai error.

## Validasi

Selalu tulis draft lewat `propose_spec_file` — validasi structural berjalan
otomatis (schema + engine + referensi lintas-manifest). Perbaiki semua
problem sebelum `apply_draft`.
