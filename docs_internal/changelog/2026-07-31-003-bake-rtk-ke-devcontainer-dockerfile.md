# 2026-07-31-003 — Bake rtk ke .devcontainer Dockerfile

**Apa:** Menambahkan instalasi `rtk` (Rust Token Killer, `github.com/rtk-ai/rtk`)
langsung ke `.devcontainer/Dockerfile` sehingga binary + hook-nya bertahan
setelah container di-rebuild.

## Perubahan

- **Binary**: di-install sebagai user `vscode` via installer resmi
  (`curl -fsSL .../install.sh | sh`) dengan versi di-pin `RTK_VERSION=v0.44.1`
  (bump saat upgrade), sehingga terpasang di `~/.local/bin` — path ini sudah
  otomatis masuk PATH via `~/.profile` bawaan base image.
- **Hook**: `rtk init -g` (hook Claude Code global + `RTK.md`) ikut di-bake,
  dengan `RTK_TELEMETRY_DISABLED=1` agar non-interactive. Sebelumnya state ini
  hidup di `$HOME` (writable layer container) dan hilang saat rebuild.

## Alasan

`~/.local/bin` dan `~/.claude` tidak di-mount sebagai volume di
`.devcontainer/compose.yaml` (hanya workspace bind mount + `minio-data`),
jadi `rtk` beserta konfigurasinya hilang setiap Rebuild Container. Di-bake ke
Dockerfile agar setup otomatis pulih tanpa install ulang manual.

## File terdampak

- `.devcontainer/Dockerfile`

## Referensi

- Install resmi: `https://github.com/rtk-ai/rtk` (Quick Install + `rtk init -g`)
