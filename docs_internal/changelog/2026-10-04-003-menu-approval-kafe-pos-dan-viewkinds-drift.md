# 2026-10-04-003 — Grup menu "Persetujuan" kafe-pos + `viewKinds` yang tertinggal empat kind

Todo **5.25.9** (dibuka hari ini) · Plan `docs_internal/plan/registered-views.md`

## 1. `supervisor-inbox` kini punya entri navigasi (kafe-pos)

`examples/kafe/spec/apps/kafe-pos.yaml` mendapat grup menu authored
**"Persetujuan" → "Antrean Void"** (`module: cafe-order`, `view: supervisor-inbox`),
dan entri `cafe-order/supervisor-inbox` **dihapus** dari `registered_views` —
menu adalah deklarasi permukaan, jadi mendaftarkannya di dua tempat hanya duplikasi.

Sebelumnya `registered_views` hanya membuat route-nya **ada**, bukan
**terjangkau**: tidak ada tautan/bell/menu yang membukanya, jadi supervisor harus
mengetik `/kafe/app/pos/approval-inbox/supervisor-inbox` sendiri. Pembedaan
"ada vs terjangkau" yang sama dengan yang sudah dicatat untuk wizard di 5.25.4.

Entri ini sengaja dipasang di App, bukan di saran menu `cafe-order/module.yaml`:
antrean approval adalah tugas supervisor, bukan langkah alur kasir, sehingga
modul tidak boleh memaksakannya ke App yang tidak punya role itu.

## 2. `viewKinds` (validator) tertinggal empat kind

Menulis entri menu itu **gagal validasi**:

```
[FAIL] spec/apps/kafe-pos.yaml#0
       menu item "Antrean Void" references view "supervisor-inbox", which is not
       a registered Form/Table/Page/Wizard/Report/Kanban/Timeline/Calendar/
       Dashboard/Listing — the menu entry would navigate nowhere
```

Pesannya berbohong: `(*Registry).ResolveViewRoute` (`internal/ui/registry.go`)
**mengenal** `approval-inbox`, `notification-center`, `print`, dan `widget`, dan
`buildRoutes` (`renderers/react-shadcn/src/shell/router.tsx`) benar-benar
mendaftarkan route untuk keempatnya (`/approval-inbox/{name}`,
`/notification-center/{name}`, `/print/{name}[/{:id}]`, `/widget/{name}`).
Keempatnya hanya absen dari satu salinan konvensi: `viewKinds` di
`cmd/formspec/validate_dangling.go`.

Akibatnya seorang author diberi satu-satunya jalan keluar yang **salah** —
jangan taruh view di menu — padahal menu justru cara yang benar.

Perbaikan: keempat kind ditambahkan ke `viewKinds`, dan daftar kind di pesan
error kini **diturunkan dari `viewKinds`** (`viewKindNames()`), bukan ditulis
sebagai prosa, sehingga pesannya tidak bisa lagi mengiklankan kosakata yang lebih
sempit daripada yang diterima pemeriksa.

## Bukti

- `go test ./...` hijau.
- `make lint` (`GOLANGCI_LINT_CACHE=/tmp/glci`) **0 issues**.
- `formspec validate --schema schemas` di `examples/kafe` → **89 manifest, 0 problem**
  (sebelum perbaikan: **1 problem**, yaitu entri menu di atas).
- Regresi baru: `TestValidateDanglingRefs` sub-test _"every kind with a client
  route is accepted as a menu view"_ (Widget/Print/ApprovalInbox/NotificationCenter).
- `TestResolve_KafeSpec` + `TestBuildBundle_SurfaceGate_UnionOfMenuAndRegisteredViews`
  tetap lulus, jadi supervisor-inbox tetap reachable lewat jalur menu.

## Sisa (→ todo)

- **5.25.9 ⏸️** — `Print` tidak punya pemicu di UI: `receipt-thermal`/
  `receipt-digital` punya route (`registered_views`) tetapi tidak ada tombol yang
  membukanya (tidak ada `print/...` di `src/kinds`/`src/engine`), jadi kasir masih
  mengetik URL. Menunggu keputusan bentuk pemicunya.
- **5.25.10 ⏸️** (baru) — `navigationFootprint` (`internal/auth/materialize.go`)
  belum mengenal `approval-inbox:`/`notification-center:`, jadi grant per-inbox
  belum bisa; nama tak dikenal membuang **seluruh** grant role tanpa suara
  (`seeds/roles.yaml`). Efeknya supervisor tidak bisa diberi grant inbox —
  ia hanya boleh approve karena punya hak `cancel` pada `order`.
- **5.13.6 ✅ DITUTUP 2026-10-04** (dulu ⏸️) — sumber data inbox dulu kosong,
  sehingga tautan baru ini mendarat di "No approval source configured". Kini ada:
  `GET /{ws}/_ui/workflow/approvals` (changelog `2026-10-04-004`). Sisa yang
  benar-benar terbuka sekarang adalah `realtime` (**5.13.7 ⏸️**).
- **5.25.10 ⏸️** (sudah ada) — grant per-inbox belum bisa; tidak berubah.
