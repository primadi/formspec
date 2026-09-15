# Gap #1 & #2 — Widget `money` dan `time`

Ini gap **paling kritis** untuk aplikasi kafe, karena seluruh POS adalah
aritmetika uang.

## Gap #1 — Widget `money` & `time` tidak ada ✅ Pasti

> **Status 2026-09-15.** Akarnya sudah tertutup (TODO **1.4** / S10): kosakata
> `widget` kini himpunan tertutup, jadi `widget: money-input` **gagal validasi**
> dengan pesan yang jelas — tidak lagi lolos lalu diam-diam jadi `TextInput`.
> Yang **belum** ada adalah widget-nya sendiri: `money` dan `time` masih jatuh ke
> `input`. Itu kini item **2.14** (`MoneyInput` + `TimeInput`). Jadi gap ini
> **terbuka sebagian**, bukan tertutup.

### Bukti

`renderers/react-shadcn/src/engine/derive.ts` → fungsi `formWidget()` adalah
satu-satunya tempat tipe field dipetakan ke widget. Daftar `case`-nya:

```
string, text, richtext, integer, decimal, boolean, enum, date, datetime,
uuid, json, file, relation, child  →  default: "input"
```

**Tidak ada `case "money"` dan tidak ada `case "time"`.** Keduanya jatuh ke
`default: return "input"` → `TextInput` polos.

`renderers/react-shadcn/src/widgets/index.ts` (barrel widget) berisi:
`TextInput, TextareaInput, RichText, FileInput, NumberInput, Select, Switch,
Badge, RelationPicker, DateInput, JsonInput, ChildTable, RadioGroup, Combobox,
PasswordInput, SliderInput, TagsInput` — **tidak ada `MoneyInput` dan tidak ada
`TimeInput`**.

`renderers/react-shadcn/src/lib/zod-schema.ts` → `buildZodField()` juga tidak
punya `case "money"`/`case "time"`, jadi tidak ada validasi klien.

Ini **sudah diakui resmi** di `docs_internal/plan/widget-strategy.md`
§Further Considerations:

> 1. Field types `money` (7.16) & `time` belum punya widget — usulkan track tambahan.

Dan `ai_skills/form-layout/SKILL.md` (serta salinannya di
`.agents/skills/form-layout/SKILL.md` milik project ini) **menjanjikan** sesuatu
yang tidak ada:

| Field type | Widget default | Catatan                   |
| ---------- | -------------- | ------------------------- |
| `money`    | **MoneyInput** | format mata uang otomatis |

> **Widget `MoneyInput` tidak pernah ada di codebase.** Skill mengajarkan agent
> memakai widget yang tidak ada, lalu `formWidget()` diam-diam memberi
> `TextInput`.

### Dampak ke aplikasi kafe: **BLOCKER**

- Harga menu (`menu-item.price`), subtotal, total pesanan, diskon, uang
  diterima, kembalian, dan kas awal shift semuanya `money`.
- Kasir akan melihat **input teks kosong tanpa format mata uang dan tanpa
  pembatas desimal** untuk setiap nilai uang. Mengetik `25000` di input teks
  tidak salah secara angka, tapi tidak ada pembatas ribuan, tidak ada simbol
  `Rp`, dan tidak ada pembulatan yang dipaksakan.
- Di POS, input uang tanpa numpad/formatter adalah pengalaman yang tidak layak
  dipakai.

### Catatan tambahan — `money` juga hilang dari daftar sortable

`derive.ts`:

```ts
function isSortable(field: Field): boolean {
  return [
    "string",
    "integer",
    "decimal",
    "date",
    "datetime",
    "enum",
    "boolean",
  ].includes(field.type)
}
```

`money` dan `text` tidak termasuk → **kolom harga tidak bisa di-sort** di Table.
Untuk laporan penjualan per menu, sort berdasarkan omzet tidak bisa dilakukan
lewat UI.

`tableWidget()` juga tidak mengenal `money` (hanya `enum`/`doc_status`/`boolean`).

### Usulan

1. Tambah `MoneyInput` (numpad-friendly, format `settings.currency`, batasi
   `decimal_places`) dan `TimeInput` (`type="time"`).
2. Tambah `case "money"` / `case "time"` di `formWidget()` **dan** di
   switch `FormFieldWidget` di `FormRenderer.tsx`.
