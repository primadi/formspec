# Datastore

**Version:** 0.2.0 · **Status:** Draft

> Draft: isi di bawah kontrak yang berlaku. Implementasi referensi:
> `resource/datastoreregistry.go` (Infra & App Registry), `internal/starlark/`
> (resolusi `ctx.*`).

## 1. Kind Datastore — Infra Registry (Level 1)

`kind: Datastore` adalah resource **Control Plane** yang meregistrasikan
**service infrastruktur fisik** — satu instance nyata (Postgres, Valkey,
MinIO, NATS, SQLite, filesystem) dengan logical name. Semua definisi backend
berasal dari Control Plane; Resource Plane tidak punya mekanisme membuat atau
mengubah definisi backend sendiri, ia menerima service yang sudah
diotorisasi untuk workspace-nya lewat snapshot Plane Protocol
([`05-plane-protocol.md`](05-plane-protocol.md) §4.1).

Prinsip:

- **Registrasi eksplisit** — setiap service teregistrasi lewat manifest
  `kind: Datastore` (atau API Control Plane); tidak ada backend implisit di
  luar registry. Dev mode (`formspec dev`) auto-provision service `'default'`
  sebagai convenience, dan hasilnya tetap tercatat di registry sebagai
  service biasa.
- **Multi-service per primitive** — satu jenis primitive boleh dilayani
  banyak service (mis. 2 database: `pg-main` dan `pg-analytics`). Tiap
  primitive punya satu **default** yang menunjuk ke salah satu service
  teregistrasi (pointer, bukan backend implisit) dan dapat dioverride
  (§5).
- **Transparansi teknologi** — developer mereferensi service by name lewat
  `ctx.*` (§2), tidak perlu tahu apakah di baliknya Postgres, SQLite, atau
  Valkey.
- **Kredensial tidak pernah inline** — selalu lewat `credential_ref` ke
  KMS/Vault.

```yaml
apiVersion: formspec.dev/v1
kind: Datastore
metadata:
  name: pg-analytics
spec:
  serves: [db, kvstore]        # primitive ctx.* yang dilayani service ini
  driver: postgres
  connection:
    host: pg-analytics.internal
    port: 5432
    database: formspec_analytics
    pool: { max_open: 100, max_idle: 20, max_lifetime: 1h }
    lazy: false                # true = connect saat pemakaian pertama
  credential_ref: kms://prod/pg-analytics
  access:
    filter:                    # opsional — kosong = semua workspace
      environment: production
      workspaces: [corp-456]
      labels: { tier: enterprise }
    permission:                 # opsional — kosong = read_write
      default: read             # ceiling untuk seluruh operasi
      rules:
        - { scope: "store.*", access: read_write }
        - { scope: "billing.invoice", access: write }
```

### 1.1 App Registry (Level 2) — Seleksi Logical Primitive

**Infra Registry** (level 1) menyimpan service fisik; **App Registry**
(level 2, milik app builder) menentukan logical primitive mana yang dipakai.
Setiap mapping menghasilkan **logical name** yang dipakai kode aplikasi.

Deklarasi di `kind: App` (`spec.datastores`) dan `kind: Module`
(`spec.datastores`) — map dengan dua bentuk key:

| Key | Arti | Diakses lewat |
|---|---|---|
| `db` | default service untuk primitive `db` milik App/Module ini | `ctx.db()` |
| `db/analytics` | **named logical primitive** `analytics` untuk `db` | `ctx.db.named("analytics")` |

```yaml
# kind: App
spec:
  modules: [billing, reporting]
  datastores:
    db: pg-main              # default db App ini (App lain boleh beda)
    db/analytics: pg-analytics   # named logical primitive
```

```yaml
# kind: Module — override per module
spec:
  datastores:
    db: pg-analytics         # module ini pakai db berbeda dari App
    db/rollup: pg-main       # named primitive milik module (butuh App)
```

**Chain resolusi** (dari paling spesifik ke paling umum):

```
ctx call → action uses.datastores → module datastores → App datastores
        → workspace binding (§4) → Infra Registry (service fisik)
```

Mengarah ke bawah = **mempersempit, tidak melebar**: level lebih rendah
tidak bisa membuat logical primitive baru atau menembus service yang tidak
lolos `access.filter`.

