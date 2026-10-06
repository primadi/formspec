# 2026-10-04-004 — `ApprovalInbox` akhirnya punya sumber: `/_ui/workflow/approvals`

Menutup **5.13.6 ⏸️** (plan `docs_internal/plan/approval-inbox-endpoint.md`).
Pemicu: `GET /kafe/app/pos/approval-inbox/supervisor-inbox` merender
**"No approval source configured"** — permanen.

## Kenapa permanen

Kind ini zero-config menurut kontrak (`frontend/06-page-kinds.md` §11): sumbernya
langkah Workflow pending, **bukan entity**. Tetapi `ApprovalInboxRenderer`
mencari entity konvensional (`formspec.core.approval` / `approval-task` /
`workflow-task`) yang **tidak ada di repo ini**, sehingga lookup selalu kosong dan
komponen tidak pernah mengirim satu request pun. Akar masalahnya satu lapis lebih
dalam: barisnya hidup di tabel framework `formspec_workflow_approval`, dan tabel
itu bukan Entity — **tidak ada route yang mengeksposnya sama sekali**.

Dari tiga jalur yang dicatat di itemnya, **(b) endpoint khusus** adalah
satu-satunya yang bisa diimplementasikan: (a) entity bawaan mustahil karena
`WorkflowApprovalRow` bukan baris entity (tak punya `title`/`display_fields`),
dan (c) bertentangan dengan kontrak zero-config.

## Yang landing

```
GET  /{ws}/_ui/workflow/approvals?app={app}
POST /{ws}/_ui/workflow/approvals/{id}   {"decision":"approve"|"reject"}
```

Empat keputusan, masing-masing punya alasan yang bisa diperiksa:

1. **Store baru `ListPendingForTenant`** (`jsonb-persist/workflow_approval.go`).
   `ListPending` yang dipakai worker eskalasi **sengaja tenant-blind** (ia menyapu
   lintas workspace); melayani request darinya akan menyerahkan approval satu
   workspace ke workspace lain. `ListPendingForTenant("")` membalas kosong —
   fail closed.
2. **Eligibility memakai `workflow.Engine.CanApprove`** — predikat yang **sama**
   dengan jalur approve nyata, bukan salinannya. Jadi "yang boleh ia tindak"
   berarti persis itu, termasuk larangan pemohon menyetujui permintaannya sendiri
   (7.4.5).
3. **`can_decide` memisahkan dua pertanyaan:** role pada step menentukan apa yang
   **terdaftar**, permission yang menggerbangi route transisi menentukan apa yang
   bisa **dijalankan**. Permission-nya diambil dari `RouteDescriptor` yang sama
   dengan registrasi route (fallback `{module}.{plural}.update` untuk transisi
   tanpa `impl`), sehingga gerbang inbox tidak bisa melenceng dari gerbang asli.
   Tugas yang tak bisa dieksekusi tetap terdaftar dan tombolnya non-aktif.
4. **`display_fields` dipagari `{module}.{plural}.view`.** Pembacaan store di
   handler melewati pemeriksaan permission HTTP, jadi pemberian nilai record harus
   eksplisit. Tugasnya tetap terlihat; nilainya tidak.

`POST` **mendelegasikan ke `handleWorkflowApproval`** — bukan implementasi kedua.
Di situlah kuorum, 7.4.5, audit bertanda tangan, dan emit event transisi hidup.

## Bug yang ditemukan jalan ini

### 1. Approval yang selesai tetap `pending`

`handleWorkflowApproval` menaikkan `ActiveStep` melewati step terakhir saat
approval selesai, tetapi **tidak pernah mengubah `Status`** — baris yang sudah
selesai tetap `status='pending'`. Setiap pembaca yang memfilter `'pending'`
karenanya terus mengembalikannya, sehingga request void kedua untuk record yang
sama membalas **403 "workflow step out of range"**, bukan approval baru. Kini
`Status = approved` diset saat `AllStepsApproved`. Efeknya terukur: dua test baru
(HTTP + inbox) **gagal** saat baris itu dihapus, hijau sesudahnya. Ditemukan live
saat browser mengirim Approve dua kali.

