# 2026-09-28-004 — Kontrak input untuk transisi & action (`params.inputs`)

**Plan:** `docs_internal/plan/action-input-contract.md`
**Menutup:** todo `5.12.9` ⏸️, `7.4.8` ⏸️

## Apa yang diubah

Transisi state machine, action, dan action Service kini punya **kontrak input
deklaratif** yang bisa dirender — dan yang benar-benar ditegakkan di jalur tulis.

| Permukaan                                 | Sebelum                                                      | Sesudah                                                                   |
| ----------------------------------------- | ------------------------------------------------------------ | ------------------------------------------------------------------------- |
| `pkg/spec`                                | `ParamsDecl.Validate` saja (rule per nama field, tanpa tipe) | +`inputs` (`ParamInput`), `inputs_from`, `render`; `EntitySpec.InputSets` |
| Tombol transisi (`DetailPage`)            | `POST .../{id}/{action}` **tanpa body**                      | dialog input; body dikirim ke POST **dan** PATCH                          |
| Row/bulk action (`TableRenderer`), Kanban | `POST` tanpa body                                            | satu dialog; body sama untuk seluruh seleksi                              |
| Conditions transisi di jalur PATCH        | **tidak dievaluasi**                                         | dievaluasi → 422                                                          |
| `params` transisi di jalur PATCH          | **tidak divalidasi**                                         | divalidasi → 422                                                          |
| Transisi approval-gated                   | input pemohon hilang (202 tanpa write)                       | disimpan di baris approval, diterapkan saat approve                       |
| `ActionSummary` bundle                    | `has_params` bool (0 konsumen)                               | `params` mentah + `EntitySchema.input_sets`                               |
| `formspec generate`                       | `{Action}Params` semua `unknown`                             | bertipe dari `inputs`                                                     |

## Kenapa

Tiga cacat nyata, semuanya berasal dari satu akar: `params` hanya berisi rule
validasi, jadi tidak ada yang bisa **membuat** body — hanya menolaknya.

1. **Kontrak input transisi tidak punya jalur penegakan.** `PATCH …/{id}` adalah
   satu-satunya jalur bagi transisi tanpa `impl`, dan jalur itu hanya
   mengevaluasi `guard` — bukan `conditions` transisi, dan bukan
   `params.validate`-nya. Jadi `void-order` di kafe mendeklarasikan
   `conditions: len(params.get('void_reason','')) > 0` dan tampak menuntut alasan,
   sementara `{"status":"cancelled"}` polos lolos.
2. **Tidak ada permukaan yang bisa mengumpulkan input.** Tombol transisi POST
   tanpa body; `void_reason` adalah field entity yang tidak pernah terisi di
   jalur itu. Deklarasinya benar dan tidak dapat dipenuhi.
3. **Input pemohon hilang saat approval.** 202 dikembalikan **sebelum ada write**,
   dan `handleWorkflowApproval` hanya membaca verb `decision` — tidak ada kolom
   params di `formspec_workflow_approval`. Pesanan yang di-void berakhir
   `cancelled` tanpa alasan; kalau transisi menggerbang pada nilai itu, check-nya
   gagal tanpa cara memenuhinya.

Bentuk yang dipilih: **satu mekanisme, deklarasi per-transisi.** Input yang
namanya sama dengan field Entity **merujuk** field itu (tipe/enum/cardinality
diwarisi, nilai disimpan ke sana); input ad-hoc wajib mendeklarasikan `type`.
`params.inputs_from` memakai set bernama di level Entity (Tier 1). Container
diturunkan dari jumlah input, boleh di-override lewat `params.render.mode`.

## File yang terdampak

**Spec & validasi**

- `pkg/spec/action_input.go` (baru) — `ParamInput`, `ParamsRenderHint`,
  `InputSet`, `ValidateActionInputs`, `ValidFieldType`, `EffectiveActionSpec`,
  `TransitionInputParams`, `EffectiveParamValidation`, `PersistableInputs`
- `pkg/spec/entity.go` — `ParamsDecl` diperluas, `EntitySpec.InputSets`,
  `ActionSources()` menyalin `Params`, `ValidateEntitySpec` memanggil validator baru

**Jalur tulis** (`internal/api/handler.go`)

