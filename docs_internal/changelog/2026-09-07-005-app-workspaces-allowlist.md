# 2026-09-07-005 — `AppSpec.Workspaces[]` — Allowlist Mount Workspace per App

Plan: `docs_internal/plan/named-workspaces.md` (lanjutan changelog
2026-09-07-004; keputusan diskusi 2026-09-07 — `App.workspaces[]` dua lapis,
AppVersion = artifact + binding control plane).

## Apa yang berubah

`kind: App` kini punya field opsional `spec.workspaces` (`*[]string`) yang
membatasi di workspace mana App di-mount. **Tiga state** (pointer membedakan
absen vs eksplisit kosong):

1. **Field absen** (default) → App di-mount di semua workspace (semua
   manifest existing tetap valid, zero migration).
2. **`workspaces: []` eksplisit** → App _staged_: tervalidasi + ter-reload,
   tapi di-mount di mana pun (`formspec check` memberi warning).
3. **`workspaces: [slug,...]`** → hanya workspace tersebut.

## Enforcement (request-time — tidak perlu wiring ReloadSpec baru)

- `GET /{ws}/_ui/_meta/apps` — hanya menampilkan App yang mengizinkan `ws`;
  sekaligus kini mengekspos `version` + `vendor` (keputusan AppVersion:
  inspeksi versi live; per-workspace versioning = control-plane concern).
- App-scoped `/_meta/ui` — App di luar allowlist ditolak dengan pesan sama
  dengan App tidak-ada (anti-enumeration).
- SPA mount `root_url` App — 404 bila workspace tidak diizinkan (non-dev-ui;
  di dev-ui shell tetap dilayani Vite, enforcement tetap di meta API).

## Perubahan lain

- `internal/api/middleware.go` — fallback "no validator" TIDAK lagi memaksa
  workspace `"demo"`, sehingga workspace URL dipertahankan (test wshub
  dimigrasi broadcast ke workspace URL).
- `internal/genjsonschema` — dukungan `*[]T` (pointer-slice → array + nullable)
  di schema generator (`AppSpec.Workspaces` dulunya ter-render "object").
- `formspec check` — warning baru untuk App staged.
- `pkg/spec` — `IsValidWorkspaceSlug` diekspor; `AppSpec.MountsWithin(ws)`;
  `ValidateAppSpec` kini memvalidasi `version` sebagai semver bila diisi.

## File terdampak

`pkg/spec/{workspace.go,resources.go}`, `internal/api/{meta.go,router.go,
middleware.go}`, `internal/genjsonschema/{converter.go,generator.go}`,
`cmd/formspec/check.go`, schemas regenerated, `docs/kind/curation/App.md`
(regen), `docs/spec/platform/02-workspace-app-module.md` §3.1 (baru).

## Verifikasi runtime (cafe, allowlist `workspaces: [cafe]`)

- `/cafe/app/kafe` → 200; `/default/app/kafe` → 404 (SPA embedded)
- `/cafe/_ui/_meta/apps` → `cafe@1.0.0 (formspec)`; `/default/...` → `[]`
- Hot-reload menerapkan perubahan allowlist tanpa restart.
