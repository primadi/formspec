# 2026-10-04-010 — Empat workflow yang tak bisa disetujui kini punya role (5.13.9)

Menutup **5.13.9**. Empat workflow approval menamai role yang **tidak pernah
dideklarasikan di tree-nya sendiri** — kelas bug yang sama dengan 5.13.8, di dua
example yang tidak punya role seed sama sekali.

| Example          | Workflow                       | Role yang tidak ada |
| ---------------- | ------------------------------ | ------------------- |
| `crc-management` | `cap-approval`                 | `crc.cap-approver`  |
| `crc-management` | `foreman-review`               | `crc.foreman`       |
| `crc-management` | `customer-approval`            | `crc.customer`      |
| `service-demo`   | `product-discontinue-approval` | `demo.manager`      |

Akibatnya sama persis: `workflow.hasAnyRole` membandingkan nama secara **literal**,
jadi role yang tidak ada berarti approver yang tidak ada — approval **403
selamanya**, dengan manifest yang terlihat benar.

## Yang landing

- **`examples/crc-management/spec/modules/formspec.core/seeds/roles.yaml`** (baru):
  lima role — `crc.foreman`, `crc.foreman-supervisor`, `crc.cap-approver`,
  `crc.cap-approver-supervisor`, `crc.customer`.
- **`examples/service-demo/.../formspec.core/seeds/roles.yaml`** (baru): dua role —
  `demo.manager`, `demo.head`.
- **Keempat workflow memakai DUTY, bukan nama role.** Step-nya kini menyatakan
  `permission:`, dan grant di seed yang memberikannya:

  ```yaml
  # workflow
  - name: cap-sign-off
    permission: cap-sign-off
  # seed
  - { page: "workflow:cap-approval", actions: [{ name: cap-sign-off }] }
  ```

  Jadi kedua example ini memakai bentuk yang dituju plan
  `approval-duty-permission.md` — sama seperti kafe — dan **seluruh workflow di
  repo kini memakai `permission`**.

- `notify_roles`/`reassign_roles` tetap memakai role (mereka memang menamai role
  PENERIMA, bukan penentu eligibilitas), dan role-nya kini benar-benar ada —
  termasuk role yang **hanya** menerima notifikasi eskalasi
  (`crc.foreman-supervisor`, `crc.cap-approver-supervisor`).

## Gate baru: pemeriksaan yang sama untuk SEMUA example

`TestAllExamples_SeedGrantsResolve` dan
`TestAllExamples_WorkflowStepsAreSatisfiable` (`internal/auth/examples_seed_grants_test.go`).

Sebelumnya gate ini hanya menjaga kafe (`kafe_seed_grants_test.go`) — dan itu
tidak cukup, karena perubahan di atas adalah persis jenis perubahan yang gate itu
ada untuk memeriksa: menambah grant duty ke dua tree, di mana salah ketik nama page
atau nama step akan **diam**. Example adalah yang orang **copy**, jadi grant yang
rusak di sana ikut tercopy.

Test-nya **menemukan** tree-nya sendiri (glob `examples/*/spec` yang punya role
seed), jadi example baru tertutup begitu ia ada, tanpa daftar yang harus
diperbarui. Yang diperiksa:

1. setiap page/action yang dinamai seed **resolve** di tree-nya (grant yang tidak
   resolve dibuang diam-diam oleh resolver), dan role tidak materialisasi ke nol;
2. setiap step workflow **bisa dipenuhi**: `roles`-nya dideklarasikan, atau —
   bila step memakai `permission` — ada role yang benar-benar **di-grant** duty
   itu. Deklarasi tanpa grant hanya separuh kontrak.

**Bukti gate-nya nyata:** mencabut satu grant duty dari seed crc membuat test
merah dengan pesan yang menyebut permission persisnya —
`no role is granted the duty "workflow.crc-field.foreman-review.foreman-sign-off"`.

## Verifikasi

