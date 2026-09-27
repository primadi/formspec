# Tombol transisi tidak pernah muncul + kliknya 404 (`description` di transition)

**Plan:** `docs_internal/plan/via-sebagai-action-penuh.md` (L1 + D2)
**Ledger:** kafe **10.49 ✅**, **10.50 ✅** · sisa → 10.47 ⏸️, 10.46 ⏸️, 10.48 ⏸️

## Dua defect yang saling menutupi

Keduanya ditemukan saat memverifikasi di browser, bukan dari membaca kode.

### 10.49 ✅ — `authorized_actions` tidak memuat action kustom → **tombol tak pernah muncul**

`authorizedActions` (`internal/ui/meta.go`) hanya menambahkan CRUD standar +
soft-deactivate. Comment-nya mengklaim "renderer mengambil permission kustom dari
`ActionSummary.Permission`" — **tidak benar**: `canDoEntityAction` memeriksa
`authorized_actions.includes(action)` **lebih dulu**, dan hanya jatuh ke
`me.permissions` bila field itu **absen**. Karena field-nya ada tapi tidak memuat
action kustom, fallback tidak pernah dipakai → **setiap tombol transisi kustom
disembunyikan** `DetailPage` sebelum cek izin di klik sempat jalan.

**Terukur sebelum fix:** kasir `authorized_actions = [list, find, update]` —
tanpa `release`/`reserve`; blok "Actions" **tidak ada** di DetailPage, padahal
server menerima `PATCH` yang sama (**200**).

**Fix:** ikutkan action yang sudah diselesaikan di `schema.Actions`
(`authorizedActions` kini menerima `action` + `perm` per entri, plus dedup).
**Terukur sesudah:** kasir `[list, find, update, release, reserve]`, pelayan
`[list, find, update, mark-table-served]`, manajer
`[..., mark-not-available, mark-available]`; blok Actions muncul.
**Test pengunci:** 2 subtest baru di `internal/ui/authorized_actions_test.go`,
**dibuktikan gagal** saat blok `schema.Actions` dibuang.

### 10.50 ✅ — tombol transisi `POST` ke route yang tidak ada → **klik selalu 404**

`DetailPage.handleTransition` memanggil `POST /{entity}/{id}/{action}`. Route itu
hanya dibuat untuk action **ber-`impl`** (`internal/api/generator.go` melewati
`Impl == nil`). Transisi yang hanya menamai `via` tidak punya route — jalur yang
menerapkannya adalah `PATCH` (dicocokkan per **(from, to)**).
**Terukur:** klik tombol "Reserve" → **404 `no such file field or action: reserve`**.

**Fix (frontend, mengikuti keputusan C):** coba route action; bila **404**,
fallback ke `PATCH {status_field: target_state}` memakai `to` dari bundle.
**Terukur sesudah:** klik → toast "State updated", badge `Available → Reserved`,
`Version` 6 → 7.

**Jebakan yang sempat menipu:** percobaan pertama memakai
`err instanceof HTTPError` → **selalu false**, karena `afterResponse` di
`lib/api/authHooks.ts` sudah mengonversi kegagalan menjadi **`FormaApiError`**
sebelum ky sempat membungkusnya. Helper `errorStatus(err)` menangani **kedua**
bentuk. (Pola yang sudah ada di repo: `lib/api/errors.ts`.)

## `description` di transition (keputusan 2) — L1

Field baru `TransitionDecl.Description` (`pkg/spec/entity.go`), dan **wajib**
ditambahkan di struct lokal `UnmarshalYAML` — kalau tidak, nilai **dibuang
diam-diam saat load** (validate hijau membaca file, runtime kosong). Test
pengunci `TestTransitionDecl_UnmarshalKeepsDescription` **dibuktikan gagal** saat
field dihapus dari struct lokal.

Dipakai sebagai label tombol bila `ui.button_label` tidak ada
(`engine/lifecycle.ts`), dan disalin ke `TransitionDecl` di TypeScript.
**Terukur:** tombol kasir kini berlabel **"Tandai meja dipesan"** (dari
`description`), bukan "Reserve" (dari nama `via`).

Sekaligus menutup **keputusan C** di klien: `getAvailableTransitions` memfilter
transisi ber-`via` kosong **secara eksplisit** — sebelumnya tersembunyi karena
kebetulan `canDoEntityAction("")` bernilai false, yang juga menyisakan React key
ganda `""`.

## File

`internal/ui/meta.go` · `internal/ui/authorized_actions_test.go` ·
`pkg/spec/entity.go` · `pkg/spec/transition_permission_test.go` ·
`examples/kafe/.../dining-table/entity.yaml` (9 transisi + `description`) ·
`renderers/react-shadcn/src/{engine/lifecycle.ts,types/manifest.ts,kinds/page/DetailPage.tsx}` ·
`schemas/**` + `docs/kind/**` (regenerasi).

## Bukti

Kafe `validate` 85 manifest 0 problem · `gofmt` bersih · `go build ./...` ·
**`go test ./...` hijau** · **vitest 529 lulus / 39 file** · `tsc -b` bersih.
Browser: tombol muncul, berlabel `description`, klik → status berubah.

## Sisa

- **10.48 ⏸️** — transisi ber-`via` **tanpa route**: klien kini menanganinya
  dengan coba-POST lalu fallback PATCH. Itu bekerja, tetapi **satu round-trip
  terbuang** untuk setiap transisi tanpa `impl`, dan 404-nya tampil di konsol.
  Penutup yang benar: bundle menyatakan kemampuan itu (mis. flag `has_route` per
  transition/action) sehingga klien tidak perlu menebak.
- **10.47 ⏸️** tetap: nama permission lebih baik diturunkan dari `via` agar tidak
  perlu diselaraskan manual.
- 10.46 ⏸️ (gate tidak ditegakkan di jalur script), 10.42 ⏸️ (backfill enum),
  10.40b ⏸️ (pemicu otomatis saat bayar), 10.41 ⏸️ (agregat "semua disajikan").
