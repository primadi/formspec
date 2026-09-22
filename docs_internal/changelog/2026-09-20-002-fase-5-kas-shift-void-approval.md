# Fase 5 kafe — kas, shift, void & approval (5.1–5.5)

**Tanggal:** 2026-09-20 · **TODO:** `examples/kafe/gaps_found/TODO.md` Fase 5

## Apa yang diubah

Lima item Fase 5 ditutup. Dua di antaranya sudah tertutup oleh pekerjaan
sebelumnya dan hanya perlu diverifikasi ulang; tiga butuh perubahan.

| Item | Hasil |
| --- | --- |
| **5.1** partial unique shift | ✅ sudah tertutup 1.6 — diverifikasi ulang (`TestMigrationRunner_UniqueIndexRejectsDuplicates`) |
| **5.2** void multi-state-asal | ✅ sudah tertutup 1.7 — diverifikasi ulang (`TestRegistry_ForTransitionByName`, `TestValidateWorkflows_*`) |
| **5.3** `WorkflowStep` label | ✅ spec-level: `Title`/`Description`/`DisplayFields` + validasi `display_fields` |
| **5.4** `render: drawer` shorthand | ✅ schema kini `oneOf: [string, object]` |
| **5.5** docs simetri cancel 7.7.2 | ✅ didokumentasikan di `02-core-extended.md` §5 |

## Perubahan kode

- `pkg/spec/resources.go` — `WorkflowStep.Title`, `.Description`,
  `.DisplayFields`.
- `cmd/formspec/validate_workflow.go` — `buildEntityFieldIndex` +
  `workflowDisplayFieldError`: `display_fields` wajib menunjuk field entity.
- `internal/genjsonschema/generator.go` — `FormRenderDecl` → `oneOf: [string,
  object]` (pola sama dengan `ValidationRule`/`TransitionDecl.from`).
- `docs/spec/backend/02-core-extended.md` — §2 (`WorkflowStep` label) + §5
  (simetri cancel 7.7.2: mengapa + bagaimana + contoh).
- `examples/kafe/spec/.../order-void-approval.yaml` — `title`, `description`,
  `display_fields`.
- `schemas/` — ter-regenerasi.

## Kenapa

- **5.3** — `ApprovalInbox` zero-config hanya bisa bilang "ada tugas menunggu"
  tanpa label; approver tidak tahu apa yang disetujui. `display_fields` yang
  salah ketik akan tampil sebagai kolom kosong (terbaca "tidak ada data"), jadi
  validator menolaknya.
- **5.4** — loader menerima `render: drawer` tetapi schema menolaknya: manifest
  yang sah di engine gagal di editor. Divergensi loader↔schema adalah kelas
  "spec terlihat salah padahal schema yang basi".
- **5.5** — aturan 7.7.2 bagus tetapi tidak terdokumentasi, sehingga penemunya
  sulit; kini dijelaskan dengan contoh pasangan Integrator.

## Bukti

| Perintah | Hasil |
| --- | --- |
| `go test ./cmd/formspec/ -run TestValidateWorkflows` | 7 PASS (termasuk `DisplayFieldsMustExist`) |
| `go test ./internal/genjsonschema/ -run TestFormRenderDecl` | PASS |
| `go test ./renderers/jsonb-persist/ -run TestMigrationRunner_UniqueIndexRejectsDuplicates` | PASS |
| `go test ./internal/workflow/ -run ForTransitionByName` | PASS |
| spec uji `render: drawer` | **0 problem** (sebelumnya 1) |
| `formspec validate` kafe | **0 problem** (69 manifest) |
| `go test ./...` | hijau |

## Sisa

**5.3 wiring runtime** (dicatat): `ApprovalInbox` zero-config mengambil dari
langkah workflow yang menunggu, tetapi engine belum mengisi `title`/
`display_fields` ke item inbox. Renderer sudah menampilkan `item.title` bila
ada; sisanya pekerjaan engine, di luar cakupan spec-level item ini.
