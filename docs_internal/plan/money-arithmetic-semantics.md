# Plan — S7: Semantik aritmetika & agregasi `money`

**Status**: ✅ selesai (2026-09-15) — changelog `docs_internal/changelog/2026-09-15-001-money-arithmetic-aggregation.md`
**TODO item**: `examples/kafe/gaps_found/TODO.md` **1.3** (prioritas §F #3)
**Menutup**: gap **#28** (`examples/kafe/gaps_found/01-widget-money-time.md`) · menopang **#28 → 4.8**
**Sumber spec**: `docs/spec/frontend/08-formspec-expr.md`, `docs/spec/backend/05-field-types.md` §2
**Prasyarat** (sudah selesai): #2 renderer `money` (todo 2.4), #46 normalisasi batas API (todo 2.3)

---

## 1. Masalah

`money` adalah tipe kelas satu dan nilainya **objek** `{amount, currency}` (kanonik sejak 2.3),
sedangkan **kedua** evaluator ekspresi bekerja atas **skalar**. Dua jalur, keduanya salah,
dan salahnya **berbeda** — sehingga bug-nya tak terlihat:

| Jalur            | Kode                                                                                                     | Perilaku sekarang                                                                | Kenyataannya                                                              |
| ---------------- | -------------------------------------------------------------------------------------------------------- | -------------------------------------------------------------------------------- | ------------------------------------------------------------------------- |
| Server (persist) | `internal/starlark/evaluator.go` (`EvalExpr`) via `renderers/jsonb-persist/crud.go` (`evaluateComputed`) | `dict - dict` → Starlark error → field computed **diam-diam absen** (`continue`) | `payment.change`, `shift.difference`, `line_total`, `total_amount` hilang |
| Klien (tampilan) | `renderers/react-shadcn/src/lib/formspec-expr/eval.ts` (`toNumber`)                                      | `toNumber({...})` → **0**                                                        | kembalian tampil `Rp0`                                                    |
| Laporan/widget   | `ReportRenderer.computeTotals`, `DashboardRenderer.aggregate`                                            | `Number({...})` → `NaN` → dibuang → **sum = 0**                                  | omzet harian `Rp0`                                                        |

`formspec check` melaporkan **0 error** untuk semuanya, jadi bentuknya "disetujui" tetapi
semantiknya tidak pernah ditetapkan. Untuk data uang, "belum ditetapkan" = "tidak bisa diandalkan".

**Bukti bahwa bentuk kanonik sudah dipakai di spec nyata** (jadi bukan spec yang harus berubah,
engine yang harus menyusul):

```yaml
# examples/cafe/spec/modules/cafe-order/transaction/order/entity.yaml
- name: unit_price # type: money
- name: line_total # type: money
  computed: { formula: "quantity * unit_price" } # integer × money
- name: total_amount # type: money
  computed: { formula: 'sum([i["line_total"] for i in items])' } # sum atas money
```

```yaml
# examples/kafe/spec/modules/cafe-order/transaction/payment/entity.yaml
- name: change # type: money
  computed: { formula: "tendered - amount" } # money - money
# examples/kafe/spec/modules/cafe-order/transaction/shift/entity.yaml
- name: difference # type: money
  computed: { formula: "counted_cash - expected_cash" }
```

## 2. Keputusan — bentuk kanonik

**Uang beroperasi langsung; tidak ada sintaks baru yang diwajibkan.**

Nilai uang = objek `{amount, currency}`. Operand diklasifikasikan menjadi **money** atau
**skalar** (number / string numerik / bool).

| Ekspresi                     | Hasil   | Catatan                                                                       |
| ---------------------------- | ------- | ----------------------------------------------------------------------------- |
| `m1 + m2`, `m1 - m2`         | money   | currency harus sama; beda → **error**                                         |
| `m * n`, `n * m`, `m / n`    | money   | `n` skalar; `n = 0` → **error**                                               |
| `m1 / m2`                    | number  | rasio (mis. margin); currency harus sama                                      |
| `-m`                         | money   | unary                                                                         |
| `m1 <ring> m2` (`< <= > >=`) | boolean | currency harus sama                                                           |
| `m1 == m2`, `m1 != m2`       | boolean | kesetaraan **dalam** (deep), bukan identitas                                  |
| `amount(x)`                  | number  | akses eksplisit ke komponen scalar; `amount(money)` / number / string numerik |
| `currency(x)`                | string  | kode mata uang                                                                |
| `sum([m…])`                  | money   | semua elemen money & securrency                                               |
| `sum([n…])`                  | number  | seperti sekarang                                                              |

**Operand tidak sah = error, bukan 0.** Objek yang bukan money, atau array, di posisi
aritmetika/perpindahan (`<`, `>`) → error evaluasi (di klien: warning → `valid: false` →
`evalCompute` = `null`; di server: `EvalExpr` mengembalikan error). Ini yang menutup kalimat
"field non-numerik **ditolak dengan error**, bukan diam-diam salah".

**Agregasi.** Untuk `sum`/`avg`/`min`/`max` (Report `columns[].aggregate`, `totals[].fn`,
Widget `config.aggregate`) atas field bertipe `money` → engine mengagregasi **`amount`**
(sub-path `$.f.amount` di SQL, `moneyAmount()` di klien). Hasilnya number (formatter
`currency` sudah menerima number maupun objek). Field yang **bukan** numerik/money (dan bukan
`count`) → **error statis** di `formspec check`, dan **error runtime** di klien/persist.

`count` tidak terpengaruh (boleh atas field apa pun, termasuk tanpa field).

## 3. Perubahan per file

| File                                                               | Perubahan                                                                                                                                                                                           | Effort |
| ------------------------------------------------------------------ | --------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- | ------ | --- | --------------------------- | ------ |
| `docs/spec/frontend/08-formspec-expr.md`                           | § aritmetika & uang: tabel kanonik di atas jadi normatif                                                                                                                                            | small  |
| `docs/spec/backend/05-field-types.md` §2                           | agregasi money = agregasi `amount`; non-numerik ditolak                                                                                                                                             | small  |
| `docs/reference/primitives.md`                                     | catat `amount()`/`currency()` + aturan error pada dialek Starlark                                                                                                                                   | small  |
| `internal/starlark/money.go` **(baru)**                            | tipe Starlark `moneyValue` (`HasBinary` `+ - * /`, `HasUnary` `-`, `Comparable`), builtin `amount()`/`currency()`                                                                                   | medium |
| `internal/starlark/evaluator.go`                                   | `toStarlark` mengenali `spec.Money` → `moneyValue`; daftarkan builtin                                                                                                                               | small  |
| `renderers/jsonb-persist/crud.go`                                  | `evaluateComputed`: konversi nilai money (struct/map) → `spec.Money` sebelum evaluasi (child + top-level). `Aggregate`/`Window`: `columnRefExpr` → sub-path `amount` untuk tipe `money`; tolak `sum | avg    | min | max` atas field non-numerik | medium |
| `renderers/react-shadcn/src/lib/formspec-expr/eval.ts`             | aritmetika sadar-money, `amount()`/`currency()`, operand tak sah → warning, `==`/`!=` deep                                                                                                          | medium |
| `renderers/react-shadcn/src/kinds/report/ReportRenderer.tsx`       | `computeTotals` sadar-money (`moneyAmount`) + error terlihat untuk field non-numerik                                                                                                                | small  |
| `renderers/react-shadcn/src/kinds/dashboard/DashboardRenderer.tsx` | `aggregate` sadar-money + error terlihat                                                                                                                                                            | small  |
| `cmd/formspec/check.go`                                            | gate statis: agregat non-`count` atas field non-numerik → error (butuh tipe field di `entityIndex`)                                                                                                 | medium |
| `examples/kafe/spec/**`                                            | hapus marker `# GAP-28` (semantiknya kini ditetapkan)                                                                                                                                               | small  |
| Test                                                               | `internal/starlark/money_test.go`, `renderers/jsonb-persist/aggregate_money_test.go`, `eval` (vitest), `check_test.go`                                                                              | medium |

**Verifikasi akhir (bukti, bukan inferensi):** buat `payment` dengan `amount` + `tendered`
via `POST /_ui/entity/...` → `change` = selisih, bukan kosong; `GET` report
`sales-by-period`/widget `omzet-hari-ini` atas `order.total_amount` → nilai ≠ 0;
`formspec check` menolak `aggregate: sum` atas field `text`.

## 4. Dependensi & risiko

- Tidak ada spec kafe yang perlu diubah bentuk → **tidak ada degradasi spec** (aturan §"Spec kafe tidak di-degradasi").
- Risiko: ekspresi lama yang mengandalkan `toNumber(obj) = 0` menjadi error. Audit atas
  `examples/**`, `verticals/**`, `resource/**` hanya menemukan aritmetika atas field
  numerik/money → aman.
- `window` (`running_total`) memakai jalur kolom yang sama → ikut benar.

## 5. Urutan kerja

1. `internal/starlark/money.go` + test (server lebih dulu: ini yang menyimpan nilai).
2. `crud.go` `evaluateComputed` + `Aggregate`/`Window`.
3. `eval.ts` + test (klien, paritas semantik dengan server).
4. `ReportRenderer`/`DashboardRenderer`.
5. `check.go` (gate statis).
6. Docs + hapus marker GAP-28 + changelog + todo.
