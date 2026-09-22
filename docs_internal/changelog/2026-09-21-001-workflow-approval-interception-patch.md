# 9.4 skenario 6 — interception approval pada jalur PATCH

**Tanggal:** 2026-09-21 · **TODO:** `examples/kafe/gaps_found/TODO.md` 9.4 (skenario 6) ·
**Plan:** `docs_internal/plan/kafe-sisa-gap.md`

## Masalah (BUG ENGINE, temuan walkthrough skenario 6)

Void pesanan yang sudah dibayar langsung `cancelled` **tanpa approval**. Workflow
`order-void-approval` mengawal transisi `void-order` (`paid/in_kitchen/ready/served
→ cancelled`) dan wajib menangkapnya — tapi tidak pernah dipanggil.

**Akar dua lapis, di `internal/api/handler.go`:**

1. **Interception tidak ada di jalur PATCH.** `wfEngine.RequiresApproval` hanya
   dipanggil di `HandleCustomAction`; `HandleUpdate` (PATCH) langsung
   `store.Update`. Transisi **tanpa `impl`** hanya bisa dicapai lewat PATCH
   (route `/{id}/{action}` hanya untuk action ber-`impl`, kontrak 2.7) — jadi
   workflow yang mengawal transisi semacam itu **selalu bypass**. Jalur custom
   action punya interception; jalur update tidak.
2. **`merged := current.Data` bukan salinan.** Merge loop menulis ke
   `current.Data` juga, jadi state asal yang dibaca SETELAH merge selalu sama
   dengan state tujuan (`from == to`) — deteksi "state crossed" mustahil menyala.

**Perbaikan (`internal/api/handler.go`, `internal/entity/state_machine.go`):**

1. `StateMachineEngine.FindTransitionByStates(entity, from, to)` — reverse
   lookup transisi. PATCH membawa state **tujuan**, bukan nama transisi; S9
   memilih workflow lewat nama (`via`), dan transisi multi-asal (void punya 4
   state asal) tak bisa diidentifikasi dari satu state saja.
2. `HandleUpdate`: sebelum merge, state asal diambil ke variabel
   `preUpdateState`; ketika update melintasi transisi yang diawal workflow →
   `handleWorkflowApproval` (202/approve/reject, satu handler yang sama dengan
   jalur custom action).
3. `decision` dibuang dari payload record (`delete(merged, "decision")`) — ia
   verb approval, bukan field entity; store menolak field tak dikenal.

## Bukti E2E (skenario 6, spec kafe)

| Langkah | Hasil |
| --- | --- |
| Kasir ajukan void (`PATCH status: cancelled`) | **202** `approval_required` — `workflow: order-void-approval`, `from: paid`, `to: cancelled` |
| User tanpa role step mencoba approve | **403** `WORKFLOW_DENIED: user does not hold any of the step's required roles` |
| Supervisor (role `cafe-order.supervisor`) approve (`{"decision":"approve"}`) | **200** `transition_completed`, `to: cancelled` |
| GET order | `status: cancelled`, `void_reason` tersimpan |
| Baris `formspec_workflow_approval` | tersimpan: requester, approver id, from/to |

Sebelum fix: PATCH yang sama langsung **200 `cancelled`** tanpa approval.

Test pengunci: `TestFindTransitionByStates` (unit, termasuk transisi
multi-asal void-order).

## Catatan

- **Sisa kecil 15.10 kini JUGA diperbaiki (2026-09-21, item master todo 15.11):**
  baris approval tetap `status: pending` setelah quorum + transisi. Fix: set
  `approval.Status = ApprovalApproved` saat `AllStepsApproved` sebelum persist.
  Ini bukan kosmetik: baris pending itu yang di-scan escalation worker, jadi
  approval yang sudah selesai akan meng-eskalasi selamanya (4 jam per
  `escalation.after`). Bukti E2E: approve → `transition_completed` + baris
  `status = approved`; jalur reject diverifikasi — reject → `rejected`, order
  **tetap `paid`** (`on_reject.to`: transisi tidak pernah dieksekusi, state
  asal tidak pernah berubah).
- Skenario 6 sekarang ✅ API-level penuh (ajukan → approve → tercatat; ajukan →
  reject → kembali). 9.4 tersisa: skenario UI-browser (keranjang QR, report
  tampilan, kanban drag).
