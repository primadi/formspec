# Strategi Skema

**Updated:** 2026-07-16 · Status: Outline

> Outline: heading menetapkan cakupan; isi ditulis bertahap.

## 1. Hybrid JSONB
Layout kolom, tipe, path payload. Ini satu-satunya strategi skema renderer ini
— PersistBackend dengan strategi berbeda (mis. tiap field jadi kolom nyata)
adalah implementasi terpisah, bukan mode di dalam `jsonb-persist`; lihat
[`../README.md`](../README.md) soal tempat implementasi lain terdaftar.

## 2. Extension Column
Tiap extension ([`../../spec/backend/03-entity-extension.md`](../../spec/backend/03-entity-extension.md))
dapat **kolom fisik baru**, bukan tabel terpisah, bukan nested path di dalam
`data`:

```sql
-- Tabel asli (dimiliki module billing)
CREATE TABLE financial.billing_invoices (
  id          uuid PRIMARY KEY DEFAULT gen_uuid_v7(),
  tenant_id   uuid NOT NULL,
  version     integer NOT NULL DEFAULT 1,
  data        jsonb NOT NULL DEFAULT '{}',   -- field inti, dimiliki billing
  ...
);

-- Setelah di-extend module my-customization
ALTER TABLE financial.billing_invoices
  ADD COLUMN ext_kastem1 jsonb NOT NULL DEFAULT '{}';
```

Kenapa kolom terpisah, bukan nested path (`data->'ext'->'kastem1'`): uninstall
lewat `ALTER TABLE ... DROP COLUMN` sifatnya metadata-only dan instan —
alternatif nested path butuh `UPDATE` di setiap baris untuk mencabut key-nya
(mahal dan berisiko di tabel besar), dan setiap pembacaan `data` ikut
men-detoast payload extension walau tidak dibutuhkan. Prefiks `ext_`
menghindari tabrakan dengan kolom reserved framework (`data`, `tenant_id`,
`version`, `doc_status`, dst).

