# Changelog 2026-08-14-003 — Rename `renderers/jsonbpersist` → `renderers/jsonb-persist`

**Apa:** Folder PersistBackend renderer di-rename dari `renderers/jsonbpersist/`
menjadi `renderers/jsonb-persist/`, menyelaraskan nama folder kode dengan docs
identity `jsonb-persist` (`docs/renderers/jsonb-persist/` — nama sengaja menandai
strategi skema JSONB, bukan engine SQL). Semua import path Go, komentar, skill,
dan docs aktif diperbarui.

**Kenapa:** Menutup mismatch nama folder kode vs docs. Karena paket
`renderers/jsonbpersist/` mendeklarasikan `package db` (dan `datastore/` →
`package datastore`), rename **tidak mengubah identitas paket** — hanya string
path yang berubah; import unaliased tetap resolve ke `db` (Go memakai nama dari
`package` clause, bukan elemen terakhir path).

**File terdampak:**

- Rename: `renderers/jsonbpersist/` → `renderers/jsonb-persist/` (paket `db` + subpaket `datastore`)
- Go import (21 file): `cmd/formspec/{dev.go,generate.go}`, `internal/action/{deliver.go,
deliver_test.go,sidecar.go,sidecar_test.go}`, `internal/api/{api_test.go,generator.go,
handler.go,handler_txscope_starlark_test.go,handler_txscope_test.go,handler_update_test.go,
meta_test.go,wshub_permission_test.go,wshub_test.go}`, `internal/entity/{registry.go,
registry_test.go}`, `internal/sidecar/{ctx.go,txscope_test.go}`, `resource/{formspec.go,
syncagent.go}`
- Go komentar (11 file): `cmd/formspec/{generate.go,generate_test.go}`, `internal/action/
{deliver.go,sidecar.go}`, `internal/api/handler.go`, `internal/events/hub.go`,
  `internal/sidecar/txscope_test.go`, `internal/starlark/{executor.go,resource.go}`,
  `renderers/jsonb-persist/datastore/{connection.go,factory.go}`
- Non-Go aktif: `.github/copilot-instructions.md`, `.github/skills/forma-backend/SKILL.md`,
  `docs/architecture/08-repo-structure.md`, `docs/renderers/jsonb-persist/{01-architecture,
04-query-and-keys}.md`, `docs/renderers/realtime.md`, `docs/runtimes/04-formspec-sidecar.md`,
  `examples/arisan/docs/engine-sqlite-deadlock.md`

**Tidak diubah (scope exclusion):** file historis `docs/changelog/`, `docs/plan/`,
`docs_old/`, `reff_docs/`, `docs-site/dist/` (build output), `docs-site/docs` (symlink).

**Verifikasi:** `go build ./...` ✅ · test paket terdampak (jsonb-persist,
datastore, internal/api, internal/action, internal/entity, internal/sidecar,
cmd/formspec) ✅ · grep sisa `jsonbpersist` di area live = 0.

**Referensi:** task 1 `2026-08-14-002` (pola rename + scope exclusion),
keputusan user (nama `jsonb-persist`, scope docs aktif saja).
