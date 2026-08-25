package api

import (
	"bytes"
	"testing"

	"github.com/primadi/formspec/pkg/spec"
)

// TestRenderPrintPDF verifies the server-side PDF renderer (todo 5.13.2)
// produces a valid PDF from a Print manifest + record.
func TestRenderPrintPDF(t *testing.T) {
	ps := &spec.PrintSpec{
		Entity: "billing.order",
		Header: &spec.PrintHeader{
			Title:    "Receipt {order.number}",
			Subtitle: "Paid invoice",
		},
		Body: []spec.PrintBodyItem{
			{Fields: []string{"number", "customer.name", "total"}},
			{
				ChildTable: &spec.PrintChildTable{
					Field:   "items",
					Columns: []string{"product_id", "quantity", "price"},
				},
			},
		},
		Footer: &spec.PrintFooter{Text: "Thank you"},
	}

	record := map[string]any{
		"order":    map[string]any{"number": "INV-001"},
		"number":   "INV-001",
		"customer": map[string]any{"name": "Acme Corp"},
		"total":    1250.5,
		"items": []any{
			map[string]any{"product_id": "P1", "quantity": 2, "price": 100.0},
			map[string]any{"product_id": "P2", "quantity": 1, "price": 50.5},
		},
	}

	pdf, err := renderPrintPDF(ps, record)
	if err != nil {
		t.Fatalf("renderPrintPDF: %v", err)
	}
	if len(pdf) == 0 {
		t.Fatal("expected non-empty PDF")
	}
	// PDF magic header.
	if !bytes.HasPrefix(pdf, []byte("%PDF")) {
		t.Fatalf("expected PDF header, got %q", pdf[:8])
	}
}

// TestInterpolatePrint verifies {path} token interpolation.
func TestInterpolatePrint(t *testing.T) {
	record := map[string]any{
		"number":   "INV-001",
		"customer": map[string]any{"name": "Acme"},
	}
	if got := interpolatePrint("Receipt {order.number}", record); got != "Receipt " {
		t.Errorf("missing path should resolve to empty, got %q", got)
	}
	if got := interpolatePrint("Receipt {number}", record); got != "Receipt INV-001" {
		t.Errorf("flat path interpolation failed, got %q", got)
	}
	if got := interpolatePrint("Customer {customer.name}", record); got != "Customer Acme" {
		t.Errorf("dot-path interpolation failed, got %q", got)
	}
}
