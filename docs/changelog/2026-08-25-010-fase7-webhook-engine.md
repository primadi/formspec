# Webhook Engine (7.6) — Inbound Verified Endpoint

**Tanggal:** 2026-08-25 · **Todo:** §7.6.1–7.6.4 · **Plan:** `docs/plan/fase7-webhook-engine.md`

## Apa yang ditambahkan

`kind: Webhook` kini punya runtime penuh — endpoint masuk yang **diverifikasi
sebelum handler berjalan** (02-core-extended.md §4):

- **Registry + route (7.6.1)** — `internal/webhook/registry.go` memetakan
  `{module}.{name}` → `WebhookSpec`; `buildWebhookRegistry` di
  `resource/formspec.go` (boot + reload); `GenerateWebhookRoutes` di
  `internal/api/generator.go` (path auto-derive `/api/v1/webhooks/{module}/{name}`
  atau `spec.path` relatif ke `/api/v1`); `HandleWebhook` di
  `internal/api/handler.go` dispatch ke Service action `spec.for`.
- **Signature verification (7.6.2)** — `internal/webhook/verify.go`:
  HMAC-SHA256/SHA512 atas raw body, key di-resolve dari Config manifest via
  `config.Registry.ResolveKey(configName, keyName)`. Verifikasi SEBELUM handler.
- **Token auth (7.6.3)** — strategi `token` via `WebhookTokenConfig` (header +
  key ref config), dukung prefix "Bearer ".
- **Reject before handler (7.6.4)** — verifikasi gagal → `401` (auth) / `500`
  (misconfig), handler tidak pernah jalan.

## Perubahan spec

- `pkg/spec/resources.go` — `WebhookAuth` ditambah field
  `Token *WebhookTokenConfig` (strategi token).
- `internal/config/registry.go` — method baru `ResolveKey(configName, keyName)`
  untuk resolve key dari Config manifest bernama (dipakai sebagai
  `webhook.KeyResolver`).

## Verifikasi end-to-end (via `formspec dev` + curl)

- `POST /api/v1/webhooks/payment` dengan HMAC-SHA256 valid → 200 + hasil
  dispatch service action ✅
- Signature salah → `401` (ditolak sebelum handler) ✅
- `go test ./...` hijau (termasuk unit test `internal/webhook` + `internal/api`).

## File terdampak

- `internal/webhook/registry.go`, `verify.go`, `verify_test.go` (baru)
- `internal/api/webhook_test.go` (baru)
- `internal/api/handler.go`, `generator.go`, `router.go`
- `internal/config/registry.go`
- `pkg/spec/resources.go`
- `internal/manifest/loader.go` (`RawSpecToWebhookSpec`)
- `resource/formspec.go` (`buildWebhookRegistry` + wiring boot/reload)
- `examples/service-demo/` — contoh webhook (config + service + webhook + script)
