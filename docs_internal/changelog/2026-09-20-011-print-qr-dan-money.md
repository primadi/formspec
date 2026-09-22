# 2.6 — QR di dokumen cetak + money terformat (gap #3/#1) + bug generator `$defs`

**Tanggal:** 2026-09-20 · **TODO:** `examples/kafe/gaps_found/TODO.md` 2.6 · **Plan:** `docs_internal/plan/kafe-sisa-gap.md`

## Apa yang diubah

### 1. Bahasa spec: body item `qrcode` (S4 / #3)

`PrintBodyItem` mendapat `Qrcode *PrintQrcode` (`pkg/spec/frontend.go`):

```yaml
- qrcode:
    payload: "/kafe/status/{guest_token}"   # `{dotted.path}`, sama seperti header/footer
    label: "Struk digital"
    absolute: true                          # origin dokumen ditambahkan di depan
    size_mm: 25                             # default 30
```

- `payload` boleh token tunggal (`"{qr_token}"`) atau URL relatif; `absolute: true`
  menambahkan **origin dokumen** (browser: origin halaman; server: scheme+host
  request, `X-Forwarded-*` dihormati). Origin adalah pengetahuan deployment —
  tidak ada field URL absolut di entity.
- **Token yang tidak ter-resolve = elemen dihilangkan**, bukan QR rusak. Interpolasi
  server mengosongkan token (`/status/` — tampak valid!) sehingga jalur QR memakai
  `interpolateStrict`; klien menolak payload yang masih memuat `{…}`. Order POS
  tanpa token tamu ⇒ struk tercetak tanpa QR.
- `payload` kosong literal ditolak schema (`minLength: 1`) — itu cacat manifest,
  bukan cacat data.

### 2. Money diformat di dokumen cetak (#1 sisa)

`spec.FormatMoneyDisplay` (`pkg/spec/money.go`) + `printContext.path/formatPrintValue`
(`internal/api/print.go`) dan `formatValue` (PrintRenderer) membuat field/total
money tampil `Rp62.500` (simbol + pengelompokan dari `settings.currency`/`locale`).
Sebelumnya struk mencetak `map[amount:25000 currency:IDR]`. Angka biasa (mis.
`quantity`) tidak pernah diperlakukan sebagai uang.

### 3. Ketiga pipeline merender QR

| Pipeline | Cara |
| --- | --- |
| `html` (klien) | widget `QrCode` (SVG, `qrcode.react`) di dalam `PrintRenderer` |
| `pdf` (server) | PNG dari `github.com/skip2/go-qrcode` ditanam via `pdf.RegisterImageOptionsReader` |
| `thermal` (server) | perintah QR native ESC/POS `GS ( k` (model 2, EC level M) — bukan raster, jadi tetap tajam |

Dependency baru: `github.com/skip2/go-qrcode` (MIT, tanpa transitive deps) —
sebelumnya encoder QR sama sekali tidak ada di sisi Go.

### 4. Bug generator schema yang ditemukan & ditutup

`Print.schema.json` meng-emit `"$ref": "#/$defs/PrintQrcode"` sementara
`sharedTypes` di `internal/genjsonschema/generator.go` tidak memuat `PrintQrcode`
→ `formspec validate` gagal **untuk setiap manifest Print** dengan
`compile Print schema: json-pointer …#/$defs/PrintQrcode not found`.

Guard yang sudah ada (`TestGeneratedKindSchemas_HaveNoDanglingRefs`) **buta**
terhadap kasus ini: ia hanya menelusuri dokumen schema *kind*, sedangkan
`$ref`-nya berada di dalam **shared def** (`PrintBodyItem.qrcode`). Guard-nya
kini juga menelusuri seluruh `$defs` root — dibuktikan bisa gagal lebih dulu
(test merah dengan pesan `root $defs: refs with no matching $defs entry:
PrintQrcode`), lalu hijau setelah `PrintQrcode` ditambahkan ke `sharedTypes`.

## Adopsi kafe

QR ditambahkan pada **kedua** struk (`receipt-thermal.yaml`, `receipt-digital.yaml`)
dengan payload `/kafe/status/{guest_token}` (`absolute: true`) — halaman status
pelanggan yang memang publik (`{ws}/status/:guest_token`). Marker GAP-03 ditutup
di kedua manifest.

**Kartu meja belum** — dan alasannya bukan lagi jalur cetak: QR meja harus
men-resolve meja dari `qr_token` lalu membuat `table-session` dan mengarahkan ke
halaman menu, dan halaman masuk seperti itu belum bisa dinyatakan (resolve entity
dari token route + create-lalu-redirect). Dicatat sebagai task deferred di ledger,
bukan dipasang supaya terlihat tertutup.

## Bukti

**E2E (dev server kafe, `:8099`, record nyata `ORD-2026-00001`)**

| Perintah | Hasil |
| --- | --- |
| `GET /kafe/_ui/print/cafe-order/receipt-thermal/{id}` | `200`, `application/octet-stream`, `.escpos`, 503 byte |
| isi byte | init `ESC @` ✓ · urutan QR `GS ( k` ✓ · payload `http://127.0.0.1:8099/kafe/status/TOK-E2E-1` ✓ · label `Struk digital` ✓ |
| money di struk | `Rp62.500` / `Rp68.750` / `Rp100.000` / `Rp31.250` — **tanpa** `map[amount` |
| order kasir tanpa `guest_token` | 366 byte, **tanpa** urutan QR, total tetap `Rp20.000` |
| `GET …/receipt-digital/{id}` | `501 NOT_IMPLEMENTED` dengan pesan eksplisit (`html` dirender klien) |
| `/kafe/status/x` (tujuan QR) | `200` — halaman status memang hidup |

**Test**

| Perintah | Hasil |
| --- | --- |
| `go test ./pkg/spec/ -run "PrintQrcode\|FormatMoneyDisplay"` | PASS (10 kasus money + 8 kasus payload + default size) |
| `go test ./internal/api/ -run Print` | PASS (`_MoneyAndQR`, `_QRPayloadEmptyOmits`, `_QR`) |
| `go test ./internal/genjsonschema/` | PASS (guard diperkuat + `MatchOnDisk`) |
| `npx vitest run` | 276 → **288** lulus (12 test baru `printRenderer.test.tsx`) |
| `npx tsc --noEmit` | bersih |
| `formspec validate --schema ../../schemas` (kafe) | **0 problem** (69 manifest) |
