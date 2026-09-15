# Gap #25 & #26 — Config dan Settings Global

Ditemukan saat menulis `kind: Config` untuk aplikasi kafe, lewat validasi
sungguhan terhadap JSON Schema (`formspec v0.0.8`).

Keduanya penting karena `kind: Config` adalah tempat pengaturan yang diminta
bisa diubah admin tanpa menyentuh spec (D7) — dan salah bentuk di sini
berdampak langsung ke runtime.

---

## Gap #25 — Skill mendokumentasikan `Config.spec.data`, schema menolaknya ✅ Terverifikasi

### Bukti

Skill `formspec-kinds` (baik salinan di project ini maupun sumber di repo
FormSpec) mencontohkan:

```yaml
apiVersion: formspec.dev/v1
kind: Config
metadata:
  name: billing-config
  module: billing
spec:
  data:
    tax_rate: 0.11
    currency: IDR
```

Saya menulis persis bentuk itu, dan `formspec validate` menjawab:

```
schema: /spec: additional properties 'data' not allowed
36 manifest(s) validated, 1 problem(s) found
```

Schema sebenarnya (`schemas/v1/kinds/Config.schema.json`) hanya mengizinkan
**dua** properti, dengan `additionalProperties: false`:

| Properti | Isi |
| --- | --- |
| `keys` | map `nama_kunci -> ConfigKey` |
| `settings` | namespace presentasi/config global |

Dan `ConfigKey` (`formspec.schema.json` → `$defs.ConfigKey`) adalah **objek**,
bukan skalar:

```json
"ConfigKey": {
  "properties": {
    "default": {},
    "public":  { "type": "boolean" },
    "secret":  { "type": "boolean" },
    "type":    { "description": "int | string | bool | decimal | json", "type": "string" }
  },
  "required": ["type"],
  "additionalProperties": false
}
```

Jadi bentuk yang benar:

```yaml
spec:
  keys:
    tax_rate: { type: decimal, default: 0.11 }
    currency: { type: string, default: "IDR" }
```

### Dampak ke aplikasi kafe: **MEDIUM**

Pengaturan kafe (tarif pajak, laju poin, batas diskon manual, prefix nomor
pesanan) adalah kebutuhan yang diminta pemilik diatur dari admin panel. Menulis
`spec.data` **gagal validasi**, jadi biayanya adalah waktu agent/developer untuk
menemukan bentuk yang benar — bukan data rusak.

### ✅ Sisi baiknya — dan ini penting

Berbeda dari Gap #18 dan #21, kesalahan ini **GAGAL KERAS**. `additionalProperties: false`
pada schema Config membuat `formspec validate` menolak dengan pesan yang cukup
jelas (`additional properties 'data' not allowed`).

Ini menunjukkan FormSpec **punya** mekanisme yang tepat — dan memperkuat usulan
di Gap #18/#21: **terapkan pola `additionalProperties: false` yang sama secara
konsisten**, minimal untuk key yang sudah lama usang/diganti nama.

### Usulan

1. Perbarui contoh `kind: Config` di skill `formspec-kinds` (dan salinan di
   setiap project) dari `data:` ke `keys:` + `settings:`.
2. Idealnya tabel/daftar atribut kind **digenerate** dari `pkg/spec` (FormSpec
   sudah punya generator untuk tabel atribut di `docs/kind/`) — jadi contoh
   manual yang bisa basi sebaiknya tidak ditulis ulang di banyak tempat.

---

## Gap #26 — `settings.currency` WAJIB untuk field `money` ⚠️ DIKOREKSI

> **DIKOREKSI setelah verifikasi runtime (2026-09-14).** Separuh klaim di bawah
> salah. Yang diamati pada API nyata:
>
> | Dikirim | Tersimpan |
> | --- | --- |
> | `{ amount: "50000", currency: "IDR" }` | `{ "amount": "50000", "currency": "IDR" }` |
> | `{ amount: "15000" }` | `{ "amount": "15000" }` — **currency tidak diisi** |
> | `25000` (angka) | `25000` — **bukan objek** |
>
> Jadi aplikasi **tidak gagal** seperti yang saya klaim — ia menerima money
> tanpa currency secara **diam-diam**, dan `settings.currency` **tidak**
> diterapkan. Severity tetap HIGH, tapi alasannya berganti: bukan "gagal keras
> tanpa settings", melainkan "data finansial tanpa mata uang bisa masuk tanpa
> keluhan". Kegagalan keras bisa diperbaiki; data tanpa mata uang tidak.
>
> Analisis lengkap: `12-hasil-verifikasi-runtime.md` → Gap #46.

