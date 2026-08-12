# Webhook

<!-- generated:meta -->
| | |
|---|---|
| Grup | `data` |
| Plane | `resource` |
| Spec struct | `WebhookSpec` |

<!-- /generated:meta -->

## Kapan Memakai

`kind: Webhook` adalah **endpoint masuk terverifikasi** — inbound endpoint dengan verifikasi signature.

**Kapan memakai Webhook:**
- Provider mengirim data ke aplikasi (payment callback, delivery update)
- Endpoint yang perlu idempotency + signature verification

**Kapan TIDAK pakai Webhook:**
- Aplikasi memanggil keluar ke provider → `kind: Service` (wrap external integration)
- Endpoint internal biasa → cukup action Entity + expose

**Sumber kontrak:** [`docs/spec/backend/02-core-extended.md`](../spec/backend/02-core-extended.md) §4.

## Contoh Manifest

```yaml
apiVersion: formspec.dev/v1alpha1
kind: Webhook
metadata:
  name: midtrans-payment
  module: billing
spec:
  for: billing.invoice
  method: POST
  path: /webhooks/midtrans
  auth:
    strategy: signature
    signature:
      header: X-Midtrans-Signature
  idempotent: true
```

## Atribut

<!-- generated:attributes -->
| Atribut | Tipe | Wajib | Contoh | Deskripsi |
|---|---|---|---|---|
| `for` | `string` | ✅ | billing.invoice |  |
| `method` | enum (GET · POST · PUT · PATCH · DELETE) | ✅ | POST |  |
| `path` | `string` | — | /webhooks/midtrans |  |
| `auth` | `WebhookAuth` | ✅ |  |  |
| `idempotent` | `boolean` | — |  |  |
| `idempotency_key` | `IdempotencyDecl` | — |  |  |

<!-- /generated:attributes -->

## Gotchas

- **Verifikasi wajib** — endpoint inbound harus punya auth strategy (signature/key) + idempotency untuk melindungi double-delivery.
- **External integrations MUST be wrapped** — arah keluar (outbound) bukan Webhook, itu `kind: Service`.
- **Cross-ref:** [`docs/spec/backend/02-core-extended.md`](../spec/backend/02-core-extended.md) §4 · [`ai_skills/formspec-kinds`](../../ai_skills/formspec-kinds/SKILL.md)
