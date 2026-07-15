# 2026-07-08 — Integrasi Document Model ke Spec Docs (Tahap 1)

**Referensi:** `reff_docs/Forma-Technical-Note-Document-Model.md`
**Plan:** `docs/plan/todo.md`

## Ringkasan

Mengintegrasikan **Document Model** dari technical note ke seluruh spec docs (`docs/spec/`). Ini adalah Tahap 1 dari plan integrasi — hanya spec docs yang diubah, tidak ada perubahan code. Code alignment akan menyusul setelah user verifikasi manual.

## Filosofi Baru

- `kind: Entity` → `kind: Document` (backward-compatible: `Entity` diterima sebagai deprecated alias)
- `doc_status` CLOSED 3-value: `draft`, `submitted`, `cancelled`
- 6 reserved actions dengan built-in framework guards: `create`, `update`, `submit`, `cancel`, `delete`, `amend`
- Two-layer state model: `doc_status` (built-in lifecycle) + user-defined `state_machine` (business field, independent)
- `summary` = characteristic pada `type: document`, BUKAN type terpisah
- Composite action (multi-step atomik), `kind: Integrator` (cross-module bridge), transaction date semantics, archiving, error glossary

## File yang Diubah

| File | Perubahan |
|---|---|
| `docs/spec/02-core-basic.md` | **Revisi besar.** Version bump v0.2.0 → v0.3.0. Rename Entity→Document global. Tambah 5 sub-section baru (4.1a–4.1e): reserved fields, reserved actions/lifecycle, composite action, master data pattern, summary documents. Revisi §9 (Document Anatomy), §10 (reserved fields validation), §11 (reserved actions table + composite), §12 (event naming convention + handler priority), §14 (two-layer state machine + 14a transaction date + 14b archiving + 14c error glossary + 14d saga). Update normative table structure (`doc_status` column). Part VII Conformance diperluas ke 14 item. |
| `docs/spec/01-overview.md` | Update §3 source table (Frappe DocType→Document), §4 prinsip #1 (Entity→Document + lifecycle mention) |
| `docs/spec/03-core-extended.md` | Tambah `kind: Integrator` di Part I (§5). Entity→Document rename di seluruh dokumen. |
| `docs/spec/05-frontend.md` | Entity→Document rename di seluruh prose text. Tambah §1.7 UI Patterns (3 pola: 2-step+auto-save, 2-step manual, 1-step composite). |
| `docs/spec/10-entity-extension.md` | Title: "Entity Extension" → "Document Extension". Entity→Document rename di seluruh dokumen. Update framework-reserved columns list (tambah `doc_status`). |
| `docs/spec/error-glossary.yaml` | **NEW FILE.** Canonical error codes: 19 kode FORMA.* dengan format `code` + `params` + `default_message`. |
| `docs/spec/README.md` | Belum diupdate — pending. |

## Belum Dikerjakan

- **Code alignment** — menunggu user approval spec docs (Tahap 2)
- **`docs/spec/11-reference.md`** — glossary dan decisions log belum diupdate
- **`docs/spec/README.md`** — index belum diupdate dengan `error-glossary.yaml`
- Semua fitur baru (composite action executor, Integrator runtime, boundary detection, saga, transaction date enforcement, archiving engine, UI pattern renderer) → akan dimasukkan ke `docs/plan/todo.md`

## Design Decisions (Finalized)

1. `doc_status` = CLOSED 3-value set. Tidak bisa ditambah. D17.
2. Business process granularity via field terpisah + state machine independen.
3. Guard `delete` = mutlak (ON DELETE RESTRICT) berdasarkan tipe field `relation`.
4. Guard `cancel` = bisa dibuka via `before_cancel` handler.
5. `summary` = characteristic pada `type: document`, BUKAN type terpisah.
6. Boundary = runtime detection, BUKAN design-time declaration.
7. `transaction_date` = wajib dideklarasikan eksplisit untuk `characteristics: [transaction]`.
