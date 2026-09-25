# 2026-09-24-007 — FormSpecExpr: paritas kutip tunggal + gate statis yang benar-benar menahan

## Apa yang diubah

Halaman `/kafe/app/pos/cafe-master/promos?action=create&form=promo-form&mode=drawer`
menampilkan empat banner `Expression error: unexpected token: '` (kolom 1:16/22
adalah kutip pembuka, 1:27/31/32 penutup). Akarnya: `FormSpecExpr` adalah subset
**Starlark** (`docs/spec/frontend/08-formspec-expr.md` §2) dan Starlark
menerima kutip tunggal, tetapi lexer klien (`lib/formspec-expr/lexer.ts`)
hanya punya `case '"'` — `'` jatuh ke `ILLEGAL`. Enam field `promo-form`
(`percent`, `value`, `buy_qty`, `get_qty`, `menu_item_id`, `menu_category_id`)
memakai `visible_when: "fields.type == 'percentage'"`, sehingga semuanya gagal.
Server tidak terpengaruh: `internal/starlark` adalah Starlark sungguhan, dan
`TestEvaluateGuard_SumLineBuiltin` sudah lulus dengan `sum_line('debit')` —
sisi klien yang menyimpang dari kontrak.

**Klien (`lib/formspec-expr/lexer.ts`).** `nextToken()` menerima `'` maupun `"`;
`readString(quote)` menutup hanya pada kutip pembukanya (sehingga `"it's"` tidak
berakhir di apostrof) dan menangani escape kedua gaya kutip. String tak
tertutup kini menghasilkan `ILLEGAL`, bukan `STRING` — menerimanya sebagai string
sah adalah fail-safe yang dilarang §4.

**Gate deploy-time (`cmd/formspec/check.go`).** `validateExprGrammar` dahulu
hanya memeriksa `ctx.`, kata kunci terlarang, dan delimiter seimbang. Itu
meloloskan **manifest yang dijamin gagal di runtime**: `formspec check -f
examples/kafe/spec` melaporkan `0 error(s)` untuk keenam ekspresi di atas,
padahal kontrak §4 menjanjikan ekspresi yang lolos apply bisa dievaluasi.
Sekarang gate memindai token dengan aturan lexer klien: string (kedua gaya
kutip) bersifat opaque — delimiter di dalamnya tidak dihitung, sehingga
`fields.name != "("` tidak lagi salah dianggap tak seimbang — karakter di luar
grammar, string tak tertutup, dan operator asing (`=`, `&`, `|`) ditolak.

## Dampak & verifikasi

- `examples/` berisi 16 situs berekspresi kutip tunggal (kafe 11,
  Clinic-UI-Showcase 5); semuanya kini benar. `formspec check` pada 8 app contoh
  tidak memunculkan false positive baru — sisa error hanya kelas `uses.resources`
  yang sudah ada sebelumnya (arisan, Clinic-UI-Showcase, tidak tersentuh).
- Regresi sempat lolos saat pengembangan: versi pertama gate menolak `*`
  (`sum([i.quantity * i.unit_price for i in fields.items])`), ketahuan karena
  `examples/cafe` justru gagal; diperbaiki sebelum commit.
- Bukti: `vitest` **365 lulus** (25 file, +15 baru), `tsc -b` bersih,
  `go test ./...` hijau, `gofmt` bersih. Browser:
  `/kafe/app/pos/cafe-master/promos?...&form=promo-form` → tanpa banner;
  `type = Percentage` memunculkan `percent`, `type = Fixed` memunculkan `value`,
  `applies_to = Menu item` memunculkan `menu_item_id`.
- Dokumentasi: `docs/spec/frontend/08-formspec-expr.md` §2 (kutip tunggal/ganda
  setara) + §4 (gate wajib memindai token, bukan hanya delimiter).

## Sisa (lihat todo)

- **5.11.7 ⏸️** Paritas grammar masih dijaga manual: gate Go memindai token
  dengan aturan yang **diduplikasi** dari lexer TypeScript, tanpa fixture
  bersama. Kelas bug ini (klien menolak yang server terima) bisa terulang untuk
  konstruk lain; test yang ada hanya mem-pin ekspresi yang sudah ada. Versi
  pertama gate menolak `*` — ketahuan hanya karena `examples/cafe` kebetulan
  memakainya.
- **5.11.8 ⏸️** Identifier tak dikenal dalam perbandingan lolos sebagai `null`:
  `fields.status == open` (tanpa kutip) dievaluasi `false` **tanpa warning**,
  jadi gate juga meloloskannya (teramati: `evalFormSpecExpr` →
  `{value:false, valid:true, warnings:[]}`). Berbeda kelas dari item ini —
  ekspresi ini _bisa_ diparse, hanya bermakna lain — tapi sama-sama membuat
  manifest salah tampak sah.
