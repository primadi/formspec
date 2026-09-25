# 2026-09-22-004 — Harness approval level-API + fix requester self-approve (todo 7.4.7)

**Plan/Todo**: item **7.4.7** (`docs_internal/plan/todo.md`); memperbaiki jalur yang
diklaim **7.4.5**.

Seluruh Fase 7.4 (workflow approval) hanya terverifikasi **unit-level**
(`internal/workflow`). Tidak ada test yang melewati HTTP, jadi regresi di
`HandleCustomAction → RequiresApproval → handleWorkflowApproval` tidak
tertangkap. Ditutup dengan `internal/api/workflow_approval_api_test.go`: harness
menyambungkan dependensi yang sama seperti produksi (entity dengan state machine
`posted→voided` via `void-order`, workflow registry satu step role `supervisor`,
`WorkflowApprovalStore`, `specLookup`, plus executor no-op) dan mendorong record
lewat HTTP sungguhan.

**Cakupan**: interrupt → 202 `approval_required` + state tidak pindah · role
salah → 403 · requester self-approve → 403 · approver ber-role → 200 + state
pindah · reject → 200 `rejected` + state tetap · kontrol tanpa workflow → 200
(dispatch langsung, bukan 202).

**Bug nyata yang ditemukan harness ini** (bukan hipotetis): `handleWorkflowApproval`
mengambil requester dari `resourceData["created_by"]`. `created_by` adalah
**kolom framework** (`EntityRecord.CreatedBy`) yang hanya diproyeksikan ke wire
oleh `EntityRecord.MarshalJSON` — ia **tidak pernah** ada di map `Data`. Jadi
`RequesterID` selalu kosong, dan jaminan **7.4.5** ("requester can never approve
their own request") **tidak pernah menendang di jalur HTTP**: requester yang
memegang role step bisa menyetujui requestnya sendiri. Terukur — `POST
…/void-order {"decision":"approve"}` oleh requester mengembalikan **200
`transition_completed`** sebelum perbaikan, **403** sesudah.

**Perbaikan**: helper `HandlerFactory.requesterIDFor(ctx, module, entity,
resourceID, workspaceID, resourceData)` — baca `Data` lebih dulu (menghormati
entity yang memang mendeklarasikan field `created_by`), lalu fallback ke
`store.GetByID(...).CreatedBy`. Dipakai di titik pembuatan approval.

**File terkena dampak**: `internal/api/workflow_approval_api_test.go` (baru),
`internal/api/handler.go` (`requesterIDFor` + call site).

**Bukti**: `TestWorkflowApproval_RequesterCannotSelfApprove_Regression` gagal
sebelum perbaikan, hijau sesudah; 5 test lain di file yang sama hijau;
`go test ./...` hijau; `go vet ./...` bersih.

**Catatan model (dinyatakan apa adanya)**: kontrol "tanpa workflow" meng-assert
**dispatch terjadi** (200, bukan 202), bukan bahwa state record berpindah —
menerapkan transisi adalah tugas handler business action itu sendiri, bukan
lapisan workflow. Meng-assert state berpindah di sana akan menyandikan model
mental yang salah tentang di mana transisi diterapkan.
