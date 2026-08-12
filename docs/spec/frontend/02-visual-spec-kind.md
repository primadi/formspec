# VisualSpecKind

**Version:** 0.1.0 · **Status:** Draft

> Draft: isi di bawah kontrak yang berlaku. Bagian yang masih terbuka ditandai
> eksplisit sebagai **Open**.

## 1. Peran
Meta-kind untuk mendeklarasikan *jenis view baru* tanpa mengubah framework
inti: skema instance + kontrak yang wajib dipenuhi renderernya. Precedent-nya
`MockupModule` kind — pola meta-kind yang sama, diterapkan ke lapisan visual.

```yaml
apiVersion: formspec/v1
kind: VisualSpecKind
metadata:
  name: kanban
spec:
  tier: page                 # WAJIB — app | page | component
  schema: {...}              # field wajib di instance spec (mis. columns, card_fields)
  renderer_contract: {...}   # interface yang WAJIB dipenuhi renderer manapun
```

Instance yang ditulis app developer tetap seperti Form kind sekarang — cuma
`kind: Kanban` alih-alih `kind: Form`.

## 2. Field `tier` (wajib)
`app | page | component` — menentukan di mana kind ini boleh dipakai/
dikomposisi:
- Tanpa `tier` eksplisit, `formspec apply` tidak tahu apakah `kind: Kanban` boleh
  dipasang langsung sebagai isi route App (perlu `tier: page`), atau cuma boleh
  mengisi slot milik Page lain (`tier: component`).
- `tier` juga jadi dasar validasi slot compatibility (§4): `accepts_slots`
  hanya sah dideklarasikan `VisualSpecKind` bertier `page` (atau `app`, bila
  App-level slot dibutuhkan); `implements_slot` hanya sah dari `tier:
  component`.
- Shell **tidak** termasuk nilai `tier` — Shell bukan sesuatu yang
  dideklarasikan lewat VisualSpecKind sama sekali; ia wadah yang menghosting
  App/Page/Component renderer via `Renderer.stack_family`
  ([`03-renderer-kind.md`](03-renderer-kind.md), lihat juga
  [`01-visual-hierarchy.md`](01-visual-hierarchy.md) §2).

## 3. Skema Instance — Shell-Agnostic
`spec.schema` adalah kontrak instance yang ditulis app developer — **satu
definisi dipakai semua Shell** tanpa ditulis ulang (mis. Kanban yang sama
dipakai baik oleh Shell shadcn/React maupun Shell Flutter, hanya dengan
Renderer berbeda per Shell). Ini yang membuat "write once" mungkin: developer
menulis kontrak sekali, dapat web app dan mobile app dari spec yang sama —
lihat [`01-visual-hierarchy.md`](01-visual-hierarchy.md) §5.

## 4. Slot System
Perluasan `VisualSpecKind` untuk pola "Page tertentu menerima Component
tertentu di posisi tertentu" (mis. Dashboard menerima Widget) — dideklarasikan
sebagai **slot**: lubang dengan kontrak data-shape, bukan referensi ke
komponen bernama spesifik.

```yaml
apiVersion: formspec/v1
kind: VisualSpecKind
metadata:
  name: dashboard
spec:
  tier: page
  schema: {...}
  accepts_slots:
    - name: widget
      contract:
        required_props: [title, data_binding, size_unit]
        optional_props: [refresh_interval_sec]
```

```yaml
apiVersion: formspec/v1
kind: VisualSpecKind
metadata:
  name: kpi-widget
spec:
  tier: component            # WAJIB component — formspec apply menolak implements_slot dari tier lain
  schema: {...}
  implements_slot: widget
```

Instance spec aplikasi reference widget mana yang dipasang di posisi mana:

```yaml
kind: Dashboard
spec:
  layout:
    - slot: widget
      use: kpi-widget
      position: { row: 0, col: 0, w: 2, h: 1 }
      data_binding: sales-summary
```

**Batasan v1:**
1. Slot filling hanya valid dalam satu Shell yang sama (mengisi slot = berbagi
   render tree, aturan yang sama dengan `stack_family` —
   [`01-visual-hierarchy.md`](01-visual-hierarchy.md) §3).
2. Kontrak slot adalah data-shape, bukan visual — komunitas tetap bebas
   berkreasi secara visual selama kontrak data dipenuhi.
3. Kedalaman rekursi dibatasi satu level (Page menerima Component;
   Component-di-dalam-Component didefer sampai ada use case mendesak).

## 5. Validasi `formspec apply`
`formspec apply` memvalidasi `tier` sebelum menerima slot binding:
`accepts_slots` hanya sah dari `tier: page` (atau `app`); `implements_slot`
hanya sah dari `tier: component`. Kombinasi lain (mis. Page mendeklarasikan
`implements_slot`, atau Component mendeklarasikan `accepts_slots`) ditolak saat
apply — tidak dibiarkan lolos ke runtime.

## 6. Menambah Jenis View Baru
Siapa pun bisa mendefinisikan `VisualSpecKind` baru sama sekali (mis.
`seat-map-booking`) plus Renderer resminya
([`03-renderer-kind.md`](03-renderer-kind.md)). Distribusi lewat marketplace
([`../platform/07-marketplace.md`](../platform/07-marketplace.md)) — trust
tier (official/verified/community) yang sama berlaku di sini seperti di Module
Registry.

**Mekanisme konformansi (normatif).** Berjenjang mengikuti trust tier, sama
dengan Renderer ([`03-renderer-kind.md`](03-renderer-kind.md) §5): validasi
statis skema + konsistensi `renderer_contract` adalah syarat minimum
pendaftaran semua tier; `VisualSpecKind` yang naik ke `verified`/`official`
wajib menyertakan **test-suite konformansi** (fixture `renderer_contract`)
yang dipakai memverifikasi setiap Renderer pengimplementasinya. Hanya kind
`official` yang terpilih otomatis oleh tooling; tier lain wajib dipilih
eksplisit oleh developer.
