# Forma Example: Midtrans Payment Gateway Service

**Status:** Draft — test drive `kind: Service`, `kind: Mockup`, `kind: Webhook`,
dan environment binding berdasarkan Core Basic v0.2.0 + Extended stub.
**Fungsi:** contoh kanonik integrasi eksternal — dari mock dev sampai live API.

---

## 1. Kebutuhan Bisnis: Payment Gateway Wrapper

**Alur:** checkout → buat sesi pembayaran di Midtrans → customer bayar →
Midtrans kirim webhook → verifikasi → teruskan ke order.

| # | Requirement | Kenapa sulit tanpa konvensi |
|---|---|---|
| FR1 | Saat dev/CI: simulasi tanpa panggil API asli | Mock dan real harus punya kontrak sama |
| FR2 | Saat production: panggil Midtrans API asli | Kunci API tidak boleh bocor ke kode |
| FR3 | Webhook Midtrans diverifikasi (HMAC-SHA512) | Verifikasi manual = rawan salah/lupa |
| FR4 | Webhook bisa dikirim berkali-kali | Idempotency wajib |
| FR5 | Environment switch tanpa ubah kode | Config per-environment, bukan if-else |
| FR6 | Semua panggilan tercatat log terstruktur | Observability via `ctx.log` |

---

## 2. Dari FR ke Konstruksi Forma

### 2.1 Struktur project

```
midtrans-gateway/
  module.yaml
  services/
    midtrans.yaml              # kind: Service (konektor asli)
  mockups/
    midtrans-mock.yaml         # kind: Mockup (simulasi dev)
  webhooks/
    midtrans-webhook.yaml      # kind: Webhook (verifikasi signature)
  scripts/
    midtrans_create_session.star
    midtrans_webhook.star
    midtrans_mock_create_session.star
    midtrans_mock_webhook.star
  config/
    midtrans.yaml              # kind: Config (server_key, mock_enabled, ...)
```

### 2.2 Setiap FR punya rumah spesifik

| FR | Konstruksi | Kenapa |
|---|---|---|
| FR1 | `kind: Mockup` — implementasi dummy dengan kontrak sama | Mock dan real = kontrak identik; framework yang routing |
| FR2 | `kind: Service` + `ctx.config` untuk server_key | Kunci di Config, tidak pernah di kode |
| FR3 | `kind: Webhook` dengan `auth.strategy: signature` | Framework verifikasi SEBELUM handler |
| FR4 | `idempotent: true` + `idempotency_key` di Service action | Ditegakkan framework (§11.8/D32) |
| FR5 | `mock_enabled` di Config per-environment | Framework routing, bukan if-else handler |
| FR6 | `ctx.log` — correlation ID otomatis | Sama seperti di semua resource Forma |

---

## 3. Manifest

### 3.1 Config — per-environment values

```yaml
apiVersion: forma.dev/v1alpha1
kind: Config
metadata:
  name: midtrans
  module: billing
  description: Konfigurasi Midtrans — nilai berbeda per environment
spec:
  keys:
    mock_enabled:
      type: bool
      default: true
      description: true = pakai mockup; false = konektor asli
    server_key:
      type: string
      secret: true
      description: Midtrans server key (sandbox atau live)
    merchant_id:
      type: string
      description: Midtrans merchant ID
    api_url:
      type: string
      default: "https://api.midtrans.com/v2"
      description: Base URL Midtrans API
```

Nilai aktual `server_key` dan `merchant_id` diisi per-environment
melalui Control Plane. `mock_enabled: true` di dev/CI, `false` di
staging/production.

### 3.2 Service — konektor asli

```yaml
apiVersion: forma.dev/v1alpha1
kind: Service
metadata:
  name: midtrans
  module: billing
  description: Wrapper Midtrans payment gateway (konektor asli)
spec:
  version: v1

  actions:
    - name: create-session
      description: Buat sesi pembayaran di Midtrans (Snap/popup)
      required_permission: midtrans.create-session
      audit: true
      uses:
        config: { read: [billing.midtrans.server_key, billing.midtrans.api_url] }
      impl: { type: script_ref, ref: billing/midtrans_create_session }

    - name: webhook
      description: Terima callback pembayaran — verifikasi oleh framework
      required_permission: midtrans.webhook  # dipegang api-key Midtrans
      idempotent: true
      idempotency_key: { from: param, field: transaction_id }
      audit: true
      uses:
        resources: [order.mark-paid]
      impl: { type: script_ref, ref: billing/midtrans_webhook }

    - name: check-status
      description: Cek status transaksi ke Midtrans API
      required_permission: midtrans.check-status
      uses:
        config: { read: [billing.midtrans.server_key, billing.midtrans.api_url] }
      impl: { type: script_ref, ref: billing/midtrans_check_status }
```

