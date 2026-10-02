# 2026-10-02-004 — Tutup shift 1 klik dari daftar (row action → wizard)

Todo: **5.25.8** · Melanjutkan `2026-10-02-002` (peluncur + commit) dan
`2026-10-02-003` (warisan reachability).

## Apa

Dua perubahan kecil agar alur kasir tidak perlu membuka detail dulu:

1. **`TableRenderer.handleRowAction`** memakai `findWizardForTransition` —
   aturan yang sama dengan `DetailPage` — sebelum menjalankan action. Dulu klik
   jatuh ke `POST /{module}/{entity}/{id}/{action}`, route yang transisi `via`
   tidak punya (`has_route: false`), jadi 404.
2. **Transisi `close-shift`** (`examples/kafe/.../shift/entity.yaml`) diberi
   `ui: {button_label: "Tutup Shift", icon: clock}`. Derived table hanya
   menambahkan custom action yang punya `ui` (`engine/derive.ts`), jadi tanpa
   ini tombolnya tidak muncul di baris.

Hasilnya: **Kas & Shift → Shift Kasir → [Tutup Shift]** langsung membuka wizard.

## Kenapa

Wizard hanya bisa dijangkau lewat halaman detail (View → Actions). Untuk operasi
rutin tutup shift, itu dua langkah tambahan tanpa manfaat — dan sebelum
`2026-10-02-002`, jalur itu pun bypass.

## File

- `renderers/react-shadcn/src/kinds/table/TableRenderer.tsx` — cabang wizard.
- `renderers/react-shadcn/src/kinds/table/wizard-row-action.test.ts` (baru).
- `examples/kafe/spec/modules/cafe-order/transaction/shift/entity.yaml` — `ui`.

## Bukti

**Terukur di browser (kafe-pos, `kasir`):**

| Observasi                            | Hasil                                                                                                 |
| ------------------------------------ | ----------------------------------------------------------------------------------------------------- |
| Baris **Open** di daftar Shift Kasir | tombol **"Tutup Shift"**                                                                              |
| Tiga baris **Closed**                | **tidak** ada tombol itu (`isActionAllowedForRow` menyaring `from`)                                   |
| Klik tombol                          | `/kafe/app/pos/wizard/close-shift-wizard?id=01a0fc5b-…`                                               |
| Selesai                              | `counted_cash={amount:"260000",currency:"IDR"}`, `note="lebih 10000"`, `status=closed`, `version` 1→2 |
| Sesudahnya                           | tombol hilang dari baris itu (status jadi `closed`)                                                   |

**Guard test** `wizard-row-action.test.ts` (3 test) — **dibuktikan gagal** (3/3)
saat cabang wizard disuntik-keluar dari `TableRenderer.tsx`, lalu file
dipulihkan dari backup `/tmp` (bukan `git checkout`).

**Otomatis:** `tsc -b` bersih; `npx vitest run` **48 file / 611 test** lulus;
`go test ./...` 0 FAIL; `formspec check -f examples/kafe/spec` 0 error / 0
warning (transisi dengan `ui:` tetap valid).

## Sisa

- **5.25.2 / 5.25.3 / 5.25.6 ⏸️** tak tersentuh.
- `expected_cash`/`difference` tetap dihitung di klien (computed field), bukan
  server — pre-existing, di luar scope.
