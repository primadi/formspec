# Plan: Kind Reference Docs (docs/kind) — Generated + Enhanced

**Last Updated**: 2026-08-10
**Status**: ✅ P1 complete · ✅ P2 complete · ✅ P3 complete · ✅ P4 complete

> `⬜` not started · `🔄` in progress · `✅` complete · `⏸️` deferred

**Scope**: Per-kind reference docs yang lengkap di `docs/kind/` (33 file, 4 grup),
dengan tabel atribut **generated dari `pkg/spec`** (zero drift) + narasi manual.
**Keputusan user (2026-08-10)**: hybrid generated+narrative · output = file final
dengan protected region · 33 kind saja (tanpa VisualSpecKind) · `@schema {example}`
lengkap · plan terpisah.

---

## Arsitektur

```
make generate-kind-docs ─► generator (reuse internal/genjsonschema collection)
  │                            │
  │              menimpa hanya region <!-- generated:... -->
  ▼                            ▼
docs/kind/<group>/<Kind>.md  (file final = template + output)
```

- **Generated**: tabel atribut (field, tipe, wajib, contoh, deskripsi) dari Go
  struct + godoc + `// @schema {...}` annotations.
- **Manual (protected)**: `Kapan Memakai`, `Contoh Manifest`, `Gotchas` — di luar
  marker, tidak pernah ditimpa. Idempotent (diverifikasi `diff -r`).
- **Sumber tunggal**: `pkg/spec` — sama dengan `schemas/kinds/*.schema.json`.

## Struktur Output

```
docs/kind/
  README.md                    # index 4 grup + marker contract + cara regenerate
  curation/{App,Module}.md                     (2)
  data/{Entity,Service,Config,Migration,Subscription,Workflow,Api,Webhook,Mockup,Integrator,KindDefinition}.md  (11)
  ui/{Page,Form,Table,Dashboard,Widget,Report,Wizard,Kanban,Timeline,Calendar,Listing,ApprovalInbox,NotificationCenter,Print,Theme}.md  (15)
  infra/{Renderer,PersistBackend,Environment,Policy,Datastore}.md   (5)
```

---

## Fase

### P1 — Infra Generator ✅ COMPLETE
- `internal/genjsonschema/converter.go`: export `SchemaAnnotation`; `extractSchemaBody`
  (balanced-brace parse untuk `@schema`)
- `internal/genkinddocs/markdown.go` (baru): markdown emitter + protected region merge
- `cmd/formspec-gen-kind-docs/main.go` (baru): CLI
- `Makefile`: target `generate-kind-docs`

### P2 — Bootstrap ✅ COMPLETE
- `docs/kind/README.md`: index + marker contract + enrichment guide
- Narrative semua 33 file (Kapan Memakai, Contoh Manifest, Gotchas + cross-ref)

### P3 — Enrichment `@schema {example}` ✅ COMPLETE
- Annotation `example` di spec struct: resources.go, entity.go, frontend.go,
  control.go, datastore.go, spec.go

### P4 — Integrasi & Changelog ✅ COMPLETE
- Link dari `ai_skills/formspec-kinds` + `docs/spec/README` + `platform/03-kind-system` ke `docs/kind/`
- Changelog 006 + 007

---

## Verification (semua lulus)

1. ✅ `cp -r docs/kind /tmp/x && generate && diff -r` → kosong (idempotent)
2. ✅ 34 file (33 kind + README)
3. ✅ Tidak ada `TODO:` tersisa di `docs/kind/`
4. ✅ Tidak ada artefak `@schema {` / `"}` bocor (kecuali README yang sengaja)
5. ✅ `go build ./...` untuk genjsonschema + genkinddocs + cmd
6. ✅ `go test ./pkg/spec/...` pass