### 3.3 Mockup — simulasi dev/CI

```yaml
apiVersion: forma.dev/v1alpha1
kind: Mockup
metadata:
  name: midtrans-mock
  module: billing
  description: Simulasi Midtrans API — aktif saat mock_enabled: true
spec:
  for: midtrans

  actions:
    - name: create-session
      description: Kembalikan URL pembayaran dummy + transaction_id fiktif
      impl: { type: script_ref, ref: billing/midtrans_mock_create_session }

    - name: webhook
      description: Simulasi callback sukses — langsung panggil order.mark-paid
      impl: { type: script_ref, ref: billing/midtrans_mock_webhook }
```

### 3.4 Webhook — verifikasi signature

```yaml
apiVersion: forma.dev/v1alpha1
kind: Webhook
metadata:
  name: midtrans-webhook
  module: billing
  description: Verifikasi signature HMAC-SHA512 sebelum handler jalan
spec:
  for: midtrans.webhook
  method: POST
  path: /webhooks/midtrans
  auth:
    strategy: signature
    signature:
      algorithm: hmac-sha512
      header: X-Midtrans-Signature
      key: { config: billing.midtrans.server_key, secret: true }
      # Framework compute: HMAC-SHA512(raw_body, server_key)
      # Cocokkan dengan header → mismatch → 401 sebelum handler
  idempotent: true
  idempotency_key: { from: param, field: transaction_id }
```

---

## 4. Script Handler

### 4.1 `midtrans_create_session.star` — konektor asli

```python
# modules/billing/scripts/midtrans_create_session.star

def execute(resource, params, ctx):
    # HANYA dipanggil saat mock_enabled: false (framework routing)
    server_key = ctx.config.get("billing.midtrans.server_key")
    api_url = ctx.config.get("billing.midtrans.api_url")

    payload = {
        "transaction_details": {
            "order_id": params.order_id,
            "gross_amount": int(params.amount),
        },
        # ... customer_details, item_details, dll.
    }

    # Panggil Midtrans API via HTTP client (disediakan framework)
    response = http.post(
        api_url + "/charge",
        headers={
            "Authorization": "Basic " + base64(server_key + ":"),
            "Content-Type": "application/json",
        },
        body=payload,
    )

    ctx.log.info("midtrans.session_created", {
        "order_id": params.order_id,
        "transaction_id": response.transaction_id,
    })

    return ok({
        "payment_url": response.redirect_url,
        "transaction_id": response.transaction_id,
    })
```

### 4.2 `midtrans_mock_create_session.star` — simulasi dev

```python
# modules/billing/scripts/midtrans_mock_create_session.star

def execute(resource, params, ctx):
    # HANYA dipanggil saat mock_enabled: true (framework routing)
    import uuid

    fake_transaction_id = "mock-" + str(uuid.uuid4())[:8]
    fake_payment_url = "https://mock.local/pay/" + fake_transaction_id

    ctx.log.info("midtrans.mock.session_created", {
        "order_id": params.order_id,
        "transaction_id": fake_transaction_id,
    })

    return ok({
        "payment_url": fake_payment_url,
        "transaction_id": fake_transaction_id,
    })
```

### 4.3 `midtrans_webhook.star` — konektor asli

```python
# modules/billing/scripts/midtrans_webhook.star

def execute(resource, params, ctx):
    # Framework SUDAH verifikasi signature (kind: Webhook).
    # Framework SUDAH menolak duplikat transaction_id (idempotent: true).
    # Handler hanya meneruskan ke order.mark-paid.

    # Panggil action mark-paid di Entity order
    order.call("mark-paid", {
        "gateway_reference": params.transaction_id,
        "event_id": params.transaction_id,  # idempotency key
    })

    ctx.log.info("midtrans.webhook_processed", {
        "transaction_id": params.transaction_id,
        "status": params.transaction_status,
    })

    return ok({"status": "processed"})
```

