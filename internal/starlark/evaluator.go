// Package starlark provides a Starlark expression evaluator for FormSpec computed fields.
//
// Computed fields use Starlark expressions (a Python-like subset) to derive values
// from other fields in the same record. The evaluator runs in a sandboxed environment
// with no I/O or network access — only arithmetic, string ops, and logical operators.
//
// Example formula: "data.subtotal * (1 + data.tax_rate / 100)"
package starlark

import (
	"fmt"
	"math"
	"time"

	"go.starlark.net/starlark"
	"go.starlark.net/syntax"

	"github.com/primadi/formspec/pkg/spec"
)

// EvalExpr evaluates a Starlark expression with the given environment variables.
//
// Parameters:
//   - expr: Starlark expression string (e.g. "subtotal * 1.1")
//   - env: map of variable names to values (e.g. {"subtotal": 100.0, "tax_rate": 10.0})
//
// Returns:
//   - The evaluated result as a Go value (int, float64, string, bool, or nil)
//   - Error if evaluation fails or the expression is invalid
//
// The expression runs in a sandboxed Starlark thread with no I/O or network access.
// Env variables are injected as top-level predeclared identifiers, so `subtotal`
// is accessible directly (not via `data.subtotal`). Use bracket syntax for computed
// keys: data["key"].
func EvalExpr(expr string, env map[string]any) (any, error) {
	// Built-in constants and helpers are registered first so that environment
	// variables always win: a field named `amount` is, first and foremost, that
	// field. The money accessors it shadows are also available under their
	// collision-proof aliases (money_amount / money_currency).
	predeclared := make(starlark.StringDict, len(env)+10)
	predeclared["math_pi"] = starlark.Float(math.Pi)
	predeclared["math_e"] = starlark.Float(math.E)
	predeclared["today"] = starlark.NewBuiltin("today", func(
		thread *starlark.Thread,
		fn *starlark.Builtin,
		args starlark.Tuple,
		kwargs []starlark.Tuple,
	) (starlark.Value, error) {
		return starlark.String(time.Now().Format("2006-01-02")), nil
	})
	predeclared["days_ago"] = starlark.NewBuiltin("days_ago", func(
		thread *starlark.Thread,
		fn *starlark.Builtin,
		args starlark.Tuple,
		kwargs []starlark.Tuple,
	) (starlark.Value, error) {
		var n int
		if err := starlark.UnpackArgs("days_ago", args, kwargs, "days", &n); err != nil {
			return nil, err
		}
		return starlark.String(time.Now().AddDate(0, 0, -n).Format("2006-01-02")), nil
	})
	predeclared["empty"] = starlark.NewBuiltin("empty", func(
		thread *starlark.Thread,
		fn *starlark.Builtin,
		args starlark.Tuple,
		kwargs []starlark.Tuple,
	) (starlark.Value, error) {
		var v starlark.Value
		if err := starlark.UnpackArgs("empty", args, kwargs, "value", &v); err != nil {
			return nil, err
		}
		if v == nil || v == starlark.None {
			return starlark.True, nil
		}
		switch x := v.(type) {
		case starlark.String:
			return starlark.Bool(string(x) == ""), nil
		case *starlark.List:
			return starlark.Bool(x.Len() == 0), nil
		case *starlark.Dict:
			return starlark.Bool(x.Len() == 0), nil
		}
		return starlark.False, nil
	})
	// sum() is money-aware: a list of money values sums to money, a list of
	// numbers to a number, and a mixed/illegal list is an error (S7).
	predeclared["sum"] = starlark.NewBuiltin("sum", func(
		thread *starlark.Thread,
		fn *starlark.Builtin,
		args starlark.Tuple,
		kwargs []starlark.Tuple,
	) (starlark.Value, error) {
		var iterable starlark.Value
		if err := starlark.UnpackArgs("sum", args, kwargs, "iterable", &iterable); err != nil {
			return nil, err
		}
		return sumValues(iterable)
	})

	// amount(x) / currency(x) — explicit scalar extraction from a money value,
	// plus collision-proof aliases for entities that have a field named
	// `amount` or `currency`.
	for name, b := range moneyBuiltins() {
		predeclared[name] = b
	}

	// Environment variables last: they shadow any same-named builtin.
	for k, v := range env {
		sv, err := toStarlark(v)
		if err != nil {
			return nil, fmt.Errorf("starlark eval: convert env %q: %w", k, err)
		}
		predeclared[k] = sv
	}

	// Create a sandboxed thread
	thread := &starlark.Thread{
		Name: "computed",
		Print: func(_ *starlark.Thread, msg string) {
			// Suppress print — computed fields shouldn't produce output
		},
	}

	// Evaluate
	val, err := starlark.EvalOptions(syntax.LegacyFileOptions(), thread, "computed", expr, predeclared)
	if err != nil {
		return nil, fmt.Errorf("starlark eval: %w", err)
	}

	return fromStarlark(val), nil
}

