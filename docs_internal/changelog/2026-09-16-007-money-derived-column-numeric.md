# 2026-09-16-007 — Kolom turunan `money` numerik (`.amount`), #23

Item `examples/kafe/gaps_found/TODO.md` **3.2** (gap **#23**). Plan:
`docs_internal/plan/money-derived-column-numeric.md`.

**Akar masalahnya.** `fieldTypeToSQL` tidak mengenal `FieldMoney`, jadi kolom
turunan field `money` berisi **teks JSON objeknya** (`_price text GENERATED …
json_extract(data,'$.price')`). Konsekuensinya `"9000"` dianggap lebih besar
dari `"10000"`: sortir harga, filter rentang, dan laporan margin salah **tanpa
gejala** — dan index di atasnya hanya mempercepat jawaban yang salah. Catatan
kafe bahkan menyimpan konsekuensi ini sebagai alasan sengaja **tidak** memasang
`index: true` pada harga.

**Yang dikerjakan.** `generateGeneratedColumn` kini menerima tipe field; untuk
`money` ekspresinya membaca `.amount` (`json_extract(data,'$.x.amount')` di
SQLite, `data->'x'->>'amount'` di Postgres) dan tipenya `numeric(20,8)`.
`fieldTypeToSQL` mendapat `case spec.FieldMoney`, dan jalur ALTER di
`migrate.go` ikut meneruskan tipe sehingga tabel lama pun konsisten.
`columnRefExpr` didahulukan ke kolom turunan sebelum ekspresi `.amount`: kolomnya
sekarang numerik, jadi memakai ekspresi justru membuat index tidak terpakai pada
setiap sort/filter money.

**Bukti runtime** (DDL nyata dari `migrate apply` pada DB segar):
`_price numeric(20,8) GENERATED ALWAYS AS (CAST(json_extract(data,
'$.price.amount') AS REAL)) STORED` + `CREATE INDEX … (_price)` +
`idx_cafe_order_orders_total_amount`. **Bukti perilaku:**
`TestEntityStore_MoneySortAndRangeAreNumeric` (9000 & 10000 → sort naik memberi
9000 lebih dulu; `price >= 9500` hanya 10000) dan
`TestGenerateEntityDDL_MoneyDerivedColumnReadsAmount` (kedua driver).

**Adopsi kafe.** `menu-item-price.price` dan `order.total_amount` kini
`index: true` — workaround "sengaja tidak diindeks" dihapus — dan kolom Total di
table POS ditandai `sortable`.

Normatif: `docs/spec/backend/05-field-types.md` §2.2, termasuk batasannya (nilai
kanonik tetap di payload; kolom turunan adalah proyeksi, cast SQLite `REAL`), dan
catatan bahwa DB lama perlu recreate/`kind: Migration`.
