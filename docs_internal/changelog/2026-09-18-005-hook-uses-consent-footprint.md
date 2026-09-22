# 4.7 — `HookDecl.uses`: akses script hook terlihat di consent footprint (#34)

**Tanggal:** 2026-09-18 · **Plan:** `docs_internal/plan/fase-4-kafe-stok-hpp.md`
(4.7) · **TODO:** `examples/kafe/gaps_found/TODO.md` 4.7

## Apa yang diubah

`HookDecl` (`pkg/spec/entity.go`) mendapat field `Uses *UsesDecl` — bentuk yang
sama dengan `Action.Uses`. Hook uses didaftarkan di permission registry dengan
nama sintetis `hook:<on>:<action|event>`, dan honesty scan membandingkan `uses`
hook dengan pemakaian nyata di script.

## Kenapa

Sebelum ini `HookDecl` tidak punya `uses`, sehingga akses script hook **tidak
terlihat di consent footprint** — script bisa membaca/menulis resource yang tidak
pernah dinyatakan manifest. Itu kelas kegagalan yang sama dengan #49/#50: sesuatu
yang berjalan tanpa bisa diaudit dari manifest.

## File yang terkena dampak

- `pkg/spec/entity.go` — `HookDecl.Uses`.
- `internal/entity/registry.go` — registrasi hook uses + helper `hookNameFor`.
- `cmd/formspec/honesty.go` — scan `uses` hook (sebelumnya hanya `ctx.environment`).
- `cmd/formspec/honesty_test.go` — `TestHonestyScan_HookUsesDeclared`.
- `docs/spec/backend/02-core-extended.md` §15 — `uses` pada hook.
- `examples/kafe/spec/modules/cafe-master/master/menu-item-price/entity.yaml`,
  `examples/kafe/spec/modules/cafe-order/transaction/shift/entity.yaml` —
  `uses: {primitives: [db]}` pada 3 hook guard.
- `schemas/` — ter-regenerasi (`HookDecl` bertambah field).

## Bukti

| Perintah | Hasil |
| --- | --- |
| `go test ./cmd/formspec/ -run TestHonestyScan` | 7 PASS |
| `formspec validate` kafe **sebelum** deklarasi uses | **2 problem** — `hook:before:create: uses ctx.db() but does not declare it` (gerbang bekerja) |
| `formspec validate` kafe **sesudah** deklarasi uses | **0 problem** (69 manifest) |
| `go test ./...` | hijau |
