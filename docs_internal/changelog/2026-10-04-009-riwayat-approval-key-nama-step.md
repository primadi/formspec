# 2026-10-04-009 — Riwayat approval di-key nama step, dan jalur approve berhenti membaca posisi (5.13.13)

Menutup **5.13.13**. Dua cacat, keduanya dari akar yang sama: **sebuah step
dirujuk lewat POSISINYA, bukan identitasnya.**

## 1. Jalur approve memeriksa step yang berbeda dari yang menunggu keputusan

`handleWorkflowApproval` mengambil langkah dari `ApplicableSteps` (**list yang
berlaku**), lalu `CanApprove(wf, …, approval.ActiveStep, …)` meng-index `wf.Steps`
(**list authored**). Begitu satu step di-skip `when`, kedua list berbeda di
**setiap** index — jadi eligibility dievaluasi terhadap step yang tidak sedang
menunggu keputusan, dan step yang benar-benar menunggu **tidak pernah diperiksa**.

Ini kemunculan **keempat** dari satu bug yang sama di satu berkas alur: escalation
worker (5.13.10), inbox (5.13.10), dan sekarang jalur approve. Tiga perbaikan
sebelumnya masing-masing menambal satu pembaca; yang ini menutup polanya.

Sekarang **satu aturan**: `Approval.ActiveIndex(steps)` meresolusi posisi — **nama
lebih dulu**, index sebagai fallback untuk baris lama — dan itu dipakai oleh
`CanApprove`, `StepApproved`, `Approve`, `Reject`, `Advance`, dan audit record.
Karena nama meresolusi ke posisi masing-masing list, **kedua list tidak bisa lagi
berbeda pendapat** tentang step mana yang dibicarakan.

## 2. Riwayat approval di-key posisi, jadi sebuah edit manifest mengalihkannya

`Approvals`/`EscalatedSteps` disimpan sebagai `{"0": [...]}` — index step. Menyisipkan
step di depan membuat `approvals[0]` kemarin menunjuk step yang **berbeda hari ini**:
penandatangan yang tercatat diam-diam menjadi "siapa pun yang ada di index 0
sekarang". `when` mencapai hal yang sama **tanpa ada yang mengedit apa pun**.

Key-nya sekarang `StepKey(steps, idx)` — **nama step** kalau ada, `#{index}` kalau
tidak. `#` dipilih dengan alasan yang bisa diperiksa: nama step divalidasi
`[a-z][a-z0-9-]*`, jadi tidak ada nama yang bisa terlihat seperti index, dan tidak
ada key numerik lama yang bisa terlihat seperti bentuk ini. Kedua ruang tidak bisa
bertabrakan.

**Baris lama tetap terbaca, tanpa migrasi data.** Key objek JSON memang selalu
string, jadi `{"0": [...]}` masuk sebagai key `"0"` ke map yang sama; pembaca
mencoba nama dulu, lalu index. Yang penting dan mudah salah: **menulis juga
memindahkan bucket lama** (`mergeLegacyBucket`) — kalau tidak, tanda tangan yang
sudah tercatat akan tertinggal di key numerik dan hilang dari hitungan, mengubah
"dua orang sudah menandatangani" menjadi "belum ada yang menandatangani" — persis
kegagalan yang fitur ini ada untuk mencegah.

**Yang sengaja TIDAK diubah:** `reject_step` tetap integer (audit-only, tidak
pernah dibaca untuk keputusan) dan dicatat sebagai posisi **terresolusi**, supaya
ia tidak menunjuk step lain daripada yang dilaporkan di audit.

## Bukti

- **Live, baris lama benar-benar diuji** (bukan hanya unit): baris pending ditulis
  gaya lama — `active_step_name` kosong, `approvals` = `{"0":["…manajer"]}` — lalu
  di-approve lewat endpoint. Hasilnya **200**, dan tersimpan sebagai
  `{"supervisor-check":["…manajer","…supervisor"]}`: tanda tangan lama **terbaca**
  (menghitung kuorum) dan **dimigrasikan** ke key nama, key numeriknya hilang.
- **Live, baris baru**: alur void penuh → 202, `approvals` tersimpan
  `{"supervisor-check":[…]}` (bukan `{"0":…}`), approve 200
  `transition_completed`, record `cancelled` + `void_reason`.
- Test baru `internal/workflow/step_key_test.go` (5): bentuk key + ruang yang
  tidak bertabrakan · baris lama terbaca · **migrasi bucket tidak menghilangkan
  tanda tangan** (+ penjaga duplikat tetap bekerja lintas key) · riwayat selamat
  dari penyisipan step (dan step baru **tidak** mewarisi riwayat) · escalation dan
  key menunjuk step yang sama.
- Test baru `internal/api`: `TestApprovalInbox_HistorySurvivesAManifestEdit` —
  lewat store dan endpoint nyata, membuktikan key yang ditulis adalah nama dan
  **tidak ada key numerik lagi**.
- `TestCanApprove_ResolvesByStepNameNotPosition` **menulis ulang** asersi lama:
  sebelumnya test saya menuntut "list authored harus gagal"; setelah perbaikan,
  kedua list resolve ke step yang sama — dan itu perilaku yang benar. Test kini
  mengunci properti itu (key dan eligibility sama dari list mana pun).
- `go test ./...` hijau · `make lint` **0 issues** · kafe `validate` 89/0.

## File terdampak

- `internal/workflow/engine.go` — `StepKey`, `ActiveIndex`, `approvalsFor`,
  `escalatedFor`, `lookupStepBucket`, `mergeLegacyBucket`, `MarkEscalated`;
  `Approvals`/`EscalatedSteps` jadi `map[string][]string`;
  `Approve(steps, user)`/`Reject(steps, user)`/`StepApproved(steps, idx)`.
- `internal/workflow/escalation.go` — resolusi step mengembalikan posisi;
  escalation ditulis lewat key step; `isEscalated` menerima key lama.
- `internal/api/handler.go` — audit memakai `StepKey`; kuorum & approve memakai
  posisi terresolusi.
- `renderers/jsonb-persist/workflow_approval.go` — tipe map + doc bentuk key.
- `docs/spec/backend/02-core-extended.md` §2.1 · `docs/kind/data/Workflow.md` —
  doc key riwayat.

## Catatan

`examples/{arisan,cafe,crc-management}/.agents/skills/` adalah salinan skill
**untracked** (ditulis `formspec init`), jadi tidak ada yang perlu disinkronkan.

## Sisa (→ todo)

- **5.13.12 ⏸️** — migrasi 4 workflow contoh + hapus `roles` sebagai hard error
  (keputusan produk per example, bersama **5.13.9 ⏸️**).
- **5.13.7 ⏸️** — realtime inbox.
