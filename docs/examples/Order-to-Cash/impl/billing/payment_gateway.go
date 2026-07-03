// impl/billing/payment_gateway.go
//
// Implementasi native untuk kind: Service "payment-gateway".
// Saat mock_enabled: false, framework routing ke sini.

package billing

import (
	"context"
	"fmt"
)

// PaymentGateway adalah konektor payment gateway asli (Midtrans/Xendit).
// Method-method:
//   - create-session: impl: { type: native, ref: "PaymentGateway.CreateSession" }
//   - webhook:        impl: { type: native, ref: "PaymentGateway.Webhook" }

type PaymentGateway struct{}

// CreateSession membuat sesi pembayaran di gateway.
// Dipanggil dari order.checkout → ctx.resource.call("payment-gateway", "create-session", ...)
func (p *PaymentGateway) CreateSession(ctx context.Context, params CreateSessionParams) (*CreateSessionResult, error) {
	// TODO: implement
	// 1. Baca config server_key, api_url
	// 2. POST ke gateway API
	// 3. Return payment_url + transaction_id
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

// Webhook menangani callback pembayaran dari gateway.
// Framework SUDAH verifikasi signature (kind: Webhook) + idempotency check.
func (p *PaymentGateway) Webhook(ctx context.Context, params WebhookParams) (*WebhookResult, error) {
	// TODO: implement
	// 1. Payload sudah verified
	// 2. Panggil order.mark-paid meneruskan event_id + gateway_reference
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
