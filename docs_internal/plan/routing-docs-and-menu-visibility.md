# Plan — Routing: dokumen otoritatif + fix `ResolveViewRoute` + visibilitas menu

**Tanggal**: 2026-09-25 · **Status**: In progress
**Referensi**: `docs/spec/frontend/04-spec-resolution-api.md`,
`docs/spec/frontend/06-page-kinds.md` §1–§2, `docs/spec/frontend/08-formspec-expr.md`
§2–§4, `docs/spec/platform/02-workspace-app-module.md` §4,
`docs/renderers/shadcn-shell/01-architecture.md` §3–§4
**Todo**: 5.22.1–5.22.5, 5.11.9, 5.12.9
**Changelog**: `2026-09-25-001..005` (lihat §7)

## 1. Latar

Pertanyaan yang memicu pekerjaan ini: _"route `/cafe-master/promos` dihandle
entity `promo`; karena ada `promo-form`, apakah form itu yang dipakai?"_ — dan
lanjutannya _"kalau ada lebih dari 1 Form, apakah router bisa memilih?"_.

Jawabannya tersebar di empat berkas tanpa satu pun dokumen yang merangkumnya,
dan saat dirunut ketemu **empat cacat nyata** (bukan sekadar dokumentasi
kurang). Karena itu plan ini mencakup fix kode + dokumen + test, bukan hanya
tulisan.

## 2. Temuan (terverifikasi, dengan bukti)

### 2.1 Empat jenis route, dan pemilihan Form tidak dilakukan router

| Jalur | Dibuat oleh                | Form dipilih oleh                                     |
| ----- | -------------------------- | ----------------------------------------------------- |
| A     | `kind: Page` (manifest)    | `block.form.ref` → `resolveForm(explicitRef)`         |
| B     | server: `makeDerivedPage`  | name → hanya `formRef`; **mode selalu `view`**        |
| C     | klien: `buildRoutes` §2    | konvensi `resolveForm`: `{entity}-create/-edit/-form` |
| D     | overlay (`?action=&form=`) | `authoredForm` = `forms.find()` pertama (alfabetis)   |

Bukti kafe: entity `order` punya **dua** Form (`order-form-pos` edit/drawer +
`order-form-qr` create/separate_page). Klik "New" di `/cafe-order/orders`
memakai `order-form-pos` (menang alfabetis), sedangkan `/cafe-order/orders/new`
**menderivasi** form (tidak ada `order-create`/`-edit`/`-form`).

### 2.2 `ResolveViewRoute` kehilangan `Listing` → validate hijau, App gagal

`cmd/formspec/validate_dangling.go` `viewKinds` **mengizinkan** `Listing`, dan
`navigationPrefix` (`internal/ui/meta.go`) + `buildRoutes` (router SPA) punya
`/listing/<name>` — tetapi `ResolveViewRoute` tidak punya cabangnya. Akibatnya
`formspec validate` hijau, lalu `app.Resolve` → `"view not found"` → **App tidak
resolve sama sekali**. Contoh `examples/storefront` lolos hanya karena memakai
escape hatch `route: /listing/product-catalog`.

`ResolveViewRoute` adalah **empat salinan konvensi yang sama** (bersama
`buildRoutes`, `routeExists`/`navigationPrefix`, `viewKinds`) — inilah sumber
kelas bug ini.

### 2.3 `MenuItem.When` tidak dievaluasi **dan** tidak divalidasi

- Field ada di `pkg/spec/resources.go` + schema, tidak dibaca kode mana pun.
- `formspec check` tidak memeriksa grammar-nya, padahal `08-formspec-expr.md` §4
  menyebut gate deploy-time itu "wajib".
- Satu-satunya contoh nyata, `examples/Clinic-UI-Showcase/.../clinic/module.yaml`:
  `when: "user.has('clinic.settings.update')"` — **tidak bisa dievaluasi**:
  `evalCall` klien hanya `len/sum/amount/currency`; `user.has(…)` menghasilkan
  AST `CallExpr` dengan callee `MemberExpr` sehingga `node.callee.name`
  `undefined` → warning runtime "unknown function: undefined". `validateExprGrammar`
  (pemindai karakter) meloloskannya.
- `08-formspec-expr.md` §3 melarang identitas/permission di FormSpecExpr — jadi
  contoh itu juga **melanggar kontrak**, bukan sekadar belum jalan.

### 2.4 `MenuItem.Permissions` diperiksa klien tetapi tidak ada di spec

