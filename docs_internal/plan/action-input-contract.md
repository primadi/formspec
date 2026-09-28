# Plan: Kontrak input untuk transisi & action (`params.inputs`)

**Status:** In progress
**Fase plan:** lintas-fase
**Referensi spec:** `docs/spec/backend/01-core-basic.md` §1.6/§5.1,
`docs/spec/backend/02-core-extended.md` §1.6/§2, `docs/spec/frontend/06-page-kinds.md` §2/§11

## Masalah

Transisi state machine, action, dan keputusan approval **tidak punya kontrak input deklaratif**.
Yang ada:

- `ParamsDecl.Validate` (`pkg/spec/entity.go:1716`) — hanya daftar rule validasi per nama field
  (`Field`, `Rules []ValidationRule`). Tanpa tipe, tanpa widget, tanpa label. Tidak bisa
  dirender menjadi form.
- `ActionSummary.HasParams` (`internal/ui/meta.go:51`) — bool yang sudah dikirim server dan sudah
  ada di tipe frontend (`src/types/manifest.ts:1338`), tapi **nol konsumen**.
- `TransitionDecl` (`pkg/spec/entity.go:1754`) punya `Params`/`Conditions` yang **sudah**
  terserialisasi ke bundle (karena `state_machine` dikirim mentah, `internal/ui/meta.go:1206`),
  tapi tidak ada yang membacanya.

Akibatnya, di UI:

- Tombol transisi `DetailPage.handleTransition` → `POST .../{id}/{action}` **tanpa body**, atau
  `PATCH {state_field: to}` (`src/kinds/page/DetailPage.tsx:139-180`).
- Row/bulk action `TableRenderer` → `POST` tanpa body (`:848`, `:555`).
- Bulk action **hanya berguna untuk aksi tanpa parameter** — aksi yang butuh input gagal
  validasi per baris (todo `5.12.9` ⏸️).

Dua bug nyata yang lahir dari sini:

1. **Kontrak input transisi tidak pernah ditegakkan.** Di jalur `POST` (butuh `impl`),
   `EvaluateConditions(actionSpec.Conditions, …, params)` dijalankan (`handler.go:2098`). Di jalur
   `PATCH`, hanya `guard` yang dievaluasi (`internal/entity/state_machine.go:60-80`) dan handler
   mengevaluasi conditions milik action **`update`** — bukan conditions transisi
   (`handler.go:998-1009`). Jadi transisi `via` **tanpa `impl`** (mayoritas) tidak punya satu pun
   jalur yang mengevaluasi `conditions`-nya.
   Contoh: kafe `void-order` mendeklarasikan
   `conditions: - script: "len(params.get('void_reason','')) > 0"`, tetapi `void_reason` tidak
   bisa diisi dari tombol transisi mana pun.

2. **Input pemohon hilang saat approval.** `PATCH {"status":"cancelled","void_reason":"…"}` pada
   transisi approval-gated → `202 approval_required` **sebelum ada write**
   (`handler.go:1059-1073`). `handleWorkflowApproval` (`:2312`) hanya membaca key `decision`
   (`:2390`); tidak ada kolom params di `formspec_workflow_approval`
   (`renderers/jsonb-persist/workflow_approval.go:21`). Kalau approver tidak mengirim ulang,
   `void_reason` hilang di `executeWorkflowTransition`.

## Keputusan

| #   | Keputusan           | Pilihan                                                                                   |
| --- | ------------------- | ----------------------------------------------------------------------------------------- |
| D1  | Scope               | Transisi + action Entity + action Service + bulk action                                   |
| D2  | Deklarasi           | Referensi field Entity + boleh input ad-hoc                                               |
| D3  | Tujuan nilai        | By-declaration; nama == field Entity → persist; ad-hoc → params saja                      |
| D4  | Enforcement PATCH   | Diperbaiki dalam plan ini                                                                 |
| D5  | Input saat approval | Disimpan di baris `formspec_workflow_approval`, dipakai saat eksekusi                     |
| D6  | Nama key            | `params.inputs`                                                                           |
| D7  | Reuse               | Tier 0 inline + Tier 1 input set bernama di Entity. Tier 2 (`params.form.ref`) ditunda ⏸️ |
| D8  | Container           | Diturunkan dari jumlah input, boleh di-override                                           |

## Prinsip

