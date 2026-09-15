package spec

import (
	"math"
	"testing"
)

// TestResolveMoneyCurrency verifies the currency resolution order (7.16.2):
// explicit field currency → settings.currency → error (never guess).
func TestResolveMoneyCurrency(t *testing.T) {
	settings := &Settings{Currency: &CurrencySettings{Code: "IDR", DecimalPlaces: intPtr(0)}}

	// Explicit field currency wins.
	f := &Field{Name: "fee", Type: FieldMoney, Currency: "USD", DecimalPlaces: intPtr(2)}
	code, dp, err := ResolveMoneyCurrency(f, settings)
	if err != nil || code != "USD" || dp != 2 {
		t.Fatalf("explicit currency: code=%q dp=%d err=%v", code, dp, err)
	}

	// No explicit currency → global settings.
	f2 := &Field{Name: "total", Type: FieldMoney}
	code, dp, err = ResolveMoneyCurrency(f2, settings)
	if err != nil || code != "IDR" || dp != 0 {
		t.Fatalf("global currency: code=%q dp=%d err=%v", code, dp, err)
	}

	// Neither → error.
	f3 := &Field{Name: "total", Type: FieldMoney}
	_, _, err = ResolveMoneyCurrency(f3, nil)
	if err == nil {
		t.Fatal("expected error when no currency source")
	}
}

// TestValidateMoneyField verifies 7.16.4: non-default currency MUST declare
// decimal_places.
func TestValidateMoneyField(t *testing.T) {
	settings := &Settings{Currency: &CurrencySettings{Code: "IDR"}}

	// Non-default currency without decimal_places → error.
	f := &Field{Name: "fee", Type: FieldMoney, Currency: "USD"}
	if err := ValidateMoneyField(f, settings); err == nil {
		t.Fatal("expected error for non-default currency without decimal_places")
	}

	// Non-default currency WITH decimal_places → ok.
	f2 := &Field{Name: "fee", Type: FieldMoney, Currency: "USD", DecimalPlaces: intPtr(2)}
	if err := ValidateMoneyField(f2, settings); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Default currency without decimal_places → ok (inherits global scale).
	f3 := &Field{Name: "total", Type: FieldMoney}
	if err := ValidateMoneyField(f3, settings); err != nil {
		t.Fatalf("unexpected error for default currency: %v", err)
	}
}

// TestRoundMoney_Banker verifies banker's rounding (round-half-to-even) is
// the default (7.16.3).
func TestRoundMoney_Banker(t *testing.T) {
	cases := []struct {
		in   float64
		dp   int
		want float64
	}{
		{2.5, 0, 2},      // half → even (2)
		{3.5, 0, 4},      // half → even (4)
		{2.4, 0, 2},      // below half → down
		{2.6, 0, 3},      // above half → up
		{0.125, 2, 0.12}, // 2dp half → even (0.12)
		{0.135, 2, 0.14}, // 2dp half → even (0.14)
	}
	for _, c := range cases {
		got := RoundMoney(c.in, c.dp, RoundingHalfEven)
		if math.Abs(got-c.want) > 1e-9 {
			t.Errorf("RoundMoney(%v, %d, half_even) = %v, want %v", c.in, c.dp, got, c.want)
		}
	}
}

// TestRoundMoney_Modes verifies the other rounding modes.
func TestRoundMoney_Modes(t *testing.T) {
	if got := RoundMoney(2.5, 0, RoundingHalfUp); got != 3 {
		t.Errorf("half_up(2.5) = %v, want 3", got)
	}
	if got := RoundMoney(2.5, 0, RoundingHalfDown); got != 2 {
		t.Errorf("half_down(2.5) = %v, want 2", got)
	}
	if got := RoundMoney(2.1, 0, RoundingUp); got != 3 {
		t.Errorf("up(2.1) = %v, want 3", got)
	}
	if got := RoundMoney(2.9, 0, RoundingDown); got != 2 {
		t.Errorf("down(2.9) = %v, want 2", got)
	}
}

// TestResolveRoundingMode verifies settings.rounding mapping with banker's
// default.
func TestResolveRoundingMode(t *testing.T) {
	if ResolveRoundingMode("") != RoundingHalfEven {
		t.Error("empty should default to half_even")
	}
	if ResolveRoundingMode("half_up") != RoundingHalfUp {
		t.Error("half_up not mapped")
	}
	if ResolveRoundingMode("HALF_UP") != RoundingHalfUp {
		t.Error("case-insensitive mapping failed")
	}
}

// TestValidateMoneyValue verifies stored money value validation (currency
// mismatch + scale).
func TestValidateMoneyValue(t *testing.T) {
	settings := &Settings{Currency: &CurrencySettings{Code: "IDR", DecimalPlaces: intPtr(0)}}
	f := &Field{Name: "total", Type: FieldMoney}

	// Currency mismatch → error.
	if err := ValidateMoneyValue(f, map[string]any{"amount": "100", "currency": "USD"}, settings); err == nil {
		t.Fatal("expected currency mismatch error")
	}

	// Scale exceeded → error.
	f2 := &Field{Name: "fee", Type: FieldMoney, Currency: "USD", DecimalPlaces: intPtr(2)}
	if err := ValidateMoneyValue(f2, map[string]any{"amount": "1.234", "currency": "USD"}, settings); err == nil {
		t.Fatal("expected scale error")
	}

	// Valid → ok.
	if err := ValidateMoneyValue(f2, map[string]any{"amount": "1.23", "currency": "USD"}, settings); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

// TestNormalizeMoneyValue verifies that the three shapes a client can send are
// collapsed into one canonical {amount, currency} value at the API boundary,
// with the currency resolved from the field or settings (gap #46).
func TestNormalizeMoneyValue(t *testing.T) {
	settings := &Settings{Currency: &CurrencySettings{Code: "IDR", DecimalPlaces: intPtr(0)}}
	f := &Field{Name: "value", Type: FieldMoney}

	// Bare number → object with settings currency.
	got, err := NormalizeMoneyValue(float64(25000), f, settings)
	if err != nil {
		t.Fatalf("number: %v", err)
	}
	if got.Amount != "25000" || got.Currency != "IDR" {
		t.Errorf("number: got %+v, want {25000 IDR}", got)
	}

	// Numeric string → same.
	if got, err = NormalizeMoneyValue("7000", f, settings); err != nil || got.Currency != "IDR" {
		t.Errorf("string: got %+v err %v", got, err)
	}

	// Object without currency → currency filled in.
	if got, err = NormalizeMoneyValue(map[string]any{"amount": "15000"}, f, settings); err != nil {
		t.Fatalf("object: %v", err)
	}
	if got.Amount != "15000" || got.Currency != "IDR" {
		t.Errorf("object: got %+v, want {15000 IDR}", got)
	}

	// Explicit currency in the payload wins.
	if got, err = NormalizeMoneyValue(map[string]any{"amount": "9", "currency": "USD"}, f, settings); err != nil {
		t.Fatalf("explicit currency: %v", err)
	}
	if got.Currency != "USD" {
		t.Errorf("explicit currency: got %q, want USD", got.Currency)
	}

	// Non-numeric amount → error, never stored as-is.
	if _, err = NormalizeMoneyValue("Rp25.000", f, settings); err == nil {
		t.Error("expected error for non-numeric amount")
	}

	// No currency anywhere → error (never guess).
	if _, err = NormalizeMoneyValue(float64(1), &Field{Name: "value", Type: FieldMoney}, nil); err == nil {
		t.Error("expected error when no currency can be resolved")
	}
}

func intPtr(i int) *int { return &i }
