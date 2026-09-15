// ─── Money value for computed formulas (S7 / gap #28) ───
//
// `money` is a first-class type (pkg/spec/money.go) whose value is the object
// `{amount, currency}`. Computed formulas, however, are scalar-oriented
// expressions — so before this file existed, `tendered - amount` reached
// Starlark as `dict - dict`, failed, and the computed field was silently left
// absent.
//
// This file makes money a Starlark value with real arithmetic:
//
//	m + m, m - m   → money          (currencies must match; mismatch = error)
//	m * n, n * m   → money          (n scalar)
//	m / n          → money          (n scalar; 0 = error)
//	m / m          → number         (ratio; currencies must match)
//	-m             → money
//	m <op> m       → boolean        (same currency)
//
// and registers `amount(x)` / `currency(x)` for explicit scalar extraction.
//
// Arithmetic is exact (math/big.Rat, never float64): `0.1 + 0.2` must be `0.3`
// and not `0.30000000000000004`, which would blow the field's declared scale
// (05-field-types.md §2, ValidateMoneyValue).
//
// Anything else — a non-money object, an array, a bool, a non-numeric string —
// is an ERROR, never a silent 0. The whole point of the gap is that a wrong
// money number must be loud.

package starlark

import (
	"fmt"
	"math/big"
	"strconv"
	"strings"

	"go.starlark.net/starlark"
	"go.starlark.net/syntax"

	"github.com/primadi/formspec/pkg/spec"
)

// moneyValue is the Starlark representation of a money value.
type moneyValue struct {
	amount   *big.Rat
	currency string
	// scale is the number of decimal places used when rendering the amount
	// back to a string. It is derived from the operands so that exact results
	// keep their natural precision (25000 * 2 → "50000", not "50000.00").
	scale int
}

var (
	_ starlark.Value      = (*moneyValue)(nil)
	_ starlark.HasBinary  = (*moneyValue)(nil)
	_ starlark.HasUnary   = (*moneyValue)(nil)
	_ starlark.Comparable = (*moneyValue)(nil)
)

// newMoneyValue parses a decimal amount string into a moneyValue.
func newMoneyValue(amount, currency string) (*moneyValue, error) {
	trimmed := strings.TrimSpace(amount)
	if trimmed == "" {
		return nil, fmt.Errorf("money: missing amount")
	}
	r, ok := new(big.Rat).SetString(trimmed)
	if !ok {
		return nil, fmt.Errorf("money: invalid amount %q (want a plain decimal number)", amount)
	}
	if c := strings.TrimSpace(currency); c != "" {
		currency = c
	} else {
		return nil, fmt.Errorf("money: missing currency (a money value always carries its currency)")
	}
	return &moneyValue{amount: r, currency: currency, scale: decimalScale(trimmed)}, nil
}

// decimalScale counts the decimal places in a decimal string literal.
func decimalScale(s string) int {
	if i := strings.IndexByte(s, '.'); i >= 0 {
		// Trailing zeros are significant for the rendered form but not beyond
		// the literal's own precision.
		return len(s) - i - 1
	}
	return 0
}

func (m *moneyValue) String() string {
	return fmt.Sprintf("%s %s", m.amountString(), m.currency)
}

func (m *moneyValue) Type() string { return "money" }
func (m *moneyValue) Freeze()      {}

func (m *moneyValue) Truth() starlark.Bool { return m.amount.Sign() != 0 }

func (m *moneyValue) Hash() (uint32, error) {
	return 0, fmt.Errorf("unhashable type: money")
}

// amountString renders the amount with the value's scale, minus insignificant
// trailing zeros.
func (m *moneyValue) amountString() string {
	if m.scale <= 0 {
		return m.amount.FloatString(0)
	}
	return trimTrailingZeros(m.amount.FloatString(m.scale))
}

