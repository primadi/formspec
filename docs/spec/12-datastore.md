# Forma Datastore Spec v0.1.0

**Status:** Draft
**License:** Creative Commons CC0
**Governed by:** Forma Overview · Forma Reference (D51–D59)
**Companions:** Core Basic v0.3.0 · Control Spec v0.1.0 · Plane Protocol v0.1.0

> `kind: Datastore` adalah Control Plane resource yang mendefinisikan **named infrastructure backend**.
> Semua definisi backend berasal dari Control Plane. Resource Plane hanya menerima
> datastore yang sudah diotorisasi untuk workspace-nya via Plane Protocol snapshot.

---

## 1. Scope & Fundamental Principles

1. **All datastores originate from the Control Plane.** Tidak ada mekanisme bagi Resource Plane untuk membuat atau mengubah definisi backend.
2. **Access is controlled by the Control Plane.** Dua dimensi: **siapa** (`filter`) dan **boleh apa** (`permission`).
3. **Technology transparency.** Developer di Resource Plane mereferensi datastore by name — tidak perlu tahu apakah backend-nya PostgreSQL, Valkey, atau Redis.
4. **Credentials never inline.** Semua kredensial lewat `credential_ref` ke KMS/Vault.

---

## 2. Manifest Format

```yaml
apiVersion: forma.dev/v1alpha1
kind: Datastore
metadata:
  name: prod-postgres          # kebab-case, unique per kind
  description: "..."           # recommended
spec:
  serves: [db, kvstore]        # which ctx.* primitives this datastore backs
  driver: postgres             # backend technology
  connection:                  # connection parameters
    host: pg-prod.internal
    port: 5432
    database: forma_shared
    pool:
      max_open: 100
      max_idle: 20
      max_lifetime: 1h
    lazy: false                # true = connect on first use
  credential_ref: kms://prod/postgres-shared
  access:                      # who can use + what they can do
    filter:                    # optional — if omitted, all workspaces
      environment: production
      workspaces: [corp-456]
      labels:
        tier: enterprise
    permission:                # optional — if omitted, read_write
      default: read            # ceiling for all operations
      rules:                   # granular overrides
        - scope: "store.*"
          access: read_write
        - scope: "billing.invoice"
          access: write
```

---

## 3. Field Reference

### 3.1 `spec.serves`

Daftar primitive type yang dilayani oleh datastore ini. Satu backend fisik bisa melayani banyak primitive.

| Value | Primitive |
|---|---|
| `db` | `ctx.db` |
| `cache` | `ctx.cache` |
| `lock` | `ctx.lock` |
| `queue` | `ctx.queue` |
| `pubsub` | `ctx.pubsub` |
| `storage` | `ctx.storage` |
| `config` | `ctx.config` |
| `kvstore` | `ctx.kvstore` |
| `log` | `ctx.log` |

### 3.2 `spec.driver`

| Driver | Kompatibel dengan `serves` |
|---|---|
| `sqlite` | `db`, `kvstore` |
| `postgres` | `db`, `kvstore` |
| `valkey` | `cache`, `lock`, `kvstore`, `queue`, `pubsub` |
| `redis` | `cache`, `lock`, `kvstore`, `queue`, `pubsub` |
| `s3` | `storage` |
| `minio` | `storage` |
| `nats` | `queue`, `pubsub` |
| `memory` | `cache`, `lock`, `queue`, `pubsub`, `kvstore` |
| `fs` | `storage` |

> Validasi: `forma apply` MUST reject datastore yang `serves`-nya tidak kompatibel dengan `driver`.

### 3.3 `spec.connection`

| Field | Type | Required | Notes |
|---|---|---|---|
| `host` | string | driver-dependent | |
| `port` | int | driver-dependent | |
| `database` | string | driver-dependent | DB name / bucket name / stream name |
| `pool.max_open` | int | no | Default: 10 |
| `pool.max_idle` | int | no | Default: 5 |
| `pool.max_lifetime` | string | no | e.g. "1h", "30m" |
| `lazy` | bool | no | true = connect on first use; false = eager at boot |
| `extra` | map[string]string | no | driver-specific parameters |

### 3.4 `spec.credential_ref`

URI ke KMS/Vault. **Normative: credentials MUST NOT be inlined in YAML.**

Format: `kms://{provider}/{path}`

