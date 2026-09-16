# Plan — 1.7 / S9: workflow merujuk nama transisi

**Kafe ledger:** `examples/kafe/gaps_found/TODO.md` 1.7 (prioritas §F #7) ·
kelengkapan spec S9 (`13-kelengkapan-spec-untuk-kafe.md:417`)
**Menutup akar:** #38 (void dari 4 state asal tak bisa dilewati)
**Effort:** medium (spec + registry/engine + 2 call site API + validator + adopsi)

---

## Masalah

Transisi state machine punya identitas (`via: void-order`) dan boleh punya
**banyak state asal** (`from: [paid, in_kitchen, ready, served]`). Pemicu
Workflow hanya bisa menyebut **satu pasangan from/to** (`WorkflowTransitionRef`
= dua string). Jadi satu workflow hanya mengawal satu state asal; void dari tiga
state lain lolos approval **tanpa error, tanpa log** — dan `formspec validate`
tetap hijau karena validator tidak tahu transisi aslinya punya empat state asal.

## Keputusan desain

Ledger mengusulkan `on: {transition: cafe-order.order.void-order}` (string).
Itu bentuk **string-atau-objek** pada satu field. Repo sudah punya preseden
shorthand seperti ini (`guard: "expr"` vs `guard: {expression}`) — dan
`cmd/formspec/validate.go` mencatat konsekuensinya: generator schema hanya bisa
mengekspresikan bentuk objek, jadi lapis schema **lebih ketat** dari engine.
Menambah satu lagi union serupa berarti menambah satu lagi kelas "manifest lolos
engine tapi ditolak schema".

Maka bentuk yang dipilih — tetap merujuk transisi, tapi tanpa union:

```yaml
on:
  transition:
    name: void-order # nama transisi (state machine `via`)
```

Alasan:

- Satu field baru di dalam objek yang sudah ada → schema bisa
  mengekspresikannya apa adanya, tanpa divergensi engine↔schema.
- `name` scoped ke `spec.entity` Workflow (entity sama dengan field `entity`),
  jadi tidak ada ambiguitas lintas entity; nama berkualifikasi
  (`cafe-order.order.void-order`) diterima **hanya bila cocok** dengan entity
  yang dideklarasikan.
- Dua bentuk (name vs from/to) bersifat **saling eksklusif** dan divalidasi —
  bukan dua jalur yang bisa saling menutupi.

## Yang akan dikerjakan

1. **`pkg/spec`** — `WorkflowTransitionRef.Name`; `From`/`To` dapat `omitempty`
   (kalau tidak, keduanya tetap `required` di schema dan bentuk name-only gagal
   validasi schema). `ValidateWorkflowSpec`: tepat satu bentuk, bukan dua,
   bukan nol.
2. **`internal/workflow/registry.go`** — index kedua `byName`
   (`{entity}.{name}`); `ForTransition(entity, transition, from, to)` =
   gabungan index byName + byTransition (dedup). `List()` mengekspos `transition`.
3. **`internal/workflow/engine.go`** — `RequiresApproval`/`WorkflowsFor` menerima
   `transition`. Ini perubahaan signature, disengaja: tanpa nama transisi,
   pemetaan byName tidak bisa diresolusi dan pemanggil akan menebak dari
   (from,to) — persis sumber lubangnya.
4. **`internal/api/handler.go`** — teruskan `actionName` (nama transisi = nama
   action, sudah tersedia di kedua call site) lewat `handleWorkflowApproval`.
5. **`cmd/formspec/validate.go`** — Layer 1.5 baru `validateWorkflows`, pola yang
   sama dengan `validateIntegrators` (satu-satunya tempat yang melihat seluruh
   manifest sekaligus, sehingga bisa meresolusi transisi):
   - bentuk `name` → transisi harus ada di state machine entity itu (salah ketik
     ditolak, dengan daftar `via` yang tersedia);
   - bentuk `from`/`to` → pasangan state harus ada; **dan** bila transisi itu
     punya state asal lain, tolak dengan pesan yang mengarahkan ke `name:` —
     inilah yang mengubah lubang senyap S9 jadi error validate.
6. **Adopsi kafe** — `order-void-approval` memakai `name: void-order`; komentar
   GAP-38 dihapus.
7. **Bukti** — unit test registry/engine (void dari keempat state asal lolos
   interupsi), test validator (typo ditolak; from/to pada transisi multi-asal
   ditolak), `formspec validate` + `check` hijau, dan E2E HTTP bila memungkinkan.

## Risiko

- Perubahan signature `RequiresApproval`/`WorkflowsFor` menyentuh test yang ada
  (`registry_test.go`, `escalation_test.go`) — itu memang kontrak yang berubah,
  jadi test diperbarui, bukan ditambal.
- Deteksi multi-asal harus **tepat**: hanya menolak bila transisi yang
  dirujuk pasangan (from,to) memang punya state asal lain. Empat Workflow lain di
  repo (`service-demo`, tiga di `crc-management`) memakai transisi satu-asal, jadi
  tidak terpengaruh — sudah diperiksa sebelum memutuskan menjadikannya error.
