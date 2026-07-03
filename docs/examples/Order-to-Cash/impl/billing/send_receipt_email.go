// impl/billing/send_receipt_email.go
//
// Job handler: dipicu oleh deliver channel: queue, job: send-receipt-email
// dari event billing.order.paid.
//
// Email dikirim via modul resmi forma/mail (Foundation D12).

package billing

import (
	"context"
	"fmt"
)

// SendReceiptEmailHandler mengirim email nota ke customer.
// Dipicu oleh order.paid → deliver: [{ channel: queue, job: send-receipt-email }]
func SendReceiptEmailHandler(ctx context.Context, evt PaidEvent) error {
	// TODO: implement
	// 1. Load order + customer (dapat email)
	// 2. Render template email dengan link download nota
	// 3. Kirim via forma/mail
	return fmt.Errorf("not implemented")
}
