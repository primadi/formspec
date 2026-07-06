// impl/notifications/send_wa_notification.go
//
// Job handler: dipicu oleh kind: Subscription → on: billing.order.paid
// deliver: [{ channel: queue, job: send-wa-notification }]

package notifications

import (
	"context"
	"fmt"
)

// PaidEvent adalah payload dari billing.order.paid event.
type PaidEvent struct {
	ID         string  `json:"id"`
	Number     string  `json:"number"`
	Total      float64 `json:"total"`
	CustomerID string  `json:"customer_id"`
	PaidAt     string  `json:"paid_at"`
}

// SendWANotificationHandler mengirim notifikasi WhatsApp saat order dibayar.
// Dipicu oleh Subscription wa-on-order-paid.
func SendWANotificationHandler(ctx context.Context, evt PaidEvent) error {
	// TODO: implement
	// 1. Load customer (dapat phone number)
	// 2. Format pesan: "Pembayaran order {number} sebesar {total} berhasil."
	// 3. Panggil notify-wa.send
	return fmt.Errorf("not implemented")
}
