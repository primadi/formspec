// impl/billing/customer_handler.go
//
// Implementasi native untuk action customer yang butuh Go runtime.
// File ini TIDAK termasuk dalam deployment artifact — dikompilasi dan di-fuse ke binary.

package billing

import (
	"context"
	"fmt"
)

// CustomerResource handles native actions on Entity customer.
type CustomerResource struct{}

// BlacklistParams is the input for the blacklist action.
type BlacklistParams struct {
	CustomerID string `json:"customer_id"`
	Reason     string `json:"reason"`
}

// Blacklist marks a customer as blacklisted.
// Implements: action "blacklist" → impl: { type: native, ref: "CustomerResource.Blacklist" }
//
// Business rules:
//   - Sets is_blacklisted = true
//   - Logs the reason and timestamp
//   - Publishes event "customer.blacklisted" for downstream consumers
func (r *CustomerResource) Blacklist(ctx context.Context, params BlacklistParams) error {
	// TODO: implement
	// 1. Validate customer exists
	// 2. Update is_blacklisted = true
	// 3. Store blacklist reason + timestamp in audit log
	// 4. Publish event customer.blacklisted
	return fmt.Errorf("not implemented")
}

// UpdateTierParams is the input for the update-tier action.
type UpdateTierParams struct {
	CustomerID string `json:"customer_id"`
	MemberTier string `json:"member_tier"` // regular, silver, gold
}

// UpdateTier changes a customer's membership tier.
// Implements: action "update-tier" → impl: { type: native, ref: "CustomerResource.UpdateTier" }
//
// Business rules:
//   - Validates tier is valid enum
//   - Updates member_tier field
//   - Invalidates cache for member discounts
//   - Logs tier change with old→new values
func (r *CustomerResource) UpdateTier(ctx context.Context, params UpdateTierParams) error {
	// TODO: implement
	// 1. Validate customer exists
	// 2. Validate member_tier ∈ {regular, silver, gold}
	// 3. Update member_tier
	// 4. Invalidate cache: ctx.Cache().Delete("member-discount:" + customerID)
	// 5. Audit log tier change
	return fmt.Errorf("not implemented")
}
