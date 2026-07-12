// impl/inventory/create_out_movement.go
//
// Implementasi native untuk Subscription job "create-out-movement".
// Dipicu oleh kind: Subscription → on: billing.order, event: paid → deliver: queue, job: create-out-movement.
// File ini TIDAK termasuk dalam deployment artifact.

package inventory

import (
	"context"
	"fmt"
)

// CreateOutMovementHandler menangani job "create-out-movement".
// Dipicu saat order.paid event terkirim (via Subscription).
// Membuat stock-movement baru dengan type=out untuk setiap item di order.
func CreateOutMovementHandler(ctx context.Context, evt PaidEvent) error {
	// TODO: implement
	// 1. Terima PaidEvent payload (order_id, items)
	// 2. Untuk setiap item di order, buat line movement dengan quantity
	// 3. Gunakan warehouse default dari config
	// 4. Set reference = order.number, status = auto-confirmed
	// 5. Panggil action "apply" pada stock-movement (atau auto-apply via job)
	return fmt.Errorf("not implemented")
}

// PaidEvent adalah payload dari billing.order.paid event.
type PaidEvent struct {
	ID         string     `json:"id"`
	Number     string     `json:"number"`
	CustomerID string     `json:"customer_id"`
	Items      []LineItem `json:"items"`
}

// LineItem adalah item dalam order.
type LineItem struct {
	ProductID string  `json:"product_id"`
	Quantity  float64 `json:"quantity"`
}