// trimTrailingZeros drops insignificant trailing zeros ("4000.50" → "4000.5",
// "12500.0" → "12500"). A computed amount must not claim precision it does not
// have: an IDR field (decimal_places 0) would otherwise reject the perfectly
// exact result of `25000 * 0.5` for having "one decimal place".
func trimTrailingZeros(s string) string {
	if !strings.Contains(s, ".") {
		return s
	}
	return strings.TrimSuffix(strings.TrimRight(s, "0"), ".")
}

// Money returns the canonical Go representation (pkg/spec.Money).
func (m *moneyValue) Money() spec.Money {
	return spec.Money{Amount: m.amountString(), Currency: m.currency}
}

func (m *moneyValue) result(amount *big.Rat, scale int) *moneyValue {
	return &moneyValue{amount: amount, currency: m.currency, scale: scale}
}

// Binary implements +, -, *, / for money (S7 canonical semantics).
func (m *moneyValue) Binary(op syntax.Token, y starlark.Value, side starlark.Side) (starlark.Value, error) {
	other, isMoney := y.(*moneyValue)

	switch op {
	case syntax.PLUS, syntax.MINUS:
		if side == starlark.Right {
			// scalar ± money has no unit meaning.
			return nil, moneyOperandError(op, m, y, side)
		}
		if !isMoney {
			return nil, moneyOperandError(op, m, y, side)
		}
		if other.currency != m.currency {
			return nil, currencyMismatchError(op, m.currency, other.currency)
		}
		sum := new(big.Rat)
		if op == syntax.PLUS {
			sum.Add(m.amount, other.amount)
		} else {
			sum.Sub(m.amount, other.amount)
		}
		return m.result(sum, maxInt(m.scale, other.scale)), nil

	case syntax.STAR:
		if isMoney {
			return nil, fmt.Errorf(
				"money * money is not defined: multiplying two amounts gives %s·%s, which is not a currency (use money / money for a ratio)",
				m.currency, other.currency)
		}
		// Either money * number or number * money — both are money. When the
		// operator is `number * money`, Starlark consults the right operand.
		scalar, scalarScale, err := scalarRat(y)
		if err != nil {
			return nil, err
		}
		product := new(big.Rat).Mul(m.amount, scalar)
		return m.result(product, m.scale+scalarScale), nil

	case syntax.SLASH:
		if isMoney {
			if other.currency != m.currency {
				return nil, currencyMismatchError(op, m.currency, other.currency)
			}
			if other.amount.Sign() == 0 {
				return nil, fmt.Errorf("money / money: division by zero")
			}
			ratio := new(big.Rat).Quo(m.amount, other.amount)
			f, _ := ratio.Float64()
			return starlark.Float(f), nil
		}
		if side == starlark.Right {
			// scalar / money is not a currency.
			return nil, moneyOperandError(op, m, y, side)
		}
		divisor, _, err := scalarRat(y)
		if err != nil {
			return nil, err
		}
		if divisor.Sign() == 0 {
			return nil, fmt.Errorf("money / number: division by zero")
		}
		return m.result(new(big.Rat).Quo(m.amount, divisor), m.scale), nil
	}

	// Decline anything else (e.g. % or //) so Starlark reports it normally.
	return nil, nil
}

// Unary implements -money.
func (m *moneyValue) Unary(op syntax.Token) (starlark.Value, error) {
	switch op {
	case syntax.MINUS:
		return m.result(new(big.Rat).Neg(m.amount), m.scale), nil
	case syntax.PLUS:
		return m, nil
	}
	return nil, nil
}

// CompareSameType implements <, <=, >, >=, ==, != between two money values.
func (m *moneyValue) CompareSameType(op syntax.Token, y starlark.Value, _ int) (bool, error) {
	other, ok := y.(*moneyValue)
	if !ok {
		return false, fmt.Errorf("money cannot be compared with %s", y.Type())
	}
	if other.currency != m.currency {
		return false, currencyMismatchError(op, m.currency, other.currency)
	}
	cmp := m.amount.Cmp(other.amount)
	switch op {
	case syntax.EQL:
		return cmp == 0, nil
	case syntax.NEQ:
		return cmp != 0, nil
	case syntax.LT:
		return cmp < 0, nil
	case syntax.LE:
		return cmp <= 0, nil
	case syntax.GT:
		return cmp > 0, nil
	case syntax.GE:
		return cmp >= 0, nil
	}
	return false, fmt.Errorf("money %s money not implemented", op)
}

