// impl/billing/generate_receipt.go
//
// Job handler: dipicu oleh deliver channel: queue, job: generate-receipt
// dari event billing.order.paid.

package billing

import (
	"context"
	"fmt"
)

// GenerateReceiptHandler membuat PDF nota dan menyimpannya ke object storage.
// Dipicu oleh order.paid → deliver: [{ channel: queue, job: generate-receipt }]
func GenerateReceiptHandler(ctx context.Context, evt PaidEvent) error {
	// TODO: implement
	// 1. Load order by evt.ID
	// 2. Cek cache diskon member: ctx.Cache().Get("member-discount:" + tier)
	//    Jika tidak ada: fetch dari aturan, set cache TTL 3600
	// 3. Render PDF nota
	// 4. ctx.Storage().Write("receipts/" + order.Number + ".pdf", pdfBytes)
	return fmt.Errorf("not implemented")
}

// PaidEvent adalah payload dari billing.order.paid event.
type PaidEvent struct {
	ID         string  `json:"id"`
	Number     string  `json:"number"`
	Total      float64 `json:"total"`
	CustomerID string  `json:"customer_id"`
	PaidAt     string  `json:"paid_at"`
}