### 4.4 `midtrans_mock_webhook.star` — simulasi dev

```python
# modules/billing/scripts/midtrans_mock_webhook.star

def execute(resource, params, ctx):
    # Simulasi: langsung panggil order.mark-paid tanpa ke Midtrans
    order.call("mark-paid", {
        "gateway_reference": params.transaction_id,
        "event_id": params.transaction_id,
    })

    ctx.log.info("midtrans.mock.webhook_processed", {
        "transaction_id": params.transaction_id,
    })

    return ok({"status": "processed"})
```

---

## 5. Pola Environment Detection

Yang terjadi saat `order.checkout` memanggil
`ctx.resource.call("midtrans", "create-session", {...})`:

```
ctx.resource.call("midtrans", "create-session", ...)
        │
        ▼
Framework: cek config "billing.midtrans.mock_enabled"
        │
    ┌───┴───┐
    ▼       ▼
  true    false
    │       │
    ▼       ▼
Mockup   Service
(§3.3)   (§3.2)
    │       │
    ▼       ▼
midtrans_mock_    midtrans_
create_session    create_session
    │       │
    └───┬───┘
        ▼
  return ok({payment_url, transaction_id})
```

Handler checkout TIDAK PERNAH menulis:

```python
# ❌ JANGAN — ini yang dihindari
if ctx.environment == "dev":
    # mock logic
else:
    # real API call
```

Framework yang memutuskan routing — handler checkout cukup satu
panggilan `ctx.resource.call(...)`, sama untuk semua environment.

---

## 6. Pemetaan ke Primitif

| Primitif | Dipakai di | FR |
|---|---|---|
| `ctx.config` | server_key, api_url, mock_enabled | FR2, FR5 |
| `ctx.log` | semua titik penting | FR6 |
| `ctx.kvstore` | idempotency store (framework — handler tidak menyentuh) | FR4 |

`ctx.db`, `ctx.cache`, `ctx.lock`, `ctx.queue`, `ctx.pubsub`, `ctx.storage`
tidak dipakai oleh Service ini — Service adalah thin wrapper, business
state tetap di Entity.

---

## 7. Temuan Test Drive

1. **Handler mock dan real harus diduplikasi** — satu `script_ref` untuk
   mockup, satu untuk konektor asli. Ini disengaja: mock dan real adalah
   dua implementasi berbeda dari kontrak yang sama. Yang menjamin
   konsistensi: action name & response shape identik di manifest.
2. **Verifikasi signature butuh raw body** — `kind: Webhook` perlu
   mengakses raw HTTP body sebelum parsing JSON. Detail implementasi
   di luar scope spec (framework concern).
3. **HTTP client di Starlark?** — `http.post(...)` di §4.1 adalah
   placeholder. Core Basic tidak menyediakan HTTP client di sandbox
   Starlark (no network). Panggilan HTTP eksternal harus dari
   `impl: native`. Alternatif: `kind: Service` action dengan
   `impl: native` untuk panggilan API, dan script hanya untuk
   orchestration ringan.
4. **Satu Service, dua impl type** — idealnya satu Service bisa punya
   dua implementasi (mock vs real) tanpa Mockup terpisah. Tapi
   memisahkan `kind: Mockup` memberi keuntungan: mockup bisa di-share
   antar modul, independen dari konektor asli, dan eksplisit di
   consent footprint.

---

## 8. Mockup vs Real — Perbandingan

| Dimensi | `kind: Mockup` | `kind: Service` (konektor asli) |
|---|---|---|
| Kapan aktif | `mock_enabled: true` | `mock_enabled: false` |
| Panggilan HTTP keluar | Tidak ada | Ya (ke Midtrans API) |
| Butuh secret API key | Tidak | Ya (`server_key`) |
| Idempotency | Tetap enforced | Tetap enforced |
| Signature webhook | Tidak (auto-approve) | Ya (HMAC-SHA512 via `kind: Webhook`) |
| Log | `ctx.log` (prefix "mock") | `ctx.log` (real transaction_id) |
| Consent footprint | "simulasi Midtrans" | "akses Midtrans API + baca server_key" |
