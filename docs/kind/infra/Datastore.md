# Datastore

<!-- generated:meta -->
| | |
|---|---|
| Grup | `infra` |
| Plane | `control` |
| Spec struct | `DatastoreSpec` |

<!-- /generated:meta -->

## Kapan Memakai

`kind: Datastore` adalah **koneksi infrastruktur bernama** — database/object-storage yang di-bind ke Module.

**Kapan memakai Datastore:**
- Mendeklarasikan koneksi database/object-storage bernama
- Binding `ctx.db()` per Module (default `'default'`)

**Kapan TIDAK pakai Datastore:**
- Menyusun data bisnis → `kind: Entity`
- Implementasi penyimpanan → `kind: PersistBackend`

> ⚠️ **Control Plane kind** — dikelola oleh Platform Operator.

**Sumber kontrak:** [`docs/spec/platform/06-datastore.md`](../spec/platform/06-datastore.md).

## Contoh Manifest

```yaml
apiVersion: formspec.dev/v1
kind: Datastore
metadata:
  name: default
spec:
  # dikelola Platform Operator — binding ke Module via Module.spec.datastore
  serves: [transactional, storage]
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

- **Multi-datastore per workspace dimungkinkan** — binding di level Module; beda datastore = beda deployment boundary = wajib async (event/outbox), TIDAK ADA escape hatch `ctx.db` lintas-Datastore.
- **`ctx.db()` tanpa argumen** resolve ke Datastore yang di-bind ke Module pemanggil (default `'default'`).
- **Akses `ctx.db` lintas-module wajib deklarasi di `uses`** — undeclared = diblokir + alert + suspend module (`USES_VIOLATION`).
- **Control Plane kind** — dikelola Platform Operator.
- **Cross-ref:** [`docs/spec/platform/06-datastore.md`](../spec/platform/06-datastore.md) · [`docs/spec/backend/01-core-basic.md`](../spec/backend/01-core-basic.md) §3 · [`ai_skills/formspec-kinds`](../../ai_skills/formspec-kinds/SKILL.md)
