# 2026-09-23-001 — Katalog publik: kartu bisa diklik tapi tidak terlihat bisa diklik; foto dipaksa rasio 2.22:1

**Konteks.** Menutup kafe **10.9** (penilaian visual katalog) — sisa satu-satunya
dari verifikasi browser 10.10 yang menilai _keberadaan_ halaman, bukan
_tampilannya_. Sesi meja nyata dibuat lewat alur pelanggan
(`POST /_ui/entity/cafe-order/table-session` → 201), lalu halaman publik
`/kafe/menu/{session_id}` dibuka dan diukur.

Dua cacat terukur, keduanya tidak terlihat dari endpoint mana pun.

## 1. Kartu menu bisa diklik tetapi kursornya `default`

Seluruh kartu menu adalah `<button>` (seluruh kartu = hit target) — jadi ia
memang bisa diklik, dan klik pada **gambarnya** menambahkan item ke keranjang
(diukur: klik `<img>` "Kopi Tubruk" → baris keranjang + total Rp18.000).

Tetapi kursor di atasnya `default`, bukan `pointer`. Penyebabnya: **Tailwind v4
menghapus aturan base v3 `button { cursor: pointer }`** — jadi setiap `<button>`
kembali ke kursor default browser kecuali ia menyebut kelas kursor sendiri.

Yang membuatnya mencolok: **chip kategori** di layar yang sama adalah `<button>`
dengan `cursor-pointer`, jadi dua kontrol bertipe sama menunjukkan dua kursor
berbeda. Terukur di halaman kafe: chip `Semua`/`Kopi`/`Makanan`/`Minuman` →
`pointer`; sembilan kartu menu → `default`.

**Perbaikan** (`src/index.css`, `@layer base`): memulihkan kursor untuk kontrol
yang bisa diklik, dan **mengecualikan yang nonaktif** supaya
`disabled:cursor-not-allowed` tetap bermakna:

```css
button:not(:disabled, [aria-disabled="true"]),
[role="button"]:not([aria-disabled="true"]),
summary,
label[for],
select:not(:disabled) {
  cursor: pointer;
}
```

Diukur di browser: `button` → `pointer`; `button[disabled]` → `default`;
`button[aria-disabled=true]` → `default`; `div[role=button]` → `pointer`.

## 2. Foto dipaksa ke rasio 2.22:1

Kartu memakai kotak **tinggi tetap** `h-28 w-full object-cover`, yang pada lebar
kolom 251px memaksa bingkai **251×112 (2.22:1)** apa pun rasio aslinya. Foto
katalog sengaja beragam (`960×643`, `960×720`, `960×1280`, `480×360`); dua yang
portrait (960×1280) kehilangan **~66% tinggi** — piring tertutup, subjek terpotong.

**Perbaikan** (`src/kinds/form/PickerPanel.tsx`): `aspect-square w-full
object-cover` — bingkai persegi, tidak butuh mendekode gambar (jadi benar juga
saat file masih dimuat). Yang penting: ia juga memberi **setiap** kartu bingkai
sama sehingga baris grid tetap rata; sebelumnya sel tanpa foto memakai kotak lain
dan barisnya jadi tidak rata.

Diukur sesudah perbaikan (sembilan kartu, termasuk dua tanpa foto): **semua
bingkai 251×251 (rasio 1.00)**; pada layar 390px bingkai 147×147, tanpa overflow
horizontal, nama & harga tetap terbaca.

**Sisa (bukan cacat kode).** Tinggi kartu masih beda 16px antar-baris (360 vs 344) karena jumlah baris deskripsi berbeda (2 vs 1) — itu isi data, bukan
bingkai. Dicatat di 10.9.

## Dampak & verifikasi

- `renderers/react-shadcn/src/index.css` — aturan base kursor.
- `renderers/react-shadcn/src/kinds/form/PickerPanel.tsx` — `aspect-square`
  untuk gambar dan untuk placeholder sel tanpa foto.
- `internal/ui/meta.go` — **pembersihan**: empat helper yang jadi mati setelah
  refaktor `routeExists` (`viewPermission`, `entityBacking`, `kindEntity`,
  `blockEntity`) dihapus; `gofmt`/`go vet`/`go test` bersih, `tsc` bersih,
  vitest 306 lulus.

Gate: `gofmt`/`go vet`/`go test ./...` bersih · `tsc --noEmit` bersih ·
vitest 306 lulus.

Referensi plan: `docs_internal/plan/todo.md`; item kafe `10.9`.
