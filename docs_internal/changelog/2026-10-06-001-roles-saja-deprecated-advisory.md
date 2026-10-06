# 2026-10-06-001 — Keputusan 5.13.12: `roles`-saja deprecated (advisory), `sequential` dikecualikan

Item 5.13.12 bertanya: apakah eligibilitas berbasis `roles` saja di-deprecate?
Jawabannya **ya, sebagai peringatan** — bukan error, dan bukan penghapusan field.

## Kenapa peringatan, bukan error

Step roles-only **bekerja**: `CanApprove` menerima pemegang role. Memfailkan
run berarti mematahkan manifest yang baik-baik saja, dan repo ini sendiri pernah
memakainya. Yang layak disampaikan bukan "ini rusak", melainkan **apa yang
dirugikan**: nama role ditulis langsung di manifest, jadi mengganti nama role
diam-diam mematikan approval itu. Itu persis yang AGENTS.md aturan 6 minta
dihindari, dan duty (`resource + action`, di-grant dari seed role) menghindarinya.

## Kenapa `mode: sequential` dikecualikan — dan itu load-bearing

Rantai berurutan **diurutkan oleh `roles`**; sebuah duty adalah permission datar
tanpa posisi di urutan itu, sehingga `validateApprovalStepMode` sendiri
**menolak** kombinasi `sequential` + `permission`. Jadi pada rantai, `roles`
bukan warisan — ia **satu-satunya** bentuk yang bisa menyatakan urutan. Ini juga
alasan koreksi 2026-10-04 pada item ini benar: `roles` tidak boleh dihapus.

## Yang ditambahkan

- `scanApprovalRoleOnlySteps` (`cmd/formspec/validate_approval.go`) — peringatan
  per step non-rantai yang bersandar pada `roles` saja, **beserta jalan keluarnya**:
  menyebut duty yang perlu ditambahkan dan grant persisnya
  (`{ page: "workflow:{entity}.{transition}", actions: [{name: {step}}] }`).
- Advisory, di jalur yang sama dengan peringatan delivery-channel: tidak
  menggagalkan run.
- Docs: `02-core-extended.md` §2.1 — `roles` dinyatakan bukan bentuk yang dituju,
  dengan pengecualian rantai ditulis eksplisit. Field `ApprovalStep.Roles` di
  `pkg/spec` diberi godoc yang sama, sehingga alasan itu ikut ke skema/kind docs.

## Bukti

- **4 test** (`validate_approval_roles_test.go`): peringatan menyebut nama step,
  role, `permission:`, dan grant-nya + alasan ("renaming a role"); duty senyap
  (termasuk step yang punya **keduanya**); **`sequential` exempt**; step
  tanpa roles+duty **tidak** dilaporkan di sini (itu error keras di tempat lain —
  tidak boleh dilaporkan dua kali dengan severity berbeda).
- **Bukti live** (salinan tree nyata): step `manager-review` diubah menjadi
  roles-only → `[WARN] … rests on 'roles: demo.manager' alone … add
'permission: <duty-name>' and grant it with
`{ page: "workflow:product.discontinue", actions: [{name: manager-review}] }``,
ringkasan **1 warning / 0 problem** (advisory terbukti). Rantai `sequential`
  yang sah → **0** peringatan roles-only.
- **Blast radius: nol** di seluruh repo — tidak ada step roles-only dan tidak ada
  rantai `sequential` (grep `roles:` hanya menemukan definisi seed role), jadi
  kafe 88/0 · crc 33/0 · service-demo 13/0 tetap.
- `go build` · `go vet` · `gofmt` bersih · `go test ./...` hijau · skema + kind
  docs digenerate ulang (godoc `Roles` ikut).