### 2. `display_fields` kosong untuk field yang belum ditulis — padahal itu justru field keputusan

Transisi yang di-intercept **tidak menulis apa pun** sampai approval selesai.
Jadi `void_reason` yang diisi pemohon tidak ada di record; satu-satunya salinan
ada di baris approval (`params`, dibawa melewati batas approval oleh kontrak
input). Membaca record saja membuat **tepat field yang approver butuhkan untuk
memutuskan** tampil kosong — ironi yang `02-core-extended.md` §2.1 sebut sebagai
alasan `display_fields` ada. **Terukur:** baris simpan
`{"void_reason":"salah input"}` sementara `display_fields` membalas `null`.
Perbaikan: nilai record **jatuh ke `params`** bila field belum ada di record.
Direproduksi di test dengan menghapus field dari record lebih dulu
(`clearRecordField`), lalu membuktikan test **gagal** saat fallback dihapus.

### 3. Nilai `money` tampil `[object Object]`

Renderernya melakukan `String(f.value)`, sehingga kafe **Total Amount**
(`{amount, currency}`) tampil `[object Object]` — approver diminta memeriksa
total untuk sebuah refund dan yang ia lihat adalah blob. Formatting semua angka
sebagai uang juga salah (akan melabeli `decimal`/`integer` sebagai mata uang),
jadi **server kini mengirim `label` + `type`** dari deklarasi field entity — satu
aturan, dikirim dari sumber yang mengetahuinya, bukan ditebak klien. **Terukur:**
`{"field":"total_amount","label":"Total","type":"money","value":{...}}`;
renderer memformatnya `Rp125.000`.

### Temuan yang **belum** dibereskan → 5.13.8 ⏸️

Role pada step workflow di kafe adalah `cafe-order.supervisor` (bentuk
module-qualified), sedangkan role IAM yang di-seed bernama `supervisor`. Jadi
`CanApprove` **selalu menolak** supervisor kafe: pesan yang terukur adalah
`403 user does not hold any of the step's required roles`, dan antreannya
**selalu kosong** bagi satu-satunya approver yang ada. Ini bukan efek perubahan
ini (endpoint-nya benar), tetapi membuat seluruh alur tidak bisa dijalankan di
kafe. Dibuktikan dengan probe DB (menambah `cafe-order.supervisor` ke user →
`GET` mengembalikan task, `POST` → `200 transition_completed`, record
`cancelled` **dengan** `void_reason` tersimpan). Pilihan perbaikannya
(seed memakai nama qualified vs `CanApprove` menerima keduanya) **tidak diambil
diam-diam** karena itu menyentuh pencocokan otorisasi.

## Bukti

- `go test ./...` hijau · `make lint` (`GOLANGCI_LINT_CACHE=/tmp/glci`) **0 issues**.
- `npx vitest run` **636 lulus** / 52 file · `tsc -p tsconfig.app.json --noEmit` bersih.
- **Live di kafe** (bukan hanya unit): `PATCH` void → **202** `approval_required`;
  `GET` inbox → task dengan `title`/`description` dari step, `display_fields`
  berisi `Nomor Pesanan`/`Total Rp125.000`/`Alasan Void`, `can_decide: true`;
  `POST` approve → **200** `transition_completed`, record `cancelled` **dengan**
  `void_reason` tersimpan, baris approval `approved`.
- **Di browser** (`/kafe/app/pos/approval-inbox/supervisor-inbox`): halaman
  merender **"1 pending approval"** + badge, judul "Persetujuan Void Pesanan",
  deskripsi step, tiga nilai display, tautan ke record pesanan, `paid →
cancelled`, dan tombol Approve/Reject — bukan lagi "No approval source
  configured". Snapshot inilah yang menyingkap cacat (3): `Total Amount:
