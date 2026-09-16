# Plan — Kolom turunan `money` numerik (kafe TODO 3.2 / gap #23)

Sumber: `examples/kafe/gaps_found/TODO.md` 3.2; `08-ddl-index-dan-devtools.md`
gap #23.

## Masalah

`fieldTypeToSQL` tidak punya `case spec.FieldMoney` → jatuh ke `default: "text"`,
sehingga kolom turunan field `money` berisi **teks JSON objeknya**:

```sql
_price text GENERATED ALWAYS AS (json_extract(data, '$.price')) STORED
```

Akibatnya `"9000"` dianggap **lebih besar** dari `"10000"`: sortir harga, filter
rentang, dan laporan "margin tertinggi" salah tanpa gejala — dan index di atas
kolom itu hanya mempercepat jawaban yang salah.

## Konstruk

Kolom turunan `money` membaca **`.amount`** dan bertipe numerik:

| Driver     | Ekspresi                                         |
| ---------- | ------------------------------------------------ |
| SQLite     | `CAST(json_extract(data, '$.x.amount') AS REAL)` |
| PostgreSQL | `data->'x'->>'amount'`                           |

Tipe kolom: `numeric(20,8)`.

Konsekuensi yang dinyatakan di dokumen: nilai kanonik tetap objek
`{amount, currency}` di payload; kolom turunan adalah **proyeksi untuk
sortir/agregasi**, bukan sumber kebenaran nilai (SQLite cast-nya `REAL`).

## File

- `renderers/jsonb-persist/ddl.go` — `generateGeneratedColumn` menerima tipe
  field; `fieldTypeToSQL` mendapat `case money`.
- `renderers/jsonb-persist/migrate.go` — jalur ALTER ikut meneruskan tipe.
- `renderers/jsonb-persist/crud.go` — `columnRefExpr` mendahulukan kolom turunan
  (sekarang numerik) supaya index benar-benar terpakai.
- `renderers/jsonb-persist/money_ddl_test.go` — 2 test (DDL kedua driver +
  sortir/rentang numerik).
- kafe: `menu-item-price.price` + `order.total_amount` → `index: true`; table POS
  → `sortable: true`.
- `docs/spec/backend/05-field-types.md` §2.2 (baru).

## Bukti

`migrate apply` pada DB segar → `_price numeric(20,8) GENERATED ALWAYS AS
(CAST(json_extract(data, '$.price.amount') AS REAL)) STORED` +
`CREATE INDEX … (_price)` + `idx_cafe_order_orders_total_amount`. Test: 9000
sebelum 10000 saat sort naik; `price >= 9500` hanya mengembalikan 10000.
`go test ./...` hijau · kafe `validate` 0 problem.

## Sisa

DB lama perlu recreate/`kind: Migration` untuk mengubah tipe kolom.

## Estimasi: **small-medium**