// scalarRat converts a Starlark scalar operand to an exact rational.
// Booleans, objects, arrays and non-numeric strings are rejected — for money
// arithmetic, "cannot tell" must never become 0.
func scalarRat(v starlark.Value) (*big.Rat, int, error) {
	switch x := v.(type) {
	case starlark.Int:
		return new(big.Rat).SetInt(x.BigInt()), 0, nil
	case starlark.Float:
		f := float64(x)
		r := new(big.Rat).SetFloat64(f)
		if r == nil {
			return nil, 0, fmt.Errorf("money: non-finite number %v", f)
		}
		return r, floatScale(x), nil
	case starlark.String:
		s := strings.TrimSpace(string(x))
		r, ok := new(big.Rat).SetString(s)
		if !ok {
			return nil, 0, fmt.Errorf("money: %q is not a number (currency symbols and thousands separators are not allowed)", string(x))
		}
		return r, decimalScale(s), nil
	}
	return nil, 0, fmt.Errorf("money: %s is not a number — money may only be combined with a number or another money value", v.Type())
}

// floatScale returns the decimal scale needed to render a float64 exactly
// (so that `price * 0.5` keeps its half-cent rather than being truncated).
func floatScale(f starlark.Float) int {
	s := strconv.FormatFloat(float64(f), 'f', -1, 64)
	return decimalScale(s)
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func currencyMismatchError(op syntax.Token, left, right string) error {
	return fmt.Errorf("money %s money: currency mismatch (%s vs %s) — convert explicitly before combining currencies", op, left, right)
}

func moneyOperandError(op syntax.Token, m *moneyValue, y starlark.Value, side starlark.Side) error {
	left, right := m.Type(), y.Type()
	if side == starlark.Right {
		left, right = y.Type(), m.Type()
	}
	return fmt.Errorf("%s %s %s is not defined: money can only be added to, subtracted from, multiplied by, or divided by a number or another money value of the same currency", left, op, right)
}

// ── Builtins ──

// moneyBuiltins returns the money accessor builtins: amount(x) and currency(x),
// plus the collision-proof aliases money_amount(x) / money_currency(x) for
// entities whose own fields are named `amount` / `currency` (environment
// variables shadow builtins, so the aliases always resolve).
func moneyBuiltins() starlark.StringDict {
	amountFn := starlark.NewBuiltin("amount", func(
		_ *starlark.Thread, _ *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple,
	) (starlark.Value, error) {
		var v starlark.Value
		if err := starlark.UnpackArgs("amount", args, kwargs, "value", &v); err != nil {
			return nil, err
		}
		// A money value yields its amount; a number is already a scalar.
		if m, ok := v.(*moneyValue); ok {
			return ratToStarlark(m.amount), nil
		}
		r, _, err := scalarRat(v)
		if err != nil {
			return nil, fmt.Errorf("amount(): %w", err)
		}
		return ratToStarlark(r), nil
	})
	currencyFn := starlark.NewBuiltin("currency", func(
		_ *starlark.Thread, _ *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple,
	) (starlark.Value, error) {
		var v starlark.Value
		if err := starlark.UnpackArgs("currency", args, kwargs, "value", &v); err != nil {
			return nil, err
		}
		m, ok := v.(*moneyValue)
		if !ok {
			return nil, fmt.Errorf("currency(): expected a money value, got %s", v.Type())
		}
		return starlark.String(m.currency), nil
	})
	return starlark.StringDict{
		"amount":         amountFn,
		"currency":       currencyFn,
		"money_amount":   amountFn,
		"money_currency": currencyFn,
	}
}

// ratToStarlark renders an exact rational as an int when it is integral, so
// that amount(25000) == 25000 rather than 25000.0.
func ratToStarlark(r *big.Rat) starlark.Value {
	if r.IsInt() {
		return starlark.MakeInt64(r.Num().Int64())
	}
	f, _ := r.Float64()
	return starlark.Float(f)
}

// asMoneyValue converts a Go money-shaped value into a moneyValue.
// Accepts spec.Money / *spec.Money and JSON-decoded maps.
func asMoneyValue(v any) (*moneyValue, bool, error) {
	switch x := v.(type) {
	case spec.Money:
		m, err := newMoneyValue(x.Amount, x.Currency)
		return m, true, err
	case *spec.Money:
		if x == nil {
			return nil, true, nil
		}
		m, err := newMoneyValue(x.Amount, x.Currency)
		return m, true, err
	case map[string]any:
		if _, ok := x["amount"]; !ok {
			return nil, false, nil
		}
		if _, ok := x["currency"]; !ok {
			return nil, false, nil
		}
		amount, err := moneyString(x["amount"])
		if err != nil {
			return nil, true, err
		}
		currency, _ := x["currency"].(string)
		if currency == "" {
			return nil, false, nil
		}
		m, err := newMoneyValue(amount, currency)
		return m, true, err
	}
	return nil, false, nil
}

// moneyString renders a JSON-decoded amount of unknown numeric type as a string.
func moneyString(v any) (string, error) {
	switch a := v.(type) {
	case string:
		return a, nil
	case float64:
		return strconv.FormatFloat(a, 'f', -1, 64), nil
	case int:
		return strconv.Itoa(a), nil
	case int64:
		return strconv.FormatInt(a, 10), nil
	}
	return "", fmt.Errorf("money: unsupported amount type %T", v)
}

// sumValues is the money-aware `sum` builtin body: a list of money values sums
// to money; a list of scalars sums to a number. Mixing the two — or summing
// anything non-numeric — is an error rather than a silent 0.
func sumValues(iterable starlark.Value) (starlark.Value, error) {
	var elems []starlark.Value
	switch x := iterable.(type) {
	case *starlark.List:
		elems = make([]starlark.Value, x.Len())
		for i := range elems {
			elems[i] = x.Index(i)
		}
	case starlark.Tuple:
		elems = []starlark.Value(x)
	default:
		return nil, fmt.Errorf("sum: expected list or tuple, got %s", iterable.Type())
	}

	for _, e := range elems {
		if _, ok := e.(*moneyValue); ok {
			return sumMoney(elems)
		}
	}

	total := new(big.Rat)
	for _, e := range elems {
		if e == starlark.None {
			continue
		}
		r, _, err := scalarRat(e)
		if err != nil {
			return nil, fmt.Errorf("sum: %w", err)
		}
		total.Add(total, r)
	}
	f, _ := total.Float64()
	return starlark.Float(f), nil
}

// sumMoney sums a list whose elements are all money values of one currency.
func sumMoney(elems []starlark.Value) (starlark.Value, error) {
	total := new(big.Rat)
	var currency string
	scale := 0
	for _, e := range elems {
		if e == starlark.None {
			continue
		}
		m, ok := e.(*moneyValue)
		if !ok {
			return nil, fmt.Errorf("sum: cannot mix money and %s in one sum — a total must be all money or all numbers", e.Type())
		}
		if currency == "" {
			currency = m.currency
		} else if m.currency != currency {
			return nil, fmt.Errorf("sum: currency mismatch (%s vs %s) — convert explicitly before totalling", currency, m.currency)
		}
		total.Add(total, m.amount)
		scale = maxInt(scale, m.scale)
	}
	if currency == "" {
		return starlark.Float(0), nil
	}
	return &moneyValue{amount: total, currency: currency, scale: scale}, nil
}
