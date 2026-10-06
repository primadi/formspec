# 2026-10-04-002 — Grant anonim App publik diturunkan dari permukaan view

Plan: `docs_internal/plan/implicit-public-grants.md`.

**Apa yang diubah.** `App.spec.public_entities` **dihapus** dari kontrak
manifest. Allowlist anonim untuk App `access: public` kini **diturunkan** dari
permukaan App (target menu ∪ `registered_views`) × flag `public` per-view, sekali
per App, lewat `(*ui.Registry).DerivePublicGrants` (`internal/ui/surface.go`).

Aturan turunannya adalah **graf FETCH klien**, bukan graf referensi spec:
Table block → `list`; Form `mode: create/edit/view` → `create` / `update`+`find` /
`find`; `context: {source: entity}` → `find`; `picker.entity` dan
`picker.display.price_entity` → `list`; field `relation` yang ter-render →
target `list`+`find`. `delete` **tidak pernah** implisit. Scope baris diturunkan
dari `param` Table block yang nilainya placeholder route (`":guest_token"`), dan
`find` dibuang saat grant ber-scope (aturan lama, kini di turunan).

**Kenapa.** Daftar manual menduplikasi fakta yang sudah tersirat di view, jadi ia
bisa drift dua arah: entri yang ditulis tetapi tidak pernah di-fetch klien
(over-grant tak terlihat), atau view baru yang lupa didaftarkan (jalur yang
seharusnya jalan malah 401). Pengukuran pada pohon kafe: daftar lama
meng-grant `menu-category` `[list, find]` dan `menu-item` `find` yang **tidak
pernah** di-fetch klien (`category_field` membaca label dari baris yang sudah
dimuat); turunan tidak memberikannya. Arahnya memperketat, bukan melonggarkan.

**Yang diubah:**

- `internal/ui/surface.go` (baru) — `computeAppSurfaceLocked` (diekstrak dari
  `BuildBundle`, kini satu sumber untuk route & entity reachable) +
  `DerivePublicGrants`.
- `internal/ui/meta.go` — `BuildBundle` memakai surface bersama itu, sehingga
  grant dan route tidak bisa lagi dihitung dari dua tempat; `AppContext.
PublicEntities` kini berarti allowlist **hasil turunan**.
- `internal/api/router.go` — `publicGrants()` dari turunan + cache
  (`sync.Once`); `publicScope()` dari turunan; jalur legacy module-wide hanya
  tersisa bila UI/entity registry tidak ada (builder buatan tangan di test).
- `internal/api/meta.go` — `AppContext.PublicEntities` diisi
  `derivedPublicEntitiesFor(app)`.
- `pkg/spec/resources.go` — field + validatornya dihapus; `PublicEntityDecl`
  tetap sebagai tipe internal hasil turunan; `accessLabel` (kini tak terpakai)
  dihapus.
- `internal/permission/validator.go`, `internal/manifest/loader.go` — pesan/komentar
  penolakan `public: true` pada entity action kini menunjuk view publik, bukan
  field yang sudah tidak ada.
- Schema & kind docs diregenerasi (`make generate-schema`,
  `make generate-kind-docs`); `docs/spec/frontend/05-app-kinds.md` §1.1 ditulis
  ulang; `docs/runtimes/06-ui-rest-contract.md` §5 diselaraskan.
- `examples/kafe/spec/apps/kafe-qr.yaml` — blok `public_entities` diganti
  komentar yang menyebut **hasil turunan** dan delta yang sengaja dibuka.

**Bukti (terukur):**

- `internal/app/kafe_spec_test.go::TestDerivePublicGrants_KafeQR` — hasil
  turunan pohon kafe nyata: order `[create, list]` (+scope guest_token),
  table-session `[create, find]`, dining-table `[find]`, menu-item `[list]`,
  menu-item-price `[list]`; `member`/`employee`/`shift`/`cash-movement`/
  `payment` **tidak** ikut; `menu-category` **tidak** ikut.
- `internal/api/kafe_public_agreement_test.go::TestKafeQR_BundleAndEndpointAgree`
  — **menutup kafe 10.13**: himpunan entity yang dikirim ke anonim (bundle) dan
  yang dilayani endpoint kini identik (**5 = 5**), sebelumnya jalur bundle dan
  endpoint dihitung terpisah dan insiden 2026-09-22 mencatat **13 vs 4**.
- `go test ./...` hijau; `make lint` **0 issues**; `formspec validate --schema
schemas` pada pohon kafe **89 manifest, 0 problem**.

**Pelemahan yang dicatat, bukan disembunyikan → kafe 10.76 ⏸️.**
`menu-item-price` kehilangan scope `branch_id`: `price_filter` picker bernilai
`{session.branch_id}` (cabang datang dari record sesi, bukan parameter route pada
entity harga), dan `from: route` adalah satu-satunya sumber yang sah untuk
permukaan publik. Turunan tidak bisa mereproduksinya, jadi anonim kini membaca
harga **semua** cabang. Tidak ada PII, tetapi nyata. Dua opsi penutup dicatat di
itemnya.
