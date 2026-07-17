# Renderer

**Version:** 0.1.0 · **Status:** Draft

> Draft: isi di bawah kontrak yang berlaku. Bagian yang masih terbuka ditandai
> eksplisit sebagai **Open**.

## 1. Peran
Implementasi konkret sebuah `VisualSpecKind`
([`02-visual-spec-kind.md`](02-visual-spec-kind.md)). Satu VisualSpecKind bisa
punya banyak Renderer — filosofi UX berbeda, stack berbeda:

```yaml
apiVersion: forma/v1
kind: Renderer
metadata:
  name: kanban-vue-community
spec:
  implements: kanban
  stack_family: vue
  trust_tier: community
```

Siapa pun bisa: (a) menulis Renderer baru untuk VisualSpecKind yang sudah ada
(mis. Kanban versi Vue dengan filosofi UX berbeda dari Kanban resmi shadcn),
atau (b) mendefinisikan `VisualSpecKind` sama sekali baru plus Renderer
resminya (lihat [`02-visual-spec-kind.md`](02-visual-spec-kind.md) §6).

## 2. Field
| Field | Wajib | Arti |
|---|---|---|
| `implements` | Ya | Nama `VisualSpecKind` yang diimplementasikan. Harus merujuk `VisualSpecKind` yang sudah terdaftar. |
| `stack_family` | Ya | Kecocokan shell (mis. `react-shadcn`, `vue`, `flutter`) — dipakai untuk validasi kompatibilitas App shell + Page shell-integrated + Component, lihat [`01-visual-hierarchy.md`](01-visual-hierarchy.md) §3. Bukan enum tertutup: nilai baru muncul begitu Shell/stack baru ada. |
| `trust_tier` | Ya | `official \| verified \| community` — seragam dengan trust tier Module Registry ([`../platform/07-marketplace.md`](../platform/07-marketplace.md)). |

## 3. Registry & Resolusi
Engine memilih Renderer untuk sebuah instance VisualSpecKind lewat pasangan
`(implements, stack_family)` — App/Page/Component instance memakai Renderer
resmi (`trust_tier: official`) untuk `stack_family` Shell-nya secara default.

**Resolusi & override (normatif).** Aturannya:

1. **Hanya Renderer `official` yang terpilih otomatis.** Kalau ada Renderer
   resmi untuk `(implements, stack_family)`, ia dipakai tanpa deklarasi apa
   pun.
2. **Tanpa Renderer resmi → wajib eksplisit, bukan fallback diam-diam.**
   `forma apply` **error** bila sebuah instance membutuhkan
   `(implements, stack_family)` yang tidak punya Renderer resmi dan tidak
   ada pilihan eksplisit. Pesan errornya wajib **menyarankan kandidat**
   ber-tier `verified` lalu `community` yang terdaftar untuk pasangan itu —
   developer memilih secara eksplisit, tooling tidak pernah memilihkan
   Renderer non-official.
3. **Override eksplisit** dideklarasikan di manifest App — pemetaan
   `implements → renderer` (mis. `renderers: {kanban: community/super-kanban}`)
   berlaku untuk seluruh instance kind itu di App tersebut; instance
   individual boleh menimpa lewat field `renderer:` di manifest-nya sendiri.
   Renderer non-official yang dipilih ikut muncul di consent footprint App.

## 4. Renderer = Interpreter Runtime
Renderer komunitas pun interpreter runtime yang mengonsumsi Spec Resolution
API ([`04-spec-resolution-api.md`](04-spec-resolution-api.md)) — bukan build
step per kombinasi spec+renderer. Konsekuensinya: menambah Renderer baru tidak
mengubah cara instance existing di-deploy, hanya menambah pilihan interpreter
yang tersedia untuk `(implements, stack_family)` itu.

**Renderer sebagai library yang bisa di-embed (arah embed terbalik, opsional).**
Selain Forma merender pohon App/Page/Component-nya sendiri, sebuah Renderer
**boleh** juga mengekspos adapter embeddable — mis. component bergaya
`<FormaPage name="..." />` — yang memungkinkan aplikasi **host** (React/Vue/dll
yang **bukan** dibangun di atas Forma) menyisipkan satu layar hasil render Forma
ke dalam dirinya. Ini kebalikan arah embed yang biasa — Forma menyisipkan
component custom lewat escape hatch
([`07-component-kinds.md`](07-component-kinds.md) §4). Kapabilitas ini
**opsional**: sebuah Renderer boleh menawarkannya, tapi bukan syarat konformansi
(§5).

## 5. Konformansi
Renderer wajib memenuhi `renderer_contract` yang dideklarasikan
`VisualSpecKind`-nya (lihat [`02-visual-spec-kind.md`](02-visual-spec-kind.md)
§1). Mekanisme verifikasinya **berjenjang mengikuti trust tier**:

- **Validasi statis skema** (deklarasi `implements`/`stack_family`/kontrak
  lengkap) adalah syarat minimum pendaftaran untuk semua tier, termasuk
  `community`.
- **Test-suite konformansi** — fixture yang didefinisikan bersama
  `renderer_contract` masing-masing VisualSpecKind (pola serupa *Extended
  Conformance* Module) — **wajib lulus untuk tier `verified` dan `official`**.
  Renderer `community` tidak diwajibkan lulus suite, dan justru karena itu
  tidak pernah terpilih otomatis (§3): pemakainya menanggung risikonya secara
  eksplisit.