**Konsekuensi lintas-service.** Dua Module yang resolve ke service fisik
berbeda tidak pernah berbagi transaksi — mutasi yang menyentuh keduanya
**tidak mungkin** atomik dalam satu commit. Interaksi antar keduanya
**wajib** lewat event-subscribe/outbox
([`../backend/01-core-basic.md`](../backend/01-core-basic.md) §3, §7) —
tidak ada escape hatch `ctx.db` langsung ke service Module lain. Named
logical primitive (`.named()`) adalah satu-satunya jalur eksplisit ke
service lain, dan hanya ke yang teregistrasi di App Registry + dideklarasikan
di `uses.datastores`.

### 1.2 Named Logical Primitive

`ctx.db.named("analytics")` resolve alias dari App Registry milik App
pemanggil. Alias bersifat **app-scoped** — dua App boleh punya alias
`analytics` yang menunjuk service fisik berbeda. Alias yang tidak
teregistrasi → `DATASTORE_NOT_FOUND`; terdaftar tapi tidak dideklarasikan di
`uses.datastores` action → `DATASTORE_ACCESS_DENIED`.

## 2. Field Reference

`spec.serves` — daftar tipe primitive yang dilayani: **closed set 9
primitive**: `db` (`ctx.db`), `cache` (`ctx.cache`), `lock` (`ctx.lock`),
`queue` (`ctx.queue`), `pubsub` (`ctx.pubsub`), `storage` (`ctx.storage`),
`kvstore` (`ctx.kvstore`), `config` (`ctx.config`), `log` (`ctx.log`). Satu
backend fisik boleh melayani banyak primitive.

`spec.driver` — kompatibilitas dengan `serves` divalidasi `formspec apply`:

| Driver | Kompatibel `serves` |
|---|---|
| `sqlite`, `postgres` | `db`, `kvstore`, `config`, `log` |
| `valkey`, `redis` | `cache`, `lock`, `kvstore`, `queue`, `pubsub`, `config`, `log` |
| `s3`, `minio` | `storage` |
| `nats` | `queue`, `pubsub` |
| `memory` | `cache`, `lock`, `queue`, `pubsub`, `kvstore`, `config`, `log` |
| `fs` | `storage`, `log` |

`spec.connection` — `host`/`port`/`database` (driver-dependent), `pool.{
max_open (default 10), max_idle (default 5), max_lifetime}`, `lazy`
(default `false` — connect eager saat boot), `extra` (map parameter
spesifik-driver).

`spec.credential_ref` — URI `kms://{provider}/{path}`. **Normatif:**
kredensial tidak boleh inline di YAML. Standar lanjutannya: **tidak ada DSN
statis berumur panjang** untuk datastore produksi — kredensial per-koneksi
bersifat *short-lived* dan ber-TTL, diterbitkan dinamis oleh backend di
balik `credential_ref`, tiap penerbitan diaudit individual, dan kredensial
kedaluwarsa lalu rotate otomatis.

**Set primitive tertutup.** Daftar 9 primitive di atas adalah *closed set* —
app developer **tidak boleh** mendefinisikan primitive infrastruktur baru
sendiri. Kebutuhan yang tampak seperti primitive baru (scheduler, mail,
notification, seeder/factory) diwujudkan sebagai **module resmi di atas
primitive yang ada** (mail → `ctx.queue`, notification → `ctx.pubsub`),
bukan sebagai primitive tambahan.

## 3. Relasi dengan PersistBackend
Datastore menyediakan **koneksi**; PersistBackend
([`../backend/04-persist-backend.md`](../backend/04-persist-backend.md))
adalah implementasi kontrak penyimpanan entity yang **mengonsumsi** sebuah
Datastore ber-`serves: [db]`. Satu Datastore boleh dikonsumsi banyak
PersistBackend (mis. jsonb-persist di atas Datastore `driver: postgres`
yang sama dengan module lain yang pakai `ctx.db` mentah).

## 4. Workspace Binding (Level 3)

**Workspace Binding** adalah pemetaan logical name → service fisik per
workspace, dihasilkan Control Plane saat membangun snapshot:

