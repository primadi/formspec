# Datastore

<!-- generated:meta -->
| | |
|---|---|
| Grup | `infra` |
| Plane | `control` |
| Spec struct | `DatastoreSpec` |

<!-- /generated:meta -->

## Kapan Memakai

`kind: Datastore` adalah **registrasi service infrastruktur fisik** di Infra
Registry (level 1) — satu instance nyata (Postgres, Valkey, MinIO, SQLite,
filesystem) dengan logical name, yang melayani satu atau lebih `ctx.*`
primitive.

**Kapan memakai Datastore:**
- Meregistrasi service infrastruktur (db, cache, storage, dst) dengan logical name
- Menyediakan banyak service untuk primitive yang sama (mis. 2 database: `pg-main` + `pg-analytics`)
- Menjadi target seleksi App Registry (`App.spec.datastores` / `Module.spec.datastores`)

**Kapan TIDAK pakai Datastore:**
- Menyusun data bisnis → `kind: Entity`
- Implementasi penyimpanan → `kind: PersistBackend`

> ⚠️ **Control Plane kind** — dikelola oleh Platform Operator.

**Sumber kontrak:** [`docs/spec/platform/06-datastore.md`](../spec/platform/06-datastore.md) — model 3-level (Infra Registry → App Registry → Workspace Binding), chain resolusi, named logical primitive.

## Contoh Manifest

```yaml
apiVersion: formspec.dev/v1
kind: Datastore
metadata:
  name: pg-analytics
spec:
  serves: [db, kvstore]   # primitive ctx.* yang dilayani service ini
  driver: postgres
  connection:
    host: pg-analytics.internal
    port: 5432
    database: formspec_analytics
  credential_ref: kms://prod/pg-analytics
```

Seleksi di App/Module (level 2):

```yaml
# kind: App
spec:
  datastores:
    db: pg-main              # default db App ini
    db/analytics: pg-analytics   # named primitive → ctx.db.named("analytics")
```

## Atribut

<!-- generated:attributes -->
| Atribut | Tipe | Wajib | Contoh | Deskripsi |
|---|---|---|---|---|
| `serves` | []enum (db · cache · lock · queue · pubsub · storage · config · kvstore · …) | — | [db] | Serves lists which ctx.* primitives this datastore backs. |
| `driver` | enum (sqlite · postgres · valkey · redis · s3 · minio · nats · memory · …) | ✅ | postgres | Driver identifies the backend technology. |
| `connection` | `DatastoreConnection` | ✅ |  | Connection holds connection parameters for the backend. |
| `credential_ref` | `string` | — | kms://workspace-default | CredentialRef is a reference to KMS/Vault for credentials. |
| `access` | `DatastoreAccess` | — |  | Access controls who (filter) can use this datastore and what |

<!-- /generated:attributes -->

## Gotchas

- **Multi-service per primitive didukung** — satu primitive boleh dilayani banyak service; tiap primitive punya satu default (per-App, overridable per Module/workspace).
- **`ctx.db()` tanpa argumen** resolve lewat chain: action `uses.datastores` → module `datastores` → App `datastores` → workspace binding → service fisik (06-datastore.md §1.1).
- **Named logical primitive** — `ctx.db.named("analytics")` hanya ke alias yang teregistrasi di App Registry (`db/analytics`); unknown → `DATASTORE_NOT_FOUND`, tidak dideklarasikan di `uses.datastores` → `DATASTORE_ACCESS_DENIED`.
- **Beda service fisik = beda deployment boundary** — interaksi lintas service wajib async (event/outbox); tidak ada escape hatch `ctx.db` langsung.
- **Kredensial tidak pernah inline** — selalu `credential_ref` ke KMS/Vault.
- **Control Plane kind** — dikelola Platform Operator.
- **Cross-ref:** [`docs/spec/platform/06-datastore.md`](../spec/platform/06-datastore.md) · [`docs/spec/backend/01-core-basic.md`](../spec/backend/01-core-basic.md) §3 · [`ai_skills/formspec-kinds`](../../ai_skills/formspec-kinds/SKILL.md)
