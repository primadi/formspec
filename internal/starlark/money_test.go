package starlark

import (
	"strings"
	"testing"

	"github.com/primadi/formspec/pkg/spec"
)

// money is the (now canonical) arithmetic type for `money` fields: the value
// is the object {amount, currency}. These tests pin the semantics decided in
// docs_internal/plan/money-arithmetic-semantics.md (S7 / gap #28).

func idr(amount string) spec.Money {
	return spec.Money{Amount: amount, Currency: "IDR"}
}

func usd(amount string) spec.Money {
	return spec.Money{Amount: amount, Currency: "USD"}
}

// TestEvalExpr_MoneySubtract — kembalian: tendered - amount must be a money
// value, not an error and not 0 (the kafe `payment.change` formula).
func TestEvalExpr_MoneySubtract(t *testing.T) {
	got, err := EvalExpr("tendered - amount", map[string]any{
		"tendered": idr("100000"),
		"amount":   idr("75000"),
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	m, ok := got.(spec.Money)
	if !ok {
		t.Fatalf("want spec.Money, got %T (%v)", got, got)
	}
	if m.Amount != "25000" || m.Currency != "IDR" {
		t.Fatalf("want {25000 IDR}, got %+v", m)
	}
}

// TestEvalExpr_MoneyAddAndSubtract — selisih kas: counted - expected, which may
// be negative (shortage).
func TestEvalExpr_MoneyAddAndSubtract(t *testing.T) {
	got, err := EvalExpr("counted_cash - expected_cash", map[string]any{
		"counted_cash":  idr("500000"),
		"expected_cash": idr("512500"),
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if m := got.(spec.Money); m.Amount != "-12500" || m.Currency != "IDR" {
		t.Fatalf("want {-12500 IDR}, got %+v", m)
	}

	sum, err := EvalExpr("a + b", map[string]any{"a": idr("1500"), "b": idr("2500.50")})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Insignificant trailing zeros are dropped — an amount never claims
	// precision it does not have (a 0-decimal currency would reject it).
	if m := sum.(spec.Money); m.Amount != "4000.5" {
		t.Fatalf("want 4000.5, got %s", m.Amount)
	}
}

// TestEvalExpr_MoneyMultiply — quantity * unit_price (the cafe_order
// `line_total` formula): number × money → money.
func TestEvalExpr_MoneyMultiply(t *testing.T) {
	for _, expr := range []string{"quantity * unit_price", "unit_price * quantity"} {
		got, err := EvalExpr(expr, map[string]any{
			"quantity":   3,
			"unit_price": idr("25000"),
		})
		if err != nil {
			t.Fatalf("%s: unexpected error: %v", expr, err)
		}
		if m := got.(spec.Money); m.Amount != "75000" || m.Currency != "IDR" {
			t.Fatalf("%s: want {75000 IDR}, got %+v", expr, m)
		}
	}

	// Decimal multiplier keeps its precision, and drops the insignificant zero
	// that `25000 * 0.5` would otherwise carry (0-decimal IDR fields reject it).
	got, err := EvalExpr("price * 0.5", map[string]any{"price": idr("25000")})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if m := got.(spec.Money); m.Amount != "12500" {
		t.Fatalf("want 12500, got %s", m.Amount)
	}
}

// TestEvalExpr_MoneyExactDecimal — money arithmetic must be exact; float64
// would give 0.30000000000000004 and blow the field's declared scale.
func TestEvalExpr_MoneyExactDecimal(t *testing.T) {
	got, err := EvalExpr("a + b", map[string]any{"a": idr("0.1"), "b": idr("0.2")})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if m := got.(spec.Money); m.Amount != "0.3" {
		t.Fatalf("want 0.3, got %s", m.Amount)
	}
}

// TestEvalExpr_MoneySumOverChildren — total_amount = sum of child line_totals
// (the cafe_order formula, which is already in the shipped spec).
func TestEvalExpr_MoneySumOverChildren(t *testing.T) {
	got, err := EvalExpr(`sum([i["line_total"] for i in items])`, map[string]any{
		"items": []any{
			map[string]any{"line_total": idr("25000")},
			map[string]any{"line_total": idr("12500")},
		},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if m := got.(spec.Money); m.Amount != "37500" || m.Currency != "IDR" {
		t.Fatalf("want {37500 IDR}, got %+v", m)
	}
}

// TestEvalExpr_MoneyDivision — money / number → money; money / money → ratio.
func TestEvalExpr_MoneyDivision(t *testing.T) {
	got, err := EvalExpr("total / 2", map[string]any{"total": idr("25000")})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if m := got.(spec.Money); m.Amount != "12500" {
		t.Fatalf("want 12500, got %+v", m)
	}

	ratio, err := EvalExpr("gross / net", map[string]any{
		"gross": idr("15000"), "net": idr("10000"),
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if f, ok := ratio.(float64); !ok || f != 1.5 {
		t.Fatalf("want ratio 1.5, got %T (%v)", ratio, ratio)
	}
}

// TestEvalExpr_MoneyComparison — comparisons between money values.
func TestEvalExpr_MoneyComparison(t *testing.T) {
	got, err := EvalExpr("tendered >= amount", map[string]any{
		"tendered": idr("100000"), "amount": idr("75000"),
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != true {
		t.Fatalf("want true, got %v", got)
	}

	eq, err := EvalExpr("a == b", map[string]any{"a": idr("1000"), "b": idr("1000")})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if eq != true {
		t.Fatalf("want a == b to be true (deep equality), got %v", eq)
	}
}

// TestEvalExpr_MoneyAccessors — amount()/currency() explicit extraction.
func TestEvalExpr_MoneyAccessors(t *testing.T) {
	got, err := EvalExpr("amount(price)", map[string]any{"price": idr("25000")})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if f, ok := got.(int64); !ok || f != 25000 {
		t.Fatalf("want 25000, got %T (%v)", got, got)
	}

	cur, err := EvalExpr("currency(price)", map[string]any{"price": idr("25000")})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cur != "IDR" {
		t.Fatalf("want IDR, got %v", cur)
	}

	// amount() on a plain number is the identity — the escape hatch for
	// mixing money with a plain-count field without inventing a currency.
	mixed, err := EvalExpr("amount(total) / item_count", map[string]any{
		"total": idr("25000"), "item_count": 4,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if f, ok := mixed.(float64); !ok || f != 6250 {
		t.Fatalf("want 6250, got %T (%v)", mixed, mixed)
	}
}

// TestEvalExpr_MoneyErrorsAreLoud — the whole point of S7: anything the engine
// cannot compute must be an error, never a silent 0.
func TestEvalExpr_MoneyErrorsAreLoud(t *testing.T) {
	cases := []struct {
		name string
		expr string
		env  map[string]any
		want string
	}{
		{
			name: "currency mismatch on subtract",
			expr: "a - b",
			env:  map[string]any{"a": idr("100000"), "b": usd("10")},
			want: "currency mismatch",
		},
		{
			name: "money minus number",
			expr: "total - 5",
			env:  map[string]any{"total": idr("100000")},
			want: "not defined",
		},
		{
			name: "money times money",
			expr: "a * b",
			env:  map[string]any{"a": idr("100"), "b": idr("2")},
			want: "not defined",
		},
		{
			name: "money divided by number zero",
			expr: "a / 0",
			env:  map[string]any{"a": idr("100")},
			want: "division by zero",
		},
		{
			name: "money divided by money zero",
			expr: "a / b",
			env:  map[string]any{"a": idr("100"), "b": idr("0")},
			want: "division by zero",
		},
		{
			name: "non-numeric operand",
			expr: "total - note",
			env:  map[string]any{"total": idr("100"), "note": "not money"},
			want: "not defined",
		},
		{
			name: "sum over mixed money and number",
			expr: "sum([a, 5])",
			env:  map[string]any{"a": idr("100")},
			want: "cannot mix money",
		},
		{
			name: "sum over currency mismatch",
			expr: "sum([a, b])",
			env:  map[string]any{"a": idr("100"), "b": usd("1")},
			want: "currency mismatch",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := EvalExpr(tc.expr, tc.env)
			if err == nil {
				t.Fatalf("want error containing %q, got value %T (%v)", tc.want, got, got)
			}
			if !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("want error containing %q, got %q", tc.want, err.Error())
			}
		})
	}
}

// TestEvalExpr_MoneyFromJSONMap — a record read back from the database carries
// money as a JSON object; it must behave the same as the struct form.
func TestEvalExpr_MoneyFromJSONMap(t *testing.T) {
	got, err := EvalExpr("tendered - amount", map[string]any{
		"tendered": map[string]any{"amount": "100000", "currency": "IDR"},
		"amount":   map[string]any{"amount": "25000", "currency": "IDR"},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if m := got.(spec.Money); m.Amount != "75000" {
		t.Fatalf("want 75000, got %+v", m)
	}
}

// TestEvalExpr_NonMoneyObjectStillCoerces — a plain (non-money) object must not
// silently become a money value; it stays a dict.
func TestEvalExpr_NonMoneyObjectStillCoerces(t *testing.T) {
	got, err := EvalExpr(`address["city"]`, map[string]any{
		"address": map[string]any{"city": "Bandung"},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != "Bandung" {
		t.Fatalf("want Bandung, got %T (%v)", got, got)
	}

	// and arithmetic on it is an error, not 0.
	if _, err := EvalExpr("address - 1", map[string]any{
		"address": map[string]any{"city": "Bandung"},
	}); err == nil {
		t.Fatal("want error for arithmetic on a non-money object")
	}
}

// TestEvaluateGuard_MoneyComparison — guards compare money with money
// (`resource.difference < resource.zero`). Comparison against a bare number is
// an error by design (there is no unit to compare to); `amount(x)` is the
// documented way to compare money with a plain number.
func TestEvaluateGuard_MoneyComparison(t *testing.T) {
	short, msg, err := EvaluateGuard("resource.difference < resource.limit", map[string]any{
		"difference": map[string]any{"amount": "-12500", "currency": "IDR"},
		"limit":      map[string]any{"amount": "0", "currency": "IDR"},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !short {
		t.Fatalf("a negative difference must be below a zero limit (msg=%q)", msg)
	}

	over, _, err := EvaluateGuard("resource.difference > resource.limit", map[string]any{
		"difference": map[string]any{"amount": "5000", "currency": "IDR"},
		"limit":      map[string]any{"amount": "0", "currency": "IDR"},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !over {
		t.Fatal("a positive difference must be above a zero limit")
	}
}

// TestEvaluateGuard_MoneyVsNumberIsAnError — money cannot be compared with a
// bare number (which currency would it be?). Loud, not a silent false; the
// escape hatch is amount(x).
func TestEvaluateGuard_MoneyVsNumberIsAnError(t *testing.T) {
	if _, _, err := EvaluateGuard("resource.difference > 100", map[string]any{
		"difference": map[string]any{"amount": "5000", "currency": "IDR"},
	}); err == nil {
		t.Fatal("comparing money with a plain number must error, not silently evaluate")
	}

	// The escape hatch: extract the scalar first.
	ok, _, err := EvaluateGuard("amount(resource.difference) > 100", map[string]any{
		"difference": map[string]any{"amount": "5000", "currency": "IDR"},
	})
	if err != nil {
		t.Fatalf("amount(x) must make the comparison legal: %v", err)
	}
	if !ok {
		t.Fatal("5000 > 100 should hold")
	}
}

// TestEvaluateGuard_AmountAccessor — amount(x) is available to guard/script
// expressions, not only to computed fields.
func TestEvaluateGuard_AmountAccessor(t *testing.T) {
	ok, msg, err := EvaluateGuard("amount(resource.total) > 100000", map[string]any{
		"total": map[string]any{"amount": "250000", "currency": "IDR"},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !ok {
		t.Fatalf("expected the guard to pass (msg=%q)", msg)
	}
}