3. Tambah `case "money"` di `buildZodField()`.
4. Masukkan `money` ke `isSortable()`.
5. Sampai itu land: `formWidget()` sebaiknya **gagal keras** (atau menampilkan
   placeholder jujur) untuk tipe yang belum punya komponen — bukan jatuh ke
   `TextInput` senyap. Ini pola yang sudah tercatat sebagai masalah nyata di
   `docs/renderers/shadcn-shell/03-kind-renderers.md`.

---

## Gap #2 — Nilai `money` berbentuk objek, renderer hanya format angka ⚠️

### Bukti

`pkg/spec/entity.go` — `money` adalah tipe kelas satu:

```go
FieldMoney FieldType = "money" // {amount, currency} pair
```

`pkg/spec/money.go`:

```go
// Money is the first-class money value: an exact decimal amount (string, to
// preserve precision) paired with an ISO-4217 currency code.
type Money struct {
    Amount   string `json:"amount"`
    Currency string `json:"currency"`
}
```

`cmd/formspec/generate.go` → `tsFieldType()` mengonfirmasi bentuk di klien:

```go
case spec.FieldMoney:
    // money is a first-class {amount, currency} pair (05-field-types.md §2).
    // amount is arbitrary-precision decimal → string, never number.
    return "{ amount: string; currency: string }"
```

Tapi `renderers/react-shadcn/src/lib/renderCell.tsx` hanya memformat **angka**:

```ts
if (format === "currency" && typeof value === "number") {
  return formatter.money(value)
}
```

Sementara `cellHintsForField()` memetakan `money` → `{ format: "currency" }`.
Karena nilai `money` adalah **objek** (`{amount, currency}`), kondisi
`typeof value === "number"` **tidak terpenuhi**, sehingga eksekusi jatuh ke:

```ts
if (typeof value === "object") return JSON.stringify(value)
```

### Dampak ke aplikasi kafe: **BLOCKER** ⚠️ Perlu verifikasi runtime

Kalau benar, maka di **Table, Listing, Report, dan child table** nilai uang
dirender sebagai JSON mentah, contoh:

```
{"amount":"25000","currency":"IDR"}
```

bukan `Rp25.000`. Ini bukan sekadar kosmetik — daftar pesanan dan laporan
penjualan akan sulit dibaca.

`renderers/jsonb-persist/ddl.go` → `fieldTypeToSQL()` juga **tidak punya `case
spec.FieldMoney`**, sehingga jatuh ke `default: return "text"`. Perlu
dipastikan apakah ada normalisasi `money` → `decimal` sebelum DDL; kalau tidak,
kolom uang disimpan sebagai `text` di database.

### Yang perlu diverifikasi

1. Bentuk `money` di wire JSON saat `GET /_ui/entity/...` — objek atau angka?
2. Isi kolom `money` di SQLite hasil `formspec dev` — `text` atau `numeric`?
3. Render aktual di Table/Listing untuk entity dengan field `money`.

### Usulan

- `renderCellValue()` perlu menangani `{amount, currency}`:
  `formatter.money(Number(value.amount))`, bukan `typeof value === "number"`.
- `DetailPage.tsx` juga memperlakukan `decimal` sebagai uang
  (`if (field.type === "decimal" && typeof value === "number")`) — perlu cabang
  untuk `money`.
- Tambah `case spec.FieldMoney` di `fieldTypeToSQL()`.

