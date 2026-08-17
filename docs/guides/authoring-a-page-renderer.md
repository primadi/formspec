# Menulis Page Renderer

**Status:** Draft

> Panduan praktis — bukan kontrak. Definisi normatif ada di
> `docs/spec/frontend/`, dirujuk di tiap langkah di bawah.

## 1. Prasyarat
Baca berurutan sebelum mulai menulis kode:
1. [`../spec/frontend/01-visual-hierarchy.md`](../spec/frontend/01-visual-hierarchy.md)
   — hirarki Shell → App renderer → Page renderer → Component renderer, dan
   aturan `stack_family` (App shell + Page shell-integrated + Component
   wajib satu stack).
2. [`../spec/frontend/02-visual-spec-kind.md`](../spec/frontend/02-visual-spec-kind.md)
   — `VisualSpecKind`: field `tier`, skema instance shell-agnostic, slot
   system.
3. [`../spec/frontend/03-renderer-kind.md`](../spec/frontend/03-renderer-kind.md)
   — kind `Renderer`: `implements`, `stack_family`, `trust_tier`.
4. [`../spec/frontend/04-spec-resolution-api.md`](../spec/frontend/04-spec-resolution-api.md)
   — endpoint yang akan dikonsumsi renderer-mu saat runtime.

## 2. Memilih Jalur
Dua jalan berbeda, pilih salah satu sebelum menulis apa pun:

- **Renderer baru untuk VisualSpecKind yang sudah ada** (mis. Kanban versi
  Vue dengan filosofi UX berbeda dari Kanban resmi shadcn) — kamu cuma
  perlu menulis Renderer, skema instance dan `renderer_contract` sudah ada.
- **VisualSpecKind sama sekali baru** (mis. `seat-map-booking`) — kamu
  mendeklarasikan `VisualSpecKind` (skema + `renderer_contract`) **dan**
  Renderer resminya sekaligus
  ([`../spec/frontend/02-visual-spec-kind.md`](../spec/frontend/02-visual-spec-kind.md)
  §6).

Kalau ragu: 95% kebutuhan sudah tercakup katalog kind yang ada
([`../spec/frontend/06-page-kinds.md`](../spec/frontend/06-page-kinds.md),
[`07-component-kinds.md`](../spec/frontend/07-component-kinds.md)) — jalan
pertama biasanya yang tepat.

## 3. Memenuhi `renderer_contract`
`renderer_contract` di `VisualSpecKind` (§ contoh Kanban,
[`02-visual-spec-kind.md`](../spec/frontend/02-visual-spec-kind.md) §1)
adalah interface yang wajib dipenuhi Renderer manapun — props yang wajib
diterima dari instance spec, dan event/callback yang wajib diemisikan balik
(mis. Kanban wajib mengemisikan perubahan `status_field` saat drag-drop,
lihat contoh perilaku di
[`../spec/frontend/06-page-kinds.md`](../spec/frontend/06-page-kinds.md) §6
Wizard dan §4 Kanban untuk pola serupa).

Renderer-mu **wajib**:
- Membaca instance spec lewat Spec Resolution API
  ([`04-spec-resolution-api.md`](../spec/frontend/04-spec-resolution-api.md)
  §2), bukan build-time — kamu menulis interpreter, bukan generator kode
  (§4 di bawah).
- Menghormati filtering permission yang sudah dilakukan server
  ([`04-spec-resolution-api.md`](../spec/frontend/04-spec-resolution-api.md)
  §4) — kalau sebuah action tidak dikirim/permission-nya tidak dimiliki
  caller, sembunyikan/nonaktifkan kontrol terkait; jangan menganggap
  "kalau dikirim berarti boleh dipanggil" sebagai satu-satunya guard (server
  tetap menolak di resource level).
- Mengevaluasi `visible_when`/`readonly_when`/`required_when`/`compute`
  lewat interpreter FormSpecExpr
  ([`08-formspec-expr.md`](../spec/frontend/08-formspec-expr.md)) — bukan
  mengimplementasikan ulang grammar-nya sendiri; hasil evaluasi harus
  identik dengan shell resmi untuk ekspresi yang sama.

## 4. Renderer = Interpreter Runtime, Bukan Codegen
Konsekuensi dari [`03-renderer-kind.md`](../spec/frontend/03-renderer-kind.md)
§4: Renderer komunitas pun interpreter runtime yang mengonsumsi Spec
Resolution API — bukan build step per kombinasi spec+renderer. Kalau
rancanganmu perlu "compile" instance spec jadi kode per-app, itu tanda
desainnya belum tepat — mundur ke §3, cek ulang apakah renderer-mu benar
membaca spec saat runtime.

## 5. Registrasi dan Distribusi
```yaml
apiVersion: formspec/v1
kind: Renderer
metadata:
  name: kanban-vue-community
spec:
  implements: kanban
  stack_family: vue
  trust_tier: community
```

`stack_family`-mu menentukan kompatibilitas — App shell + Page
shell-integrated + Component wajib satu `stack_family` yang sama
([`01-visual-hierarchy.md`](../spec/frontend/01-visual-hierarchy.md) §3).
Kalau Renderer-mu untuk Page yang benar-benar lepas dari App manapun (bukan
shell-integrated), tidak perlu Renderer kind sama sekali — cukup konsumsi
API generik.

`trust_tier` seragam dengan Module Registry
([`../spec/platform/07-marketplace.md`](../spec/platform/07-marketplace.md)
§2) — `community` untuk mulai, naik ke `verified`/`official` lewat proses
review yang sama dengan Module.

**Open — mekanisme konformansi** ([`03-renderer-kind.md`](../spec/frontend/03-renderer-kind.md)
§5): validasi statis skema vs test-suite konformansi belum diputuskan.
Sampai itu final, uji Renderer-mu manual terhadap contoh instance di katalog
kind ([`06-page-kinds.md`](../spec/frontend/06-page-kinds.md),
[`07-component-kinds.md`](../spec/frontend/07-component-kinds.md)) dan
dokumentasikan gap-nya secara eksplisit di listing marketplace-mu.

## 6. Contoh End-to-End (ringkas)
Kanban versi minimal untuk stack baru (`stack_family: svelte`):
1. Baca `renderer_contract` milik `VisualSpecKind: kanban` lewat
   `/_meta/entities` atau dokumentasi VisualSpecKind-nya.
2. Fetch instance Kanban lewat `/_meta/ui` (bundle) — dapat `columns`,
   `card`, `status_field`, dan daftar permission caller.
3. Render kolom dari `columns[]`; render kartu dari field `card.*`; drag
   antar kolom memanggil `update` action pada `status_field` lewat REST API
   ([`../spec/backend/01-core-basic.md`](../spec/backend/01-core-basic.md)
   §8), bukan mutasi lokal yang tidak tersinkron ke server.
4. Daftar sebagai `kind: Renderer` (§5), publish dengan `trust_tier:
   community` ke marketplace.
