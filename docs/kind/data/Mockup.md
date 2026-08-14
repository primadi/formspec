# Mockup

<!-- generated:meta -->
| | |
|---|---|
| Grup | `data` |
| Plane | `resource` |
| Spec struct | `MockupSpec` |

<!-- /generated:meta -->

## Kapan Memakai

`kind: Mockup` adalah **simulasi integrasi pihak ketiga** untuk testing/dev — menggantikan Service eksternal yang belum ada atau mahal dipanggil.

**Kapan memakai Mockup:**
- Development/testing tanpa server payment/sms yang nyata
- Demo aplikasi tanpa bergantung provider eksternal
- Merespons `for: <module.service>` atau `<module.entity>` dengan perilaku simulasi

**Kapan TIDAK pakai Mockup:**
- Integrasi nyata → `kind: Service`
- Provider inbound → `kind: Webhook`

**Sumber kontrak:** [`docs/spec/backend/02-core-extended.md`](../spec/backend/02-core-extended.md) §8.

## Contoh Manifest

```yaml
apiVersion: formspec.dev/v1
kind: Mockup
metadata:
  name: midtrans-mock
  module: billing
spec:
  for: billing.payment-gateway
  config_ref: billing.mockup-config
```

## Atribut

<!-- generated:attributes -->
| Atribut | Tipe | Wajib | Contoh | Deskripsi |
|---|---|---|---|---|
| `for` | `string` | ✅ | billing.payment-gateway |  |
| `config_ref` | `string` | — |  |  |

<!-- /generated:attributes -->

## Gotchas

- **`for` wajib** — menunjuk module.service atau module.entity yang disimulasikan (`module.name` qualified).
- **Simulasi bukan pengganti** integrasi nyata — untuk dev/testing, jangan sampai ter-publish ke produksi tanpa di-review.
- **Cross-ref:** [`docs/spec/backend/02-core-extended.md`](../spec/backend/02-core-extended.md) §8 · [`ai_skills/formspec-kinds`](../../ai_skills/formspec-kinds/SKILL.md)