> **Catatan positif:** `lib/format.ts` sudah benar dan lengkap —
> `createFormatter(settings)` membaca `settings.currency` (code,
> `decimal_places`, symbol), `settings.locale`, `settings.rounding`
> (termasuk `half_even`/banker's rounding). Jadi fondasi format uang
> **sudah ada**; yang hilang hanya widget input dan pemetaan objek→format.

---

## Gap #28 — Aritmetika & agregasi `money` ✅ `CLOSED` (2026-09-15)

> **Ditutup oleh TODO 1.3 (S7).** Bentuk kanonik ditetapkan — uang beroperasi
> langsung (`money - money`, `number * money`, `sum([money…])` → money), operand
> tidak sah **error** (bukan `0`), agregasi `money` menjumlahkan `.amount`-nya
> dan field non-numerik ditolak `formspec check`. Normatif:
> [`docs/spec/backend/05-field-types.md`](../../../docs/spec/backend/05-field-types.md)
> §2.1 + [`docs/spec/frontend/08-formspec-expr.md`](../../../docs/spec/frontend/08-formspec-expr.md) §5.
> Bukti runtime: `payment.change` `100000-75000` → `{25000 IDR}`;
> `shift.difference` `500000-512500` → `{-12500 IDR}`. Catatan asli di bawah
> dipertahankan sebagai jejak temuan.

Karena `money` adalah **objek** `{amount, currency}` sementara FormSpecExpr
bekerja atas **skalar**, setiap operasi hitung atas uang layak dipertanyakan.
Spec kafe memakai tiga bentuk:

### Bentuk 1 — `computed` atas money (dipakai sekarang)

```yaml
# payment.change
computed:
  formula: "tendered - amount"

# shift.difference
computed:
  formula: "counted_cash - expected_cash"
```

`formspec check -f spec` melaporkan **0 error, 0 warning** — jadi bentuk ini
**diterima** oleh analisis statis. Yang belum diketahui: apakah evaluator
FormSpecExpr benar-benar menghitung `{amount,currency} - {amount,currency}`,
atau menghasilkan `null`/garbage saat runtime.

Ini penting karena **kembalian (change)** dan **selisih kas (difference)**
adalah dua angka yang paling terlihat di operasional kafe — salah di situ
langsung berujung pada selisih uang riil.

### Bentuk 2 — agregasi money di Widget/Dashboard/Report (Tahap 3b)

```yaml
config: { field: total_amount, aggregate: sum }
columns: [{ field: total_amount, aggregate: sum, format: currency }]
```

`order.total_amount` adalah `money`. `SUM()` atas objek JSON tidak bermakna;
yang diharapkan adalah `SUM(total_amount.amount)`. Belum diverifikasi apakah
engine melakukan `json_extract(data, '$.total_amount.amount')` atau mencoba
menjumlahkan objek.

### Dampak ke aplikasi kafe: **HIGH (berpotensi)**

Seluruh laporan berbasis uang bergantung pada ini: omzet harian, penjualan per
periode, profitabilitas menu, rekap kas per shift, pembelian per supplier.
Kalau agregasi money tidak jalan, **semua laporan finansial di aplikasi ini
salah atau kosong** — dan penyebabnya di lapisan engine, bukan spec.

### Usulan

1. **Tentukan satu bentuk kanonik** untuk aritmetika money dan dokumentasikan:
   `amount(a) + amount(b)`? `a.amount + b.amount`? Atau `money` **tidak** boleh
   dipakai di `computed` sama sekali (wajib lewat script)?
2. Untuk agregasi, engine sebaiknya secara eksplisit mengagregasi `.amount`
   saat field bertipe `money`, dan menolak (bukan diam-diam salah) bila diminta
   `sum` atas field non-numerik.
3. Tambahkan `formspec check` rule: `computed`/`aggregate` yang melibatkan field
   `money` → **peringatan** sampai semantiknya ditetapkan.

### Status verifikasi

Belum bisa ditutup dengan CLI yang tersedia: butuh evaluasi FormSpecExpr dan
jalur query — artinya **butuh runtime**, dan runtime HTTP/UI terblokir
**GAP-24** (`formspec dev` mati di Windows). Jadi dua gap ini saling mengunci:
selama GAP-24 belum diperbaiki, GAP-28 tidak bisa dipastikan.

**Cara verifikasi yang tersisa — dan mengapa belum cukup.**

`formspec repl -e <expr>` memang bisa mengevaluasi Starlark dengan akses `ctx.*`
tanpa HTTP. Tetapi itu **tidak** menutup gap ini:

| Bentuk                              | Dieksekusi oleh                                    | Bisa diuji lewat |
| ----------------------------------- | -------------------------------------------------- | ---------------- |
| `computed` pada field entity        | **Renderer** (client-side, `evalCompute` di React) | Runtime UI       |
| `aggregate: sum` pada Widget/Report | **Query engine** (jalur list/agregasi)             | Runtime HTTP     |

Keduanya **tidak** lewat runtime Starlark, jadi `formspec repl` bukan jalan.
Satu-satunya jalan adalah runtime HTTP/UI — dan itu terblokir **GAP-24**
(`formspec dev` mati di Windows).

Jadi **GAP-28 dan GAP-24 saling mengunci**: selama dev server tidak bisa start,
seluruh kelas pertanyaan "apakah nilai ini benar saat runtime" tidak bisa
dijawab — termasuk apakah aplikasi kafe ini benar-benar menghasilkan uang yang
benar.
