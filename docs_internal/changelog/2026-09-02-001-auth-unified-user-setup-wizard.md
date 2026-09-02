# 2026-09-02-001 — Fase 1 & 2 Redesign Auth: Unified User + Setup Wizard

## Apa

Mulai eksekusi plan redesign auth (`/memories/session/plan.md`).

### Fase 1 — Unified User Model

- Hapus avatar workspace kedua di `SideNavShell.tsx` / `TopNavShell.tsx` —
  kini hanya satu avatar (user). Workspace ditampilkan sebagai label di
  dropdown `UserMenu` ("Workspace: default").
- Tambah field user `email` + `status` (active|pending|disabled) di
  `internal/auth/user.go` (User struct, userFromRecord, CreateUser) dan
  entity manifest `internal/auth/module/master/user/entity.yaml` + forms.
- Login memblokir user `status: pending` (belum di-approve admin).

### Fase 2 — First-Run Setup Wizard

- Deteksi setup: `Service.SetupRequired` (workspace tanpa user) +
  `Service.SetupFirstAdmin` (buat admin pertama + seed owner roles).
- Endpoint publik `GET/POST /{ws}/_ui/setup` (`internal/api/setup_handler.go`).
- Flag `setup_required` di meta bundle (`internal/ui/meta.go` + meta handler)
  — non-fatal jika deteksi gagal.
- Frontend: `SetupScreen.tsx` (form admin pertama) + route
  `/{ws}/_admin/setup` + redirect saat `bundle.setup_required && !token`.

## Kenapa

User: (1) dua avatar membingungkan — harus satu jenis user di level
workspace; (2) prod self-host butuh bootstrap admin pertama tanpa
`formspec-ctl` — muncul dialog setup saat user kosong.

## Verifikasi

- Fase 1: browser `_admin` → satu avatar; dropdown menampilkan
  "tester1 / Workspace: default". Login user pending → 401.
- Fase 2 (DB kosong): `GET /_ui/setup` → `setup_required: true`; meta
  `"setup_required":true`; `POST /_ui/setup` → created; setelahnya
  `setup_required: false`; login admin → 200 admin meta; POST setup ulang → 409.
- `go test ./internal/auth ./internal/api ./internal/ui`: hijau.

## File terdampak

- `renderers/react-shadcn/src/shell/{SideNavShell,TopNavShell,UserMenu,index}.tsx`
- `renderers/react-shadcn/src/shell/SetupScreen.tsx` (baru)
- `renderers/react-shadcn/src/App.tsx`, `types/manifest.ts`
- `internal/auth/{user,service}.go`, `internal/auth/module/master/user/*`
- `internal/api/{setup_handler.go (baru),router.go,meta.go}`
- `internal/ui/meta.go`
- `registry/web/dist/` (sync build)

## Referensi

- Plan: `/memories/session/plan.md` (Fase 1 & 2)
- Todo: 5.2.11 (Fase 1), 5.2.12 (Fase 2)
