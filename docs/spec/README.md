# Forma Spec — Kontrak Normatif

Section ini memuat **kontrak** Forma: apa yang wajib dipenuhi oleh implementasi
manapun, terlepas dari teknologi yang dipakainya. Pernyataan prinsipnya, sekali
dan otoritatif:

> **Spec adalah kontrak; renderer adalah implementasi.**
> Sebuah kontrak ditulis sekali dan bersifat implementation-agnostic. Implementasi
> (frontend shell, persist backend) boleh diganti atau ditambah tanpa mengubah
> kontraknya — asalkan seam-nya dirancang sejak implementasi pertama dibangun.

Konsekuensi praktis:

- **Satu spec, banyak renderer.** Spec `kind: Kanban` yang sama dirender oleh
  shell React/shadcn hari ini dan bisa dirender shell lain (mis. Flutter) besok,
  tanpa ditulis ulang.
- **Dua keluarga seam.** Sisi visual: `Shell` menghosting App/Page/Component
  renderer ([`frontend/`](frontend/)). Sisi penyimpanan: `PersistBackend`
  ([`backend/04-persist-backend.md`](backend/04-persist-backend.md)).
  Kontrak lintas keduanya ada di [`platform/`](platform/).
- **Rendering adalah interpretasi runtime, bukan code generation.** Shell membaca
  spec lewat Spec Resolution API saat runtime
  ([`frontend/04-spec-resolution-api.md`](frontend/04-spec-resolution-api.md)).
  Codegen (`forma generate`) hanya untuk developer Tier 2/3 yang menulis handler
  native atau frontend custom.

## Sub-section

| Direktori | Kontrak untuk |
|---|---|
| [`platform/`](platform/) | Kedua sisi: model workspace/app/module, kind system & meta-kinds, control plane, plane protocol, datastore, marketplace |
| [`backend/`](backend/) | Engine & PersistBackend manapun: model dokumen, action, lifecycle, extension, interface penyimpanan |
| [`frontend/`](frontend/) | Shell & renderer visual manapun: hirarki 4 tier, VisualSpecKind, Renderer, Spec Resolution API, katalog kind, FormaExpr |

Status dokumen: `Outline` (kerangka cakupan) → `Draft` (isi lengkap, terbuka
revisi) → `Final` (mengikat; perubahan lewat bump versi).
