# Kind System

**Version:** 0.1.0 · **Status:** Draft

> Draft: isi di bawah kontrak yang berlaku.

> **Referensi atribut per-kind:** [`docs/kind/`](../../kind/README.md) — 33 file
> (4 grup), tabel atribut generated dari `pkg/spec` + narasi manual. Dokumen ini
> mendefinisikan taksonomi & meta-kind; `docs/kind/` mendefinisikan atribut tiap
> kind.

## 1. Taksonomi Kind

Seluruh 33 kind FormSpec memakai format manifest yang sama
(`apiVersion/kind/metadata/spec`,
[`../backend/01-core-basic.md`](../backend/01-core-basic.md) §1) dan
dikelompokkan dalam **4 grup** yang mencerminkan struktur `docs/spec/`:

| #            | Grup | Jumlah                                                                                                                                                                                                              | Definisi    | Mirror `docs/spec/` |
| ------------ | ---- | ------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- | ----------- | ------------------- |
| **Curation** | 2    | `App`, `Module` — struktur workspace, kurasi module, menu                                                                                                                                                           | `platform/` |
| **Data**     | 11   | `Entity`, `Service`, `Config`, `Migration`, `Subscription`, `Workflow`, `Api`, `Webhook`, `Mockup`, `Integrator`, `KindDefinition` — model domain, behaviour, integrasi                                             | `backend/`  |
| **UI**       | 15   | `Page`, `Form`, `Table`, `Dashboard`, `Widget`, `Report`, `Wizard`, `Kanban`, `Timeline`, `Calendar`, `Listing`, `ApprovalInbox`, `NotificationCenter`, `Print`, `Theme` — presentasi visual, override auto-derived | `frontend/` |
| **Infra**    | 5    | `Renderer`, `PersistBackend`, `Environment`, `Policy`, `Datastore` — runtime infrastructure, renderer, storage, governance                                                                                          | `platform/` |

### Rincian per Grup

**Curation** — didklarasikan pertama saat membangun aplikasi:
| Kind | Didefinisikan di |
|---|---|
| `App` | `spec/platform/02-workspace-app-module.md` |
| `Module` | `spec/platform/02-workspace-app-module.md` |

**Data** — model domain, behaviour, event, integrasi:
| Kind | Didefinisikan di |
|---|---|
| `Entity` | `spec/backend/01-core-basic.md` §1 |
| `Service` | `spec/backend/01-core-basic.md` §1 |
| `Config` | `spec/backend/01-core-basic.md` §10 |
| `Migration` | `spec/backend/01-core-basic.md` §4 |
| `Subscription` | `spec/backend/01-core-basic.md` §7 |
| `Workflow` | `spec/backend/02-core-extended.md` §2 |
| `Api` | `spec/backend/02-core-extended.md` §12 |
| `Webhook` | `spec/backend/02-core-extended.md` §4 |
| `Mockup` | `spec/backend/02-core-extended.md` §8 |
| `Integrator` | `spec/backend/02-core-extended.md` §5 |
| `KindDefinition` | §2 di bawah |

**UI** — presentasi visual. Instance `VisualSpecKind` dengan tier `page`.
Semua ada hanya untuk _override_ auto-derived defaults dari Entity:
| Kind | Didefinisikan di |
|---|---|
| `Page` | `spec/frontend/06-page-kinds.md` §1 |
| `Form` | `spec/frontend/06-page-kinds.md` §2 |
| `Table` | `spec/frontend/06-page-kinds.md` §3 |
| `Dashboard` | `spec/frontend/06-page-kinds.md` §7 |
| `Widget` | `spec/frontend/07-component-kinds.md` §2 |
| `Report` | `spec/frontend/06-page-kinds.md` §8 |
| `Wizard` | `spec/frontend/06-page-kinds.md` §6 |
| `Kanban` | `spec/frontend/06-page-kinds.md` §4 |
| `Timeline` | `spec/frontend/06-page-kinds.md` §9 |
| `Calendar` | `spec/frontend/06-page-kinds.md` §5 |
| `Listing` | `spec/frontend/06-page-kinds.md` §10 |
| `ApprovalInbox` | `spec/frontend/06-page-kinds.md` §11 |
| `NotificationCenter` | `spec/frontend/06-page-kinds.md` §12 |
| `Print` | `spec/frontend/06-page-kinds.md` §8 |
| `Theme` | `spec/frontend/05-app-kinds.md` §6 |

