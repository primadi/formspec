# Spec Resolution API

**Version:** 0.1.0 · **Status:** Outline

> Dokumen berstatus Outline: heading di bawah menetapkan cakupan final; isi
> ditulis bertahap.

## 1. Peran
Seam runtime antara engine dan Shell manapun: kontrak internal yang dipanggil
interpreter Shell untuk mendapat representasi siap-render dari App/Page/Entity.
Rendering adalah interpretasi runtime — Shell di-deploy sekali dan membaca spec
saat runtime; tidak ada build artifact per-app.

## 2. Endpoint
```
GET /_forma/view-spec/{app}/{page}
→ instance VisualSpecKind (resolved, permission-filtered per user)
→ metadata field (type, validation, relation target, …)
→ referensi slot yang sudah di-resolve
```
Bentuk lengkap endpoint (bundle app, identitas, entity schema) dispesifikasikan
di sini.

## 3. Wajib Backend-Agnostic
API menyerahkan bentuk data (field/type/validation/permission) — bukan query
result mentah. Dilarang membocorkan detail PersistBackend (nama kolom fisik,
path JSONB). Ini syarat agar Shell tidak perlu tahu backend apa di baliknya.

## 4. Permission Filtering
Field yang `required_permission`-nya tidak dipenuhi user tidak ikut terkirim —
filtering terjadi di sisi resolusi, bukan di renderer.

## 5. Realtime
Konvensi event/websocket yang menyertai spec resolution (update data, invalidasi).

## 6. Versi & Kompatibilitas
Versioning API ini terhadap shell yang mengonsumsinya.