### 3.5 `spec.access.filter`

Semua field opsional. Jika semua kosong atau `access` tidak ada → datastore tersedia untuk **semua workspace**.

| Field | Type | Notes |
|---|---|---|
| `environment` | string | Nama environment, e.g. "production" |
| `workspaces` | []string | Daftar workspace ID (1, 2, atau N) |
| `labels` | map[string]string | Cocokkan dengan labels workspace (AND logic) |

**AND logic:** Jika multiple field diisi, workspace harus memenuhi **semua** kriteria.

### 3.6 `spec.access.permission`

Ceiling untuk operasi pada datastore ini. Module `uses` declaration tidak bisa melampaui ceiling.

| Field | Type | Default | Notes |
|---|---|---|---|
| `default` | `read` / `write` / `read_write` | `read_write` | Baseline untuk semua scope |
| `rules` | []PermissionRule | — | Granular override |

**PermissionRule:**

| Field | Type | Notes |
|---|---|---|
| `scope` | string | Glob pattern: `"store.*"`, `"billing.invoice"`, `"*.*"` |
| `access` | `read` / `write` / `read_write` | Override `default` untuk scope ini |

**Resolution:** Rules di-override `default` untuk scope yang match. Scope paling spesifik menang (longest match).

---

## 4. Access Control Flow

```
1. Cloud Owner creates kind: Datastore in Control Plane

2. Control Plane builds snapshot per workspace:
   - Evaluates spec.access.filter against workspace
   - If match → includes datastore + permission ceiling in snapshot
   - If no match → datastore not in snapshot

3. Resource Plane receives snapshot:
   - Registry.Register() only for datastores in snapshot
   - Workspace CANNOT see datastores it's not authorized for

4. At runtime, when ctx.* primitive is called:
   - Resolve named datastore from registry
   - Check access.permission ceiling
   - If operation exceeds ceiling → DATASTORE_PERMISSION_DENIED
   - Module uses declaration cannot override ceiling
```

---

## 5. The `'default'` Datastore

Setiap primitive type memiliki konsep datastore **default** bernama `'default'`.

- `ctx.db()` tanpa argumen → auto-resolve ke datastore `'default'` tipe `db`
- `ctx.cache()` → auto-resolve ke datastore `'default'` tipe `cache`
- Dst.

**Dev mode:** `forma dev` auto-provisions 9 `'default'` datastores dengan backend ringan:
- `db`, `kvstore` → SQLite
- `cache`, `lock`, `queue`, `pubsub` → in-memory
- `storage` → local filesystem

**Production:** Cloud Owner HARUS mendefinisikan minimal datastore `'default'` untuk setiap primitive type yang digunakan.

---

## 6. Error Codes

| Code | HTTP | Condition |
|---|---|---|
| `DATASTORE_NOT_FOUND` | 500 | Named datastore tidak ditemukan di registry |
| `DATASTORE_ACCESS_DENIED` | 500 | Datastore tidak dideklarasi di `uses.datastores` |
| `DATASTORE_PERMISSION_DENIED` | 500 | Operasi melampaui `access.permission` ceiling |
| `DATASTORE_DRIVER_INCOMPATIBLE` | — | `serves` tidak kompatibel dengan `driver` (saat `forma apply`) |
| `DATASTORE_CREDENTIAL_MISSING` | — | `credential_ref` required tapi tidak diisi |

---

## 7. Design Decisions

| # | Decision |
|---|---|
| D51 | Semua `kind: Datastore` berasal dari Control Plane — single source of truth untuk infrastruktur backend |
| D52 | `spec.access` punya dua dimensi: `filter` (siapa) dan `permission` (boleh apa) |
| D53 | `'default'` datastore auto-provisioned per primitive type — backward compatible |
| D54 | `serves: [db, kvstore]` — satu backend fisik layani banyak primitive |
| D55 | Credentials selalu via `credential_ref` — tidak pernah inline |
| D56 | Dev mode auto-provisions semua `'default'` datastores — zero config |
| D57 | `00-kind-plane-mapping.md` sebagai referensi kanonik pemisahan plane |
| D58 | `access.permission` adalah ceiling — module `uses` tidak bisa melampaui |
| D59 | Filter model tunggal — tanpa filter = global; `workspaces: [id]` = dedicated; kombinasi AND logic |
