# KindDefinition

<!-- generated:meta -->
| | |
|---|---|
| Grup | `data` |
| Plane | `resource` |
| Spec struct | `KindDefinitionSpec` |

<!-- /generated:meta -->

## Kapan Memakai

`kind: KindDefinition` adalah **mekanisme ekstensi** — mendeklarasikan kind baru (CRD-like).

**Kapan memakai KindDefinition:**
- Module resmi mendaftarkan kind baru (`Seed`, `Schedule`, `MailTemplate`)
- Module pihak ketiga dengan kind namespaced

**Kapan TIDAK pakai KindDefinition:**
- Membangun aplikasi biasa — **95% kasus jawabannya `Entity`**. Butuh kind baru berarti memperluas framework, bukan membangun app.

**Sumber kontrak:** [`docs/spec/platform/03-kind-system.md`](../spec/platform/03-kind-system.md) §2.

## Contoh Manifest

```yaml
apiVersion: formspec.dev/v1alpha1
kind: KindDefinition
metadata:
  name: Seed
  module: formspec/seed
spec:
  group: seed.formspec.dev
  version: v1
  scope: module
  handler:
    type: native
    ref: "FormaSeed.Apply"
```

## Atribut

<!-- generated:attributes -->
| Atribut | Tipe | Wajib | Contoh | Deskripsi |
|---|---|---|---|---|
| `group` | `string` | ✅ | seed.formspec.dev |  |
| `version` | `string` | ✅ | v1 |  |
| `schema` | any | — |  |  |
| `handler` | `ImplDecl` | ✅ |  |  |
| `scope` | enum (module · app) | — | module |  |

<!-- /generated:attributes -->

## Gotchas

- **Penamaan dinamespace via grup `apiVersion`** (pola CRD) — built-in pakai `formspec.dev`, module pakai grup sendiri — tabrakan namespace mustahil.
- **Handler berjalan di bawah `uses` module pendeklarasi** — KindDefinition tidak memberi kekuatan runtime di luar footprint module-nya.
- **Tiga layer ekstensi**: built-in spec → module resmi (KindDefinition) → pihak ketiga namespaced (Verified Badge).
- **Cross-ref:** [`docs/spec/platform/03-kind-system.md`](../spec/platform/03-kind-system.md) §2 · [`ai_skills/formspec-kinds`](../../ai_skills/formspec-kinds/SKILL.md)
