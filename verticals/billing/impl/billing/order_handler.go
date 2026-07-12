// impl/billing/order_handler.go
//
// Implementasi native untuk action order yang butuh Go runtime.
// File ini TIDAK termasuk dalam deployment artifact.

package billing

import (
	"context"
	"fmt"
)

// OrderResource menangani action native pada Entity order.
// Method-method:
//   - update-discount-rule: impl: { type: native, ref: "OrderResource.UpdateDiscountRule" }

type OrderResource struct{}

// UpdateDiscountRule mengubah persen diskon untuk satu tier membership.
// Dipanggil dari admin panel / API.
// Setelah update, invalidasi cache agar checkout berikutnya pakai aturan baru.
func (r *OrderResource) UpdateDiscountRule(ctx context.Context, params UpdateDiscountRuleParams) error {
	// TODO: implement
	// 1. Validasi tier + persen diskon
	// 2. Simpan aturan baru ke config/db
	// 3. ctx.Cache().Delete("member-discount:" + tier) — invalidasi cache
	return fmt.Errorf("not implemented")
}

type UpdateDiscountRuleParams struct {
	Tier    string  `json:"tier"`    // regular, silver, gold
	Percent float64 `json:"percent"` // 0.0 - 100.0
}
