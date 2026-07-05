// impl/billing/midtrans.go
//
// Implementasi native untuk kind: Service "midtrans".
// File ini TIDAK termasuk dalam deployment artifact.
// Saat build: dikompilasi ke .forma/build/native.so, lalu di-fuse ke forma-resource binary.
// Saat deploy: hanya spec/ + binary yang dikirim; impl/ dihapus.

package billing

import (
	"context"
	"fmt"
)

// PaymentGateway adalah konektor Midtrans asli.
// Method-method di sini merefer ke impl type "native" di services/midtrans.yaml:
//   - create-session:  impl: { type: native, ref: "PaymentGateway.CreateSession" }
//   - webhook:         impl: { type: native, ref: "PaymentGateway.Webhook" }
//   - check-status:    impl: { type: native, ref: "PaymentGateway.CheckStatus" }

type PaymentGateway struct{}

// CreateSession membuat sesi pembayaran di Midtrans Snap API.
// Dipanggil dari order.checkout → ctx.resource.call("midtrans", "create-session", ...)
// Saat dev (mock_enabled: true): framework routing ke kind: Mockup, method ini tidak dipanggil.
func (p *PaymentGateway) CreateSession(ctx context.Context, params CreateSessionParams) (*CreateSessionResult, error) {
	// TODO: implement
	// 1. Baca config: ctx.Config().Get("billing.midtrans.server_key"), api_url
	// 2. POST ke {api_url}/charge dengan Basic Auth (server_key:)
	// 3. Return redirect_url + transaction_id
	return nil, fmt.Errorf("not implemented")
}

type CreateSessionParams struct {
	OrderID string  `json:"order_id"`
	Amount  float64 `json:"amount"`
}

type CreateSessionResult struct {
	PaymentURL    string `json:"payment_url"`
	TransactionID string `json:"transaction_id"`
}

// Webhook menangani callback pembayaran dari Midtrans.
// Framework SUDAH melakukan:
//  1. Verifikasi signature HMAC-SHA512 (berdasarkan kind: Webhook)
//  2. Idempotency check (idempotent: true + idempotency_key: transaction_id)
//
// Handler hanya meneruskan ke order.mark-paid.
func (p *PaymentGateway) Webhook(ctx context.Context, params WebhookParams) (*WebhookResult, error) {
	// TODO: implement
	// 1. Payload sudah verified — langsung panggil order.mark-paid
	// 2. Return status
	return nil, fmt.Errorf("not implemented")
}

type WebhookParams struct {
	TransactionID     string `json:"transaction_id"`
	TransactionStatus string `json:"transaction_status"`
	OrderID           string `json:"order_id"`
	GrossAmount       string `json:"gross_amount"`
}

type WebhookResult struct {
	Status string `json:"status"`
}

// CheckStatus mengecek status transaksi ke Midtrans API.
func (p *PaymentGateway) CheckStatus(ctx context.Context, params CheckStatusParams) (*CheckStatusResult, error) {
	// TODO: implement
	return nil, fmt.Errorf("not implemented")
}

type CheckStatusParams struct {
	TransactionID string `json:"transaction_id"`
}

type CheckStatusResult struct {
	TransactionStatus string `json:"transaction_status"`
	PaymentType       string `json:"payment_type"`
}
