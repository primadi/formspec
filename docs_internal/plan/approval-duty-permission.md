# Plan — Approval duty sebagai permission (menghapus hardcoded role dari `kind: Workflow`)

Menutup: kafe **5.13.8** (selesai), dan pelanggaran `AGENTS.md` aturan 6
(_"Permission = resource + action, never hardcoded role names dalam YAML"_).

**Status:**

| Fase | Isi                                                                                | Status                                                                           |
| ---- | ---------------------------------------------------------------------------------- | -------------------------------------------------------------------------------- |
| 0    | Validator nama role di `steps[].roles`/`notify_roles`/`reassign_roles`             | ✅ `2026-10-04-005`                                                              |
| 1    | Kafe memakai nama role yang benar + komentar palsu dihapus                         | ✅ `2026-10-04-005`                                                              |
| 2    | `WorkflowStep.Name` + `active_step_name` + perbaikan bug indeks escalasi           | ✅ `2026-10-04-006`                                                              |
| 3    | `WorkflowStep.Permission` (duty) + `CanApprove` menerima duty ATAU role            | ✅ `2026-10-04-006`                                                              |
| 4a   | `case "workflow"` + `SetWorkflowDuties` + `GetByName`; **kafe jadi `roles:`-free** | ✅ `2026-10-04-007`                                                              |
| 4b   | Migrasi 4 workflow contoh + seed role-nya (crc, service-demo)                      | ✅ `2026-10-04-010`                                                              |
| 4c   | Hapus `roles` sebagai hard error                                                   | ⚠️ **dikoreksi → 5.13.12**: tidak bisa dihapus, `mode: sequential` mewajibkannya |

Dua koreksi terhadap rencana awal, keduanya karena pengukuran:

- **Duty tidak di-derive otomatis.** Rencana menulis "derived bila absen"; itu membuat
  **memberi nama pada step diam-diam mengubah siapa yang boleh menyetujui**. Kini duty
  bersifat **eksplisit** lewat `permission:`, dan `name` wajib ada bila duty dideklarasikan.
- **`can_decide` tidak boleh lebih ketat dari jalur tulis.** Versi pertama menuntut duty
  secara AND; halaman record memakai OR (`CanApprove`). Dua pintu ke satu alur harus
  sepakat, jadi inbox kini memanggil `CanApprove` yang sama.

Hasil Fase 4a yang bisa diperiksa: dengan `roles:` **dihapus** dari step kafe, supervisor
tetap bisa menyetujui lewat grant duty — dan **mencabut grant itu** (tanpa menyentuh
manifest workflow) langsung mencabut haknya: inbox 0 task, approve 403, record tetap `paid`.
Batas 4b: `crc-management` (3 workflow) dan `service-demo` (1) masih memakai `roles` yang
bahkan tidak dideklarasikan di tree-nya (`5.13.9`) — keputusan produk per example, jadi
penghapusan `roles` sebagai hard error menunggu itu.

**Diperbarui setelah 4b (`2026-10-04-010`):** keempat workflow itu kini memakai `permission`

- seed role-nya, jadi **seluruh workflow di repo memakai duty** — tidak ada migrasi tersisa.
  Gate-nya juga diperluas: pemeriksaan "setiap grant seed resolve" kini menjaga setiap example
  yang punya role seed (menemukan tree-nya sendiri), plus pemeriksaan baru bahwa setiap step
  **bisa dipenuhi** (`roles` dideklarasikan, atau duty-nya benar-benar di-grant).

**Dan 4c dibatalkan karena bertentangan dengan validator sendiri:** `roles` **tidak bisa**
dihapus — `mode: sequential` mewajibkannya (rantai itulah urutannya, dan `validateWorkflowStepMode`
menolak rantai tanpa role), sementara duty justru **dilarang** digabung dengan `sequential`.
Maka `roles` tetap sah; yang tersisa hanyalah keputusan apakah eligibilitas `roles`-saja
(tanpa duty) di-deprecate (**5.13.12**), bukan penghapusan field. ✅ **Diputuskan 2026-10-06:**
deprecate sebagai **peringatan advisory** (bukan error), dan `mode: sequential` dikecualikan —
rantai diurutkan oleh `roles`, jadi di sana `roles` satu-satunya bentuk yang bisa menyatakan
urutan. Changelog `2026-10-06-001`.

