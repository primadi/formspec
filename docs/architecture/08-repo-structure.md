# Struktur Repo FormSpec

**Status:** Draft
**License:** Creative Commons CC0

> Dokumen ini untuk **kontributor codebase FormSpec** (bukan app developer yang membangun di atas FormSpec, dan bukan platform operator yang menjalankan FormSpec). Ia memetakan folder repo ke prinsip **spec adalah kontrak, renderer adalah implementasi** yang mendasari seluruh `docs/` — lihat [`docs/README.md`](../README.md) untuk prinsip itu sendiri.

---

## 1. Tree Repo Saat Ini, Diberi Anotasi

```
formspec/
├── cmd/                       # Binary entrypoint
│   ├── formspec/                 # CLI utama + dev server — docs/cli-tools/01, 02-formspec-dev
│   ├── formspec-ctl/              # Binary Control Plane — docs/cli-tools/02-formspec-ctl, docs/runtimes/01
│   └── formspec-operator/         # K8s operator — docs/runtimes/03
│
├── pkg/
│   └── spec/                  # ★ REALISASI KODE DARI docs/spec/ — struct Go yang MENJADI kontrak
│                               #   (Manifest, Kind enum, Document/Entity, Datastore, Frontend kinds).
│                               #   Backend-agnostic, renderer-agnostic. Dipakai bersama oleh runtime
│                               #   engine dan seluruh target codegen (cmd/formspec/generate_*.go).
│
├── internal/
│   ├── db/, datastore/        # ⚙ RENDERER — mengimplementasikan kontrak PersistBackend
│   │                           #   (docs/spec/backend/04-persist-backend.md) lewat driver
│   │                           #   Postgres+SQLite (docs/renderers/jsonb-persist/). Target lokasi:
│   │                           #   renderers/jsonb-persist/ — lihat §2.
│   │                           #   Lihat §4 — belum benar-benar PersistBackend yang bersih.
│   ├── api/                   # ⚙ ENGINE PENGHUBUNG — menyajikan kontrak Spec Resolution API
│   │                           #   (docs/spec/frontend/04-spec-resolution-api.md) lewat `/_meta`,
│   │                           #   sekaligus menghasilkan kontrak REST delivery
│   │                           #   (docs/spec/backend/01-core-basic.md §6, §8). Menjembatani
│   │                           #   renderer backend dan renderer frontend — bukan renderer itu
│   │                           #   sendiri.
│   ├── action/, starlark/     # Eksekusi Action — kontrak docs/spec/backend/01-core-basic.md §5
│   ├── ui/                    # Registry manifest UI sisi server — dikonsumsi shadcn-shell
│   ├── entity/                # Model runtime Document/Entity
│   ├── sidecar/                # Sisi server protokol sidecar — docs/runtimes/04-formspec-sidecar.md
│   ├── control/, permission/, auth/  # Control plane + keamanan — docs/spec/platform/04
│   ├── manifest/               # Loader & validator YAML
│   ├── operator/               # Controller K8s — docs/architecture/06-k8s-operator.md
│   ├── resource/, app/, artifact/, validation/, events/, tenant/, service/, ctx/  # pendukung
│
├── renderers/react-shadcn/     # ⚙ RENDERER — shadcn-shell resmi (React) — docs/renderers/shadcn-shell/.
│                                #   Mengimplementasikan hirarki Shell/App/Page/Component
│                                #   (docs/spec/frontend/01-03). Renderer PersistBackend di
│                                #   renderers/jsonb-persist/ — lihat §2.
│
├── sdk/                        # Client library tipis, satu per bahasa (go, php, python, node/
│                                #   typescript, java, dotnet, ruby, rust, browser) — mengimplementasikan
│                                #   sisi handler protokol sidecar (docs/runtimes/04-formspec-sidecar.md
│                                #   §4.4). Inilah yang membuat handler tiap Module bisa ditulis
│                                #   dalam bahasa berbeda — lihat docs/spec/platform/08-project-layout.md.
│
├── resource/                   # formspec-resource: library engine Go yang di-embed ke binary
│                                #   formspec/formspec-sidecar — BUKAN untuk di-import app developer.
│
├── verticals/, examples/       # App referensi — instance spec (YAML workspace/app/module),
│                                #   bukan kode framework. Bukti kontrak bisa diimplementasikan.
│
├── docs/                       # Tree dokumentasi ini — spec (kontrak) vs renderers (implementasi
│                                #   resmi) vs architecture/runtimes/cli-tools (deskriptif) vs
│                                #   guides/reference (untuk manusia).
│
└── reff_docs/                  # Catatan diskusi desain historis — bahan sumber untuk docs/,
                                 #   bukan otoritatif sendiri (lihat docs/README.md).
```

---

## 2. Target: Folder `renderers/`

Hari ini implementasi tiap seam tersebar (`web/` di root, `internal/db`+`internal/datastore` di dalam `internal/`) — tidak ada satu tempat yang jelas untuk "tambah implementasi baru dari seam yang sama". Target strukturnya menyatukan seluruh implementasi resmi, lintas seam, di bawah satu folder `renderers/` — sejajar dengan `docs/renderers/` yang sudah memakai pola ini:

```
renderers/
├── react-shadcn/               # Shell resmi (shadcn-shell) — React + shadcn/ui
│                                #   (target slot shell kedua, mis. Flutter: renderers/<nama-shell>/)
│
└── jsonb-persist/               # PersistBackend resmi — pindahan dari internal/db + internal/datastore
                                  #   (target slot PersistBackend dengan strategi skema lain, mis.
                                  #   fully-relational: renderers/<nama-backend>/)
```

