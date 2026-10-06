package api

import (
	"net/http/httptest"
	"reflect"
	"testing"

	"github.com/primadi/formspec/pkg/spec"
	db "github.com/primadi/formspec/renderers/jsonb-persist"
)

// A literal row scope is the mechanism that closes kafe 10.67 / GAP-08: the
// predicate lives in the manifest, so the server applies it on every read no
// matter what the client sends (or omits). These tests pin the two halves of
// that claim — a client cannot WIDEN it, and a client cannot DROP it — plus the
// fail-closed behaviour for an entry that declares no source at all.

func TestApplyRowScope_LiteralOverridesClientFilter(t *testing.T) {
	f := &HandlerFactory{}
	es := &spec.EntitySpec{
		RowScope: []spec.FilterSpec{{Field: "status", Op: "in", Value: "paid,in_kitchen,ready"}},
	}
	// The client asks to see drafts. The manifest's predicate wins.
	req := httptest.NewRequest("GET", "/kafe/_ui/entity/cafe-order/order?status[in]=draft", nil)

	got, err := f.applyRowScope(req, es, "cafe-order", "order", map[string]db.FilterOp{
		"status": {Op: "in", Value: []any{"draft"}},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	want := []any{"paid", "in_kitchen", "ready"}
	if !reflect.DeepEqual(got["status"].Value, want) {
		t.Fatalf("literal scope = %#v, want %#v — a client filter must not widen or replace it", got["status"].Value, want)
	}
	if got["status"].Op != "in" {
		t.Fatalf("operator = %q, want in", got["status"].Op)
	}
}

func TestApplyRowScope_LiteralAppliesWhenClientSendsNothing(t *testing.T) {
	f := &HandlerFactory{}
	es := &spec.EntitySpec{
		RowScope: []spec.FilterSpec{{Field: "status", Op: "in", Value: "paid"}},
	}
	// No query string at all: the filter must still be present, which is what
	// makes this authoritative rather than a convenience.
	req := httptest.NewRequest("GET", "/kafe/_ui/entity/cafe-order/order", nil)

	got, err := f.applyRowScope(req, es, "cafe-order", "order", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got["status"].Value == nil {
		t.Fatal("literal scope was not applied — a caller could read every row")
	}
}

func TestApplyRowScope_LiteralBetweenNeedsTwoBounds(t *testing.T) {
	f := &HandlerFactory{}
	req := httptest.NewRequest("GET", "/kafe/_ui/entity/cafe-order/order", nil)

	// Two bounds: accepted, in order.
	ok := &spec.EntitySpec{
		RowScope: []spec.FilterSpec{{Field: "transaction_date", Op: "between", Value: "2026-01-01,2026-12-31"}},
	}
	got, err := f.applyRowScope(req, ok, "cafe-order", "order", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !reflect.DeepEqual(got["transaction_date"].Value, []any{"2026-01-01", "2026-12-31"}) {
		t.Fatalf("between bounds = %#v", got["transaction_date"].Value)
	}

	// One bound is a malformed predicate and must fail loudly, not silently
	// become an equality test on the whole string.
	bad := &spec.EntitySpec{
		RowScope: []spec.FilterSpec{{Field: "transaction_date", Op: "between", Value: "2026-01-01"}},
	}
	if _, err := f.applyRowScope(req, bad, "cafe-order", "order", nil); err == nil {
		t.Fatal("expected an error for a between literal with a single bound")
	}
}

func TestApplyRowScope_LiteralEmptyFailsClosed(t *testing.T) {
	f := &HandlerFactory{}
	es := &spec.EntitySpec{
		RowScope: []spec.FilterSpec{{Field: "status", Op: "in"}},
	}
	req := httptest.NewRequest("GET", "/kafe/_ui/entity/cafe-order/order", nil)

	if _, err := f.applyRowScope(req, es, "cafe-order", "order", nil); err == nil {
		t.Fatal("expected fail-closed error for a row_scope entry with no value source")
	}
}
