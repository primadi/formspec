# Plan — S5 / S11 / S12 / S14 + D1–D7 (TODO kafe 1.8 & 1.9)

Sumber: `examples/kafe/gaps_found/TODO.md` item **1.8** (S5/S11/S12/S14) dan **1.9**
(D1–D7 normatif), dengan rujukan `13-kelengkapan-spec-untuk-kafe.md` §B dan
`decisions-needed.md`.

## Ruang lingkup 1.8

Empat konstruk bahasa spec yang belum bisa dinyatakan. Semuanya **deklaratif +
divalidasi**: validator harus menolak bentuk yang "terlihat terpasang tapi tidak
menjalan apa pun" (kelas kegagalan yang berulang di ledger ini — #52, #53, GAP-33).

| Sub | Gap | Konstruk                                                                                     | Enforcement yang nyata hari ini                                                                                              |
| --- | --- | -------------------------------------------------------------------------------------------- | ---------------------------------------------------------------------------------------------------------------------------- |
| a   | S5  | `scope: {dimension, field, required}` + `assignments: [{dimension, field, principal_field}]` | `assignments` → atribut sesi (`from: session`); gerbang validator: atribut `from: session` wajib punya sumber                |
| b   | S11 | `FieldType` `percent`                                                                        | penyimpanan numerik + renderer/format persen                                                                                 |
| c   | S12 | `unit: {base, convertible}` pada field satuan                                                | validator: `base`/`convertible` wajib ada di `enum_values`                                                                   |
| d   | S14 | `maintained_by` + `invariants: [{unique, message}]` pada `characteristic: summary`           | `maintained_by` wajib menunjuk script yang ada; `invariants.unique` wajib punya unique index yang cocok (DB yang menegakkan) |

### Keputusan desain

1. **`row_scope` vs `scope`.** `EntitySpec.Scope []FilterSpec` (dari 1.1) berisi
   **filter yang ditegakkan**, sedangkan S5 meminta **deskriptor dimensi** dengan
   nama `scope`. Karena tidak ada satu pun spec di repo yang memakai bentuk 1.1
   (kafe sadar menundanya), bentuk filter di-`rename` menjadi `row_scope`
   (Go: `RowScope`) dan nama `scope` diberikan ke deskriptor. Tidak ada bentuk
   union — generator schema hanya bisa mengekspresikan satu bentuk per nama
   (preseden 1.7).
2. **`scope.required` tidak menegakkan apa pun sendiri.** Ia pernyataan model data
   ("setiap baris punya nilai dimensi"); penyaringan otomatis tetap item **3.5**
   sesuai urutan ledger. `required: true` divalidasi konsisten dengan field-nya
   (`field` wajib `required: true`), jadi tidak ada deklarasi kosong.
3. **Adopsi `row_scope` di kafe DITUNDA ke 3.5.** Menyalakannya sekarang membuat
   seluruh list kasir — dan identitas dev tanpa baris `employee` — **403** (fail
   closed yang benar, tapi aplikasi tak terpakai). 1.8 menyediakan konstruk +
   sumber nilai; 3.5 yang menyalakannya. Marker `GAP-08` diperbarui agar
   menunjuk 3.5, bukan diklaim tertutup.
4. **`assignments` → atribut sesi.** `internal/api/scope.go` menyelesaikan nilai
   dimensi secara lazy: hanya ketika sebuah `row_scope: {from: session}` menunjuk
   atribut yang belum ada di identitas. Sumbernya entity yang mendeklarasikan
   `assignments` (`principal_field` dicocokkan dengan `Identity.Username`), dibaca
   lewat `EntityStoreProvider` + `FindByField`, di-memo per (workspace, user,
   field) dengan TTL pendek.
5. **Gerbang validator (inti kejujuran).** `formspec validate` menolak
   `row_scope` `from: session` yang atributnya tidak punya sumber: bukan atribut
   identitas bawaan (`principal_id`/`user_id`/`username`/`workspace_id`), bukan
   `scope.field` milik entity itu, dan bukan `field` dari `assignments` mana pun.
   Tanpa gerbang ini, konstruk itu hanya 403 selamanya tanpa gejala.

## File yang tersentuh

- `pkg/spec/entity.go` — `ScopeDecl`, `AssignmentDecl`, `UnitDecl`,
  `InvariantDecl`, `MaintainedBy`; `Scope`→`RowScope`; `FieldPercent`; validasi.
- `pkg/spec/scope.go` (baru) — `AssignmentSource` + helper pencarian sumber atribut.
- `internal/entity/registry.go` — `AssignmentSources()` (scan `r.specs`).
- `internal/api/scope.go` — penyelesaian atribut sesi dari `assignments`.
- `cmd/formspec/validate*.go` — gerbang sumber atribut + invarian.
- `renderers/jsonb-persist/ddl.go` — tipe SQL untuk `percent`.
- `renderers/react-shadcn/src/**` — widget/format/zod untuk `percent`.
- `examples/kafe/spec/**` — adopsi deklarasi (scope, assignments, unit, invariants,
  percent).
- `docs/spec/backend/01-core-basic.md` (+ `05-field-types.md`) — normatif 1.8 & 1.9.
- `schemas/**` — regenerasi.

## Urutan

1. `pkg/spec` (struktur + validasi) → 2. regenerasi schema → 3. perbaikan
   kompilasi seluruh repo (`Scope`→`RowScope`) → 4. `internal/api` resolver +
   `internal/entity` index → 5. gerbang validator → 6. test → 7. adopsi kafe →
2. dokumen normatif (1.8 + 1.9) → 9. TODO + changelog.

## Estimasi

- S5 (1.8a): **large** — menyentuh spec, registry, API resolver, validator.
- S11 percent (1.8b): **small** — tipe tertutup + pemetaan storage + render.
- S12 unit (1.8c): **small** — deklarasi + validasi (konversi = 4.6).
- S14 (1.8d): **small-medium** — deklarasi + validasi ref script & unique index.
- D1–D7 (1.9): **small** — dokumen normatif.