| Tree             | Problem sebelum | Sesudah | Sisa                                                                         |
| ---------------- | --------------- | ------- | ---------------------------------------------------------------------------- |
| `crc-management` | 5               | **2**   | 2 yang sudah ada sebelumnya (integrator tanpa cancel handler, schema report) |
| `service-demo`   | 2               | **1**   | 1 yang sudah ada sebelumnya (schema `deliver.channel`)                       |

- **Seed benar-benar dijalankan**, bukan hanya divalidasi:
  `formspec seed --spec examples/crc-management/spec` → **5 inserted**;
  `service-demo` → **2 inserted**.
- Test materialisasi nyata (registry + lookup workflow seperti boot) hijau untuk
  **kafe, crc-management, service-demo**.
- `go test ./...` hijau.

## Temuan: `escalation` level workflow adalah deklarasi mati

`WorkflowEscalation` (`spec.escalation` — `after` + `notify_roles`) **tidak dibaca
siapa pun**: `EscalationWorker` hanya membaca `step.Escalation`, dan `notify_roles`
(di level mana pun) tidak punya pembaca. Manifest crc memakainya di level workflow
untuk ketiga approval-nya, jadi eskalasi 48h/72h itu **tidak pernah terjadi**.

Pemeriksaan nama role saya diperluas untuk mencakupnya (aturan yang sama, dan
melewati salah satu dari dua ejaan identik adalah cara kedua ejaan itu melenceng),
tetapi **deklarasi matinya tidak saya "perbaiki" diam-diam** — itu keputusan
fungsional: apakah eskalasi level workflow diimplementasikan, atau dihapus dari
kontrak karena per-step sudah cukup. Dicatat sebagai **5.13.14 ⏸️**.

## Koreksi: 5.13.12 tidak bisa berupa "hapus `roles`"

Item 5.13.12 saya tulis sebagai _"hapus `roles:` sebagai hard error setelah semua
workflow memakai duty"_. Setelah mengerjakan 5.13.9, itu **bertentangan dengan
validator kita sendiri**: `mode: sequential` **mewajibkan** `roles` (rantai itulah
urutannya, dan `validateWorkflowStepMode` menolak rantai tanpa role), dan `roles`
juga tetap jalur migrasi yang sah untuk langkah yang belum punya duty.

Jadi yang benar bukan penghapusan, melainkan: **`permission` adalah bentuk yang
dituju, `roles` tetap sah** — untuk rantai sequential, dan sebagai alternatif
eligibilitas. Setelah 5.13.9, **seluruh workflow di repo memakai `permission`**,
jadi tidak ada pekerjaan migrasi yang tersisa; yang tersisa hanyalah keputusan
apakah `roles`-sebagai-eligibilitas tunggal (tanpa duty) perlu di-deprecate.
Item 5.13.12 diperbarui untuk menyatakan itu, bukan menghapus field yang masih
dipakai.

## File terdampak

- `examples/crc-management/spec/modules/formspec.core/seeds/roles.yaml` (baru).
- `examples/service-demo/spec/modules/formspec.core/seeds/roles.yaml` (baru).
- Ketiga workflow crc + satu workflow service-demo → `permission:`.
- `cmd/formspec/validate_workflow.go` — pemeriksaan role mencakup `escalation`
  level workflow.
- `internal/auth/examples_seed_grants_test.go` (baru) — gate untuk semua example.
- `docs_internal/plan/todo.md` — 5.13.9 ditutup, 5.13.12 dikoreksi, 5.13.14 dibuka.

## Sisa (→ todo)

- **5.13.14 ⏸️** (baru) — `escalation` level workflow adalah deklarasi mati
  (`after` + `notify_roles` tanpa pembaca). Implementasikan atau hapus dari
  kontrak.
- **5.13.12 ⏸️** (dikoreksi) — keputusan apakah eligibilitas berbasis `roles` saja
  (tanpa duty) di-deprecate. Bukan penghapusan field.
- **5.13.7 ⏸️** — realtime inbox.
