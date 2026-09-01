# 2026-09-01-004 — Devserver helpers untuk formspec-registry (auto-kill, port resolve, hot-reload)

## Apa

Package baru `internal/devserver` mengekstrak helper dev-mode dari
`cmd/formspec/dev.go` agar dipakai bersama:

- `AutoKillPrevious` / `WritePIDFile` / `CleanupPIDFile` — PID file
  `.formspec/dev.pid` (dev) / `.formspec/registry.pid` (registry)
- `EnsurePort` / `FindProcessOnPort` / `ExtractPort` / `KillDescendants` /
  `ProcessAlive` — resolusi konflik port (kill instance sebelumnya yang
  memegang port; proses asing → error deskriptif)
- `WatchSpec` — fsnotify watcher spec dir, debounce 300ms, `App.ReloadSpec()`
- `ServeAppUntilSignal` — jalankan `App.ListenAndServe()` + graceful
  shutdown (workers + HTTP) saat SIGINT/SIGTERM

`cmd/formspec-registry/main.go` kini memakai semuanya: auto-kill instance
sebelumnya, resolusi port (`formspec-regist`, comm 15-char), graceful
shutdown, dan **spec hot-reload saat `--spec` eksplisit** (spec embedded
adalah snapshot compile-time — watcher useless; binary mencetak hint untuk
pakai `--spec registry/spec`).

`cmd/formspec/dev.go` di-refactor jadi wrapper tipis ke devserver (nama
fungsi lokal dipertahankan agar pemanggil & test tidak berubah).

## Kenapa

User: fitur auto-restart saat spec berubah + auto-kill saat port dipakai
hanya ada di `formspec dev`; `formspec-registry` yang dijalankan manual via
`log.Fatal(app.ListenAndServe())` tidak punya keduanya.

## File terdampak

- `internal/devserver/devserver.go` (baru)
- `cmd/formspec/dev.go` (refactor → delegasi)
- `cmd/formspec-registry/main.go` (wire devserver)

## Referensi

- Plan: `docs_internal/plan/section-block-align.md` (sesi dev-DX terkait),
  todo 3.2.x dev server
- Changelog terkait: `2026-09-01-003` (konteks registry dev)
