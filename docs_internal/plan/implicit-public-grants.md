# Plan: Implicit public grants — hapus `public_entities` dari manifest

Status: **implemented** (2026-10-04) — changelog `2026-10-04-002`.

## Hasil terukur (setelah implementasi)

| Item                                        | Hasil                                                                |
| ------------------------------------------- | -------------------------------------------------------------------- |
| `go test ./...`                             | 39 paket ok, 0 gagal                                                 |
| `make lint`                                 | **0 issues**                                                         |
| `formspec validate --schema schemas` (kafe) | 89 manifest, **0 problem**                                           |
| `formspec check` (kafe)                     | **0 error, 0 warning**                                               |
| Bundle vs endpoint (kafe-qr)                | **5 = 5** entity (insiden lama **13 vs 4**) → menutup kafe **10.13** |

Grant turunan kafe-qr: `order [create, list]` (+scope `guest_token`),
`table-session [create, find]`, `dining-table [find]`, `menu-item [list]`,
`menu-item-price [list]`. **Tidak** ikut: `menu-category`, `menu-item.find`,
`member`, `employee`, `branch`, `promo`, `shift`, `cash-movement`, `payment`.

## Masalah

`App.spec.public_entities` adalah allowlist manual yang menyatakan entity+aksi
apa yang boleh dipanggil anonim. Ia tidak intuitif karena:

1. Deklarasinya jauh dari tempat entity dipakai (App vs Page/Form/Table).
2. Verb yang dibutuhkan sudah tersirat di bentuk pemakaian —
   `table` jelas butuh `list`, `form mode: create` jelas butuh `create` —
   tetapi harus ditulis ulang dengan tangan.
3. Sejak `registered_views` ada (permukaan App), menulis ulang daftar yang
   sama dua kali jadi beban ganda.

## Keputusan

**Grant anonim diturunkan implisit dari permukaan App.** Field
`public_entities` DIHAPUS dari kontrak manifest (schema, validator, dokumen).
`PublicEntityDecl` tetap ada sebagai tipe internal hasil derivasi.

Sumbu otoritas menjadi satu:

> **permukaan view** (menu ∪ `registered_views`) **× `public` per-view**
> → grant anonim.

### Prinsip derivasi: graf FETCH klien, bukan graf REFERENSI spec

Ini yang membuat hasilnya presisi dan bukan asal melebar. Bukti terverifikasi:

| Konstruk                          | Di-fetch klien                     | Grant         |
| --------------------------------- | ---------------------------------- | ------------- |
| `picker.entity`                   | `GET /{m}/{e}?per_page=200`        | `list`        |
| `picker.display.price_entity`     | `GET /{m}/{e}?per_page=500`        | `list`        |
| `context: {source: entity}`       | `GET /{m}/{e}/{id}`                | `find`        |
| Table block                       | `GET /{m}/{plural}`                | `list`        |
| field `relation` ter-render       | `GET /{m}/{e}?search=` + `/{id}`   | `list`+`find` |
| select filter pada field relation | `GET /{m}/{e}?per_page=500`        | `list`        |
| `picker.display.category_field`   | — (dibaca dari baris ter-load)     | **tidak**     |
| kolom table ber-relasi            | — (alias di-inline server)         | **tidak**     |
| snapshot / `computed`             | — (nilai statis / dihitung server) | **tidak**     |

Konsekuensinya: derivasi menghasilkan grant yang **lebih sempit** daripada
daftar manual kafe-qr hari ini (`menu-category` `[list, find]` dan
`menu-item.find` tidak di-fetch klien sama sekali) — jadi ini perbaikan
over-grant, bukan pelonggaran.

## Desain

### Peta verb

| Deklarasi                                                                  | Aksi                                           |
| -------------------------------------------------------------------------- | ---------------------------------------------- |
| Table block / Table view / Kanban / Report / Listing / Timeline / Calendar | `list`                                         |
| Form `mode: create`                                                        | `create`                                       |
| Form `mode: edit`                                                          | `update` + `find`                              |
| Form `mode: view` / kosong                                                 | `find`                                         |
| Page/Form `context[].source: entity`                                       | `find`                                         |
| child field `picker.entity` / `picker.display.price_entity`                | `list`                                         |
| entity field `relation.belongs_to` yang ter-render                         | target `list` + `find`                         |
| `registered_views: [{entity: X}]`                                          | `list`+`find`+`create`+`update` (derived CRUD) |
| `Page.binds` (`mode: custom`)                                              | sesuai deklarasi                               |

Anonim **tidak pernah** otomatis mendapat `delete`.

