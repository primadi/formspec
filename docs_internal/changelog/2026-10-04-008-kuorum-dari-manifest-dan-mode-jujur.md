# 2026-10-04-008 — Kuorum dari manifest, dan `mode: all`/`sequential` berhenti berbohong (5.13.11)

Menutup **5.13.11**. Tiga cacat sekelas ditemukan saat memeriksa satu baris:
`eligibleCount := len(step.Roles)`.

## 1. Kuorum menghitung NAMA, bukan ORANG (cacat aslinya)

`Quorum()` mengembalikan `len(step.Roles)` untuk `mode: all`, dan `mode: all`
adalah **default** saat `mode` tidak ditulis. Jadi kuorum dihitung sebagai jumlah
**nama role**, diperlakukan seolah jumlah orang:

- `roles: [a, b]` yang dipegang **satu** orang menuntut **dua** tanda tangan dari
  orang yang mustahil memberikannya — approval ganda untuk satu step ditolak —
  sehingga step itu tidak akan pernah bisa disetujui;
- `roles: [a]` dengan `mode: all` berarti kuorum 1, dan itu kebetulan saja sama
  dengan `any`.

Sekarang kuorum **selalu** datang dari manifest: `approvers` (default 1), dengan
`mode: sequential` mengambilnya dari rantai (`len(roles)` — satu tanda tangan per
mata rantai, satu-satunya angka yang benar-benar diturunkan manifest). **`mode:
all` ditolak `formspec validate`** dengan penjelasan dan jalan keluar, karena
angka yang dijanjikannya tidak bisa diturunkan: daftar `roles` bukan daftar orang,
dan pemegang sebuah duty tidak bisa dienumerasi. Tidak ada manifest di repo yang
memakai `mode: all`, dan manifest yang **tidak** menulis `mode` berperilaku persis
seperti sebelumnya (`any` + `approvers`).

## 2. `mode: sequential` tidak pernah mengurutkan apa pun

`sequential` muncul **hanya** di `Quorum()` dan mengembalikan 1 — urutannya tidak
pernah ditegakkan di mana pun. Jadi ia identik dengan `any`, sementara spec
menjanjikan "approver berikutnya baru bisa bertindak setelah yang sebelumnya".

Kini rantai itu nyata: giliran ditentukan `len(approvals[step])`, dan penandatangan
harus memegang role mata rantai yang sedang giliran (atau role hasil eskalasi —
reassignment memang ada untuk membuka rantai yang macet). Kuorumnya `len(roles)`.

**Kombinasi yang tidak terdefinisi ditolak, bukan ditebak:** `sequential` +
`permission` (rantai diurutkan oleh `roles`; duty adalah permission datar tanpa
posisi di urutan itu — "bolehkah pemegang duty mengambil giliran role lain?" punya
dua jawaban yang sama-sama merugikan), `sequential` + `approvers` (dua jawaban
untuk satu pertanyaan), dan `sequential` tanpa `roles` (rantai itulah urutannya).

## 3. Eligibility memeriksa step yang BERBEDA dari yang menunggu keputusan

Ini kemunculan **ketiga** dari bug yang sama, dan yang paling panas: jalur approve
sudah mengambil `step := steps[approval.ActiveStep]` dari daftar **terfilter**
(`ApplicableSteps`), lalu memanggil `CanApprove(wf, …, approval.ActiveStep, …)`
yang di dalamnya meng-index `wf.Steps` — daftar **authored**. Begitu satu step
di-skip `when`, kedua daftar berbeda di **setiap** index, jadi eligibility
dievaluasi terhadap step yang tidak sedang menunggu keputusan.

`CanApprove` kini berbentuk `CanApprove(a *Approval, steps []spec.WorkflowStep,
approver Approver)`: daftar step menjadi **parameter**, sehingga pemanggil harus
menyatakan daftar mana yang ia maksud, dan hanya ada satu daftar yang bisa
disalahkan. Nama workflow dan module juga diambil dari `Approval` (yang memang
punya keduanya), bukan dari argumen terpisah.

### Jebakan yang ikut tertutup

