package starlark

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// runRandomScript executes a one-off script and returns the value the handler
// reported under "value".
func runRandomScript(t *testing.T, body string) string {
	t.Helper()
	dir := t.TempDir()
	scriptPath := filepath.Join(dir, "r.star")
	script := "def execute(resource, params, ctx):\n" + body + "\n"
	if err := os.WriteFile(scriptPath, []byte(script), 0o644); err != nil {
		t.Fatal(err)
	}

	ctxObj := NewCtxAPI("demo", "", "user", "", nil)
	ctxObj.Now = now

	res := NewResourceAPI("demo", "thing", "id-1", 1, map[string]any{})
	result, err := ExecuteScript(context.Background(), scriptPath, res, nil, ctxObj)
	if err != nil {
		t.Fatalf("execute: %v", err)
	}
	if !result.OK {
		t.Fatalf("script reported failure: %+v", result)
	}
	data := result.Data
	v, _ := data["value"].(string)
	return v
}

// ctx.random_digits(n) is the ONLY unpredictable value source a Starlark
// handler has (kafe 10.38). ctx.next_key is a sequence — predictable by
// construction — so using it for a join code would look correct and be forgeable.
func TestCtxRandomDigits_Shape(t *testing.T) {
	got := runRandomScript(t, `    return ok({"value": ctx.random_digits(6)})`)

	if len(got) != 6 {
		t.Fatalf("expected 6 digits, got %d (%q)", len(got), got)
	}
	for _, c := range got {
		if c < '0' || c > '9' {
			t.Fatalf("expected only decimal digits, got %q", got)
		}
	}
}

// Two calls must not return the same value. With a 10^6 space a collision is
// possible in principle, so this asserts on many independent calls rather than
// on one pair — the property under test is "not a constant", not "no birthday
// collision ever".
func TestCtxRandomDigits_IsNotConstant(t *testing.T) {
	// Concatenate 50 separate 6-digit draws; if the generator were a constant
	// (or a counter reset per call) the whole string would be one value repeated.
	got := runRandomScript(t, `
    s = ""
    for _i in range(50):
        s += ctx.random_digits(6)
    return ok({"value": s})
`)

	if len(got) != 300 {
		t.Fatalf("expected 300 digits, got %d", len(got))
	}
	distinct := map[string]bool{}
	for i := 0; i+6 <= len(got); i += 6 {
		distinct[got[i:i+6]] = true
	}
	// 50 draws from 10^6: seeing fewer than 45 distinct values would indicate a
	// badly broken generator, not bad luck.
	if len(distinct) < 45 {
		t.Fatalf("expected ~50 distinct codes, got %d distinct", len(distinct))
	}
}

// The digit distribution must be flat. Taking `byte % 10` would bias 0-5 (256 is
// not a multiple of 10), shrinking the effective search space — the exact
// property this helper exists to provide. This is the test that would catch a
// well-meaning "simplification" of the rejection sampling.
func TestCtxRandomDigits_DistributionIsFlat(t *testing.T) {
	const draws, digits = 500, 10
	got := runRandomScript(t, `
    s = ""
    for _i in range(500):
        s += ctx.random_digits(10)
    return ok({"value": s})
`)

	if len(got) != draws*digits {
		t.Fatalf("expected %d digits, got %d", draws*digits, len(got))
	}

	var counts [10]int
	for _, c := range got {
		counts[c-'0']++
	}

	total := draws * digits
	expected := float64(total) / 10
	// ±25% of expected: loose enough never to flake, tight enough that a modulo
	// bias (which skews the low digits by ~20%+) is caught.
	tolerance := expected * 0.25
	for d, n := range counts {
		if delta := float64(n) - expected; delta > tolerance || delta < -tolerance {
			t.Errorf("digit %d appeared %d times, expected ~%.0f (±%.0f) — distribution is not flat: %v",
				d, n, expected, tolerance, counts)
		}
	}
}

// A caller must not be able to ask for an unbounded (or trivially small) value.
//
// Asserting on "the call failed", not on HOW: this runtime surfaces a script
// failure either as a Go error or as `OK=false` depending on where it happens,
// and pinning one shape would make the test brittle without making it stronger.
func TestCtxRandomDigits_BoundsAreEnforced(t *testing.T) {
	cases := []struct {
		name string
		body string
	}{
		{"too small", `    return ok({"value": ctx.random_digits(2)})`},
		{"too large", `    return ok({"value": ctx.random_digits(99)})`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			scriptPath := filepath.Join(dir, "r.star")
			script := "def execute(resource, params, ctx):\n" + tc.body + "\n"
			if err := os.WriteFile(scriptPath, []byte(script), 0o644); err != nil {
				t.Fatal(err)
			}
			ctxObj := NewCtxAPI("demo", "", "user", "", nil)
			res := NewResourceAPI("demo", "thing", "id-1", 1, map[string]any{})

			result, err := ExecuteScript(context.Background(), scriptPath, res, nil, ctxObj)
			if err == nil && result.Error == "" {
				t.Fatalf("expected an out-of-range n to fail; result=%+v", result)
			}
			msg := ""
			if err != nil {
				msg = err.Error()
			} else {
				msg = result.Error
			}
			if !strings.Contains(msg, "random_digits") {
				t.Errorf("the error should name the helper, got: %v", msg)
			}
		})
	}
}