Registry namespace (memenuhi §3 kontrak "namespace tidak boleh dipakai
ulang"):

```sql
CREATE TABLE formspec_extensions (
  resource    text NOT NULL,   -- billing/invoice
  namespace   text NOT NULL,   -- kastem1
  module      text NOT NULL,   -- my-customization
  status      text NOT NULL,   -- active | dropped
  created_at  timestamptz NOT NULL,
  PRIMARY KEY (resource, namespace)
);
```

**Status implementasi:** DDL `ADD COLUMN ext_*` dan registry `formspec_extensions`
di atas nyata dan dieksekusi migration engine saat extension dipasang.
**Tapi belum ada jalur baca/tulis runtime ke kolom itu** — `ExtensionStore`
(tipe yang seharusnya menjawab `invoice.ext("kastem1").project_code`,
[`../../spec/backend/03-entity-extension.md`](../../spec/backend/03-entity-extension.md)
§1) ada di kode tapi tidak pernah dipanggil dari `EntityStore` maupun HTTP
handler. Migrasi membuat kolomnya; belum ada yang mengisi/membacanya hari
ini. Uninstall (`DROP COLUMN`) juga belum ada implementasinya sama sekali —
lihat [`01-architecture.md`](01-architecture.md) §4.

## 3. Index Generation
`persist.indexes` dan field extension ber-`index: true`
([`../../spec/backend/03-entity-extension.md`](../../spec/backend/03-entity-extension.md)
§4) diterjemahkan ke *generated column* + index biasa. Untuk kolom
**extension**, ini sudah benar dialeknya:

```sql
ALTER TABLE financial.billing_invoices
  ADD COLUMN _project_code VARCHAR
    GENERATED ALWAYS AS (ext_kastem1->>'project_code') STORED;
CREATE INDEX ON financial.billing_invoices (tenant_id, _project_code) WHERE deleted_at IS NULL;
```

Field natural key mendapat generated column + unique constraint yang sama
polanya: `UNIQUE (tenant_id, _field) WHERE deleted_at IS NULL`. Field tanpa
`index: true` (default) tidak menyentuh DDL sama sekali — tetap murni bagian
dari `data`/kolom extension JSONB.

**Bug dialek untuk kolom entity dasar (bukan extension):** generator
kolom-generated untuk field `data` dasar (bukan `ext_*`) hari ini **selalu**
memakai sintaks `json_extract(data, '$.field')` (SQLite) terlepas driver
aktif — pada Postgres seharusnya `data->>'field'`. `json_extract` tidak
dikenal Postgres. Ini bug nyata, belum ketahuan karena test DDL Postgres
yang ada cuma menguji nama schema, bukan sintaks generated column-nya.
Jangan anggap jalur Postgres untuk field ber-`index: true` sudah teruji
benar sampai ini diperbaiki.

## 4. Materialized Path — Optimasi Query Tree

Field `relation` self-referential dengan marker `tree: true`
([`../../spec/backend/05-field-types.md`](../../spec/backend/05-field-types.md)
§4) memicu strategi **materialized path** di backend ini — bukan recursive
CTE.

### 4.1 Motivasi

Recursive CTE membebani query planner untuk setiap query hierarki — bebannya
linier terhadap kedalaman tree dan tidak bisa di-index secara konvensional.
Materialized path menggantinya dengan **prefix-match string sederhana** yang
memanfaatkan B-tree index standar: `descendant_of` menjadi `LIKE 'prefix.%'`,
bukan recursive CTE. Trade-off: path harus di-maintain pada setiap mutasi
struktur tree (move, reparent) — tapi operasi tersebut jauh lebih jarang
daripada query baca hierarki di hampir semua domain bisnis (COA, org chart,
kategori produk).

### 4.2 Kolom `_tpath_`

Setiap field `relation` ber-`tree: true` mendapat satu generated column
tersembunyi:

```sql
-- Field "parent_id" bertipe relation self-referential dengan tree: true
ALTER TABLE financial.gl_accounts
  ADD COLUMN _tpath_parent_id TEXT NOT NULL DEFAULT '';
CREATE INDEX ON financial.gl_accounts (tenant_id, _tpath_parent_id)
  WHERE deleted_at IS NULL;
```

Prefiks `_tpath_` membedakan kolom path dari generated column biasa (`_`
untuk index generation, §3). Nilai path disimpan di key tersembunyi
`__tpath.<field_name>` di dalam `data` JSONB; generated column
mengekstraknya untuk indexing.

### 4.3 Format Path

Path adalah string yang merepresentasikan rantai ancestor dari root ke node
itu sendiri, dipisahkan separator `.` (titik):

```
<root_id>.<child_id>.<grandchild_id>
```

- **Root node** (parent null): path = `""` (string kosong)
- **Node biasa**: path = path parent + `.` + id sendiri
- **Separator `.`** dipilih karena tidak muncul di UUID v7 maupun integer
  (PK SQLite) — bebas dari false-positive match

Path **tidak** menggunakan kode bisnis (mis. `001.01.001`) — ia murni
berdasarkan PK (`id`) agar independen dari konvensi penamaan yang bisa
berubah. Kode bisnis tetap jadi field `string` biasa milik app; ia tidak
terlibat dalam query hierarki.

### 4.4 Maintenance

Path dikelola framework **server-side, selalu** — klien tidak bisa menulis
`__tpath.*`:

| Operasi | Perilaku |
|---|---|
| **Create root** (parent null) | path = `""` |
| **Create child** (parent = X) | path = path(X) + `.` + id_baru |
| **Move / reparent** | path node + seluruh subtree dihitung ulang dalam satu transaksi; lock di parent lama & baru mencegah concurrent reparent |
| **Delete** | `relation.on_delete` berlaku — `cascade` menghapus subtree (path ikut terhapus), `restrict` menolak jika masih ada anak |

### 4.5 Translasi Operator Tree

Lihat [`04-query-and-keys.md`](04-query-and-keys.md) §6 untuk translasi
lengkap operator `descendant_of` / `ancestor_of` / `child_of` / `root` ke
prefix-match B-tree. Ringkasan:

| Operator kontrak | Strategi jsonb-persist |
|---|---|
| `descendant_of=<X>` | `_tpath_… LIKE '<path(X)>.<X>.%'` — prefix-match, indexed |
| `ancestor_of=<X>` | Parse path(X), `WHERE id IN (…)` — lookup PK langsung |
| `child_of=<X>` | `parent_id = <X>` — field relation biasa, bukan path |
| `root` | `parent_id IS NULL` — indeks standar pada kolom FK |

**Tidak ada recursive CTE di backend ini.**
