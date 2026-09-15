# 2026-09-15-001 — S7: semantik aritmetika & agregasi `money`

Item `examples/kafe/gaps_found/TODO.md` 1.3 (prioritas §F #3) — menutup gap
**#28**. Plan: `docs_internal/plan/money-arithmetic-semantics.md`.

**Masalahnya.** `money` bernilai objek `{amount, currency}`, sedangkan keempat
jalur evaluasi bekerja atas skalar — dan salahnya **berbeda** di tiap jalur,
sehingga bug-nya tak terlihat:

| Jalur                                   | Sebelum                                                                                                                                  |
| --------------------------------------- | ---------------------------------------------------------------------------------------------------------------------------------------- | --- | ------------- |
| server `computed` (`starlark.EvalExpr`) | `dict - dict` → error Starlark → field computed **diam-diam absen** (`payment.change`, `shift.difference`, `line_total`, `total_amount`) |
| klien FormSpecExpr (`toNumber`)         | `{...}` → **0** (kembalian tampil `Rp0`)                                                                                                 |
| laporan/widget (`Number(obj)`)          | `NaN` → baris dibuang → `sum = 0`                                                                                                        |
| chart widget (`Number(obj) \\           | \\                                                                                                                                       | 0`) | memplot **0** |

**Bentuk kanonik (keputusan).** Uang beroperasi **langsung** — tidak ada sintaks
pembungkus baru. Operand diklasifikasikan money|skalar: `m ± m` → money
(securrency; beda → error) · `m × / n` → money · `m1 / m2` → rasio number ·
`m <op> m` → boolean · `m == m` → kesetaraan **nilai** (deep) · `amount(x)` /
`currency(x)` untuk ekstraksi eksplisit · `sum([m…])` → money. **Operand tidak
sah = error** (objek non-money, list, string non-numerik), termasuk `money`
vs angka mentah; untuk membandingkan dengan skalar, ekstrak dulu
(`amount(m) > 100`). Alasan memilih bentuk ini: spec yang sudah dikirim
(`examples/cafe`: `quantity * unit_price`, `sum([i["line_total"] …])`) memakai
bentuk ini apa adanya — jadi engine yang menyusul, **bukan** spec yang
di-degradasi.

**Yang diubah.**

- `internal/starlark/money.go` (baru) — tipe Starlark `moneyValue`:
  `HasBinary` (`+ - * /`), `HasUnary` (`-`), `Comparable` (`< <= > >= == !=`),
  presisi eksak lewat `math/big.Rat` (bukan float64: `0.1 + 0.2` harus `0.3`,
  kalau tidak skalanya jebol), plus builtin `amount()`/`currency()` dan alias
  `money_amount`/`money_currency` (nama env menang atas builtin — spec kafe
  memang punya field bernama `amount`).
- `internal/starlark/evaluator.go` — `toStarlark` mengenali `spec.Money` **dan**
  objek JSON money (jalur baca-DB) sehingga create dan read berperilaku sama;
  `fromStarlark` mengembalikan `{amount, currency}`; `sum` jadi money-aware;
  `starlarkNumber` (koersi non-numerik → 0) dihapus.
- `internal/starlark/executor.go` — script & guard ikut mendapat `amount()`/
  `currency()`.
- `renderers/jsonb-persist/crud.go` — `columnRefExpr` memetakan `money` ke
  sub-path `.amount` (`data->'f'->>'amount'` / `json_extract(data,'$.f.amount')`,
  dan ini menang atas kolom turunan yang menyimpan objeknya);
  `requireNumericAggregateField` menolak `sum|avg|min|max` atas field
  non-numerik **dan** field tak dikenal; berlaku juga untuk `Window
running_total`.
- `renderers/react-shadcn/src/lib/formspec-expr/eval.ts` — tabel aturan yang
  sama di sisi klien; `==`/`!=` memakai `deepEqual` (sebelumnya identitas objek,
  jadi dua money bernilai sama dianggap tidak sama).
- `renderers/react-shadcn/src/lib/aggregate.ts` (baru) — agregasi money-aware
  yang dipakai bersama oleh Report `computeTotals`, Widget metric, dan chart;
  mengembalikan `{value, error}` sehingga field yang tidak bisa diagregasi
  ditampilkan sebagai error, bukan `0`.
- `cmd/formspec/check.go` — `checkAggregates`: verb ada di closed set, field ada,
  dan `sum|avg|min|max` hanya atas field numerik/`money` (statis, sebelum
  runtime).
- `examples/kafe/spec/**` — marker `# GAP-28` dihapus (7 file).
- Dokumen: `docs/spec/backend/05-field-types.md` §2.1 (baru, normatif),
  `docs/spec/frontend/08-formspec-expr.md` §5 (baru), `docs/reference/primitives.md`
  (dialek Starlark), `ai_skills/entity-authoring/SKILL.md`.

**Bukti runtime** (spec kafe, `formspec dev`, HTTP): `payment` amount 75000 /
tendered 100000 → `change = {amount:"25000",currency:"IDR"}` (sebelumnya field
**absen**), dan tetap benar pada `GET` ulang; `shift` counted 500000 / expected
512500 → `difference = {-12500 IDR}` (kas kurang, negatif).
`formspec check -f examples/kafe/spec` → 0 error. Test: `go test ./...` **35
paket ok**, `vitest` **207 lulus**, `tsc --noEmit` bersih.

**Bukti runtime rangkaian penuh** (spec `examples/cafe`, yang sudah memakai
bentuk ini di spec-nya): `menu-item.price` 25000 → hook `fill_order_unit_price`
mengisi `items[].unit_price` → `computed "quantity * unit_price"` →
`line_total 50000` & `75000` → `computed 'sum([i["line_total"] …])'` →
`total_amount = {amount:"125000",currency:"IDR"}`. Sebelum perubahan ini seluruh
rantai itu mati di langkah pertama (`int * dict` → error Starlark → field
computed absen).

**Nol tak bermakna dibuang.** Hasil aritmetika dirender pada skala operand
**tanpa** nol di belakang koma (`4000.50` → `4000.5`, `25000 * 0.5` → `12500`),
di Go maupun klien — kalau tidak, hasil yang eksak "mengaku" punya presisi lebih
tinggi dan ditolak `ValidateMoneyValue` pada field `decimal_places: 0` (IDR).
Perilaku ini juga menyamakan keluaran kedua evaluator, jadi klien dan server
tidak menampilkan/menyimpan dua bentuk berbeda untuk nilai yang sama.

**Catatan (sisa, bukan bagian #28).** Perbandingan `money` dengan **angka
mentah** tidak bisa diizinkan: Starlark tidak menyediakan hook untuk
perbandingan lintas tipe (`HasBinary` hanya untuk operator aritmetika), jadi
aturan "kedua operand harus money" adalah satu-satunya yang bisa ditegakkan
identik di server dan klien. Bentuk resminya `amount(m) <op> n` — termasuk untuk
membandingkan dengan nol. Dicatat juga di dokumen normatif, bukan ditutupi.