Nama `jsonb-persist` (bukan `persist-postgres`) sengaja menandai bahwa yang membedakan implementasi ini bukan engine SQL-nya (backend ini sudah jalan di Postgres **dan** SQLite hari ini) — melainkan _strategi skemanya_ (JSONB). Implementasi PersistBackend lain di masa depan dibedakan dari strategi skemanya juga (mis. fully-relational), bukan dari database yang dipakainya.

Ini **rename/move**, bukan rewrite — logic yang sudah teruji di `internal/db`, `internal/datastore`, dan `web/` pindah lokasi & import path, tidak ditulis ulang (lihat argumen _port bertahap_ di §4). Belum dieksekusi; dicatat di sini supaya restrukturisasi kode berikutnya punya target yang jelas, bukan menerka-nerka ulang.

---

## 3. Pemetaan Kode ↔ Dokumen

| Kode                                                          | Dokumen terkait                                                                                                                                                              | Peran                                                            |
| ------------------------------------------------------------- | ---------------------------------------------------------------------------------------------------------------------------------------------------------------------------- | ---------------------------------------------------------------- |
| `pkg/spec/`                                                   | [`docs/spec/`](../spec/README.md) (semua)                                                                                                                                    | Skema kontrak sebagai struct Go                                  |
| `internal/db/`, `internal/datastore/`                         | [`docs/spec/backend/04-persist-backend.md`](../spec/backend/04-persist-backend.md), [`docs/renderers/jsonb-persist/`](../renderers/jsonb-persist/README.md)                  | Renderer PersistBackend (target: `renderers/jsonb-persist/`, §2) |
| `internal/api/`                                               | [`docs/spec/frontend/04-spec-resolution-api.md`](../spec/frontend/04-spec-resolution-api.md), [`docs/spec/backend/01-core-basic.md`](../spec/backend/01-core-basic.md) §6/§8 | Engine yang menyajikan kedua kontrak                             |
| `renderers/react-shadcn/`, `internal/ui/`                     | [`docs/spec/frontend/`](../spec/frontend/README.md), [`docs/renderers/shadcn-shell/`](../renderers/shadcn-shell/README.md)                                                   | Renderer Shell resmi (target: `renderers/react-shadcn/`, §2)     |
| `internal/action/`, `internal/starlark/`                      | [`docs/spec/backend/01-core-basic.md`](../spec/backend/01-core-basic.md) §5                                                                                                  | Eksekusi Action lintas 5 jenis impl                              |
| `internal/sidecar/`, `sdk/*`                                  | [`docs/runtimes/04-formspec-sidecar.md`](../runtimes/04-formspec-sidecar.md), [`docs/spec/platform/08-project-layout.md`](../spec/platform/08-project-layout.md)             | Protokol handler lintas bahasa                                   |
| `internal/control/`, `internal/permission/`, `internal/auth/` | [`docs/spec/platform/04-control-plane.md`](../spec/platform/04-control-plane.md)                                                                                             | Governance & keamanan                                            |
| `cmd/formspec/`, `cmd/formspec-ctl/`                          | [`docs/cli-tools/`](../cli-tools/README.md)                                                                                                                                  | CLI                                                              |

---

## 4. Kesenjangan Terhadap Pemisahan Ideal

Struktur kode hari ini **belum sepenuhnya** mencerminkan pemisahan kontrak-vs-renderer yang dianut `docs/spec/` — dicatat di sini supaya jelas apa yang perlu dibereskan saat fase restrukturisasi kode dimulai, bukan diam-diam diasumsikan sudah selesai:

- **Implementasi renderer belum disatukan di `renderers/`** (§2) — `web/` masih di root, `internal/db`+`internal/datastore` masih di dalam `internal/`. Tidak ada tempat tunggal untuk menaruh implementasi kedua per seam.
- **`internal/db` belum jadi interface `PersistBackend` yang bersih.** Interface `DB`/`Tx` yang ada bocor semantik SQL (`ExecContext`, `QueryContext`, `Driver() *sql.DB`) — ini seam driver SQL, bukan seam penyimpanan yang storage-agnostic seperti dikontrakkan `docs/spec/backend/04-persist-backend.md`.
- **`internal/api`'s `/_meta` belum diaudit backend-agnostic.** Syarat Spec Resolution API (tidak boleh membocorkan nama kolom fisik/path JSONB) belum diverifikasi terpenuhi.
- **Belum ada registry `VisualSpecKind`/`Renderer` di `pkg/spec`.** Hirarki Shell/App/Page/Component (`docs/spec/frontend/01-03`) masih implisit di `web/kinds/`, bukan kind formal yang bisa di-apply.
- **Sidecar masih satu-proses-per-project**, belum mendukung satu Module satu runtime bahasa berbeda — lihat gap detail di [`docs/spec/platform/08-project-layout.md`](../spec/platform/08-project-layout.md) §5.

Sisi positif (argumen untuk _port bertahap_ — rename/move ke §2, bukan rewrite total, saat fase kode nanti dimulai): `pkg/spec` sudah terpisah bersih dari engine; `web/` sudah interpreter runtime (bukan build-per-app); `internal/db` sudah punya driver ganda (Postgres+SQLite) — fondasi menuju PersistBackend sejati sudah ada, tinggal diformalkan jadi interface.