**Infra** — runtime infrastructure & governance:
| Kind | Didefinisikan di |
|---|---|
| `Renderer` | `spec/frontend/03-renderer-kind.md` |
| `PersistBackend` | `spec/backend/04-persist-backend.md` |
| `Environment` | `spec/platform/04-control-plane.md` |
| `Policy` | `spec/platform/04-control-plane.md` |
| `Datastore` | `spec/platform/06-datastore.md` |

**Derived by default:** endpoint CRUD, admin panel, dan dokumentasi API
digenerate otomatis dari manifest `Entity` — tanpa manifest tambahan
apa pun. Kind visual (Page/Form/Table/dst.) ada hanya untuk _override_
default itu. Lihat UI 3-layer wrapping model di
[`../frontend/06-page-kinds.md`](../frontend/06-page-kinds.md) §14.

**Guardrail:** app developer hampir tidak pernah perlu mendefinisikan kind
baru — butuh kind baru berarti memperluas framework. 95% kasus jawabannya
`Entity`.

## 2. Meta-Kinds

Kind yang mendeklarasikan kind lain — extensible dalam tiga layer:
(1) built-in spec (tabel §1) → (2) module resmi mendaftarkan kind lewat
`KindDefinition` (`Seed`, `Schedule`, `MailTemplate`, dst) → (3) module
pihak ketiga dengan kind namespaced, tunduk Verified Badge.

```yaml
apiVersion: formspec.dev/v1
kind: KindDefinition
metadata: { name: Seed, module: formspec/seed }
spec:
  group: seed.formspec.dev # instance pakai apiVersion: seed.formspec.dev/v1
  version: v1
  schema: { ... } # JSON Schema body instance
  handler: { type: native, ref: "FormaSeed.Apply" }
  scope: module # module | app
```

Penamaan dinamespace lewat grup `apiVersion` (pola CRD) — kind built-in
memiliki grup `formspec.dev`, kind module hidup di grup sendiri
(`seed.formspec.dev`, `gl.acme-corp.dev`) — tabrakan namespace mustahil secara
struktural. Handler berjalan di bawah `uses` module yang mendeklarasikannya
— `KindDefinition` tidak memberi kekuatan runtime di luar footprint
module-nya sendiri.

Tiga meta-kind lain, masing-masing dijelaskan penuh di spec-nya sendiri:

- **`VisualSpecKind`** — mendeklarasikan jenis view baru + skema + kontrak
  renderer ([`../frontend/02-visual-spec-kind.md`](../frontend/02-visual-spec-kind.md)).
- **`Renderer`** — implementasi konkret sebuah VisualSpecKind
  ([`../frontend/03-renderer-kind.md`](../frontend/03-renderer-kind.md)).
- **`PersistBackend`** — implementasi penyimpanan
  ([`../backend/04-persist-backend.md`](../backend/04-persist-backend.md)).

## 3. Katalog Concern → Kind

