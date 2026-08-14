# FormSpec Overview

**Version:** 0.1.0 · **Status:** Draft

> Draft: isi di bawah kontrak yang berlaku.

## 1. Apa itu FormSpec

FormSpec adalah ekosistem spec-first untuk membangun aplikasi bisnis multi-tenant
transaksional (POS multi-cabang, inventory, billing, klinik, HRM, order
management). Ia adalah **standar terbuka (CC0) dengan implementasi
referensi**, bukan sekadar framework — siapa pun boleh membangun implementasi
lain yang konform terhadap `docs/spec/`. Nilai jual inti: **tulis kontrak
sekali (`kind: Entity`, `kind: Kanban`, dst.), dapat implementasi banyak
platform** — web dan mobile dari spec yang sama
([`../frontend/01-visual-hierarchy.md`](../frontend/01-visual-hierarchy.md)
§5), backend Postgres atau SQLite dari spec penyimpanan yang sama
([`../backend/04-persist-backend.md`](../backend/04-persist-backend.md)).

Business logic boleh ditulis Go (`native`/`compiled`, performa), Starlark
(sandboxed, editable dari admin panel tanpa redeploy), atau bahasa apa pun
lewat pola sidecar (PHP/Python/Node/Java) —
[`../backend/01-core-basic.md`](../backend/01-core-basic.md) §5.

## 2. Prinsip Inti: Spec = Kontrak, Renderer = Implementasi

**Spec adalah kontrak; renderer adalah implementasi kontrak itu** — prinsip
yang sama berlaku di tiga lapisan: visual (Shell/Renderer,
[`../frontend/01-visual-hierarchy.md`](../frontend/01-visual-hierarchy.md)),
penyimpanan (PersistBackend,
[`../backend/04-persist-backend.md`](../backend/04-persist-backend.md)), dan
eksekusi action (lima jenis `impl`,
[`../backend/01-core-basic.md`](../backend/01-core-basic.md) §5). Konsekuensi
yang mengikat: kalau implementasi kedua suatu seam ingin dimungkinkan
kelak, _seam_-nya harus dirancang sejak implementasi pertama dibangun —
bukan ditambal belakangan. Rendering dan resolusi spec terjadi saat
**runtime** (interpretasi), bukan code generation — satu interpreter
di-deploy sekali, membaca spec untuk App/Page apa pun.

## 3. Anatomi Sistem

```
Spec YAML (Entity/Service/Page/…)
        │  formspec apply — dua-tahap: Control Plane registrasi → Resource Plane pull
        ▼
┌─────────────────────── Resource Plane ───────────────────────┐
│ Engine: CRUD, Action, State Machine, Event/Outbox             │
│ Spec Resolution API ──► Shell (mana pun, official/community)  │
│ PersistBackend (mana pun, official/community)                 │
└─────────────────────────────────────────────────────────────┘
        ▲ pull policy (mTLS, tanpa write-back)
┌─────────────────── Control Plane ─────────────────────────────┐
│ Governance: Environment, Policy, Datastore, kunci, kontrak,   │
│ transparency log — tidak pernah baca data bisnis/eksekusi     │
│ business logic                                                │
└─────────────────────────────────────────────────────────────┘
```

Lihat [`04-control-plane.md`](04-control-plane.md) dan
[`05-plane-protocol.md`](05-plane-protocol.md) untuk kontrak dua plane ini.

## 4. Persona dan Tier Developer

- **App developer** (Layer 0/1) — nol atau minim manifest frontend, tanpa dev
  environment lokal; CRUD derived otomatis dari Entity
  ([`../frontend/06-page-kinds.md`](../frontend/06-page-kinds.md) §14).
- **Tier 2/3 developer** — menulis handler native/script, frontend custom
  (`asset`, [`../frontend/07-component-kinds.md`](../frontend/07-component-kinds.md)
  §4), atau konsumen codegen (`formspec generate`,
  [`../../cli-tools/03-formspec-generate.md`](../../cli-tools/03-formspec-generate.md)).
- **Renderer/Shell author** — menambah Renderer visual atau PersistBackend
  baru ([`../frontend/03-renderer-kind.md`](../frontend/03-renderer-kind.md),
  [`../backend/04-persist-backend.md`](../backend/04-persist-backend.md) §7).
- **Platform operator** — menjalankan FormSpec untuk banyak workspace lewat
  Control Plane ([`04-control-plane.md`](04-control-plane.md)).

Empat peran **owner** simetris di level platform (bukan user aplikasi
bisnis) — Workspace Owner (Data Owner), App/Module Owner, Cloud Owner
(Platform Operator); satu identitas boleh memegang beberapa peran sekaligus.
Detail delegasi dan kunci di [`04-control-plane.md`](04-control-plane.md) §4.

## 5. Batas Scope

Yang bukan urusan FormSpec: Page yang benar-benar lepas dari App bebas stack
apa saja lewat API generik, tanpa Renderer kind sama sekali
([`../frontend/01-visual-hierarchy.md`](../frontend/01-visual-hierarchy.md)
§3). Unmanaged client (Flutter, native, SPA lain) adalah konsumen API kelas
satu — tidak ada satu pun kontrak di `docs/spec/frontend/` yang wajib
dipenuhinya.

## 6. Peta Dokumen Spec

| Bagian                                    | Isi                                                                                                                            |
| ----------------------------------------- | ------------------------------------------------------------------------------------------------------------------------------ |
| [`spec/platform/`](README.md)             | Kontrak lintas sisi — overview (ini), workspace/App/Module, kind system, control plane, plane protocol, datastore, marketplace |
| [`spec/backend/`](../backend/README.md)   | Kontrak data & perilaku — Entity, Action, Event, PersistBackend                                                                |
| [`spec/frontend/`](../frontend/README.md) | Kontrak visual — Shell/App/Page/Component, VisualSpecKind, Renderer, Spec Resolution API                                       |

Urutan baca disarankan per persona ada di [`../../README.md`](../../README.md)
"Jalur Baca per Persona".
