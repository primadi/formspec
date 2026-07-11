# Forma Kind → Plane Mapping

**Version:** 1.0
**Status:** Draft
**License:** Creative Commons CC0
**Governed by:** Forma Overview · Forma Reference

> Dokumen kanonik yang memetakan **semua** Forma resource kind ke plane tempatnya hidup.
> Setiap kali kind baru diperkenalkan, dokumen ini adalah referensi otoritatif untuk
> menentukan apakah kind tersebut masuk ke Control Plane atau Resource Plane.

---

## 1. Aturan Normatif

1. **Control Plane kinds** MUST NOT read business data or execute business handlers.
2. **Resource Plane kinds** MUST NOT modify governance state.
3. **Rule of thumb:** Jika kind mengonfigurasi infrastruktur, governance, deployment, atau keamanan platform → **Control Plane**. Jika kind mendefinisikan domain logic, UI, atau business behavior → **Resource Plane**.

---

## 2. Mapping Table

| Kind | Plane | Binary | Defined In | Notes |
|---|---|---|---|---|
| `App` | Resource | `forma-resource` | Core Basic | Root project manifest, unit deployment |
| `Module` | Resource | `forma-resource` | Core Basic | Package of code + manifests |
| `Document` | Resource | `forma-resource` | Core Basic | Stateful data (formerly `Entity`) |
| `Service` | Resource | `forma-resource` | Core Basic | Stateless computation |
| `Config` | Resource | `forma-resource` | Core Basic | Module configuration |
| `Migration` | Resource | `forma-resource` | Core Basic | Data migration |
| `Subscription` | Resource | `forma-resource` | Core Basic | Event subscription |
| `Workflow` | Resource | `forma-resource` | Core Extended | Approval attached to state machine |
| `Api` | Resource | `forma-resource` | Core Extended | API exposure override |
| `Webhook` | Resource | `forma-resource` | Core Extended | Verified inbound endpoints |
| `Mockup` | Resource | `forma-resource` | Core Extended | Third-party simulation |
| `Integrator` | Resource | `forma-resource` | Core Extended | Cross-module bridge |
| `KindDefinition` | Resource | `forma-resource` | Core Extended | Extension mechanism (à la CRD) |
| `Page` | Resource | `forma-resource` | Frontend Spec | Route + UI composition |
| `Form` | Resource | `forma-resource` | Frontend Spec | Input/edit layout |
| `Table` | Resource | `forma-resource` | Frontend Spec | List/browse view |
| `Dashboard` | Resource | `forma-resource` | Frontend Spec | Widget canvas |
| `Widget` | Resource | `forma-resource` | Frontend Spec | Single dashboard widget |
| `Report` | Resource | `forma-resource` | Frontend Spec | Parameterized tabular report |
| `Wizard` | Resource | `forma-resource` | Frontend Spec | Multi-step business process |
| `Kanban` | Resource | `forma-resource` | Frontend Spec | Drag-and-drop status board |
| `Timeline` | Resource | `forma-resource` | Frontend Spec | Chronological event journal |
| `Menu` | Resource | `forma-resource` | Frontend Spec | Navigation tree |
| `Print` | Resource | `forma-resource` | Frontend Spec | Printable document |
| `Theme` | Resource | `forma-resource` | Frontend Spec | Look & feel (CSS variables) |
| `Environment` | Control | `forma-control` | Control Spec | Deployment target, mode, tier |
| `Policy` | Control | `forma-control` | Control Spec | Governance rules (OPA/Rego) |
| `Datastore` | Control | `forma-control` | Datastore Spec | Named infrastructure backend |

---

## 3. Adding a New Kind

Saat menambahkan kind baru, jawab pertanyaan berikut:

1. **Apakah kind ini mengonfigurasi infrastruktur atau governance?** → Control Plane
2. **Apakah kind ini mendefinisikan business logic, UI, atau domain model?** → Resource Plane
3. **Apakah kind ini perlu membaca business data?** → HARUS Resource Plane (Control Plane tidak boleh baca business data)
4. **Apakah kind ini perlu diatur oleh Cloud Owner, bukan App Developer?** → Control Plane

Setelah plane ditentukan, tambahkan entry ke tabel di §2 dan update dokumen spec terkait.
