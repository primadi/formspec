package formspec

import (
	"context"
	"net/http"
	"testing"

	db "github.com/primadi/formspec/renderers/jsonb-persist"
)

// kafe 10.72 over HTTP: a create must not be able to land mid-lifecycle.
//
// Measured on the kafe dev server before this was enforced:
//
//	POST order {status: "paid"}   → 201, stored status=paid
//	POST order {status: "ready"}  → 201, stored status=ready
//	POST order {status: "ngawur"} → 422  (the enum CHECK, NOT the state machine)
//
// The middle outcome is the dangerous one: `paid` is a perfectly valid state, so
// nothing complained — while the order had skipped `confirm-payment` (its
// permission gate) and, because emissions are resolved from a state CHANGE on
// the update path, never published `on_paid`. No journal, no table occupancy,
// nothing — from a request that looked successful.
func TestKafe_CreateCannotBeBornPaid(t *testing.T) {
	app := bootKafe(t)
	ids := seedKafeTableScenario(t, app)
	admin := seedKafeAdminToken(t, app)

	create := func(t *testing.T, status string) (int, map[string]any) {
		t.Helper()
		body := map[string]any{
			"transaction_date": "2026-10-03T00:00:00Z",
			"branch_id":        ids.branchID,
			"channel":          "cashier",
			"lines": []any{
				map[string]any{
					"line_no":             1,
					"menu_item_id":        ids.nasiGoreng,
					"name_snapshot":       "Nasi Goreng Spesial",
					"unit_price_snapshot": map[string]any{"amount": "45000", "currency": "IDR"},
					"quantity":            1,
					"prep_station":        "kitchen",
				},
			},
		}
		if status != "" {
			body["status"] = status
		}
		return doAuthed(t, app, http.MethodPost, "/kafe/_ui/entity/cafe-order/order", admin, body)
	}

	t.Run("omitting the status creates a draft", func(t *testing.T) {
		code, out := create(t, "")
		if code != http.StatusCreated {
			t.Fatalf("create without a status: %d (%v)", code, out)
		}
		if got, _ := out["data"].(map[string]any)["status"].(string); got != "draft" {
			t.Errorf("status = %q, want draft", got)
		}
	})

	t.Run("asking for `paid` at create is refused and nothing is written", func(t *testing.T) {
		code, out := create(t, "paid")
		if code < 400 || code >= 500 {
			t.Fatalf("a mid-lifecycle create must be refused with a 4xx, got %d (%v)", code, out)
		}

		// It must NOT have been created: the refusal has to happen BEFORE the
		// write, not merely be reported after one.
		store, err := app.GetEntityStore("cafe-order", "order")
		if err != nil {
			t.Fatalf("store: %v", err)
		}
		res, err := store.List(context.Background(), db.ListParams{
			WorkspaceID: "kafe", Page: 1, PerPage: 500,
		})
		if err != nil {
			t.Fatalf("list orders: %v", err)
		}
		for _, rec := range res.Data {
			if s, _ := rec.Data["status"].(string); s != "draft" {
				t.Errorf("an order exists with status=%q despite the refusal (%v)",
					s, rec.Data["number"])
			}
		}
	})

	t.Run("the normal path still reaches `paid`", func(t *testing.T) {
		code, out := create(t, "")
		if code != http.StatusCreated {
			t.Fatalf("create: %d (%v)", code, out)
		}
		id, _ := out["data"].(map[string]any)["id"].(string)
		if id == "" {
			t.Fatalf("no id: %v", out)
		}
		patchOrderStatus(t, app, admin, id, "awaiting_payment")
		patchOrderStatus(t, app, admin, id, "paid")
	})
}