## Masalah

`WorkflowStep.Roles []string` menyimpan **nama role** di YAML. Empat cacat
mengalir dari satu keputusan itu:

| #   | Cacat                                                                                                                                                                                                                                                                  | Bukti terukur                                                                                                              |
| --- | ---------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- | -------------------------------------------------------------------------------------------------------------------------- |
| 1   | **Nama role tidak pernah divalidasi.** `roles: [cafe-order.supervisor]` di kafe tidak punya role record, dan `formspec validate` meloloskannya **89 manifest / 0 problem** padahal alur void tidak bisa disetujui                                                      | `grep Roles cmd/formspec/validate_workflow.go` → 0 hasil; live `403 "user does not hold any of the step's required roles"` |
| 2   | **Gagal senyap, dua arah.** `CanApprove` mencocokkan string literal; `RoleStore.GetByName` → `FindByField` eksak lalu `continue // unknown role — skip`. Jadi nama yang cocok untuk approval tapi tidak ada sebagai role **justru mencabut semua permission** user itu | `internal/auth/resolver.go`                                                                                                |
| 3   | **Step tidak punya identitas.** Dirujuk lewat **index**: `Approval.ActiveStep`, `EscalatedSteps map[int][]string`                                                                                                                                                      | `internal/workflow/engine.go`                                                                                              |
| 4   | **`mode`/`approvers` tidak jujur.** `eligibleCount = len(step.Roles)` = jumlah **nama role**, bukan orang — `roles: [a, b]` bisa "lulus" dengan dua orang dari role yang sama                                                                                          | `internal/api/handler.go`                                                                                                  |

Cacat 3 juga menyembunyikan **bug laten**: `escalation.go` membaca
`wf.Steps[row.ActiveStep]` sedangkan `handleWorkflowApproval` memakai list
**terfilter** (`ApplicableSteps`). Begitu ada step ber-`when: false`, escalasi
membaca step yang salah.

## Arah

Step diberi **identitas**; identitas itu menghasilkan **permission**; permission
itulah yang di-grant ke role. Bentuk grant yang dipakai sudah ada
(`{kind}:{name}` lewat `resolveFootprint`), jadi tidak perlu schema grant baru.

```yaml
steps:
  - name: supervisor-check # identitas stabil (baru)
    # permission: workflow.cafe-order.order-void-approval.supervisor-check  (derived bila absen)
```

```yaml
# roles.yaml — siapa boleh menandatangani ditentukan grant, bukan nama di workflow
grants:
  - {
      page: "workflow:order-void-approval",
      actions: [{ name: supervisor-check }],
    }
```

## Fase

### Fase 0 — Validator (small, mandiri)

Menutup cacat 1. **Tidak menunggu keputusan kontrak apa pun.**

- `buildRoleNameIndex(manifests)` di `cmd/formspec/validate_workflow.go`, meniru
  `buildEntityFieldIndex`: baca `kind: Seed`, ambil record role lewat
  `isRoleEntityName` + `RawSpecTo[spec.SeedSpec]` (pola yang **sudah dipakai**
  `cmd/formspec/validate_seed_grants.go`).
- `workflowRoleError(wf, roleNames)` memeriksa `steps[].roles`,
  `escalation.notify_roles`, `escalation.reassign_roles`.
- Wire di `cmd/formspec/validate.go` (bersama `workflowRejects`).
- **Properti pembuktinya: kafe harus MERAH setelah fase ini.** Itu buktinya
  pemeriksaan ini nyata; Fase 1 menghijaukannya.

### Fase 1 — Hijaukan kafe (small) — depends on 0

- `order-void-approval.yaml`: `roles: [cafe-order.supervisor]` → `[supervisor]`,
  `notify_roles: [cafe-order.manajer]` → `[manajer]`.
