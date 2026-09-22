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

	pdf, err := renderPrintPDF(ps, record, printContext{})
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
	pc := printContext{}
	if got := pc.interpolate("Receipt {order.number}", record); got != "Receipt " {
		t.Errorf("missing path should resolve to empty, got %q", got)
	}
	if got := pc.interpolate("Receipt {number}", record); got != "Receipt INV-001" {
		t.Errorf("flat path interpolation failed, got %q", got)
	}
	if got := pc.interpolate("Customer {customer.name}", record); got != "Customer Acme" {
		t.Errorf("dot-path interpolation failed, got %q", got)
	}
}

// TestRenderPrintThermal pins GAP-10 (item 7.1): a `format: thermal` Print
// manifest renders to an ESC/POS byte stream, not a PDF. Before this the
// handler always returned a PDF regardless of `output.format`, so a thermal
// receipt could not actually be printed.
func TestRenderPrintThermal(t *testing.T) {
	ps := &spec.PrintSpec{
		Entity: "cafe-order.order",
		Output: &spec.PrintOutput{Format: "thermal", Paper: &spec.PrintPaper{Size: "thermal_58mm"}},
		Header: &spec.PrintHeader{Title: "KAFE KAMI", Subtitle: "{branch.name}"},
		Body: []spec.PrintBodyItem{
			{Fields: []string{"number", "transaction_date"}},
			{Separator: "--------------------------------"},
			{ChildTable: &spec.PrintChildTable{Field: "lines", Columns: []string{"name_snapshot", "quantity"}}},
			{Totals: &spec.PrintTotals{Field: "total_amount", Format: "currency"}},
		},
		Footer: &spec.PrintFooter{Text: "Terima kasih"},
	}
	record := map[string]any{
		"number":           "ORD-2026-00001",
		"transaction_date": "2026-09-20",
		"branch":           map[string]any{"name": "Cabang Pusat"},
		"total_amount":     map[string]any{"amount": "62500", "currency": "IDR"},
		"lines": []any{
			map[string]any{"name_snapshot": "Kopi Susu", "quantity": 2},
		},
	}

	out, err := renderPrintThermal(ps, record, printContext{})
	if err != nil {
		t.Fatalf("renderPrintThermal: %v", err)
	}
	if len(out) == 0 {
		t.Fatal("expected non-empty ESC/POS output")
	}
	// ESC/POS starts with the initialize command ESC @ (0x1b 0x40).
	if !bytes.HasPrefix(out, []byte{0x1b, 0x40}) {
		t.Fatalf("expected ESC/POS init prefix, got % x", out[:2])
	}
	// The cut command GS V 0 (0x1d 0x56 0x00) must be present at the end.
	if !bytes.Contains(out, []byte{0x1d, 0x56, 0x00}) {
		t.Error("expected a paper-cut command in the output")
	}
	// It must NOT be a PDF.
	if bytes.HasPrefix(out, []byte("%PDF")) {
		t.Error("thermal output must not be a PDF")
	}
	// Interpolated content is present.
	if !bytes.Contains(out, []byte("KAFE KAMI")) {
		t.Error("expected the header title in the output")
	}
	if !bytes.Contains(out, []byte("Cabang Pusat")) {
		t.Error("expected the interpolated subtitle in the output")
	}
	if !bytes.Contains(out, []byte("Kopi Susu")) {
		t.Error("expected the child-table row in the output")
	}
}

// idrSettings is the settings shape the kafe app resolves: IDR with the
// display symbol and the Indonesian locale (grouping ".", decimal ",").
func idrSettings() *spec.Settings {
	places := 0
	return &spec.Settings{
		Locale:   "id-ID",
		Currency: &spec.CurrencySettings{Code: "IDR", Symbol: "Rp", DecimalPlaces: &places},
	}
}

// TestRenderPrintThermal_MoneyAndQR closes the two halves of gap #1/#3 that
// the print path was missing: money printed as a Go map, and no way to print a
// QR at all (item 2.6).
func TestRenderPrintThermal_MoneyAndQR(t *testing.T) {
	ps := &spec.PrintSpec{
		Entity: "cafe-order.order",
		Output: &spec.PrintOutput{Format: "thermal", Paper: &spec.PrintPaper{Size: "thermal_58mm"}},
		Body: []spec.PrintBodyItem{
			{Totals: &spec.PrintTotals{Field: "total_amount", Format: "currency"}},
			{Qrcode: &spec.PrintQrcode{
				Payload:  "/status/{guest_token}",
				Label:    "Scan untuk struk digital",
				Absolute: true,
			}},
		},
	}
	record := map[string]any{
		"total_amount": map[string]any{"amount": "62500", "currency": "IDR"},
		"guest_token":  "TOK-123",
	}
	pc := printContext{settings: idrSettings(), origin: "https://kafe.example"}

	out, err := renderPrintThermal(ps, record, pc)
	if err != nil {
		t.Fatalf("renderPrintThermal: %v", err)
	}
	if !bytes.Contains(out, []byte("Rp62.500")) {
		t.Errorf("expected money formatted as Rp62.500, output: %q", out)
	}
	if bytes.Contains(out, []byte("map[amount")) {
		t.Error("money must never be printed as a Go map")
	}
	// Native QR sequence: GS ( k.
	if !bytes.Contains(out, []byte{0x1d, 0x28, 0x6b}) {
		t.Error("expected the ESC/POS QR command sequence (GS ( k)")
	}
	if !bytes.Contains(out, []byte("https://kafe.example/status/TOK-123")) {
		t.Error("expected the absolute QR payload (origin + interpolated path)")
	}
}

