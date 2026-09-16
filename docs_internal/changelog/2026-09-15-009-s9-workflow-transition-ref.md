# 2026-09-15-009 — S9: workflow merujuk nama transisi (kafe 1.7)

Kafe ledger **1.7**, menutup **#38**. Transisi state machine boleh punya banyak
state asal (`from: [paid, in_kitchen, ready, served]`), tetapi pemicu Workflow
hanya bisa menyebut **satu pasangan from/to**. Jadi `from: paid` membuat void
dari `in_kitchen`/`ready`/`served` **lolos approval tanpa error dan tanpa log** —
dan `formspec validate` tetap hijau karena validator tidak melihat state machine
entity-nya sama sekali.

**Konstruksi.** `WorkflowTransitionRef.Name` — pemicu boleh merujuk transisi
lewat namanya (`via`), dan satu referensi mengawal **seluruh** state asal
transisi itu:

```yaml
on:
  transition: { name: void-order }
```

Dua bentuk (`name` vs `from`/`to`) **saling eksklusif** dan divalidasi
(`ValidateWorkflowSpec` + loader, sehingga `dev`/`apply` juga menolaknya).
`From`/`To` mendapat `omitempty` supaya bentuk name-only tidak lagi gagal di
lapis schema karena keduanya tadinya `required`.

**Penegakan runtime.** `internal/workflow` kini punya index kedua `byName`, dan
`ForTransition(entity, transition, from, to)` menggabungkan keduanya (dedup).
Signature `RequiresApproval`/`WorkflowsFor` menerima `transition` — perubahan
yang disengaja: tanpa nama transisi, pemetaan byName tidak bisa diresolusi dan
pemanggil harus menebak dari (from,to), yang justru sumber lubangnya. Kedua call
site di `internal/api/handler.go` meneruskan `actionName` (nama transisi =
nama action) lewat `handleWorkflowApproval`.

**Validator: lubangnya jadi error, bukan catatan.** Layer 1.5 baru
`validateWorkflows` (`cmd/formspec/validate_workflow.go`), pola yang sama dengan
`validateIntegrators` — satu-satunya tempat yang melihat Workflow **dan** state
machine entity sekaligus. Yang ditolak:

| Keadaan                                            | Pesan                                                                      |
| -------------------------------------------------- | -------------------------------------------------------------------------- |
| Pasangan `from`/`to` pada transisi multi-asal      | menyebut transisi mana, semua state asalnya, dan bentuk `name:` yang benar |
| Nama transisi tidak ada / salah ketik              | menyebut entity + daftar `via` yang tersedia                               |
| Entity tanpa state machine                         | approval tidak akan pernah memicu                                          |
| Nama berkualifikasi yang tidak cocok `spec.entity` | "pick one form"                                                            |
| Pasangan state yang bukan transisi                 | menyebut transisi yang dikenal                                             |

Ini yang membedakannya dari sekadar menambah fitur: bentuk lama yang bocor tidak
bisa kembali tanpa ketahuan.

**Adopsi kafe.** `order-void-approval.yaml` → `on: {transition: {name: void-order}}`;
komentar GAP-38 yang menjelaskan lubangnya dihapus, diganti penjelasan kenapa
nama transisi yang dipakai.

**Verifikasi.**

| Bukti                                                | Hasil                                                                                                                                                                                                              |
| ---------------------------------------------------- | ------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------ |
| `TestRegistry_ForTransitionByName`                   | `void-order` dari keempat state asal → 1 workflow; `cancel-order` (target state sama) → 0; entity lain → 0                                                                                                         |
| `TestValidateWorkflows_RejectsPartialStatePair`      | `from: paid` pada transisi 4-asal **ditolak**, pesan menyebut `in_kitchen` + `name:`                                                                                                                               |
| `TestValidateWorkflows_AcceptsSingleOriginStatePair` | transisi satu-asal tetap diterima (anti over-rejection)                                                                                                                                                            |
| `TestValidateWorkflows_Detects`                      | 4 kasus salah rujuk tertangkap                                                                                                                                                                                     |
| `TestValidateWorkflowSpec_TransitionForms`           | 8 kasus bentuk pemicu (name / pair / keduanya / tidak keduanya / tanpa steps)                                                                                                                                      |
| `formspec validate` (default) sebelum adopsi         | **menolak** kafe dengan pesan yang mengarahkan ke `name:`                                                                                                                                                          |
| `formspec validate --schema schemas`                 | kafe 69 manifest 0 problem; service-demo 13 manifest 0 problem; crc-management 34 manifest 1 problem (**pre-existing**, integrator 7.7.2 di `sharepoint-archiver.yaml` — berkas yang tidak disentuh perubahan ini) |
| `formspec check -f examples/kafe/spec`               | 0 error, 0 warning                                                                                                                                                                                                 |
| `go test ./...`                                      | hijau                                                                                                                                                                                                              |

Test baru: `internal/workflow/registry_test.go` (+2), `cmd/formspec/validate_workflow_test.go` (baru, 8 sub-test),
`pkg/spec/workflow_test.go` (baru, 9 sub-test).

**Keputusan bentuk (dicatat, berbeda dari usulan ledger).** Ledger mengusulkan
`transition: <string>`. Itu union string-atau-objek pada satu field; generator
schema hanya bisa mengekspresikan bentuk objek, jadi menambahkannya berarti
menambah satu lagi kelas "lolos engine, ditolak schema" — preseden yang sudah ada
pada `guard:`/`render:`. Dipilih `transition: {name: ...}`: maksud yang sama,
tanpa divergensi engine↔schema.

**Sisa (dicatat).** Tidak ada test level-API untuk interception approval —
harness auth + seed 4 state belum ada di `internal/api`. Bukti saat ini
unit-level (registry/engine/validator) dan dinyatakan begitu di ledger, bukan
diklaim sebagai E2E runtime.