`filterMenuItem` (`hooks/useResolvedMenu.ts`) memeriksa `item.permissions`, dan
`internal/api/meta.go` menyebut "menu.permissions" — tetapi field-nya tidak ada
di `pkg/spec` maupun `types/manifest.ts`. Satu-satunya cek RBAC menu hari ini
adalah cek mati di klien (bisa di-bypass, dan memang tidak pernah menerima data).

## 3. Keputusan (disetujui pengguna)

1. **`Listing` ditambahkan ke kode**, bukan dicabut dari `viewKinds` — tiga lapis
   lain sudah mendukung.
2. **Dua sumbu dipisah**: `permissions:` = RBAC (server), `when:` = kondisi
   bisnis (klien). Jangan tambah `user.has()` — dua jalan untuk satu hal.
3. **`permissions` disaring server** (`filterMenu`, sudah punya `can`), **`when`
   dievaluasi klien** (`filterMenuItem`, punya `me`). Alasan teknis: bundle
   `/_meta/ui` di-ETag-cache (`sha256` + `If-None-Match`); `when` yang bergantung
   waktu di server akan membuat hash berubah terus.
4. **Gate callable = himpunan tertutup** `{len, sum, amount, currency, today}`.
   `today()` ditambahkan ke evaluator TS + didokumentasikan (§2 spec) agar paritas
   dengan Starlark server dan `when` punya dimensi waktu. `days_ago`/`empty`
   tetap builtin guard server-only.
5. **Gate callable TIDAK diperluas ke `guard: {expression}` state-machine** — itu
   Starlark sungguhan (`internal/starlark.EvaluateGuard`) dengan builtin tambahan
   (`sum_line`), kontrak berbeda.
6. **Fail-open saat `when` gagal evaluasi** (item ditampilkan + error dilaporkan):
   `when` bukan gerbang otorisasi, jadi menyembunyikan navigasi karena bug
   renderer lebih buruk daripada menampilkan tautan.

## 4. Sembilan lapis yang menahan bypass

Menghindari salah paham "`when` di klien = celah":

| #   | Lapis                                        | Sisi   | Bypass-able |
| --- | -------------------------------------------- | ------ | ----------- |
| 1   | Entity visibility (`BuildBundle`)            | server | ❌          |
| 2   | `filterMenu` (route harus ada di bundle)     | server | ❌          |
| 3   | `MenuItem.permissions`                       | server | ❌          |
| 4   | `required_permission` per action + row scope | server | ❌          |
| 5   | `when`                                       | klien  | ✅ memang   |

Lapis 1–2 sudah berlaku; 3 ditambahkan plan ini. Yang **wajib** ditulis di
dokumen: `when` bukan gerbang otorisasi — supaya pola salah seperti contoh
`clinic` tidak terulang.

## 5. File yang dibuat/diubah

### Buat

| File                                                      | Isi                                       |
| --------------------------------------------------------- | ----------------------------------------- |
| `docs/renderers/shadcn-shell/05-routing.md`               | Dokumen otoritatif (deliverable utama)    |
| `internal/ui/registry_test.go`                            | Tabel `ResolveViewRoute` (belum ada test) |
| `renderers/react-shadcn/src/shell/router.shapes.test.tsx` | Kunci satu bentuk path per jalur A/B/C/D  |

### Ubah — kode

| File                                                   | Perubahan                                                                                |
| ------------------------------------------------------ | ---------------------------------------------------------------------------------------- |
| `pkg/spec/resources.go`                                | `MenuItem.Permissions []string` + komentar dua sumbu                                     |
| `internal/ui/registry.go`                              | Cabang `Listings` + doc comment empat-salinan                                            |
| `internal/ui/meta.go`                                  | `filterMenu(can)` menyaring `permissions`; `BuildBundle` meneruskan `can`                |
| `cmd/formspec/check.go`                                | `validateExprGrammar`: himpunan tertutup callable; `checkMenuExpr` untuk `MenuItem.When` |
| `cmd/formspec/validate_dangling.go`                    | Paritas pesan `viewKinds` dengan `ResolveViewRoute`                                      |
| `renderers/react-shadcn/src/types/manifest.ts`         | `MenuItem.permissions`                                                                   |
| `renderers/react-shadcn/src/hooks/useResolvedMenu.ts`  | Buang cek `permissions`; tambah eval `when`                                              |
| `renderers/react-shadcn/src/lib/formspec-expr/eval.ts` | `today()` di `evalCall`                                                                  |

