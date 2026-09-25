# Plan — FormSpecExpr: paritas kutip tunggal + gate statis yang benar-benar menahan

**Tanggal**: 2026-09-24 · **Status**: In progress
**Referensi**: `docs/spec/frontend/08-formspec-expr.md` §2 & §4,
`cmd/formspec/check.go`, `renderers/react-shadcn/src/lib/formspec-expr/lexer.ts`,
todo 5.11.1–5.11.3, changelog `2026-08-23-005` (yang mencatat ini sebagai "batasan")

## Masalah

Halaman `/kafe/app/pos/cafe-master/promos?action=create&form=promo-form&mode=drawer`
menampilkan empat banner:

```
Expression error: unexpected token: ' at line 1:16; unexpected token: ' at line 1:27
Expression error: unexpected token: ' at line 1:16; unexpected token: ' at line 1:22
Expression error: unexpected token: ' at line 1:22; unexpected token: ' at line 1:32
Expression error: unexpected token: ' at line 1:22; unexpected token: ' at line 1:31
```

Empat baris itu persis empat field `promo-form.yaml` yang memakai literal
berkutip tunggal (kolom 16/22 = posisi kutip pembuka, 27/31/32 = kutip penutup):

| Baris | Ekspresi                           |
| ----- | ---------------------------------- |
| 27    | `fields.type == 'percentage'`      |
| 28    | `fields.type == 'fixed'`           |
| 29    | `fields.type == 'buy_x_get_y'`     |
| 30    | `fields.type == 'buy_x_get_y'`     |
| 35    | `fields.applies_to == 'menu_item'` |
| 36    | `fields.applies_to == 'category'`  |

Dua field pertama (35, 36) juga memicu banner, sehingga `promo-form` praktis
tidak bisa dipakai: field `percent`/`value`/`buy_qty`/`get_qty`/`menu_item_id`/
`menu_category_id` semuanya gagal dievaluasi.

## Akar masalah (terverifikasi)

1. **Lexer klien hanya mengenal kutip ganda.** `lexer.ts` `nextToken()` hanya
   punya `case '"'`; `'` jatuh ke `default` → token `ILLEGAL` dengan literal
   `'` → parser melaporkan `unexpected token: '`. Pesan + kolom di atas adalah
   keluaran persis dari jalur ini.
2. **Kutip tunggal SAH menurut kontrak.** `08-formspec-expr.md` §2: FormSpecExpr
   adalah "subset **ekspresi** Starlark", dan "satu grammar dipakai bersama guard
   sisi-server". Server memakai Starlark sungguhan (`internal/starlark`), yang
   menerima kutip tunggal — `TestEvaluateGuard_SumLineBuiltin` lulus dengan
   `sum_line('debit')`. Jadi sisi klien yang menyimpang, bukan manifest-nya.
3. **Gate deploy-time tidak menahan.** `formspec check -f examples/kafe/spec`
   melaporkan `0 error(s)` untuk ekspresi yang mustahil dievaluasi klien.
   `validateExprGrammar` hanya memeriksa `ctx.`, kata kunci terlarang, dan
   delimiter seimbang — tidak ada pemeriksaan karakter/token sama sekali,
   sehingga kontrak §4 ("ekspresi yang lolos apply dijamin bisa dievaluasi")
   saat ini **tidak benar**.

Skala: bukan kasus tunggal. 16 situs di `examples/` memakai kutip tunggal
(kafe 11, Clinic-UI-Showcase 5). Changelog `2026-08-23-005` sudah mencatat ini
sebagai batasan dan menyarankan penulis manifest menulis kutip ganda di dalam
kutip tunggal YAML — workaround yang justru menyembunyikan pelanggaran kontrak.

## Perubahan

| File                                                             | Perubahan                                                                                                      | Effort |
| ---------------------------------------------------------------- | -------------------------------------------------------------------------------------------------------------- | ------ |
| `renderers/react-shadcn/src/lib/formspec-expr/lexer.ts`          | `nextToken()` menerima `'` maupun `"`; `readString(quote)` menutup pada kutip yang sama dan menghormati escape | small  |
| `renderers/react-shadcn/src/lib/formspec-expr/formaexpr.test.ts` | Test kutip tunggal: lexing, evaluasi, escape, dan ekspresi `promo-form` yang sesungguhnya                      | small  |
| `cmd/formspec/check.go`                                          | `validateExprGrammar` memindai token dengan aturan lexer klien; karakter di luar grammar → error deploy-time   | medium |
| `cmd/formspec/check_test.go`                                     | Test gate: kutip tunggal diterima, karakter asing & string tak tertutup ditolak                                | small  |
| `docs/spec/frontend/08-formspec-expr.md` §2                      | Menegaskan literal string boleh berkutip tunggal atau ganda (subset Starlark)                                  | small  |
| `docs_internal/plan/todo.md`                                     | Item 5.11.6 (paritas literal) + 5.11.7 (utang: gate masih duplikasi, belum berbagi fixture)                    | small  |

Dependensi: tidak ada urutan ketat. (1)(2) memperbaiki bug yang dilaporkan;
(3)(4) menutup celah kelas yang sama agar tidak terulang lewat ekspresi lain.

## Keputusan

- **Memperbaiki interpreter, bukan manifest.** Mengubah `promo-form.yaml` ke
  kutip ganda akan membuat halaman jalan, tetapi meninggalkan 15 situs lain
  rusak dan kontrak §2 tetap dilanggar. Arah perbaikan: klien mengikuti Starlark.
- **Gate statis memindai token, bukan hanya delimiter.** Gate saat ini lulus
  untuk ekspresi yang mustahil dievaluasi; itu persis kegagalan yang membuat bug
  ini sampai ke runtime. Setelah perubahan, karakter di luar grammar dan string
  tak tertutup ditolak saat `formspec check`.
- **Tidak** menambah parser FormSpecExpr di Go. Gate cukup memindai token
  (bukan membangun AST) agar duplikasi tetap kecil; risiko drift dicatat sebagai
  item 5.11.7, bukan diklaim sudah selesai.
- `None` (Starlark) tetap diterima di klien seperti sebelumnya — nilainya
  `null`, sehingga `fields.items != None` tetap bekerja.

## Verifikasi

- `npx vitest run src/lib/formspec-expr/formaexpr.test.ts` hijau (termasuk test
  regresi ekspresi `promo-form`).
- `npx tsc -b` bersih.
- `go test ./cmd/formspec/...` hijau.
- `go run ./cmd/formspec check -f examples/kafe/spec` → 0 error (kutip tunggal
  kini sah).
- Manual: `npm run build` lalu buka
  `/kafe/app/pos/cafe-master/promos?action=create&form=promo-form&mode=drawer`
  → tidak ada banner "Expression error"; `percent` hanya muncul saat
  `type = percentage`, `menu_item_id` hanya saat `applies_to = menu_item`.