### Scope

`table.param` / `form` filter yang nilainya route param (`":name"`) menjadi
scope server-side:

```
scope: [{field: guest_token, op: eq, from: route, param: guest_token}]
```

Invariant lama (scope tidak boleh digabung `find`) tetap ditegakkan — tapi
sekarang sebagai aturan **derivasi**: kalau entity punya scope, `find` tidak
diberikan (find resolve by id, scope tak bisa menjaganya).

### Gate `public`

View dengan `public: false` (atau Form/Table `public: false`) tidak menyumbang
grant. Ini yang menahan derivasi agar tidak membuka halaman staf yang
kebetulan berada di App publik.

### Invarian yang harus dijaga

- Hanya App `access: public` yang punya grant. App private → kosong.
- Module framework (`formspec.core`) tidak disumbang derivasi (hanya module
  yang di-mount eksplisit).
- Grant tetap di-UNION lintas App publik se-workspace pada registrasi route
  (perilaku lama, tidak diubah di plan ini).

## Langkah

### Fase 1 — Shared reachability (blokir semua)

1. Ekstrak perhitungan menu ∪ `registered_views` dari `BuildBundle`
   (`internal/ui/meta.go` ~672-720) menjadi `computeAppSurfaceLocked` di
   `internal/ui/surface.go`, lalu `BuildBundle` memakainya.

### Fase 2 — Derivasi

2. `(*Registry).DerivePublicGrants(entities EntityLister, in PublicGrantInput) []spec.PublicEntityDecl`
   di `internal/ui/surface.go` (dep: 1).
3. Scope derivation dari `param` route + aturan find/scope.

### Fase 3 — Konsumen

4. `internal/api/meta.go`: isi `AppContext.PublicEntities` dari derivasi.
5. `internal/api/router.go`: `publicGrants()` dari derivasi.

### Fase 4 — Hapus dari kontrak

6. Hapus `AppSpec.PublicEntities` + blok validasinya di `pkg/spec/resources.go`.
7. `make generate-schema` (WAJIB — gate `TestGeneratedSchemas_MatchOnDisk`).
8. Hapus blok `public_entities` di `examples/kafe/spec/apps/kafe-qr.yaml`;
   perbarui komentar di `order/entity.yaml`, `order-status-page.yaml`,
   `order-form-qr.yaml`.
9. Perbarui test: `pkg/spec/public_entities_test.go`,
   `internal/api/public_entities_test.go`, `internal/ui/menu_filter_test.go`,
   `cmd/formspec/kafe_row_scope_test.go`, `internal/manifest/loader.go`.
10. Docs: `docs/kind/curation/App.md`, `docs/spec/frontend/05-app-kinds.md` §1.1,
    `docs/runtimes/06-ui-rest-contract.md`.

### Fase 5 — Verifikasi

11. `go test ./...`, `make lint`, `formspec validate --schema schemas`.
12. E2E anonim kafe-qr: `list menu-item` 200; `list order` tanpa token 403;
    `list dining-table` 401; `create order` 201; picker + chip kategori tetap
    jalan TANPA grant `menu-category`.

## Sisa yang dicatat (⏸️)

- **kafe 10.76 ⏸️ — `menu-item-price` kehilangan scope `branch_id`.** Manifest
  lama mendeklarasikan `{field: branch_id, from: route}`; turunan tidak bisa
  mereproduksinya karena `picker.display.price_filter` bernilai
  `{session.branch_id}` (cabang dari record sesi, bukan parameter route pada
  entity harga), dan `from: session` ditolak untuk permukaan publik. Akibatnya
  anonim membaca harga **semua** cabang. Tanpa PII, tetapi pelemahan nyata.
  Effort: medium. Bukti: `TestDerivePublicGrants_KafeQR` mengunci
  `menu-item-price = [list]` tanpa scope.
- `BlockRef.needs` belum ada di Go schema meskipun docs menandainya "Open" dan
  todo 5.9.6 ✅ — derivasi untuk component block ber-asset terbatas pada
  `Page.binds` (yang sudah dibaca).
- `formspec check` tidak mencetak grant hasil derivasi (keputusan pemilik);
  satu-satunya cara melihat grant efektif adalah menjalankan server atau
  membaca test.

## Referensi spec

- `docs/spec/frontend/05-app-kinds.md` §1.1 (allowlist anonim) — **berubah**:
  field dihapus, perilaku diturunkan.
- `docs/spec/frontend/05-app-kinds.md` §1.2 (`registered_views`).
- `docs_internal/plan/registered-views.md` (permukaan App).