**Mekanisme satu, deklarasi per-transition.** Generik dicapai lewat _derivasi_: begitu sebuah
transisi mendeklarasikan `params.inputs`, itu sudah form-nya — tidak ada asset terpisah yang
perlu ditulis author. Ini pola `resolveForm` yang sudah dipakai renderer
(explicit ref → konvensi → derive).

Form bersama untuk semua transisi **ditolak** karena state machine sendiri yang memaksa
per-transisi: permission, `conditions`, intersepsi approval, dan `emit` semuanya melekat pada
transisi tertentu, bukan pada entity.

## Bentuk spec

```yaml
spec:
  input_sets:
    - name: reason
      inputs:
        - name: void_reason

  fields:
    - { name: void_reason, type: string, title: "Alasan Void" }

  state_machine:
    transitions:
      - from: paid
        to: cancelled
        via: void-order
        require_permission: cafe-order.order.void-order
        params:
          inputs:
            - name: void_reason # nama == field Entity → inherit type/title/enum
              widget: textarea
              required_when: "fields.status == 'paid'"
          render: { mode: modal }
          validate: # jalur lama, tetap didukung
            - { field: void_reason, rules: [required] }
```

- Input yang namanya **sama dengan field Entity** mewarisi type/title/enum/options dari field itu
  (D2). Field Entity tetap satu-satunya sumber kebenaran; input hanya merujuk.
- Input **ad-hoc** wajib mendeklarasikan `type`.
- `inputs_from: [reason]` merujuk `input_sets` (Tier 1).
- `render.mode` adalah keputusan design-time, sejajar `Form.render`.

## Fase

### Fase 1 — Fondasi spec (memblokir semua)

- `pkg/spec/entity.go`: `ParamInput`, `ParamsRenderHint`, `InputSet`; perluas `ParamsDecl`
  dengan `inputs`/`inputs_from`/`render`; `EntitySpec.InputSets`.
- Field baru wajib masuk literal `ActionSources()` (`:2283`) — kalau tidak, action hasil sintesis
  `via` diam-diam kehilangan kontrak input.
- Validator di `ValidateEntitySpec` (`:995`): (a) input yang mengklaim field Entity harus ada;
  (b) `inputs_from` harus resolve; (c) input ad-hoc wajib `type`; (d) `widget` harus anggota
  closed set; (e) cardinality widget ↔ `multiple`; (f) tolak nama duplikat.
- `make generate-schema` + `make generate-kind-docs` (dikunci
  `internal/genjsonschema/schema_refs_test.go:201`).

### Fase 2 — Proyeksi meta + codegen

- `internal/ui/meta.go`: `ActionSummary.Params`; `EntitySchema.InputSets`.
- `cmd/formspec/generate.go::writeActionParamsType` — tipe bertipe dari `inputs`.

### Fase 3 — Konsistensi jalur tulis

- Helper `TransitionParamsInputs`/`transitionParams` dipakai **sama** di kedua jalur.
- Jalur PATCH: validasi params → 422, conditions transisi → 422 (additive), lalu intersepsi
  workflow, lalu persist.
- Urutan: permission → params → conditions → workflow → `store.Update`.

### Fase 4 — Persist input melewati approval

- `workflow.Approval.Params`; kolom `params` di `WorkflowApprovalRow` + migrasi.
- `handleWorkflowApproval` menyimpan; `executeWorkflowTransition` merge sebelum satu
  `store.Update` → state + field atomik.
- Presedensi: nilai eksplisit approver menang, fallback ke simpanan pemohon.

### Fase 5 — Resolusi + dialog generik

- Baru `src/lib/actionParams.ts`: `resolveActionInputs()` + `toFieldDescriptors()`.
- Baru `src/shell/ActionInputDialog.tsx`. Tanpa input → tetap `ConfirmDialog`.

### Fase 6 — Sambungkan permukaan

- `DetailPage`, `TableRenderer` (row + bulk), `KanbanRenderer`.

### Fase 7 — Docs, todo, changelog

## Non-goals

- Kind baru (`ActionForm`, `ApprovalForm`).
- Input comment/reason untuk keputusan approver.
- `params.form.ref` / layout multi-section (Tier 2) — ditunda.
- Container switchable runtime.
- Menutup `5.13.6` ⏸️ (ApprovalInbox tidak punya sumber data) — masalah berbeda.
