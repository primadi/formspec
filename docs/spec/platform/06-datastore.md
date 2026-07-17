# Datastore

**Version:** 0.1.0 · **Status:** Draft

> Draft: isi di bawah kontrak yang berlaku.

## 1. Kind Datastore
`kind: Datastore` adalah resource **Control Plane** yang mendefinisikan
backend infrastruktur bernama. Semua definisi backend berasal dari Control
Plane — Resource Plane tidak punya mekanisme membuat atau mengubah definisi
backend sendiri, ia cuma menerima Datastore yang sudah diotorisasi untuk
workspace-nya lewat snapshot Plane Protocol
([`05-plane-protocol.md`](05-plane-protocol.md) §4.1).

Prinsip: **transparansi teknologi** — developer di Resource Plane
mereferensi Datastore by name lewat `ctx.*` (daftar primitive: §2 Field
Reference), tidak perlu tahu apakah di baliknya Postgres, SQLite, atau
Valkey. **Kredensial tidak pernah
inline** — selalu lewat `credential_ref` ke KMS/Vault.

```yaml
apiVersion: forma.dev/v1alpha1
kind: Datastore
metadata:
  name: prod-postgres
spec:
  serves: [db, kvstore]        # primitive ctx.* yang dilayani datastore ini
  driver: postgres
  connection:
    host: pg-prod.internal
    port: 5432
    database: forma_shared
    pool: { max_open: 100, max_idle: 20, max_lifetime: 1h }
    lazy: false                # true = connect saat pemakaian pertama
  credential_ref: kms://prod/postgres-shared
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

## 2. Field Reference
`spec.serves` — daftar tipe primitive yang dilayani: `db` (`ctx.db`),
`cache` (`ctx.cache`), `lock` (`ctx.lock`), `queue` (`ctx.queue`), `pubsub`
(`ctx.pubsub`), `storage` (`ctx.storage`), `config`, `kvstore`, `log`. Satu
backend fisik boleh melayani banyak primitive.

`spec.driver` — kompatibilitas dengan `serves` divalidasi `forma apply`:

| Driver | Kompatibel `serves` |
|---|---|
| `sqlite`, `postgres` | `db`, `kvstore` |
| `valkey`, `redis` | `cache`, `lock`, `kvstore`, `queue`, `pubsub` |
| `s3`, `minio`, `fs` | `storage` |
| `nats` | `queue`, `pubsub` |
| `memory` | `cache`, `lock`, `queue`, `pubsub`, `kvstore` |

`spec.connection` — `host`/`port`/`database` (driver-dependent), `pool.{
max_open (default 10), max_idle (default 5), max_lifetime}`, `lazy`
(default `false` — connect eager saat boot), `extra` (map parameter
spesifik-driver).

`spec.credential_ref` — URI `kms://{provider}/{path}`. **Normatif:**
kredensial tidak boleh inline di YAML. Standar lanjutannya: **tidak ada DSN
statis berumur panjang** (mis. env var `FORMA_DB_DSN` permanen berisi
password) untuk datastore produksi — kredensial per-koneksi bersifat
*short-lived* dan ber-TTL, diterbitkan dinamis oleh backend di balik
`credential_ref` (mis. fitur dynamic database credentials Vault), tiap
penerbitan diaudit individual, dan kredensial kedaluwarsa lalu rotate
otomatis — bukan secret statis yang dirotasi manual.

**Set primitive tertutup.** Daftar tipe primitive di atas adalah *closed
set* — app developer **tidak boleh** mendefinisikan primitive infrastruktur
baru sendiri. Kebutuhan yang tampak seperti primitive baru (scheduler, mail,
notification, seeder/factory) diwujudkan sebagai **module resmi di atas
primitive yang ada** (mail → `ctx.queue`, notification → `ctx.pubsub`,
`forma/scheduler`/`forma/seed` di atas primitive terkait), bukan sebagai
primitive tambahan. Set ini tetap tertutup supaya kontrak Datastore dan
PersistBackend tidak perlu tumbuh setiap kali muncul kebutuhan konvenien
baru.