### Ubah — spec & contoh

| File                                                 | Perubahan                                                                                  |
| ---------------------------------------------------- | ------------------------------------------------------------------------------------------ |
| `docs/spec/platform/02-workspace-app-module.md` §4   | Hapus "Form/Table bukan target `view`"; tambah `Listing`, `permissions`, `when`            |
| `docs/spec/frontend/08-formspec-expr.md` §2–§3       | `today()`; pertegas cakupan larangan identitas (`when` menu = pengecualian terdokumentasi) |
| `docs/kind/curation/App.md`                          | Kontradiksi "bukan Form/Table" vs "ALL visual kinds"                                       |
| `examples/Clinic-UI-Showcase/.../clinic/module.yaml` | `when: user.has(…)` → `permissions: [clinic.settings.update]`                              |
| `ai_skills/formspec-kinds/SKILL.md`                  | Blok `App.spec.menu`                                                                       |

### Dibuat ulang (generated)

`make generate-schema` → `schemas/kinds/{App,Module}.schema.json` ·
`make generate-kind-docs` → `docs/kind/curation/{App,Module}.md`

## 6. Dependensi & urutan

```
§5 kode (permissions field, Listings branch, gate callable, today())
   ├── filterMenu(can)  → needs permissions field
   ├── useResolvedMenu  → needs permissions field
   ├── migrasi clinic   → needs gate callable (supaya terbukti gagal)
   ├── generate-schema  → needs permissions field
   └── 05-routing.md    → needs semua keputusan di atas (dokumen mencatat perilaku final)
test                           → dapat paralel, setelah kode masing-masing
changelog + todo               → terakhir
```

Effort keseluruhan: **medium** — tidak ada algoritme baru; risikonya di
konsistensi empat salinan konvensi route dan di gate yang bisa mematahkan
manifest contoh (mitigasi: loop validate seluruh `examples/`).

## 7. Rencana changelog (satu entry per topik)

| #   | Topik                                                         |
| --- | ------------------------------------------------------------- |
| 001 | Cabang `Listings` di `ResolveViewRoute` + `registry_test.go`  |
| 002 | `MenuItem.Permissions` + penyaringan server + hapus cek klien |
| 003 | `MenuItem.When` dievaluasi klien + gate callable + `today()`  |
| 004 | Migrasi contoh clinic + perbaikan spec/kind-docs              |
| 005 | `docs/renderers/shadcn-shell/05-routing.md` + pendaftaran     |

## 8. Verifikasi

1. `go test ./internal/ui/... ./cmd/formspec/... ./internal/app/...` — hijau.
2. **Regresi disengaja**: hapus cabang `Listings` → `registry_test.go` harus gagal.
3. `go build ./... && go test ./...`.
4. **Loop validate semua contoh** (`for d in examples/*/spec; do go run ./cmd/formspec validate --spec $d; done`) — gate callable tidak boleh mematahkan satu pun.
5. Uji negatif gate: `when: "user.has('x')"` → **error**; `compute: "leng(x)"` → **error**; keduanya hijau setelah diperbaiki.
6. `make generate-schema && make generate-kind-docs` → diff hanya `MenuItem`.
7. `npx vitest run` (suite penuh) + 2 file test baru.
8. `cd docs-site && npm run build` — tanpa dead link.
9. Browser `/kafe/app/pos/cafe-master/promos` (rebuild SPA): klik New → URL memuat `?action=create&form=promo-form&mode=drawer`; uji `when` sementara (`today() > '2000-01-01'` tampil, `<` hilang).

## 9. Sisa yang sengaja tidak ditutup (→ item todo)

1. `routeExists` memvalidasi `/M/form/<n>` dari **keberadaan Form**, sedangkan SPA
   mendaftar dari `bundle.pages` → item menu menuju derived Page yang disuppress
   masih 404.
2. Surface `_admin` memakai `alwaysVisible` + menu `deriveMenuItems` di klien →
   `permissions`/`when` tidak berlaku di sana (kasar, bukan bocor).
3. Paritas evaluator Go↔TS tanpa fixture bersama (sudah 5.11.7 — rujuk, jangan
   duplikat).
4. `docs/renderers/shadcn-shell/01-architecture.md` §5 memuat klaim stale
   (`OverlayHost` tidak terhubung, `deriveMenuItems` mati, `TableRenderer`
   hardcode `/_admin`) — semuanya sudah tidak benar (todo 5.15.2, 5.14.2, 5.12.x).