`NewApproval` mengisi `WorkflowName` dengan **alamat pointer** (`%p`) dan pemanggil
yang tahu namanya menimpanya kemudian. Itu selamat selama nama hanya label —
tetapi duty step **diturunkan** dari nama itu (`workflow.{module}.{workflow}.{step}`),
jadi nama placeholder membuat semua duty tidak bisa dipegang, dan gejalanya adalah
**403 senyap**. `NewApproval` kini menuntut nama sebagai parameter, dan handler
memutuskan lebih awal (menolak 500 bila nama tak bisa diresolusi, bukan menyimpan
baris tanpa nama).

Sekalian: `activeStepIndex` di inbox tadinya diresolusi terhadap daftar **authored**
lalu dipakai untuk meng-index daftar **applicable** — kebetulan tidak terpakai
setelah refactor, tetapi bentuknya sama salahnya. Kini nama step diresolusi
terhadap daftar yang berlaku, dan langkah yang dipakai untuk **label** dan untuk
**eligibility** adalah langkah yang sama — `can_decide` tidak bisa lagi menjelaskan
step yang berbeda dari yang ditampilkan.

## Bukti

- **Live (canonical `formspec dev`)**: `PATCH` void → **202**; `POST` approve →
  **200** `transition_completed`; record `cancelled` + `void_reason` tersimpan.
- Test baru: `internal/workflow/quorum_test.go` (5) — tabel kuorum (termasuk "`any`
  tidak menghitung roles"), default `mode` absen, **urutantai ditegakkan**
  (giliran 1 → giliran 2, penandatangan yang sama ditolak), eskalasi membuka
  rantai, duty **tidak** membuka giliran orang lain, dan **daftar applicable
  yang menentukan** (memasukkan daftar authored justru gagal).
- Test baru di `pkg/spec` (5 sub-test): `all` ditolak dengan jalan keluar,
  `sequential` butuh `roles`, `sequential` + `approvers` ditolak, `sequential` +
  `permission` ditolak, dan bentuk-bentuk yang sah lolos.
- `go test ./...` hijau · `make lint` **0 issues** · kafe `validate` **89/0** ·
  skema di-regenerate (`TestGeneratedSchemas_MatchOnDisk` hijau).

## File terdampak

- `internal/workflow/engine.go` — `StepMode` (absen → `any`), `Quorum(step)`,
  `StepApproved(step)`, `CanApprove(a, steps, approver)`, rantai sequential,
  `NewApproval` menuntut nama.
- `internal/api/handler.go` — nama workflow diresolusi sebelum `NewApproval`;
  kuorum & eligibility memakai API baru.
- `internal/api/workflow_inbox.go` — `canDecideWorkflowTask` → `canRunTransition`
  (eligibility sudah menyaring, jadi `can_decide` = gerbang transisi saja);
  step diresolusi lewat nama terhadap daftar yang berlaku.
- `pkg/spec/resources.go` — `ValidateWorkflowSteps` (+ `validateWorkflowStepMode`).
- `pkg/spec/widget.go`, `docs/spec/backend/02-core-extended.md`,
  `docs/kind/data/Workflow.md`, `ai_skills/formspec-kinds/SKILL.md`.

## Catatan

`examples/{arisan,cafe,crc-management}/.agents/skills/` berisi salinan skill yang
**tidak ter-track** (ditulis `formspec init` ke tiap project), jadi tidak ada yang
perlu disinkronkan di repo ini.

## Sisa (→ todo)

- **5.13.12 ⏸️** — migrasi 4 workflow contoh + hapus `roles` sebagai hard error
  (bersama **5.13.9 ⏸️**).
- **5.13.13 ⏸️** (baru) — `Approvals`/`EscalatedSteps` masih di-key **index step**,
  padahal `ActiveStepName` sudah ada dan konsumen membacanya. Rantai sequential
  menambah satu pembaca lagi (`len(a.Approvals[stepIdx])`). Selama langkah di-key
  index, sebuah baris lama bisa menunjuk langkah yang berbeda setelah manifest
  berubah — persis kelas bug yang baru saja ditutup di dua tempat. Belum diubah
  karena memerlukan migrasi isi kolom JSON, bukan sekadar skema.