## 3. Relasi dengan PersistBackend
Datastore menyediakan **koneksi**; PersistBackend
([`../backend/04-persist-backend.md`](../backend/04-persist-backend.md))
adalah implementasi kontrak penyimpanan entity yang **mengonsumsi** sebuah
Datastore ber-`serves: [db]`. Satu Datastore boleh dikonsumsi banyak
PersistBackend (mis. jsonb-persist di atas Datastore `driver: postgres`
yang sama dengan module lain yang pakai `ctx.db` mentah).

## 4. Access Control
Dua dimensi: `access.filter` (**siapa** — workspace mana yang dapat
Datastore ini di snapshot-nya) dan `access.permission` (**boleh apa** —
ceiling operasi; deklarasi `uses` module tidak bisa melampaui ceiling ini).

`access.filter` — semua field opsional, **AND logic** kalau lebih dari satu
diisi: `environment` (nama environment), `workspaces` (daftar ID eksplisit),
`labels` (cocok dengan label workspace). Kosong/`access` absen → tersedia
untuk semua workspace.

`access.permission` — `default` (`read`/`write`/`read_write`, default
`read_write`) adalah baseline; `rules[]` (`scope` glob pattern + `access`)
meng-override `default` untuk scope yang cocok, **scope paling spesifik
menang** (longest match).

Alur: Cloud Owner membuat `kind: Datastore` di Control Plane → Control
Plane membangun snapshot per workspace (evaluasi `filter`; cocok →
Datastore + ceiling permission masuk snapshot; tidak cocok → Datastore
tidak muncul sama sekali, workspace **tidak bisa melihatnya**) → saat
`ctx.*` dipanggil runtime, Resource Plane resolve Datastore bernama dari
registry, cek `access.permission` ceiling — operasi yang melampaui ceiling
→ `DATASTORE_PERMISSION_DENIED`.

## 5. Datastore `'default'`
Tiap tipe primitive punya konsep Datastore default bernama `'default'` —
`ctx.db()` tanpa argumen auto-resolve ke Datastore `'default'` tipe `db`,
dst. **Dev mode** (`forma dev`) auto-provision `'default'` untuk semua tipe
primitive dengan backend ringan: `db`/`kvstore` → SQLite; `cache`/`lock`/
`queue`/`pubsub` → in-memory; `storage` → filesystem lokal — zero config.
**Produksi:** Cloud Owner **wajib** mendefinisikan minimal Datastore
`'default'` untuk tiap tipe primitive yang dipakai.

## 6. Kode Error
| Kode | Kondisi |
|---|---|
| `DATASTORE_NOT_FOUND` | Datastore bernama tidak ditemukan di registry |
| `DATASTORE_ACCESS_DENIED` | Datastore tidak dideklarasikan di `uses.datastores` module |
| `DATASTORE_PERMISSION_DENIED` | Operasi melampaui ceiling `access.permission` |
| `DATASTORE_DRIVER_INCOMPATIBLE` | `serves` tidak kompatibel `driver` (saat `forma apply`) |
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

- `forma_ops_backup` — hanya privilege `REPLICATION`, eksklusif untuk
  tooling backup/restore fisik (mis. `pg_basebackup`, WAL-G, pgBackRest
  continuous archiving); tidak punya jalur baca/tulis SQL ke data aplikasi,
  hanya akses replication-stream.
- `forma_ops_ddl` — privilege DDL skema (create/alter table, index) untuk
  migrasi struktural, dengan `NOSUPERUSER`, tanpa `GRANT OPTION`, tanpa
  `CREATEROLE`, dan `REVOKE ALL` eksplisit pada DML — role ini tidak bisa
  baca/tulis baris aplikasi, hanya ubah skema.

**Normatif:** tidak ada akun superuser manusia di cluster produksi
multi-tenant — seluruh akses operasional lewat role sempit dan
purpose-built ini.