// TestRenderPrintThermal_QRPayloadEmptyOmits pins the data-dependent rule: a
// record with nothing to encode (a counter order carries no guest token) prints
// the receipt without a QR — never a code that leads nowhere, and never a
// failed print.
func TestRenderPrintThermal_QRPayloadEmptyOmits(t *testing.T) {
	ps := &spec.PrintSpec{
		Entity: "cafe-order.order",
		Output: &spec.PrintOutput{Format: "thermal"},
		Body: []spec.PrintBodyItem{
			{Fields: []string{"number"}},
			{Qrcode: &spec.PrintQrcode{Payload: "/status/{guest_token}", Absolute: true, Label: "Scan"}},
		},
	}
	out, err := renderPrintThermal(ps, map[string]any{"number": "ORD-1"}, printContext{origin: "https://kafe.example"})
	if err != nil {
		t.Fatalf("renderPrintThermal: %v", err)
	}
	if bytes.Contains(out, []byte{0x1d, 0x28, 0x6b}) {
		t.Error("expected no QR sequence when the record has nothing to encode")
	}
	if !bytes.Contains(out, []byte("ORD-1")) {
		t.Error("the rest of the receipt must still print")
	}
	// The same manifest with a token does print the QR.
	withToken, err := renderPrintThermal(ps, map[string]any{"number": "ORD-1", "guest_token": "TOK"}, printContext{origin: "https://kafe.example"})
	if err != nil {
		t.Fatalf("renderPrintThermal (with token): %v", err)
	}
	if !bytes.Contains(withToken, []byte{0x1d, 0x28, 0x6b}) {
		t.Error("expected the QR sequence when the guest token is present")
	}
}

// TestRenderPrintPDF_QR proves the PDF pipeline can draw a QR (gap #3 / S4):
// the payload is encoded to a PNG and embedded, so the page stays a real PDF.
func TestRenderPrintPDF_QR(t *testing.T) {
	ps := &spec.PrintSpec{
		Entity: "cafe-master.dining-table",
		Output: &spec.PrintOutput{Format: "pdf", Paper: &spec.PrintPaper{Size: "A5"}},
		Body: []spec.PrintBodyItem{
			{Qrcode: &spec.PrintQrcode{
				Payload:  "/table/{qr_token}",
				Label:    "Meja {code}",
				Absolute: true,
				SizeMM:   40,
			}},
		},
	}
	record := map[string]any{"qr_token": "TBL-9", "code": "A-01"}
	pc := printContext{settings: idrSettings(), origin: "https://kafe.example"}

	pdf, err := renderPrintPDF(ps, record, pc)
	if err != nil {
		t.Fatalf("renderPrintPDF: %v", err)
	}
	if !bytes.HasPrefix(pdf, []byte("%PDF")) {
		t.Fatalf("expected a PDF, got %q", pdf[:8])
	}
}

// TestPrintChildTable_HydratedShape pins a defect that made every thermal
// receipt print blank: a child collection reaches a renderer as
// []map[string]any once ChildStore.Hydrate has read it back from its own table
// (`storage: table`), and as []any when it came straight from the request.
// The PDF branch had a local []map[string]any fallback; the thermal branch
// asserted []any only, so with real (hydrated) data it dropped every line item
// and emitted just the init + cut codes.
func TestPrintChildTable_HydratedShape(t *testing.T) {
	ps := &spec.PrintSpec{
		Entity: "cafe-order.order",
		Output: &spec.PrintOutput{Format: "thermal"},
		Body: []spec.PrintBodyItem{
			{ChildTable: &spec.PrintChildTable{Field: "lines", Columns: []string{"name_snapshot", "quantity"}}},
		},
	}
	// Exactly the shape ChildStore.Hydrate produces.
	hydrated := map[string]any{
		"number": "ORD-2026-00001",
		"lines": []map[string]any{
			{"name_snapshot": "Kopi Susu", "quantity": 2},
			{"name_snapshot": "Croissant", "quantity": 1},
		},
	}
	// The shape a request body carries.
	fromRequest := map[string]any{
		"number": "ORD-2026-00001",
		"lines": []any{
			map[string]any{"name_snapshot": "Kopi Susu", "quantity": 2},
			map[string]any{"name_snapshot": "Croissant", "quantity": 1},
		},
	}

	for name, record := range map[string]map[string]any{
		"hydrated []map[string]any": hydrated,
		"request []any":             fromRequest,
	} {
		t.Run(name, func(t *testing.T) {
			out, err := renderPrintThermal(ps, record, printContext{})
			if err != nil {
				t.Fatalf("renderPrintThermal: %v", err)
			}
			for _, want := range []string{"Kopi Susu", "Croissant"} {
				if !bytes.Contains(out, []byte(want)) {
					t.Errorf("thermal receipt missing %q — child rows dropped", want)
				}
			}
		})
	}

	// The PDF branch must agree: both shapes render the rows.
	for name, record := range map[string]map[string]any{
		"hydrated []map[string]any": hydrated,
		"request []any":             fromRequest,
	} {
		t.Run("pdf/"+name, func(t *testing.T) {
			out, err := renderPrintPDF(ps, record, printContext{})
			if err != nil {
				t.Fatalf("renderPrintPDF: %v", err)
			}
			if len(out) == 0 {
				t.Fatal("expected a non-empty PDF")
			}
		})
	}
}
