package starlark

import (
	"testing"

	"github.com/primadi/formspec/pkg/spec"
)

// money_zero(x) exists so an optional branch can fall back to a MONEY zero.
// Money arithmetic refuses to mix money with a bare number (05-field-types.md
// §2.1: "tidak ada koersi diam-diam"), so `subtotal if cond else 0` is an
// error; `subtotal if cond else money_zero(subtotal)` is not. Needed by the
// kafe order total chain (kafe 10.65).
//
// The currency comes from the operand, never a hardcoded code — the same "never
// guess the currency" rule ResolveMoneyCurrency enforces.
func TestMoneyZero_UsesOperandCurrency(t *testing.T) {
	v, err := EvalExpr(`money_zero(subtotal)`, map[string]any{
		"subtotal": map[string]any{"amount": "50000", "currency": "IDR"},
	})
	if err != nil {
		t.Fatalf("money_zero failed: %v", err)
	}
	m, ok := v.(spec.Money)
	if !ok {
		t.Fatalf("expected spec.Money, got %T (%v)", v, v)
	}
	if m.Currency != "IDR" {
		t.Errorf("currency = %q, want IDR (taken from the operand)", m.Currency)
	}
	if m.Amount != "0" {
		t.Errorf("amount = %q, want 0", m.Amount)
	}

	// A different operand currency must be reflected — proof the value is not
	// hardcoded to IDR by accident.
	v2, err := EvalExpr(`money_zero(subtotal)`, map[string]any{
		"subtotal": map[string]any{"amount": "1", "currency": "SGD"},
	})
	if err != nil {
		t.Fatalf("money_zero(SGD) failed: %v", err)
	}
	if got := v2.(spec.Money).Currency; got != "SGD" {
		t.Errorf("currency = %q, want SGD", got)
	}
}

// The whole point: a money zero must take part in money arithmetic without
// complaining about mixed types, including when the optional operand is ABSENT.
//
// `discount` is reached through the FieldMap (`resource.discount`), not as a
// bare identifier: a bare absent field is an "undefined:" COMPILE error, and
// evaluateComputed swallows that error and leaves the target field empty — the
// exact silent failure behind kafe 10.65. Pinned deliberately below.
func TestMoneyZero_ParticipatesInMoneyArithmetic(t *testing.T) {
	resource := NewFieldMap(map[string]any{
		"subtotal": map[string]any{"amount": "125000", "currency": "IDR"},
	})
	env := map[string]any{
		"resource":       resource,
		"subtotal":       map[string]any{"amount": "125000", "currency": "IDR"},
		"service_charge": map[string]any{"amount": "6250", "currency": "IDR"},
	}

	// `resource.discount` is absent from the map → None, which `sum` skips
	// while staying on the money path thanks to the money_zero anchor.
	v, err := EvalExpr(
		`subtotal + service_charge - sum([money_zero(subtotal), resource.discount])`,
		env)
	if err != nil {
		t.Fatalf("arithmetic with money_zero failed: %v", err)
	}
	m, ok := v.(spec.Money)
	if !ok {
		t.Fatalf("expected spec.Money, got %T (%v)", v, v)
	}
	// 125000 + 6250 - 0
	if m.Amount != "131250" {
		t.Errorf("amount = %q, want 131250", m.Amount)
	}

	// A bare identifier for an absent field IS an error — the reason the chain
	// must address optional operands through `resource.`.
	if _, err := EvalExpr(`subtotal + discount`, env); err == nil {
		t.Error("expected a compile error for a bare absent field (that is why `resource.` is required)")
	}
}

// money_zero on a non-money value must be a loud error, not a silent zero —
// the same reason `amount()` rejects a non-money argument.
func TestMoneyZero_RejectsNonMoney(t *testing.T) {
	if _, err := EvalExpr(`money_zero(5)`, nil); err == nil {
		t.Fatal("expected an error for a non-money argument")
	}
}

// A FieldMap answers None for a field that is not set, which is how a computed
// formula tests whether an OPTIONAL operand is present. Pinned here because the
// order total chain depends on it (kafe 10.65): before `resource` was exposed
// to computed formulas, an absent field was an undefined identifier — a compile
// error that evaluateComputed swallowed, leaving the target field empty.
func TestFieldMap_AbsentFieldIsNoneNotError(t *testing.T) {
	after := NewFieldMap(map[string]any{
		"subtotal": map[string]any{"amount": "1000", "currency": "IDR"},
	})

	v, err := EvalExpr(
		`resource.discount if resource.discount else money_zero(resource.subtotal)`,
		map[string]any{"resource": after})
	if err != nil {
		t.Fatalf("conditional over an absent optional field failed: %v", err)
	}
	if got := v.(spec.Money).Amount; got != "0" {
		t.Errorf("amount = %q, want 0 (the fallback branch)", got)
	}

	// And the present branch must win when the optional value IS there.
	present := NewFieldMap(map[string]any{
		"subtotal": map[string]any{"amount": "1000", "currency": "IDR"},
		"discount": map[string]any{"amount": "250", "currency": "IDR"},
	})
	v2, err := EvalExpr(
		`resource.discount if resource.discount else money_zero(resource.subtotal)`,
		map[string]any{"resource": present})
	if err != nil {
		t.Fatalf("conditional over a present field failed: %v", err)
	}
	if got := v2.(spec.Money).Amount; got != "250" {
		t.Errorf("amount = %q, want 250 (the optional value)", got)
	}
}
