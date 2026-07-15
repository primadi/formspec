# Katalog Kind — Tier Component

**Version:** 0.1.0 · **Status:** Outline

> Dokumen berstatus Outline: heading di bawah menetapkan cakupan final; isi
> ditulis bertahap. Setiap kind di sini adalah instance VisualSpecKind
> `tier: component`.

## 1. Input Components
`textinput`, `dateinput`, relation picker, dan keluarga input lain; kontrak
value/validation/permission per field.

## 2. `widget`
Component pengisi slot (`implements_slot: widget`): kontrak data-shape
(`title`, `data_binding`, `size_unit`, …); contoh `kpi-widget`.

## 3. Slot Filling di Instance
Cara instance Page mereferensikan widget ke posisi slot (use, position,
data_binding).

## 4. `asset` — Escape Hatch Component
Component custom yang di-mount langsung (`mount(el, props, forma)`); batas
tanggung jawab dan risiko yang ditanggung pemakainya.

## 5. Menambah Component Kind Baru
Lewat VisualSpecKind `tier: component`; validasi `implements_slot`.
