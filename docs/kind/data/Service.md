# Service

<!-- generated:meta -->
| | |
|---|---|
| Grup | `data` |
| Plane | `resource` |
| Spec struct | `ServiceSpec` |

<!-- /generated:meta -->

## Kapan Memakai

`kind: Service` adalah komputasi stateless murni — tidak ada state yang
dipersist, tidak ada `characteristic`, `doc_status`, atau lifecycle guard.
Pilihan utama untuk **external integrations**: payment gateway, tax
calculator, notification, dan logika bisnis tanpa penyimpanan.

**Kapan memakai Service:**
- Komputasi murni dari input → output (tax, diskon, validasi)
- Membungkus integrasi eksternal (payment, email, SMS) — Wajib diwajibkan
  framework: external integrations MUST be wrapped
- Logika yang butuh permission + audit tapi tidak menyimpan data

**Kapan TIDAK pakai Service:**
- Data yang harus disimpan → `kind: Entity`
- Simulasi integrasi pihak ketiga untuk testing → `kind: Mockup`
- Reaksi ke event resource lain → `kind: Subscription`

**Sumber kontrak:** [`docs/spec/backend/01-core-basic.md`](../spec/backend/01-core-basic.md) §1.1.

## Contoh Manifest

```yaml
apiVersion: formspec.dev/v1
kind: Service
metadata:
  name: tax-calculator
  module: billing
spec:
  version: v1
  actions:
    - name: calculate
      description: "Hitung pajak dari amount"
      required_permission: billing.tax-calculator.calculate
      uses:
        - ctx.log
      impl:
        type: native
        ref: "TaxService.Calculate"
```

> Catatan: `ServiceSpec` di `pkg/spec` saat ini mengekspos `actions` (bukan
> `inputs`/`outputs`/`handler` dari contoh awal docs). Pastikan mengikuti
> skema yang aktif — lihat `schemas/kinds/Service.schema.json`.

## Atribut

<!-- generated:attributes -->
| Atribut | Tipe | Wajib | Contoh | Deskripsi |
|---|---|---|---|---|
| `version` | `string` | ✅ | v1 |  |
| `actions` | []`Action` | — |  |  |
| `auth` | `EntityAuth` | — |  |  |

<!-- /generated:attributes -->

## Gotchas

- **Tidak ada lifecycle** — tidak ada `doc_status`, tidak ada guard submit/
  cancel/amend, tidak ada `characteristic`.
- **Tiap action wajib** `required_permission` + `uses` — deklarasi adalah
  satu-satunya sumber kebenaran; grant tidak diturunkan dari pemakaian kode.
- **External integrations WAJIB dibungkus** sebagai `kind: Service` — jangan
  panggil API eksternal langsung dari script/action Entity.
- **Struktur `ServiceSpec`**: pastikan mengikuti skema aktif (`actions`) —
  bukan bentuk `inputs`/`outputs`/`handler` yang usang.
- **Cross-ref:** [`docs/spec/backend/01-core-basic.md`](../spec/backend/01-core-basic.md) §1 · [`ai_skills/formspec-kinds`](../../ai_skills/formspec-kinds/SKILL.md)
