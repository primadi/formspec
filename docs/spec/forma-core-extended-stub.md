# Forma Core Extended — Stub (Mockup, Webhook, Environment Binding)

**Status:** Stub — definisi minimal untuk menjadi acuan example dokumen.
**Governed by:** Forma Foundation Document v1.9.
**Scope:** `kind: Mockup`, `kind: Webhook`, mekanisme environment binding.

> Ini bukan full Core Extended spec. Hanya konstruksi yang diperlukan
> untuk menulis example Midtrans Payment Gateway dan pola integrasi
> eksternal. Bagian lain dari Extended (hooks, storage field type, LB,
> circuit breaker, dll.) akan menyusul di dokumen terpisah.

---

## 1. `kind: Mockup`

Mockup adalah simulasi service eksternal yang berjalan di development
environment. Ia mengimplementasikan kontrak yang sama dengan konektor
asli (action names, params, response shape) — sehingga pemanggil
(`ctx.resource.call(...)`) tidak perlu tahu apakah sedang berbicara
dengan mockup atau konektor asli.

### 1.1 Spec

```yaml
apiVersion: forma.dev/v1alpha1
kind: Mockup
metadata:
  name: midtrans-mock
  module: billing
  description: Simulasi Midtrans API untuk development & CI
spec:
  for: payment-gateway            # Service yang di-mock
  actions:
    - name: create-session
      impl: { type: script_ref, ref: billing/midtrans_mock_create_session }
    - name: webhook
      impl: { type: script_ref, ref: billing/midtrans_mock_webhook }
```

### 1.2 Rules

- `spec.for` MUST reference a `kind: Service` in the same app.
- Setiap action yang di-mock HARUS punya nama dan signature yang sama
  dengan action asli di Service target.
- Mockup HANYA aktif di environment yang dikonfigurasi (lihat §3).
  Di production, mockup diabaikan — request selalu ke konektor asli.
- `forma dev` otomatis mengaktifkan semua mockup; `forma test` juga.

---

## 2. `kind: Webhook`

Webhook adalah definisi inbound HTTP endpoint yang diverifikasi
signature-nya oleh framework SEBELUM handler dijalankan.

### 2.1 Spec

```yaml
apiVersion: forma.dev/v1alpha1
kind: Webhook
metadata:
  name: midtrans-webhook
  module: billing
  description: Menerima callback pembayaran dari Midtrans
spec:
  for: payment-gateway.webhook     # action Service yang menangani
  method: POST
  path: /webhooks/midtrans         # auto-derived jika tidak diisi
  auth:
    strategy: signature            # signature | token
    signature:
      algorithm: hmac-sha512       # hmac-sha256 | hmac-sha512 | rsa
      header: X-Midtrans-Signature
      key: { config: midtrans.server_key, secret: true }
      # payload untuk verifikasi: seluruh raw body
  idempotent: true                 # diwariskan dari action; framework yang enforce
```

### 2.2 Rules

- `spec.for` MUST reference satu action di `kind: Service` yang akan
  menangani verified payload.
- Verifikasi signature dijalankan framework SEBELUM handler — handler
  hanya menerima payload yang sudah verified. Handler TIDAK perlu
  menulis kode verifikasi sendiri.
- `auth.strategy: token` untuk webhook yang pakai static token/bearer
  (lebih sederhana, kurang aman — cocok untuk internal webhook).
- `auth.strategy: signature` untuk webhook dengan kriptografi (midtrans,
  xendit, stripe, dll.).
- Idempotency ditegakkan framework (§11.8/D32) — `idempotency_key`
  diambil dari field payload (misal `transaction_id`).

---

## 3. Environment Binding — Bagaimana Service Tahu Mock vs Real

### 3.1 Environment awareness di runtime

Setiap environment (dev, staging, production) didefinisikan di Control
Plane (`forma-control.md`). Resource Plane menyediakan:

- `ctx.environment` — nama environment aktif (`"dev"`, `"production"`)
- `ctx.config.get(key)` — resolve nilai config per-environment

Tidak ada `if ctx.environment == "dev"` di handler bisnis. Sebagai
gantinya, framework memutuskan routing ke mockup atau konektor asli
berdasarkan config:

```yaml
# config/midtrans.yaml
apiVersion: forma.dev/v1alpha1
kind: Config
metadata:
  name: midtrans
  module: billing
spec:
  keys:
    mock_enabled:
      type: bool
      default: true           # dev: true; production: false
    server_key:
      type: string
      secret: true
      default: ""             # diisi per-environment via Control Plane
    merchant_id:
      type: string
      default: ""
```

### 3.2 Mekanisme routing

Framework mengevaluasi saat `ctx.resource.call("payment-gateway", ...)`:

1. Jika `mock_enabled: true` di config environment aktif → request
   ke `kind: Mockup` yang mereferensi Service tersebut.
2. Jika `mock_enabled: false` atau tidak ada mockup → request ke
   konektor asli (`impl: native` di Service).
3. Jika tidak ada mockup DAN tidak ada konektor asli → error.

Dengan mekanisme ini, handler checkout di Order-to-Cash cukup memanggil:

```python
session = payment_gateway.call("create-session", {...})
```

Tanpa perlu tahu apakah sedang dev atau production. Framework yang
menyelesaikan routing berdasarkan config environment.

### 3.3 Pattern umum

| Environment | `mock_enabled` | Efek |
|---|---|---|
| dev (`forma dev`) | `true` (default) | Mockup aktif; tidak ada panggilan HTTP keluar |
| CI/test | `true` | Mockup aktif; test deterministik |
| staging | `false` | Konektor asli dengan sandbox API key |
| production | `false` | Konektor asli dengan live API key |

---

## 4. Open Questions (untuk full Extended spec)

- Apakah mockup perlu state? (simulasi database transaksi mock)
- Apakah satu Service bisa punya multiple mockup? (untuk test scenario berbeda)
- Delay/injection untuk simulasi failure di mockup?
- Webhook: apakah perlu replay/retry dari framework untuk missed webhook?
- Apakah `kind: Webhook` bisa independen tanpa Service? (webhook yang langsung
  memicu action Entity tanpa perantara Service)
