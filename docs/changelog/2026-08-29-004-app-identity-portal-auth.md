# 2026-08-29-004 — App identity (title/logo) + portal auth UX

## Apa yang diubah

Lima permintaan portal registry:

- **AppSpec.title** — display name human-readable (spasi boleh) terpisah dari
  `metadata.name` (machine identifier). Dipakai brand bar shell,
  document.title, dan fallback footer. **AppSpec.logo** — nama icon lucide
  di samping title.
- **NoNavShell** — brand bar: logo icon + title; nav link dengan **active
  state** (font-medium + text-foreground pada path aktif); auth area:
  Sign in/Sign up saat anonim, Log out saat signed-in.
- **LoginScreen mode register** — field Display Name + POST
  `/{ws}/_ui/auth/register` lalu auto-login; toggle Sign in/Sign up; route
  `/register` (top-level + in-surface `{surfacePath}/register`).
- **Public surface boot** memakai session tersimpan — sebelumnya
  `setSession(workspace, "")` menghapus session yang baru dibuat saat
  navigasi kembali ke portal; kini `boot(workspace)` me-restore persisted
  session (signed-in user dipertahankan, permission berlaku).
- **document.title** — `<page title> · <App title>` per halaman (dedup bila
  sama); sebelumnya generik "web".
- **registry.yaml** — `title: "FormSpec Registry"` + `logo: "package"`.

## Verifikasi

- E2E browser: register `demo-vendor2` → auto-login → redirect portal →
  header Log out → session persisten lintas navigasi; nav active state;
  document.title benar.
- vitest 155 pass; `go build ./...` hijau; validate 11 manifest 0 problem.

## Deferred

- **Row-level ownership** (update/delete module milik sendiri) — butuh fitur
  **record-level authorization** di framework (permission check per-record
  via relasi `vendor.owner_username`). Saat ini: anonim read-only (permission
  gating), signed-in user dengan role mengelola via `/_admin`. Todo 14.a.

## Referensi

- Permintaan user 2026-08-29 (5 poin) · todo 14.a
