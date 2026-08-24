# 2026-08-24-007 — NumberInput: Spinner Hormati min/max + Flag Merah Out-of-Range

## Apa yang diubah

Dua permintaan user: (1) tombol step up/down harus menghormati `min`/`max`;
(2) nilai di luar range — sebaiknya di-flag merah (bukan di-ignore diam-diam,
bukan di-clamp diam-diam) supaya user tahu kenapa nilainya invalid.

### Frontend (`renderers/react-shadcn`)

- `widgets/NumberInput.tsx`:
  - Prop baru **`positive?: boolean`** (rule spec `positive` → `> 0`,
    `05-field-types.md` §3). Boundary spinner untuk `positive` = langkah positif
    terkecil (`1` untuk integer, `1/10^scale` untuk decimal) — tombol up/down
    tidak bisa ke 0/negatif.
  - **Range validation**: nilai di luar `min`/`max`/`positive` di-flag merah
    (border + teks merah + tooltip `title`), bukan di-clamp/ignore. Pesan:
    "Harus lebih dari 0" / "Minimal X" / "Maksimal Y" / "Nilai antara X dan Y".
    Hanya berlaku untuk nilai numerik (empty/null bukan error).
  - `min`/`max` diteruskan ke atribut input native → spinner native menghormati
    batas.
- `widgets/ChildTable.tsx` — child integer/decimal kini mengirim `min`/`max`
  (dari rule `min`/`max`) dan `positive` (dari rule `positive`) ke `NumberInput`.
  Sebelumnya child field tidak mengirim batas sama sekali → spinner bisa ke
  bawah 0.

## Keputusan UX (menjawab pertanyaan user)

- **Tidak di-ignore diam-diam** — user harus dapat feedback kenapa input
  ditolak.
- **Tidak di-clamp diam-diam** — clamping tanpa info membingungkan.
- **Flag merah inline** (border + teks + tooltip) — ringan, cocok untuk sel
  table; user bisa lihat & koreksi sebelum submit. Server tetap penjaga akhir
  (spec §3: duplikasi frontend murni UX).

## Verifikasi

- `tsc -b` bersih; `vitest` 103 pass.
- Browser (child `quantity` = decimal scale 2 + `positive`): `min="0.01"`;
  empty → tidak error; `0`/`-1` → tooltip "Harus lebih dari 0" + merah;
  `2.5` → valid.

## Referensi

- Spec: `docs/spec/backend/05-field-types.md` §3 (rule `positive`, `min`, `max`)
- Lanjutan: `docs/changelog/2026-08-24-006-fix-select-all-ketik-numberinput-terblokir.md`
