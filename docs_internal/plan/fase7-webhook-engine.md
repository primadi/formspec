# Fase 7 — Webhook Engine (7.6)

**Status:** ✅ Complete · **Tanggal:** 2026-08-25
**Referensi:** `docs/spec/backend/02-core-extended.md` §4 (Webhook),
`pkg/spec/resources.go` (WebhookSpec)
**Todo:** `docs/plan/todo.md` §7.6

## Konteks

`kind: Webhook` mendeklarasikan inbound endpoint terverifikasi. `spec.for`
merujuk satu Service action yang menangani payload. Saat ini `WebhookSpec`
sudah ada di `pkg/spec` tapi tidak ada runtime — tidak ada route, tidak ada
verifikasi auth.

## Scope

### WS-1 — Webhook registry + route (7.6.1) ✅

- `internal/webhook/registry.go` — `Registry` memetakan `{module}.{name}` →
  `WebhookSpec`; `Add`/`Get`/`List`.
- `buildWebhookRegistry` di `resource/formspec.go` (boot + reload).
- Route: `POST {path}` (method dari spec) di surface external. `spec.for`
  merujuk Service action → dispatch via service registry.
- `GenerateWebhookRoutes` di `internal/api/generator.go` — path auto-derive
  `/api/v1/webhooks/{module}/{name}` atau `spec.path` (relatif ke `/api/v1`).
- `HandleWebhook` di `internal/api/handler.go` — verifikasi lalu dispatch ke
  Service action `spec.for` (`{module}.{service}`).

### WS-2 — Auth verification (7.6.2, 7.6.3, 7.6.4) ✅

- `internal/webhook/verify.go` — `Verify` memeriksa request terhadap strategi
  auth yang dideklarasikan.
- `strategy: signature` — HMAC (algorithm hmac-sha256/sha512, header, key dari
  config, payload raw_body). Verifikasi SEBELUM handler.
- `strategy: token` — token statis dari config (header, dukung prefix "Bearer ").
- Gagal verifikasi → 401/403 SEBELUM handler jalan.
- `WebhookAuth` ditambah field `Token *WebhookTokenConfig` (strategi token).
- `config.Registry.ResolveKey(configName, keyName)` — resolve key dari Config
  manifest bernama (dipakai sebagai `webhook.KeyResolver`).

## Level of effort

| WS  | Effort |
| --- | ------ |
| 1   | medium |
| 2   | medium |

## Verifikasi end-to-end (via `formspec dev` + curl)

- `POST /api/v1/webhooks/payment` dengan HMAC-SHA256 valid → 200 + hasil
  dispatch service action ✅
- Signature salah → `401` (ditolak sebelum handler) ✅

## File terdampak

- `internal/webhook/registry.go` (baru) — registry
- `internal/webhook/verify.go` (baru) — verifikasi signature/token
- `internal/webhook/verify_test.go` (baru) — unit test verifikasi
- `internal/api/webhook_test.go` (baru) — test handler + route
- `internal/api/handler.go` — `HandleWebhook`, `SetWebhookRegistry`,
  `SetWebhookKeyResolver`
- `internal/api/generator.go` — `GenerateWebhookRoutes`
- `internal/api/router.go` — wire webhook registry + route handler
- `internal/config/registry.go` — `ResolveKey`
- `pkg/spec/resources.go` — `WebhookTokenConfig`
- `internal/manifest/loader.go` — `RawSpecToWebhookSpec`
- `resource/formspec.go` — `buildWebhookRegistry` + wiring (boot + reload)
- `examples/service-demo/` — contoh webhook (config + service + webhook + script)
