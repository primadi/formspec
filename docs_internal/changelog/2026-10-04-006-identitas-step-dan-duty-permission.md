# 2026-10-04-006 — Identitas step + duty sebagai permission (Fase 2–3)

Fase 2 dan 3 dari plan `docs_internal/plan/approval-duty-permission.md`.
Lanjutan changelog `2026-10-04-005` (Fase 0–1).

## Fase 2 — step punya identitas, dan escalasi berhenti membaca step yang salah

`WorkflowStep.Name` + `ValidateWorkflowStepNames`:

- nama wajib **unik** per workflow (kalau tidak, sebuah nama tidak menunjuk satu step);
- bentuknya harus bisa dipakai di permission string (`[a-z0-9-]`);
- **wajib ada pada step ber-`escalation`**, karena worker escalasi adalah satu-satunya
  konsumen yang tidak bisa melihat record — ia tidak bisa mengevaluasi `when`, jadi
  dengan index saja ia membaca step yang berbeda dari yang menunggu keputusan.

**Bug laten yang ditutup.** `escalation.go` membaca `wf.Steps[row.ActiveStep]`
(list **authored**) sedangkan `handleWorkflowApproval` meng-index list
**terfilter** (`ApplicableSteps`). Begitu satu step di-skip `when`, kedua list itu
berbeda di **setiap** index — escalasi memberi `reassign_roles` milik step lain.
Terukur: test baru melaporkan `escalated roles = [gl.wrong-head]` alih-alih
`[gl.finance-head]` sebelum perbaikan.

Kolom `active_step_name` (migrasi `ensureWorkflowApprovalColumn`) menyimpan NAMA step
aktif; konsumen memakai nama lebih dulu dan jatuh ke index untuk baris lama.
Baris lama tetap bekerja (test khusus), jadi migrasi ini tidak memutus apa pun.

## Fase 3 — duty sebagai permission