| Kebutuhan aplikasi bisnis                       | Kind yang menjawab                       |
| ----------------------------------------------- | ---------------------------------------- |
| Simpan & kelola data bertransaksi               | `Entity` (`characteristic: transaction`) |
| Data referensi stabil                           | `Entity` (`characteristic: master`)      |
| Data seed read-only                             | `Entity` (`characteristic: reference`)   |
| Projeksi/agregat sistem                         | `Entity` (`characteristic: summary`)     |
| Komputasi tanpa state                           | `Service`                                |
| Approval berbasis role atas transisi            | `Workflow`                               |
| Endpoint masuk terverifikasi (webhook provider) | `Webhook`                                |
| Simulasi integrasi pihak ketiga                 | `Mockup`                                 |
| Jembatan reaktif antar-module                   | `Integrator`                             |
| Reaksi ke event resource lain                   | `Subscription`                           |
| Override permukaan API yang sudah exposed       | `Api`                                    |
| DDL custom (index, trigger)                     | `Migration`                              |
| Layar/route                                     | `Page`                                   |
| Input/edit satu Entity                          | `Form`                                   |
| List/browse                                     | `Table`                                  |
| Proses multi-step                               | `Wizard`                                 |
| Board status drag-drop                          | `Kanban`                                 |
| Feed kronologis append-only                     | `Timeline`                               |
| Dashboard + widget                              | `Dashboard`, `Widget`                    |
| Laporan terparameterisasi                       | `Report`                                 |
| Dokumen cetak                                   | `Print`                                  |
| Tampilan & rasa                                 | `Theme`                                  |
| View kalender                                   | `Calendar`                               |
| Katalog publik                                  | `Listing`                                |
| Inbox approval                                  | `ApprovalInbox`                          |
| Pusat notifikasi                                | `NotificationCenter`                     |
| Koneksi infrastruktur bernama                   | `Datastore`                              |
| Target deployment                               | `Environment`                            |
| Aturan governance                               | `Policy`                                 |

## 4. Lampiran: Pemetaan Kind → Plane

Referensi kanonik menentukan plane tempat sebuah kind hidup — dicek setiap
kali kind baru diperkenalkan.

**Aturan normatif:** (1) Kind Control Plane **tidak boleh** membaca data
bisnis atau mengeksekusi handler bisnis. (2) Kind Resource Plane **tidak
boleh** mengubah governance state. (3) Rule of thumb: kalau kind
mengonfigurasi infrastruktur/governance/deployment/keamanan platform →
Control Plane; kalau mendefinisikan domain logic/UI/business behavior →
Resource Plane.

| Kind                                                                                                                                                                     | Plane                                                                                                   |
| ------------------------------------------------------------------------------------------------------------------------------------------------------------------------ | ------------------------------------------------------------------------------------------------------- |
| `App`, `Module`, `Entity`, `Service`, `Config`, `Migration`, `Subscription`                                                                                              | Resource                                                                                                |
| `Workflow`, `Api`, `Webhook`, `Mockup`, `Integrator`, `KindDefinition`                                                                                                   | Resource                                                                                                |
| `Page`, `Form`, `Table`, `Dashboard`, `Widget`, `Report`, `Wizard`, `Kanban`, `Timeline`, `Calendar`, `Listing`, `ApprovalInbox`, `NotificationCenter`, `Print`, `Theme` | Resource                                                                                                |
| `VisualSpecKind`, `Renderer`                                                                                                                                             | Resource (dideklarasikan bersama artifact visual; distribusi lewat marketplace §7)                      |
| `Environment`, `Policy`                                                                                                                                                  | Control                                                                                                 |
| `Datastore`                                                                                                                                                              | Control                                                                                                 |
| `PersistBackend`                                                                                                                                                         | Resource (dideklarasikan per deployment scope, dikonsumsi Resource Plane; distribusi lewat marketplace) |

Menambah kind baru: jawab (a) apakah ia mengonfigurasi infrastruktur/
governance? → Control; (b) apakah ia mendefinisikan business logic/UI/domain
model? → Resource; (c) apakah ia perlu baca data bisnis? → **wajib**
Resource; (d) apakah ia perlu diatur Cloud Owner, bukan App Developer? →
Control. Lalu tambahkan baris ke tabel §4 ini.
