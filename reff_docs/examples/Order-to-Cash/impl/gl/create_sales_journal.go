// impl/gl/create_sales_journal.go
//
// Job handler: dipicu oleh kind: Subscription → on: billing.order.paid
// deliver: [{ channel: queue, job: create-sales-journal }]
//
// Membuat journal-entry double-entry dari order yang baru dibayar.

package gl

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

// CreateSalesJournalHandler membuat journal entry otomatis saat order dibayar.
// Dipicu oleh Subscription order-to-journal.
// Membuat journal-entry dengan baris:
//
//	Debit:  Piutang Usaha (1-11001)  = order.total
//	Credit: Pendapatan Penjualan (4-40001) = order.total
func CreateSalesJournalHandler(ctx context.Context, evt PaidEvent) error {
	// TODO: implement
	// 1. Buat journal-entry baru: entry_date = paid_at, reference = order.number
	// 2. Tambah 2 baris:
	//    - Debit:  Piutang Usaha (account code 1-11001)
	//    - Credit: Pendapatan Penjualan (account code 4-40001)
	// 3. Panggil action "post" pada journal-entry
	return fmt.Errorf("not implemented")
}