// toStarlark converts a Go value to a Starlark value.
func toStarlark(v any) (starlark.Value, error) {
	if v == nil {
		return starlark.None, nil
	}

	switch x := v.(type) {
	case *FieldMap:
		// Already a Starlark value — pass through so dot-notation field
		// access (resource.amount) keeps working.
		return x, nil
	case *moneyValue:
		// Already a Starlark value (a money field inside a nested record).
		return x, nil
	case spec.Money, *spec.Money:
		// A money field is the object {amount, currency}. Exposed as a plain
		// dict, `tendered - amount` becomes `dict - dict` — which fails and
		// silently leaves the computed field absent (S7 / gap #28).
		m, isMoney, err := asMoneyValue(x)
		if err != nil {
			return nil, err
		}
		if !isMoney || m == nil {
			return starlark.None, nil
		}
		return m, nil
	case bool:
		return starlark.Bool(x), nil
	case int:
		return starlark.MakeInt(x), nil
	case int64:
		return starlark.MakeInt64(x), nil
	case float64:
		return starlark.Float(x), nil
	case string:
		return starlark.String(x), nil
	case []any:
		elements := make([]starlark.Value, len(x))
		for i, elem := range x {
			sv, err := toStarlark(elem)
			if err != nil {
				return nil, err
			}
			elements[i] = sv
		}
		return starlark.NewList(elements), nil
	case []map[string]any:
		// Query results (ctx.db().query) come back as []map[string]any.
		elements := make([]starlark.Value, len(x))
		for i, elem := range x {
			sv, err := toStarlark(elem)
			if err != nil {
				return nil, err
			}
			elements[i] = sv
		}
		return starlark.NewList(elements), nil
	case map[string]any:
		// JSON-decoded money (e.g. a record read back from the database) is
		// money-shaped: {amount, currency}. Recognize it so formulas over money
		// behave identically on create and on read-back.
		if m, isMoney, err := asMoneyValue(x); err != nil {
			return nil, err
		} else if isMoney {
			if m == nil {
				return starlark.None, nil
			}
			return m, nil
		}
		return toStarlarkMap(x)
	default:
		return starlark.String(fmt.Sprintf("%v", x)), nil
	}
}

// toStarlarkMap converts a Go map to a Starlark dict, converting any nested
// money-shaped value along the way.
func toStarlarkMap(m map[string]any) (starlark.Value, error) {
	d := starlark.NewDict(len(m))
	for k, val := range m {
		sv, err := toStarlark(val)
		if err != nil {
			return nil, err
		}
		if err := d.SetKey(starlark.String(k), sv); err != nil {
			return nil, err
		}
	}
	return d, nil
}

// fromStarlark converts a Starlark value back to a Go value.
func fromStarlark(v starlark.Value) any {
	switch x := v.(type) {
	case *moneyValue:
		// Back to the canonical wire shape {amount, currency} so the stored
		// value stays a first-class money object.
		return x.Money()
	case starlark.NoneType:
		return nil
	case starlark.Bool:
		return bool(x)
	case starlark.Int:
		n, ok := x.Int64()
		if ok {
			return n
		}
		// If it doesn't fit in int64, return as big int string
		return x.String()
	case starlark.Float:
		return float64(x)
	case starlark.String:
		return string(x)
	case *starlark.List:
		n := x.Len()
		result := make([]any, n)
		for i := 0; i < n; i++ {
			result[i] = fromStarlark(x.Index(i))
		}
		return result
	case *starlark.Dict:
		result := make(map[string]any)
		for _, item := range x.Items() {
			key, ok := starlark.AsString(item[0])
			if !ok {
				continue
			}
			result[key] = fromStarlark(item[1])
		}
		return result
	case starlark.Tuple:
		n := x.Len()
		result := make([]any, n)
		for i := 0; i < n; i++ {
			result[i] = fromStarlark(x.Index(i))
		}
		return result
	default:
		return x.String()
	}
}