`WorkflowStep.Permission` (AGENTS.md aturan 6: _"Permission = resource + action,
never hardcoded role names"_):

- nama pendek di-qualify jadi `workflow.{module}.{workflow}.{stepName}` —
  **empat segmen**, jadi tidak mungkin bertabrakan dengan permission entity
  (3 segmen) dan tidak terjangkau wildcard module seperti `cafe-order.*`;
- nilai yang sudah ber-titik ≥2 diambil apa adanya (mirror `require_permission`
  pada transisi);
- `Engine.CanApprove` menerima **duty ATAU role** selama migrasi, sehingga
  mengadopsi duty tidak jadi flag day.

`Engine.Approver` menggantikan parameter `userID`/`userRoles`: eligibility butuh
predikat permission, dan meneruskan `auth.Identity` akan membuat paket workflow
bergantung pada auth — alasan yang sama `ApprovalRow` tidak mengimpor renderer.

## Keputusan yang saya batalkan sendiri (dan kenapa)

Versi pertama `can_decide` di inbox menuntut duty **secara AND**, terpisah dari
gate transisi. Itu **salah**: halaman record dan inbox adalah dua pintu ke satu
alur, dan `handleWorkflowApproval` memakai OR (`CanApprove`). Hasilnya seorang
pemegang role bisa menyetujui dari halaman record dan **ditolak** di inbox, tanpa
aturan mana yang otoritatif. Kini `canDecideWorkflowTask` memanggil
`CanApprove` yang sama, jadi kedua pintu tidak bisa berbeda pendapat — dan test
mengunci keduanya (role saja → 200; duty saja → 200).

## Batas Fase 4 yang terukur, bukan diasumsikan

Grant `{ page: "workflow:order-void-approval", actions: [{ name: supervisor-check }] }`
**belum bisa dipakai**: `navigationFootprint` tidak mengenal kind `workflow`, jadi
grant itu mematerialisasi ke **nol**. Diukur pada server hidup — `/_ui/_meta/me`
untuk `supervisor` berisi **44** permission dan **tidak satu pun** `workflow.*`.

Karena itu grant itu **tidak ditempel** di `roles.yaml`: menaruhnya sekarang berarti
menambah satu contoh lagi dari penyakit yang sedang diberantas ("konfigurasi yang
terlihat benar dan tidak menegakkan apa pun"). Yang menegakkan hak approve tetap
`roles: [supervisor]`, dan step kafe sudah mendeklarasikan `permission:` sebagai
bentuk yang dituju — didokumentasikan di kedua manifest.

## Bukti

- **Properti pembuktian Fase 0 tetap berlaku:** kafe MERAH setelah pemeriksaan
  role, hijau setelah nama diperbaiki (`2026-10-04-005`).
- **Live, canonical (`cd examples/kafe && go run …/formspec dev --dev-ui`):**
  `PATCH` void → **202**; baris approval mencatat `active_step=0`,
  `active_step_name=supervisor-check`; `GET /_ui/workflow/approvals` → task dengan
  `can_decide: true`; `POST` approve → **200** `transition_completed`, record
  `cancelled` **dengan** `void_reason`, lalu `active_step_name` kosong karena step
  terakhir sudah lewat (perilaku `Advance` yang benar).
- **Migrasi pada DB lama:** kolom `active_step_name` ditambahkan lewat
  `ensureWorkflowApprovalColumn` dan alur penuh tetap hijau di DB kafe yang sudah ada.
- Test baru: `internal/workflow/escalation_step_identity_test.go` (4) dan
  `internal/workflow/duty_permission_test.go` (8), plus 2 test inbox untuk duty.
  `TestEscalationWorker_ResolvesStepByNameNotIndex` **gagal** saat pembacaan nama
  dikembalikan ke index; `TestCanApprove_Duty*` mengunci OR + 7.4.5 tetap berlaku.
- `go test ./...` hijau · `make lint` **0 issues** · `kafe validate` **89/0** ·
  `vitest` **636/636** · `tsc --noEmit` bersih.

## File terdampak

- `pkg/spec/resources.go` — `WorkflowStep.Name`/`Permission`,
  `ValidateWorkflowStepNames`, `StepPermission`.
- `internal/workflow/engine.go` — `Approver`, `CanApprove`, `Advance(steps)`,
  `NameForStep`, `stepNameAt`, `ActiveStepName`.
- `internal/workflow/escalation.go` — resolusi step lewat nama.
- `internal/api/handler.go` — pengisian `ActiveStepName`, `Approver`, `Advance(steps)`.
- `internal/api/workflow_inbox.go` — `canDecideWorkflowTask` memakai `CanApprove`.
- `renderers/jsonb-persist/workflow_approval.go` + `migrate.go` — kolom
  `active_step_name` dan **satu** `approvalColumns` bersama (kolom dan scan harus
  sepakat; empat SELECT yang ditulis tangan sudah mulai melenceng).
- `cmd/formspec/validate_workflow.go` + `internal/manifest/loader.go` — wire
  validasi nama step.
- `examples/kafe/.../order-void-approval.yaml`, `.../seeds/roles.yaml`,
  `examples/service-demo/.../product-discontinue-approval.yaml` (nama step wajib).

## Sisa (→ todo)

- **Fase 4 ⏸️** — `navigationFootprint` `case "workflow"` (grant duty baru bisa
  hidup), migrasi 4 workflow di `crc-management`/`service-demo`, dan penghapusan
  `roles:` sebagai hard error. **5.13.9 ⏸️** (role tidak dideklarasikan di dua
  example itu) masuk ke pekerjaan yang sama.
- **5.13.10 ⏸️** (baru) — `quorum`/`mode: all` masih memakai `len(step.Roles)`
  (jumlah NAMA role, bukan orang). Setelah duty, `approvers` sebaiknya wajib
  eksplisit. Tidak diubah sekarang karena menyentuh arti kuorum untuk semua workflow.
