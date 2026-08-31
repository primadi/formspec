# 2026-08-24-006 — Fix: Select-All + Ketik di NumberInput (scale) Terblokir

## Apa yang diubah

User melaporkan bug kecil: field decimal `scale: 2`, isi "11.21", select all,
ketik "3" → tidak berubah jadi "3".

Akar masalah: `type="number"` **tidak mengekspos `selectionStart`/`selectionEnd`**
(mengembalikan `null`). Handler `onKeyDown` untuk pembatasan `scale` memakai
`selectionStart ?? val.length` — karena `null`, dianggap "kursor di akhir",
lalu `decimals >= scale` → `preventDefault()`. Jadi mengetik digit apa pun saat
select-all (yang seharusnya mengganti seluruh nilai) ikut diblokir.

### Frontend (`renderers/react-shadcn`)

- `widgets/NumberInput.tsx` — blokir `onKeyDown` berbasis `scale` **dihapus**.
  Pembatasan `scale` kini sepenuhnya ditangani oleh sanitize-on-change
  (`n.toFixed(scale)`), yang benar untuk semua jalur edit (ketik, paste,
  select-all + replace). Blokir `onKeyDown` untuk **integer** (`.`/`,`/`e`/`E`)
  tetap dipertahankan — itu selalu benar karena karakter tersebut tidak pernah
  valid di integer.

## Verifikasi

- `tsc -b` bersih; `vitest` 103 pass.
- Browser: keydown "3" tidak lagi `preventDefault`; simulasikan select-all +
  replace "11.21" → "3" → nilai jadi "3" (sanitize-on-change benar).
- Catatan: Playwright `page.keyboard.type` tidak bisa mereproduksi select-all +
  ketik pada `type="number"` (keterbatasan simulasi keyboard), tapi logika
  inti terverifikasi.

## Referensi

- Lanjutan: `docs/changelog/2026-08-24-005-childtable-pakai-numberinput-scale-child-field.md`
