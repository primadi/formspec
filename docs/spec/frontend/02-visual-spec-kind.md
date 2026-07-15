# VisualSpecKind

**Version:** 0.1.0 · **Status:** Outline

> Dokumen berstatus Outline: heading di bawah menetapkan cakupan final; isi
> ditulis bertahap.

## 1. Peran
Meta-kind untuk mendeklarasikan *jenis view baru* tanpa mengubah framework inti:
skema instance + kontrak yang wajib dipenuhi renderernya.

```yaml
apiVersion: forma/v1
kind: VisualSpecKind
metadata:
  name: kanban
spec:
  tier: page                 # WAJIB — app | page | component
  schema: {...}              # field wajib di instance spec
  renderer_contract: {...}   # interface yang WAJIB dipenuhi renderer manapun
```

## 2. Field `tier` (wajib)
`app | page | component` — menentukan di mana kind boleh dipakai/dikomposisi dan
menjadi dasar validasi slot. Shell tidak termasuk nilai tier.

## 3. Skema Instance
Instance yang ditulis app developer (`kind: Kanban`) — shell-agnostic, satu
definisi untuk semua shell.

## 4. Slot System
Deklarasi `accepts_slots` (kontrak data-shape lubang) dan `implements_slot`
(component pengisi):
- `accepts_slots` hanya sah dari tier `page` (atau `app` bila dibutuhkan);
  `implements_slot` hanya sah dari tier `component`.
- Kontrak slot adalah data-shape, bukan visual.
- Slot filling hanya dalam satu Shell yang sama.
- Kedalaman rekursi satu level (Page menerima Component).

## 5. Validasi `forma apply`
Tier check untuk pemakaian dan slot binding; kombinasi tak sah ditolak saat
apply, bukan lolos ke runtime.

## 6. Menambah Jenis View Baru
Alur mendeklarasikan VisualSpecKind baru (mis. `seat-map-booking`) plus renderer
resminya; distribusi lewat marketplace.
