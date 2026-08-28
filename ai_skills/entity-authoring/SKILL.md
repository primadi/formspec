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
   + `type`; `required`, `unique`, `title`, `description` sesuai kebutuhan.
4. **Lifecycle** — `plain_crud` untuk CRUD murni; state machine kalau ada
   alur status (draft → submitted → approved). State machine butuh `states`,
   `initial`, `transitions` (dengan optional `guard`), dan action `submit`.
5. **Actions** — custom action butuh `uses:` (primitives/resources/secrets)
   yang JUJUR: hanya deklarasikan yang benar-benar dipakai script.
   `formspec validate` men-scan honesty (undeclared usage → error).
6. **Expose** — API tidak terbuka by default; deklarasikan
   `expose: [{type: rest, actions: [list, find, ...]}]`.

## Tipe Field yang Tersedia

| Tipe | Catatan |
|---|---|
| `string`, `text` | text = panjang, multiline |
| `integer`, `decimal` | decimal punya `precision` + `scale` (mis. scale: 2 untuk uang desimal) |
| `money` | selalu untuk nilai uang — jangan pakai decimal/float |
| `boolean` | |
| `date`, `datetime` | |
| `enum` | wajib `enum_values: [...]` |
| `relation` | referensi entity lain; wajib `target` |

## Aturan Penting

- **Uang selalu `money`** — bukan decimal/integer.
- **`transaction_date` wajib** untuk characteristic `transaction`.
- **Natural key** (`natural_key: [field]`) untuk kode bisnis unik
  (mis. `INV-2026-001`) — dipakai next_key, bukan ID teknis.
- **Permission = resource + action** — jangan hardcode nama role di YAML.
- **Cross-module reference** harus dideklarasikan di `depends` module.
- **Reserved fields** tidak boleh dipakai sebagai nama field (id, created_at,
  updated_at, version, dst. — dikelola framework).

## Validasi

Selalu tulis draft lewat `propose_spec_file` — validasi structural berjalan
otomatis (schema + engine + referensi lintas-manifest). Perbaiki semua
problem sebelum `apply_draft`.
