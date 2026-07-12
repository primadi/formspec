// impl/sales-gl-integrator/create_sales_journal.go
//
// Implementasi native untuk Subscription job "create-sales-journal".
// Dipicu oleh kind: Subscription → on: billing.order, event: paid → deliver: queue, job: create-sales-journal.
// File ini TIDAK termasuk dalam deployment artifact.

package salesglintegrator

import (
	"context"
	"fmt"
)

// CreateSalesJournalHandler menangani job "create-sales-journal".
// Dipicu saat order.paid event terkirim (via Subscription).
// Membuat journal-entry baru dengan baris:
//
//	Debit:  Piutang Usaha (1-11001)  = order.total
//	Credit: Pendapatan Penjualan (4-40001) = order.total
func CreateSalesJournalHandler(ctx context.Context, evt PaidEvent) error {
	// TODO: implement
	// 1. Terima PaidEvent payload (order_id, total, customer_id, paid_at)
	// 2. Buat journal-entry baru: entry_date = paid_at, reference = order.number
	// 3. Tambah 2 baris: debit Piutang Usaha, credit Pendapatan Penjualan
	// 4. Panggil action "post" pada journal-entry
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
