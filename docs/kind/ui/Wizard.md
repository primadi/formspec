# Wizard

<!-- generated:meta -->
| | |
|---|---|
| Grup | `ui` |
| Plane | `resource` |
| Spec struct | `WizardSpec` |

<!-- /generated:meta -->

## Kapan Memakai

`kind: Wizard` adalah **proses bisnis sekuensial multi-step lintas entity** — framework mengurus stepper, validasi per-step, dependency antar-field, autosave, dan perilaku completion.

**Kapan memakai Wizard:**
- Alur bertahap (registrasi pasien, checkout multi-langkah)
- Entry kompleks yang butuh step sekuensial + validasi per step

**Kapan TIDAK pakai Wizard:**
- Proses cuma section form linear tanpa penegakan sekuensial → `kind: Form` dengan `sections`

**Sumber kontrak:** [`docs/spec/frontend/06-page-kinds.md`](../spec/frontend/06-page-kinds.md) §6.

## Contoh Manifest

```yaml
apiVersion: formspec.dev/v1
kind: Wizard
metadata:
  name: patient-registration
  module: clinic
spec:
  title: "Patient Registration — {step.title}"
  entity: visit
  on_complete:
    restart: true
    banner:
      - { label: "Queue Number", field: response.queue_number }
  steps:
    - title: "Find Patient"
      layout: search_select
      entity: patient
      search_fields: [nik, name, phone]
      allow_create: true
    - title: "Confirm & Submit"
      on_enter: prefill-visit-defaults
      summary:
        - { label: "Patient", field: patient.name }
```

## Atribut

<!-- generated:attributes -->
| Atribut | Tipe | Wajib | Contoh | Deskripsi |
|---|---|---|---|---|
| `public` | `boolean` | — | true | If true (default), a route /module/wizard/<name> is auto-generated. |
| `title` | `string` | ✅ |  |  |
| `entity` | `string` | — | clinic.visit |  |
| `action` | `string` | — |  | server action that commits all steps; if empty, final step plain-creates Entity |
| `on_complete` | `WizardOnComplete` | — |  |  |
| `steps` | []`WizardStep` | — |  |  |

<!-- /generated:attributes -->

## Gotchas

- **`action` (level wizard) opsional** — diisi: action server-side atomik menulis seluruh data step. Tidak diisi: step akhir `create` biasa di `entity`.
- **Step sekuensial** — step N wajib selesai sebelum N+1; Back selalu diizinkan (data step N-1 tersimpan).
- **`depends_on` = filter chain client-side (UX-only)** — validasi server tetap otoritas.
- **Route sendiri** `/wizard/:name`; state di URL `?step=2`; instance `?instance=<id>`; autosave ke `localStorage` kunci `wizard:{name}:{instance}` (multi-tab aman).
- **Cross-ref:** [`docs/spec/frontend/06-page-kinds.md`](../spec/frontend/06-page-kinds.md) §6 · [`ai_skills/formspec-kinds`](../../ai_skills/formspec-kinds/SKILL.md)