- Hapus komentar yang **salah**: "`cafe-order.supervisor` memetakan ke employee
  berposisi supervisor/manajer" — tidak ada jembatan `position`→role di repo
  (`position` hanya enum di `employee.yaml`; tidak ada hook/script yang menyentuh
  `roles`). Komentar itu yang membuat nama qualified tampak masuk akal.
- Baris `roles:` ini **dihapus** di Fase 4.

### Fase 2 — Identitas step + bug escalasi (medium) — depends on 1

- `WorkflowStep.Name` + validasi (unik per workflow; wajib bila step punya
  `permission`).
- `EscalatedSteps`/`Approvals` di-key **nama step**; pembaca tetap menerima key
  integer (baris DB lama).
- Perbaiki bug escalasi: satu sumber daftar step (`ApplicableSteps`) di kedua sisi.

### Fase 3 — Duty sebagai permission ✅ (medium) — depends on 2

- `WorkflowStep.Permission`; derived `workflow.{module}.{workflow}.{step}` bila absen.
- `CanApprove`: `identity.HasPermission(duty)` bila duty ada, **fallback** ke
  `Roles` supaya manifest lama tidak putus.
- `can_decide` di inbox jadi **dua** gerbang (permission transisi **dan** duty).
- Verifikasi wildcard (jangan diasumsikan): `workflow.*` menjangkau kedalaman
  berapa pun; `workflow.cafe-order.*` menuntut **tepat satu** segmen lagi →
  **tidak** menjangkau duty 4-segmen.

**Dua koreksi dari pengukuran:** duty bersifat **eksplisit** (`permission:`), bukan
"derived bila absen" — derived membuat memberi nama pada step diam-diam mengubah
siapa yang boleh menyetujui. Dan `can_decide` memakai predikat yang **sama** dengan
jalur tulis (`CanApprove`), bukan versi yang lebih ketat: dua pintu ke satu alur
tidak boleh berbeda pendapat. (Kembar dengan keputusan kuorum di bawah.)

### Fase 4 — Grant + drop `Roles` (medium-large) — depends on 3

- `navigationFootprint` `case "workflow"` → satu `FootprintAction` per step.
- `roles.yaml`: grant duty untuk `supervisor`.
- Migrasi 4 workflow di dua example lain (`service-demo`, `crc-management` ×3).
- `Roles` dihapus sebagai **hard error** dengan masa transisi bernama.

## Keputusan

- **Bentuk permission duty** — `workflow.{module}.{workflow}.{step}`: tidak
  bertabrakan dengan permission entity 3-segmen, tidak ikut terjangkau
  `cafe-order.*`, dan grant bisa menghitungnya tanpa memuat entity. Override
  eksplisit lewat `permission:` dipertahankan untuk kasus khusus.
- **Kafe Fase 1** — ubah workflow (1 baris), bukan rename 6 role + 7 user; dan
  baris itu hilang di Fase 4.
- **`mode: all`** — sebelum Fase 3, `eligibleCount = len(step.Roles)` sudah
  **fiksi** (jumlah nama role, bukan orang). Setelah Fase 3, rekomendasi:
  `approvers` wajib eksplisit bila step memakai duty, dan `mode: all` tanpa angka
  ditolak validator. Untuk 5 workflow yang ada ini **no-op** (semuanya
  `approvers: 1`). → dikerjakan sebagai **5.13.11**, dan hasilnya lebih jauh dari
  rekomendasi: `mode: all` **seluruhnya** ditolak (angkanya tidak bisa diturunkan
  dari manifest mana pun, dengan atau tanpa duty), `mode: sequential` yang tadinya
  hanya menetapkan kuorum 1 tanpa menegakkan urutan kini **benar-benar**
  berurutan, dan `CanApprove` menerima **daftar step** sebagai parameter —
  jalur approve memeriksa step dari daftar _authored_ padahal langkahnya dari
  daftar _terfilter_, kemunculan ketiga bug daftar yang sama.

## Di luar cakupan

- `notify_roles` **inert** (grep: hanya definisi struct, tak ada pembaca) → item
  `⏸️` tersendiri.
- 5.13.7 (realtime inbox), 5.25.10 (grant per-inbox) → tetap terbuka.
