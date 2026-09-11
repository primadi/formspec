# 2026-09-11-003 — Auto-detect tab OS di section Install landing page

**Referensi plan:** `/memories/session/plan.md` (sesi Copilot — auto-detect tab Install)

## Apa yang diubah

Section `#install` di `site/src/components/Install.tsx`:

1. **Tab "Linux" dihapus** — sebelumnya redundan dengan tab "macOS / Linux"
   (perintah installer, go install, dan manual identik persis). Sekarang hanya
   2 tab: `macOS / Linux` dan `Windows`.
2. **Auto-pilih tab default dari OS browser** — helper `detectDefaultTab()`
   membaca `navigator.userAgent` (match case-insensitive `windows`) sehingga
   pengunjung Windows langsung melihat perintah `irm … install.ps1 | iex`
   tanpa klik tab. Guard `typeof window === "undefined"` untuk SSR-safety.
3. **Catatan WSL/Git Bash** di tab Windows — mengarahkan ke tab
   "macOS / Linux" (tombol yang langsung switch tab).

## Kenapa

Perintah `curl … install.sh | sh` sudah auto-detect OS/arch di
`site/public/install.sh` (linux/darwin × amd64/arm64), jadi pemisahan tab
"Linux" vs "macOS / Linux" hanya menambah noise. Autodetect yang bermakna di
landing page adalah di sisi browser (pilih tab), bukan di shell (sudah benar).

## File terkena dampak

- `site/src/components/Install.tsx` — `METHODS`, `OS_TABS`, `useState`,
  helper `detectDefaultTab()`, catatan WSL
- Tidak diubah: `site/public/install.sh`, `site/public/install.ps1`,
  `docs/guides/install.md` (sudah memuat windows-arm, tidak ada referensi 3-tab)

## Verifikasi

- `cd site && npm run build` — sukses (tsc -b + vite build, 1806 modules).
- Dev server menyajikan kode transform terbaru (curl terhadap
  `/src/components/Install.tsx` menampilkan `detectDefaultTab` + `OS_TABS`
  2 item + catatan WSL).
- Verifikasi visual via browser tidak dapat dijalankan dari environment ini
  (browser tool tidak menjangkau dev server container); build + typecheck
  sebagai pengganti.