- `access.filter` (**siapa** — environment/workspaces/labels, AND logic)
  menentukan service mana yang *terlihat* oleh workspace; yang tidak cocok
  tidak muncul sama sekali di snapshot-nya.
- `access.permission` (**boleh apa** — ceiling operasi `read`/`write`/
  `read_write` dengan rules glob per scope, longest match menang) menjadi
  batas atas yang tidak bisa dilampaui deklarasi `uses` module mana pun.
- Workspace **tidak bisa** meregistrasi logical primitive baru atau
  menembus service di luar filter — hanya memilih dan meng-override default
  di antara yang tersedia (mis. workspace enterprise → dedicated Postgres,
  workspace kecil → shared Postgres, tanpa mengubah manifest App).

Alur: Cloud Owner meregistrasi `kind: Datastore` di Control Plane →
Control Plane membangun snapshot per workspace (evaluasi `filter`; cocok →
service + ceiling permission masuk snapshot) → saat `ctx.*` dipanggil,
Resource Plane resolve lewat chain §1.1 dan cek ceiling `access.permission`
— operasi yang melampaui ceiling → `DATASTORE_PERMISSION_DENIED`.

## 5. Default per Primitive

Tiap tipe primitive punya satu **default service** — target `ctx.db()`
tanpa argumen. Default bersifat per-App (App1 dan App2 boleh berbeda),
dioverride per Module, dan pada akhirnya per workspace via binding (§4).

**Dev mode** (`formspec dev`): tanpa deklarasi apa pun, service `'default'`
auto-provision untuk semua primitive — `db`/`kvstore` → SQLite (database
utama app), `cache`/`lock`/`queue`/`pubsub`/`kvstore` → in-memory,
`storage` → filesystem lokal, `config` → KV-backed, `log` → in-memory —
zero config untuk memulai.

**Produksi:** Cloud Owner **wajib** meregistrasi minimal satu service untuk
tiap tipe primitive yang dipakai, dan default-nya dideklarasikan eksplisit
(App Registry / workspace binding) — tidak ada backend implisit.

## 6. Kode Error
| Kode | Kondisi |
|---|---|
| `DATASTORE_NOT_FOUND` | Service atau named alias tidak ditemukan di registry |
| `DATASTORE_ACCESS_DENIED` | Service/alias tidak dideklarasikan di `uses.datastores` |
| `DATASTORE_PERMISSION_DENIED` | Operasi melampaui ceiling `access.permission` |
| `DATASTORE_DRIVER_INCOMPATIBLE` | `serves` tidak kompatibel `driver` (saat `formspec apply`) |
| `DATASTORE_CREDENTIAL_MISSING` | `credential_ref` wajib tapi tidak diisi |

## 7. Lifecycle dan Kredensial
Registrasi Datastore murni tindakan Control Plane (§1) — didistribusikan ke
Resource Plane lewat snapshot Plane Protocol, tidak pernah lewat jalur lain.
Rotasi kredensial terjadi di balik `credential_ref` (KMS/Vault) tanpa
mengubah manifest Datastore itu sendiri. Health/konektivitas adalah urusan
`ConnectionPool` di sisi Resource Plane, di luar cakupan kontrak ini.

## 8. Peran Operasional Database (Normatif)
Koneksi operator/platform-managed ke datastore produksi memakai role
**least-privilege**, bukan superuser manusia:

- `formspec_ops_backup` — hanya privilege `REPLICATION`, eksklusif untuk
  tooling backup/restore fisik (mis. `pg_basebackup`, WAL-G, pgBackRest
  continuous archiving); tidak punya jalur baca/tulis SQL ke data aplikasi,
  hanya akses replication-stream.
- `formspec_ops_ddl` — privilege DDL skema (create/alter table, index) untuk
  migrasi struktural, dengan `NOSUPERUSER`, tanpa `GRANT OPTION`, tanpa
  `CREATEROLE`, dan `REVOKE ALL` eksplisit pada DML — role ini tidak bisa
  baca/tulis baris aplikasi, hanya ubah skema.

**Normatif:** tidak ada akun superuser manusia di cluster produksi
multi-tenant — seluruh akses operasional lewat role sempit dan
purpose-built ini.