[object Object]`.
- Test baru: `internal/api/workflow_inbox_test.go` (10) — daftar+label+tipe+nilai,
  penolakan non-holder & pemohon sendiri, penciutan App+tenant (termasuk id
  lintas-workspace → **404**), `can_decide` jujur (403 saat dipaksa), approve →
  record `voided` + tugas hilang + keputusan kedua **404**, reject →
  `on_reject.to`, body tak dikenal → **422**, anonim → **401**, **route benar-benar
  ter-mount** lewat `BuildHTTP` (401, bukan 404), `ListPendingForTenant` vs
  `ListPending` (1 / 2 / 0), dan fallback `params`.
- `renderers/react-shadcn/src/lib/approvalInbox.test.ts` (3) — membuktikan path ke
  server HTTP nyata: `/kafe/_ui/workflow/approvals?app=…`, dengan assertion
  **negatif** untuk dua salah tulis yang gagal senyap (`/kafe/workflow/...`,
  `/workflow/...`).
- `src/kinds/approval-inbox/formatApprovalField.test.ts` (7) — aturan format per
  tipe, termasuk `[object Object]` yang tidak boleh kembali.
- `TestWorkflowApproval_CompletedRequestIsNoLongerPending` +
  `TestApprovalInbox_DecisionApprovesAndConsumesTask` **gagal** saat perbaikan
  `Status` dihapus; `TestApprovalInbox_DisplayFieldFallsBackToRequesterInput`
  **gagal** saat fallback `params` dihapus.

## File terdampak

- `renderers/jsonb-persist/workflow_approval.go` — `ListPendingForTenant`,
  `GetPendingByID`, `listPending` bersama.
- `internal/api/workflow_inbox.go` (baru) · `internal/api/router.go` (mount).
- `internal/api/handler.go` — `Status = approved` saat semua step lolos.
- `internal/api/workflow_inbox_test.go` (baru) ·
  `internal/api/workflow_approval_api_test.go` (regresi completed-request).
- `renderers/react-shadcn/src/lib/approvalInbox.ts` + test (baru) ·
  `src/kinds/approval-inbox/ApprovalInboxRenderer.tsx` (sumber baru).
- `docs/runtimes/06-ui-rest-contract.md` §5 (kontrak surface) ·
  `docs/spec/frontend/06-page-kinds.md` §11 · `docs/kind/ui/ApprovalInbox.md`.
- `docs_internal/plan/todo.md` — 5.13.6 ditutup; **5.13.7 ⏸️** dibuka.

## Sisa (→ todo)

- **5.13.7 ⏸️** (baru) — `realtime: true` pada kind ini belum dihormati: hub WS
  mendorong per `{module}/{entity}`, sedangkan approval bukan entity. Tidak ada
  topik untuk disubscribe, jadi supervisor harus me-refresh. Alasan yang sama
  membuat `filters` belum punya arti yang jelas (`FilterSpec.field` merujuk apa
  kalau sumbernya bukan entity) — belum dikerjakan, tidak ditebak.
- **5.13.8 ⏸️** (baru) — role pada step workflow kafe (`cafe-order.supervisor`)
  tidak cocok dengan nama role IAM yang di-seed (`supervisor`), sehingga
  `CanApprove` **selalu menolak** satu-satunya approver yang ada dan antreannya
  selalu kosong. Endpoint-nya benar (dibuktikan dengan probe DB: task muncul,
  approve → `200`, record `cancelled` **dengan** `void_reason`); yang rusak adalah
  **kecocokan role di kafe**. Keputusan yang diperlukan: seed memakai nama
  qualified, atau `CanApprove` menerima keduanya. Tidak diambil diam-diam — ini
  menyentuh pencocokan otorisasi.
- **5.25.10 ⏸️** (sudah ada, tidak berubah) — `navigationFootprint` belum mengenal
  `approval-inbox:`, jadi grant per-inbox belum bisa.