Temuan paling berdampak di catatan ini — dan hampir luput.

### Bukti

`pkg/spec/money.go` mendefinisikan urutan resolusi mata uang yang **normatif**:

```go
// ResolveMoneyCurrency resolves the currency for a money field per the
// normative order (05-field-types.md §2): explicit field `currency` ->
// `settings.currency.code` -> error (never guess).
```

Dan bila keduanya kosong:

```go
return "", 0, &MoneyFieldError{
    Field:   f.Name,
    Message: "money field has no currency: declare `currency` on the field or set `settings.currency` (never guess)",
}
```

Sementara schema Config menjelaskan di mana `settings` hidup:

> `settings` is the typed, global presentation/config namespace ... It lives in
> **ONE place (workspace/App Config)** and drives cross-component
> interpretation/display: **currency, locale, timezone, date format, decimal
> scale, rounding**.

### Dampak ke aplikasi kafe: **HIGH**

Spec kafe ini punya **belasan** field `money` tanpa `currency` eksplisit:

| Entity | Field money |
| --- | --- |
| `menu-item-price` | `price` |
| `promo` | `value`, `min_purchase` |
| `ingredient` | `cost_per_unit` |
| `purchase-order` (+ child) | `total_amount`, `unit_cost`, `subtotal` |
| `stock-movement` | `unit_cost`, `total_cost` |
| `stock-level` | `moving_avg_cost`, `stock_value` |
| `menu-cost` | `cost_per_portion`, `sale_price`, `gross_margin` |
| `waste-entry` | `unit_cost` |
| `order` (+ child) | `subtotal`, `discount_amount`, `points_value`, `service_charge_amount`, `tax_amount`, `total_amount`, `paid_amount`, `change_amount`, `unit_price_snapshot`, `line_total`, `discount_amount` |
| `payment` | `amount`, `tendered`, `change` |
| `shift` | `opening_cash`, `expected_cash`, `counted_cash`, `difference` |
| `cash-movement` | `amount` |

Tanpa `settings.currency`, seluruh field itu **tidak punya mata uang** — dan
karena FormSpec menolak menebak, aplikasi akan **gagal saat runtime**.

**Yang membuat ini berbahaya:** `formspec validate` **HIJAU** — 35 manifest,
0 problem — walaupun `settings` belum ada. `formspec check` juga 0 error.
`formspec migrate plan` tetap menghasilkan 22 tabel. Jadi ketiga gerbang
otomatis **tidak menangkap** masalah ini; kegagalan baru muncul saat record
benar-benar dibuat/dibaca.

### Perbaikan yang dilakukan

`spec/config/app.yaml` (Config tingkat App/workspace):

```yaml
apiVersion: formspec.dev/v1
kind: Config
metadata:
  name: app-settings
spec:
  settings:
    currency:
      code: IDR
      decimal_places: 0
      symbol: "Rp"
    locale: "id-ID"
    timezone: "Asia/Jakarta"
    date_format: "DD/MM/YYYY"
    decimal_scale: 2
    rounding: "half_even"
```

Setelah ini: 35 manifest, 0 problem — dan field `money` punya mata uang.

### Usulan

1. **`formspec validate` harus memperingatkan** bila spec punya field `money`
   tanpa `currency` eksplisit DAN tidak ada `settings.currency`. Ini pemeriksaan
   lintas-file yang murah dan menutup satu kelas kegagalan runtime.
2. Dokumentasikan `settings` lebih menonjol. Saat ini `Settings` muncul di
   schema `Config` dan di `lib/format.ts`, tapi **tidak disebut** di tabel tipe
   field `entity-authoring` maupun di contoh Config mana pun yang saya temui —
   padahal tanpa itu `money` tidak bisa dipakai sama sekali.
3. Pertimbangkan default yang aman untuk dev (mis. mengikuti locale mesin) agar
   aplikasi tidak langsung gagal — dengan peringatan, bukan diam.

---

## Ringkasan Dampak ke Spec Kafe

| Sebelum perbaikan | Sesudah perbaikan |
| --- | --- |
| `Config.spec.data` → **1 problem** (schema menolak) | `spec.keys` → hijau |
| `settings.currency` tidak ada → **validasi hijau tapi gagal runtime** | `spec/config/app.yaml` menyediakan IDR/id-ID/Asia-Jakarta → hijau & aman |
