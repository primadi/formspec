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
	// Build predeclared identifiers from env
	predeclared := make(starlark.StringDict, len(env)+1)
	for k, v := range env {
		sv, err := toStarlark(v)
		if err != nil {
			return nil, fmt.Errorf("starlark eval: convert env %q: %w", k, err)
		}
		predeclared[k] = sv
	}

	// Add built-in constants and helpers
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
	case map[string]any:
		d := starlark.NewDict(len(x))
		for k, val := range x {
			sv, err := toStarlark(val)
			if err != nil {
				return nil, err
			}
			if err := d.SetKey(starlark.String(k), sv); err != nil {
				return nil, err
			}
		}
		return d, nil
	default:
		return starlark.String(fmt.Sprintf("%v", x)), nil
	}
}

// fromStarlark converts a Starlark value back to a Go value.
func fromStarlark(v starlark.Value) any {
	switch x := v.(type) {
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
