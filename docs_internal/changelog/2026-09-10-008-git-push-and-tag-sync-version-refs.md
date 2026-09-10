# 2026-09-10-008 — git-push-and-tag.sh: sinkronisasi versi contoh di docs + site

## Apa

Menambahkan langkah 0 di `scripts/git-push-and-tag.sh`: sebelum tagging, script
otomatis mendeteksi versi contoh yang hardcoded (mis. `v0.4.1`) di
`site/src/components/Install.tsx`, `docs/guides/install.md`,
`site/public/install.sh`, dan `site/public/install.ps1`, menggantinya ke
`VERSION` release, lalu commit — sehingga tag selalu berisi site/docs dengan
versi yang benar.

## Kenapa

Bagian Install di formspec.dev (`site/src/components/Install.tsx`) menampilkan
preview versi installer yang hardcoded. Sebelumnya versi ini tidak ikut
ter-update saat release, sehingga landing page bisa menampilkan versi lama
(mis. tetap `v0.4.1` padahal sudah release `v0.4.2`). Installer sendiri resolve
versi terbaru via GitHub API saat runtime — yang perlu di-replace hanya teks
contoh/preview.

## File terkena dampak

- `scripts/git-push-and-tag.sh` — langkah 0 `sync_version_refs` + reorder
  branch check ke atas (sebelum clean-tree check)
- `docs/guides/releasing.md` — dokumentasi perilaku baru

## Referensi

- `docs/guides/releasing.md` (Cara cepat — script satu perintah)
- Changelog terkait: `2026-09-10-006-script-git-push-and-tag.md`,
  `2026-09-10-007-release-loop-fail-fast.md`
