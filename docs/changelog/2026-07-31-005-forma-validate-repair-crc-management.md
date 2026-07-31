# 2026-07-31-005 — `forma validate` (engine+schema) + repair `crc-management` + fix skill

**Apa:** Menambahkan gate validasi manifest dan memperbaiki contoh yang
di-generate skill.

## Perubahan

- **`forma validate --spec <path>`** (baru, `cmd/forma/validate.go`): dua lapis —
  1) engine loader (`internal/manifest`) sebagai hard gate apa yang `forma dev`/
  `apply` terima, 2) JSON Schema per kind (`schemas/kinds/*` + `$defs` dari
  `forma.schema.json`, via `santhosh-tekuri/jsonschema/v6`) sebagai kontrak
  untuk semua kind. Exit 1 bila ada yang gagal. Mengakhiri status "not
  implemented" (todo 3.1.1).
- **Repair `examples/crc-management/spec` (21 manifest)** yang di-generate
  dengan sintaks usang: hapus `expose: all/read` (bentuk array adalah
  satu-satunya; tanpa expose = UI-only) dan `lifecycle: {doc_status: true}`
  (string enum, doc_status default-on); tambah `version: v1` (wajib schema);
  ubah `type: relation/child, target:` → `relation: {type: belongs_to,
  resource: <mod.entity>}`; pindahkan state machine Workflow yang salah ke
  Entity `checklist-submission` (`state_machine` + `actions`), Workflow jadi
  bentuk canonical (`entity` + `on.transition` + `steps` + `on_reject`);
  App + `version/vendor/root_url` + menu; Module `depends_on:` → `depends:`.
- **Fix `ai_skills/forma-kinds/SKILL.md`** (akar masalah): contoh Entity/App/
  Module/Workflow diperbarui ke sintaks canonical + gotchas (`expose` array,
  `target` diabaikan senyap, `depends`, `spec.version`, `on:` bukan boolean).
- **`Makefile`**: target `validate-spec SPEC=<dir>`.
- **Unit test** `cmd/forma/validate_test.go` (8 kasus lapis schema).

## Kenapa

Generate dari skill yang contohnya usang + tanpa gate validasi menghasilkan
YAML yang tidak sesuai kontrak (`schemas/`), dan `Loader.Validate` hanya
deep-validate Entity — Workflow/Form/Module yang salah lolos senyap. Skill
yang benar + gate `forma validate` mencegah ini berulang.

## Catatan / gap yang diketahui

- Lapis schema lebih ketat dari engine untuk shorthand `guard`/`render`
  (`UnmarshalYAML` scalar+map tak bisa diekspresikan generator schema) —
  gunakan bentuk objek; lihat `docs/cli-tools/02-forma-cli.md` §2.
- `examples/Crc-Checklist` & beberapa `verticals/` memakai sintaks usang
  (Form `render: drawer`, Module `order`, Config `schema/values`, guard string)
  — lolos engine tapi gagal lapis schema; kandidat cleanup terpisah.
- Honesty scan Starlark ditunda → todo 3.1.1a.

## File terdampak

- `cmd/forma/validate.go`, `cmd/forma/validate_test.go`, `cmd/forma/main.go`
- `examples/crc-management/spec/**` (juga `internal/entity/testdata/crc-management/spec` — bind mount)
- `ai_skills/forma-kinds/SKILL.md`
- `Makefile`, `go.mod` (+`jsonschema/v6`), `docs/cli-tools/02-forma-cli.md`, `docs/plan/todo.md`
- `docs/plan/schema-validation-gate.md` (baru)

## Referensi

- `docs/spec/backend/01-core-basic.md` §8.4 (`expose`), §1.2 (doc_status), §14 (state machine)
- `docs/spec/backend/02-core-extended.md` §1–§2 (state machine, Workflow)
- `docs/plan/todo.md` 3.1.1 / 3.1.1a
