# `MenuItem.When`: evaluasi klien + gate callable deploy-time + `today()`

**Tanggal**: 2026-09-25 · **Plan**: `docs_internal/plan/routing-docs-and-menu-visibility.md`
**Todo**: 5.22.3 · **Changelog terkait**: `2026-09-25-004` (migrasi contoh)

## Apa yang diubah

- `renderers/react-shadcn/src/hooks/useResolvedMenu.ts` — `filterMenuItem` kini
  mengevaluasi `item.when` (FormSpecExpr) terhadap `{ user: me }`; **fail-open**
  (item ditampilkan) bila evaluasi gagal, dengan laporan ke console.
- `cmd/formspec/check.go` — gate deploy-time diperkuat: **himpunan callable
  tertutup** `{len, sum, amount, currency, today}` di `validateExprGrammar`, plus
  `checkMenuExpr` baru untuk `MenuItem.When` di App **dan** Module.
- `renderers/react-shadcn/src/lib/formspec-expr/eval.ts` — callable `today()`
  (tanggal UTC `YYYY-MM-DD`), dan perbandingan urutan string-dengan-string.
- `renderers/react-shadcn/src/lib/formspec-expr/parser.ts` — **fix bug parser**:
  pemanggilan tanpa argumen (`today()`) tidak bisa diparse.
- `docs/spec/frontend/08-formspec-expr.md` §2–§3.

## Kenapa

`MenuItem.When` ada di `pkg/spec` + schema tetapi **tidak dibaca kode mana pun**
dan **tidak diperiksa grammar**-nya, padahal §4 spec menyebut gate deploy-time itu
wajib untuk semua FormSpecExpr. Satu-satunya contoh nyata di repo,
`examples/Clinic-UI-Showcase/.../clinic/module.yaml`, memakai
`when: "user.has('clinic.settings.update')"` — bentuk yang:

1. **tidak bisa dievaluasi** evaluator mana pun: klien hanya punya
   `len/sum/amount/currency`, dan `user.has(…)` menghasilkan AST dengan callee
   `MemberExpr` sehingga `node.callee.name` `undefined` → warning runtime
   "unknown function: undefined";
2. **melanggar §3**, yang melarang identitas/permission di FormSpecExpr.

Hasilnya item menu tampil untuk **semua** orang sambil terlihat dijaga — janji
palsu, bukan kebocoran (data tetap dijaga `required_permission`). Karena itu
dua hal dikerjakan bersama: sumbunya dipisah (`permissions` untuk RBAC —
changelog 002) dan bentuk salahnya **ditolak saat deploy**.

Gate callable menutup kelas ini secara umum: pemindai karakter tidak bisa
membedakan `len(x)` (sah) dari `user.has('x')` (mustahil) — keduanya identifier,
titik, dan tanda kurung. Tanpa gate, tiap salah ketik (`leng`, `lenn`) dan tiap
method call hanya muncul sebagai warning runtime.

## Dua bug yang ketemu saat membuat testnya

Keduanya kelas "gate menerima, runtime menolak" — persis yang §4 larang:

| Bug                          | Gejala                                      | Sebab                                                                                                                                                                           |
| ---------------------------- | ------------------------------------------- | ------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| `today()` tidak bisa diparse | `expected RPAREN but got >= at line 1:9`    | `parseCallExpr` memanggil `nextToken()` tanpa syarat, sehingga `RPAREN` terlewat sebelum `expectPeek`. Tidak pernah terlihat karena semua callable sebelumnya menerima argumen. |
| `>=` menolak string          | `cannot apply ">=" to a non-numeric string` | `evalOrder` hanya mendukung angka & money. Starlark (server) mendukung perbandingan string, jadi `today() >= '2026-01-01'` akan lolos gate + server lalu gagal di klien.        |

## Keputusan: `when` gagal → item **ditampilkan**

Fail-open, disengaja. `when` bukan batas keamanan, jadi menyembunyikan navigasi
karena bug renderer hanya merugikan (tidak bisa dibedakan dari "item ini memang
tidak ada") tanpa menahan apa pun. Karena `permissions` kini dijaga server, tidak
ada jalur di mana fail-open melebarkan akses.

## Bukti

- `TestValidateExprGrammar_RejectsUnknownCallables` — 7 kasus ditolak
  (`user.has(…)`, `session.x()`, `leng`, `lenn`, `days_ago`, `empty`, `sum_line`),
  pesannya menyebut himpunan yang sah.
- `TestValidateExprGrammar_AcceptsValidExpressions` — bertambah 8 ekspresi,
  termasuk `today() >= '2026-01-01'` dan dua kasus `(` **di dalam** string
  literal (bukan pemanggilan).
- `TestCheckMenuExpr` / `TestCheckMenuExpr_ModuleMenu` — App dan Module, item
  bersarang.
- `src/hooks/useResolvedMenu.test.ts` (10 test) — `when` true/false, `today()`,
  `me` null, fail-open + laporan.
- **Uji negatif pada contoh nyata**: mengembalikan bentuk lama di
  `clinic/module.yaml` → `formspec check` menolak:
  `FormSpecExpr "user.has('clinic.settings.update')" invalid: unknown function "user.has" — FormSpecExpr callables are a closed set: len, sum, amount, currency, today`.
  Setelah migrasi → 0.
- `formalexpr.test.ts` tetap hijau (109 → 118), vitest suite penuh **444 lulus**
  (naik dari 403), `tsc -b` bersih, `go test ./...` hijau.
