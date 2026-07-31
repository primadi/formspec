# Kind System

**Version:** 0.1.0 · **Status:** Draft

> Draft: isi di bawah kontrak yang berlaku.

## 1. Taksonomi Kind
Seluruh kind Forma memakai format manifest yang sama
(`apiVersion/kind/metadata/spec`,
[`../backend/01-core-basic.md`](../backend/01-core-basic.md) §1) tapi
terbagi per concern:

| Concern | Kind | Didefinisikan di |
|---|---|---|
| Model domain | `Entity`, `Service` | `spec/backend/01-core-basic.md` |
| Kurasi | `App`, `Module` | `spec/platform/02-workspace-app-module.md` |
| Konfigurasi | `Config` | `spec/backend/01-core-basic.md` §10 (Config & Global Settings) |
| DDL custom | `Migration` | `spec/backend/01-core-basic.md` §4 |
| Reaksi lintas-module | `Subscription` | `spec/backend/01-core-basic.md` §7 |
| Proses bisnis | `Workflow` | `spec/backend/02-core-extended.md` §2 |
| Permukaan API | `Api`, `Webhook`, `Mockup` | `spec/backend/02-core-extended.md` §12 (`Api`), §4 (`Webhook`), §8 (`Mockup`) |
| Mekanisme ekstensi | `KindDefinition` | §2 di bawah |
| Jembatan reaktif | `Integrator` | `spec/backend/02-core-extended.md` §5 |
| Renderer visual | `Renderer` | `spec/frontend/03-renderer-kind.md` |
| Renderer penyimpanan | `PersistBackend` | `spec/backend/04-persist-backend.md` |
| Visual — hirarkis, bukan flat | Instance `VisualSpecKind` (Page, Form, Table, Dashboard, Widget, Report, Wizard, Kanban, Timeline, Calendar, Listing, ApprovalInbox, NotificationCenter, Print, Theme) | `spec/frontend/02,05,06,07` |
| Governance | `Environment`, `Policy` | `spec/platform/04-control-plane.md` |
| Infrastruktur | `Datastore` | `spec/platform/06-datastore.md` |

**Derived by default:** endpoint CRUD, admin panel, dan dokumentasi API
digenerate otomatis dari manifest `Entity` — tanpa manifest tambahan
apa pun. Kind visual (Page/Form/Table/dst.) ada hanya untuk *override*
default itu ([`../frontend/06-page-kinds.md`](../frontend/06-page-kinds.md)
§9).

**Guardrail:** app developer hampir tidak pernah perlu mendefinisikan kind
baru — butuh kind baru berarti memperluas framework. 95% kasus jawabannya
`Entity`.

## 2. Meta-Kinds
Kind yang mendeklarasikan kind lain — extensible dalam tiga layer:
(1) built-in spec (tabel §1) → (2) module resmi mendaftarkan kind lewat
`KindDefinition` (`Seed`, `Schedule`, `MailTemplate`, dst) → (3) module
pihak ketiga dengan kind namespaced, tunduk Verified Badge.

```yaml
apiVersion: forma.dev/v1alpha1
kind: KindDefinition
metadata: { name: Seed, module: forma/seed }
spec:
  group: seed.forma.dev              # instance pakai apiVersion: seed.forma.dev/v1
  version: v1
  schema: { ... }                    # JSON Schema body instance
  handler: { type: native, ref: "FormaSeed.Apply" }
  scope: module                      # module | app
```

Penamaan dinamespace lewat grup `apiVersion` (pola CRD) — kind built-in
memiliki grup `forma.dev`, kind module hidup di grup sendiri
(`seed.forma.dev`, `gl.acme-corp.dev`) — tabrakan namespace mustahil secara
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
| Kebutuhan aplikasi bisnis | Kind yang menjawab |
|---|---|
| Simpan & kelola data bertransaksi | `Entity` (`characteristic: transaction`) |
| Data referensi stabil | `Entity` (`characteristic: master`) |
| Data seed read-only | `Entity` (`characteristic: reference`) |
| Projeksi/agregat sistem | `Entity` (`characteristic: summary`) |
| Komputasi tanpa state | `Service` |
| Approval berbasis role atas transisi | `Workflow` |
| Endpoint masuk terverifikasi (webhook provider) | `Webhook` |
| Simulasi integrasi pihak ketiga | `Mockup` |
| Jembatan reaktif antar-module | `Integrator` |
| Reaksi ke event resource lain | `Subscription` |
| Override permukaan API yang sudah exposed | `Api` |
| DDL custom (index, trigger) | `Migration` |
| Layar/route | `Page` |
| Input/edit satu Entity | `Form` |
| List/browse | `Table` |
| Proses multi-step | `Wizard` |
| Board status drag-drop | `Kanban` |
| Feed kronologis append-only | `Timeline` |
| Dashboard + widget | `Dashboard`, `Widget` |
| Laporan terparameterisasi | `Report` |
| Dokumen cetak | `Print` |
| Tampilan & rasa | `Theme` |
| View kalender | `Calendar` |
| Katalog publik | `Listing` |
| Inbox approval | `ApprovalInbox` |
| Pusat notifikasi | `NotificationCenter` |
| Koneksi infrastruktur bernama | `Datastore` |
| Target deployment | `Environment` |
| Aturan governance | `Policy` |

## 4. Lampiran: Pemetaan Kind → Plane
Referensi kanonik menentukan plane tempat sebuah kind hidup — dicek setiap
kali kind baru diperkenalkan.

**Aturan normatif:** (1) Kind Control Plane **tidak boleh** membaca data
bisnis atau mengeksekusi handler bisnis. (2) Kind Resource Plane **tidak
boleh** mengubah governance state. (3) Rule of thumb: kalau kind
mengonfigurasi infrastruktur/governance/deployment/keamanan platform →
Control Plane; kalau mendefinisikan domain logic/UI/business behavior →
Resource Plane.

| Kind | Plane |
|---|---|
| `App`, `Module`, `Entity`, `Service`, `Config`, `Migration`, `Subscription` | Resource |
| `Workflow`, `Api`, `Webhook`, `Mockup`, `Integrator`, `KindDefinition` | Resource |
| `Page`, `Form`, `Table`, `Dashboard`, `Widget`, `Report`, `Wizard`, `Kanban`, `Timeline`, `Calendar`, `Listing`, `ApprovalInbox`, `NotificationCenter`, `Print`, `Theme` | Resource |
| `VisualSpecKind`, `Renderer` | Resource (dideklarasikan bersama artifact visual; distribusi lewat marketplace §7) |
| `Environment`, `Policy` | Control |
| `Datastore` | Control |
| `PersistBackend` | Resource (dideklarasikan per deployment scope, dikonsumsi Resource Plane; distribusi lewat marketplace) |

Menambah kind baru: jawab (a) apakah ia mengonfigurasi infrastruktur/
governance? → Control; (b) apakah ia mendefinisikan business logic/UI/domain
model? → Resource; (c) apakah ia perlu baca data bisnis? → **wajib**
Resource; (d) apakah ia perlu diatur Cloud Owner, bukan App Developer? →
Control. Lalu tambahkan baris ke tabel §4 ini.