- PATCH: validasi params + conditions transisi **sebelum** intersepsi workflow
- `HandleCustomAction` & `HandleServiceAction`: `EffectiveParamValidation`
  (menggabungkan `validate` + `inputs` + rules field Entity)
- `handleWorkflowApproval`: menyimpan input pemohon saat 202;
  `seedStoredApprovalParams` saat panggilan approval; `mergeApprovalParams` saat
  eksekusi

**Persistensi approval**

- `renderers/jsonb-persist/workflow_approval.go` — kolom `params` di
  `WorkflowApprovalRow`, INSERT/UPDATE, kedua scanner
- `renderers/jsonb-persist/migrate.go` — kolom `params` + `ALTER TABLE` untuk DB lama
- `internal/workflow/engine.go` — `Approval.Params`

**Proyeksi & codegen**

- `internal/ui/meta.go` — `ActionSummary.Params`, `EntitySchema.InputSets`, `actionTakesParams`
- `internal/genjsonschema/generator.go` — `ParamInput`/`ParamsRenderHint`/`InputSet` di `sharedTypes`
- `cmd/formspec/generate.go` — params bertipe dari `inputs`; `tsFieldType` menangani `text`/`richtext`; loop memakai `ActionSources()`

**Renderer**

- `src/lib/actionParams.ts` (baru), `src/shell/ActionInputDialog.tsx` (baru)
- `src/engine/lifecycle.ts` — `AvailableTransition.decl`
- `src/types/manifest.ts` — `ParamInput`, `InputSet`, `ParamsRenderHint`, `params` pada `TransitionDecl`/`ActionSummary`, `input_sets`
- `src/kinds/page/DetailPage.tsx`, `kinds/table/TableRenderer.tsx`, `kinds/kanban/KanbanRenderer.tsx`

**Contoh:** `examples/kafe/.../cafe-order/transaction/order/entity.yaml` —
`void-order` kini mendeklarasikan `params.inputs: [void_reason]` (merujuk field
entity, `required_when` untuk state asal `paid`).

## Test

- `pkg/spec`: `action_input_test.go` (validator), `action_input_params_test.go`
  (helper), `action_input_validation_test.go` (`EffectiveParamValidation`)
- `internal/ui/action_inputs_test.go` — proyeksi bundle
- `internal/api/transition_contract_test.go` — PATCH: 422 tanpa input, 422 untuk
  kondisi gagal, 200 + nilai tersimpan
- `internal/api/approval_input_test.go` — input bertahan melewati approval,
  approver menang, `decision` tidak disimpan sebagai field
- `cmd/formspec/generate_inputs_test.go` — params bertipe
- Frontend: `lib/actionParams.test.ts` (17), `kinds/table/action-inputs.test.ts` (15)

`go test ./...` hijau · `npx vitest run` 580 hijau · `formspec validate` kafe 89
manifest, 0 problem.

## Sisa (item ⏸️ bernomor)

> **Koreksi nomor (2026-09-28, changelog `2026-09-28-005`).** Entri ini semula
> menyebut `5.19.1`/`5.19.2` — nomor yang **salah**: `5.19.x` adalah section
> "Temuan lint yang sudah ada sebelumnya" yang tidak berhubungan, jadi rujukan
> itu menunjuk item orang lain. Nomor yang benar adalah `5.24.x`, dan
> `5.24.3` sudah **selesai** di `2026-09-28-005`.

- **Tier 2** (`params.form.ref` → `kind: Form` bernama) ditunda sesuai keputusan D7
  → **5.24.2 ⏸️** (masih terbuka).
- **`GenerateCustomActionRoutes` tidak memakai `ActionSources()`** — transisi
  `via` ber-`impl` hanya dapat route `/_ui/entity/`, tidak dapat route
  `/api/v1/…`, sehingga tidak muncul di `formspec generate` → **5.24.3 ✅
  SELESAI 2026-09-28** (`2026-09-28-005`).
- **Kafe jalur void asli belum diuji end-to-end lewat approval** — `resource/
kafe_table_lifecycle_e2e_test.go` sengaja menghindari void (approval-gated);
  semantiknya kini dikunci di level API dengan fixture berbentuk sama
  (`void_reason`, kondisi, workflow) → **5.24.4 ⏸️** (masih terbuka).
