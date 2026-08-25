// ─── Money type (05-field-types.md §2, todo 7.16) ───
//
// `money` is a first-class type: an exact amount (decimal with a fixed
// per-currency scale) paired with an ISO-4217 currency code. Modeling it as
// its own type forces amount + currency to always travel together.
//
// Rules implemented here:
//   - 7.16.2 Currency resolution order — explicit field `currency` →
//     `settings.currency` → error (never guess from heuristics).
//   - 7.16.4 A money field that overrides `currency` to a code other than
//     `settings.currency.code` MUST declare its own `decimal_places` — there
//     is no currency catalog to look it up from; not declaring it is a
//     VALIDATION_ERROR at apply time, not a silent guess.
//   - 7.16.3 Banker's rounding (round-half-to-even) is the default; overridable
//     via `settings.rounding` (half_up | half_down | up | down). Components
//     never pick a rounding mode themselves.
//
// FX / multi-currency conversion is OUT of core scope (module formspec/currency).

package spec

import (
	"fmt"
	"math"
	"strings"
)

// Money is the first-class money value: an exact decimal amount (string, to
// preserve precision) paired with an ISO-4217 currency code.
type Money struct {
	Amount   string `json:"amount"`
	Currency string `json:"currency"`
}

// MoneyFieldError is a validation error for a money field declaration.
type MoneyFieldError struct {
	Field   string
	Message string
}

func (e *MoneyFieldError) Error() string {
	return fmt.Sprintf("field %q: %s", e.Field, e.Message)
}

// ResolveMoneyCurrency resolves the currency for a money field per the
// normative order (05-field-types.md §2): explicit field `currency` →
// `settings.currency.code` → error (never guess). Returns the resolved code
// and the field's decimal_places (explicit field value, else the global
// currency's scale).
func ResolveMoneyCurrency(f *Field, settings *Settings) (code string, decimalPlaces int, err error) {
	if f == nil || f.Type != FieldMoney {
		return "", 0, nil
	}

	// 1. Explicit field currency.
	if f.Currency != "" {
		dp := 2
		if f.DecimalPlaces != nil {
			dp = *f.DecimalPlaces
		}
		return f.Currency, dp, nil
	}

	// 2. Global settings.currency.
	if settings != nil && settings.Currency != nil && settings.Currency.Code != "" {
		dp := 2
		if settings.Currency.DecimalPlaces != nil {
			dp = *settings.Currency.DecimalPlaces
		}
		return settings.Currency.Code, dp, nil
	}

	// 3. Neither → error, never guess.
	return "", 0, &MoneyFieldError{
		Field:   f.Name,
		Message: "money field has no currency: declare `currency` on the field or set `settings.currency` (never guess)",
	}
}

// ValidateMoneyField validates a money field declaration (todo 7.16.4):
// a field that overrides `currency` to a code other than the global default
// MUST declare its own `decimal_places`. Returns an error when invalid.
func ValidateMoneyField(f *Field, settings *Settings) error {
	if f == nil || f.Type != FieldMoney {
		return nil
	}

	// No explicit currency → inherits global; nothing to validate here.
	if f.Currency == "" {
		return nil
	}

	// Explicit currency that differs from the global default MUST declare
	// decimal_places (no currency catalog to look it up from).
	globalCode := ""
	if settings != nil && settings.Currency != nil {
		globalCode = settings.Currency.Code
	}
	if f.Currency != globalCode && f.DecimalPlaces == nil {
		return &MoneyFieldError{
			Field:   f.Name,
			Message: fmt.Sprintf("money field overrides currency to %q (global %q) — must declare `decimal_places` (no currency catalog to look it up from)", f.Currency, globalCode),
		}
	}
	return nil
}

// RoundingMode is the rounding strategy for money/decimal arithmetic.
type RoundingMode string

const (
	// RoundingHalfEven is banker's rounding (round-half-to-even) — the
	// default, chosen for statistical neutrality on financial aggregates.
	RoundingHalfEven RoundingMode = "half_even"
	RoundingHalfUp   RoundingMode = "half_up"
	RoundingHalfDown RoundingMode = "half_down"
	RoundingUp       RoundingMode = "up"
	RoundingDown     RoundingMode = "down"
)

// ResolveRoundingMode maps a settings.rounding string to a RoundingMode,
// defaulting to banker's rounding (half_even).
func ResolveRoundingMode(s string) RoundingMode {
	switch RoundingMode(strings.ToLower(strings.TrimSpace(s))) {
	case RoundingHalfUp:
		return RoundingHalfUp
	case RoundingHalfDown:
		return RoundingHalfDown
	case RoundingUp:
		return RoundingUp
	case RoundingDown:
		return RoundingDown
	default:
		return RoundingHalfEven
	}
}

// RoundMoney rounds a float64 amount to the given decimal places using the
// rounding mode (default banker's rounding, 7.16.3). Returns the rounded
// value as a float64.
func RoundMoney(amount float64, decimalPlaces int, mode RoundingMode) float64 {
	if decimalPlaces < 0 {
		decimalPlaces = 0
	}
	scale := math.Pow10(decimalPlaces)
	scaled := amount * scale

	var rounded float64
	switch mode {
	case RoundingHalfUp:
		rounded = math.Floor(scaled + 0.5)
	case RoundingHalfDown:
		rounded = math.Ceil(scaled - 0.5)
	case RoundingUp:
		rounded = math.Ceil(scaled)
	case RoundingDown:
		rounded = math.Floor(scaled)
	default: // half_even (banker's)
		floored := math.Floor(scaled)
		frac := scaled - floored
		if frac > 0.5 {
			rounded = floored + 1
		} else if frac < 0.5 {
			rounded = floored
		} else {
			// Exactly .5 — round to even.
			if math.Mod(floored, 2) == 0 {
				rounded = floored
			} else {
				rounded = floored + 1
			}
		}
	}

	return rounded / scale
}

// ValidateMoneyValue validates a stored money value against the field's
// resolved currency and scale. The value may be a Money struct, a map with
// "amount"/"currency" keys, or a plain number (inherits the field currency).
// Returns an error when the currency mismatches or the amount has more
// decimal places than the field's scale.
func ValidateMoneyValue(f *Field, value any, settings *Settings) error {
	if f == nil || f.Type != FieldMoney {
		return nil
	}
	code, dp, err := ResolveMoneyCurrency(f, settings)
	if err != nil {
		return err
	}

	amount, currency := moneyParts(value)
	if currency != "" && currency != code {
		return &MoneyFieldError{
			Field:   f.Name,
			Message: fmt.Sprintf("money currency %q does not match field currency %q", currency, code),
		}
	}
	if amount == "" {
		return nil
	}
	// Scale check: count decimal places in the amount string.
	dot := strings.IndexByte(amount, '.')
	if dot >= 0 && len(amount)-dot-1 > dp {
		return &MoneyFieldError{
			Field:   f.Name,
			Message: fmt.Sprintf("money amount %q exceeds field scale %d", amount, dp),
		}
	}
	return nil
}

// moneyParts extracts (amount, currency) from a money value: a Money struct,
// a map with "amount"/"currency", or a plain scalar (amount only).
func moneyParts(value any) (string, string) {
	switch v := value.(type) {
	case Money:
		return v.Amount, v.Currency
	case *Money:
		if v == nil {
			return "", ""
		}
		return v.Amount, v.Currency
	case map[string]any:
		amount, _ := v["amount"].(string)
		currency, _ := v["currency"].(string)
		return amount, currency
	case string:
		return v, ""
	case float64:
		return fmt.Sprintf("%v", v), ""
	default:
		return "", ""
	}
}
