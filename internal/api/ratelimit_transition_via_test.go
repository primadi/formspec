package api

import (
	"context"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"github.com/primadi/formspec/internal/action"
	"github.com/primadi/formspec/internal/entity"
	"github.com/primadi/formspec/pkg/spec"
	db "github.com/primadi/formspec/renderers/jsonb-persist"
)

// kafe 10.60a (plan docs_internal/plan/via-sebagai-action-penuh.md L9): a
// `rate_limit` declared on a state-machine transition's `via` must be enforced
// on the custom-action route.
//
// Why it wasn't: `HandleCustomAction` receives its spec from the registry UNION
// (`GetActionSpec` = declared `actions:` ∪ transition `via`), but the rate-limit
// check re-resolved through `resolveAction`, which reads `es.Actions` alone.
// Since L4 REJECTS redeclaring a `via` under `actions:`, via-only is the
// intended shape — so the declared limit was silently ignored and only the
// resource-level default applied.
//
// This test drives the REAL handler (not the helper), so reverting the wiring
// inside `HandleCustomAction` back to `rateLimitFor` makes it fail. That
// matters: an earlier version of this test called `checkRateLimitAction`
// directly and stayed green with the wiring reverted — a guard that did not
// guard.
//
// Run with: go test ./internal/api/ -run TestRateLimit_TransitionViaEnforcedThroughHandler
func TestRateLimit_TransitionViaEnforcedThroughHandler(t *testing.T) {
	dir := t.TempDir()
	d, err := db.OpenSQLite(filepath.Join(dir, "ratelimit_via.db"), nil)
	if err != nil {
		t.Fatalf("OpenSQLite: %v", err)
	}
	defer func() { _ = d.Close() }()

	reg := entity.NewRegistry(d, db.DriverSQLite, dir)

	// `confirm` exists ONLY as a transition `via` — no `actions:` entry, which
	// is the shape the post-L4 contract requires. Its `rate_limit` is the
	// declaration under test.
	es := spec.EntitySpec{
		Version: "v1",
		Plural:  "orders",
		Fields:  []spec.Field{{Name: "status", Type: spec.FieldString}},
		StateMachine: &spec.StateMachine{
			Field:   "status",
			Initial: "draft",
			States:  []spec.StateDecl{{Name: "draft"}, {Name: "confirmed"}},
			Transitions: []spec.TransitionDecl{{
				From:      spec.StateList{"draft"},
				To:        "confirmed",
				Action:    "confirm",
				RateLimit: &spec.RateLimitSpec{Max: 1, Per: "1s", Strategy: "token_bucket"},
			}},
		},
	}
	registerTestEntity(t, d, reg, "billing", "order", es)

	store, err := reg.GetEntityStore("billing", "order")
	if err != nil {
		t.Fatalf("GetEntityStore: %v", err)
	}
	id, err := store.Insert(context.Background(), db.InsertParams{
		WorkspaceID: "demo", CreatedBy: "tester",
		Data: map[string]any{"status": "draft"},
	})
	if err != nil {
		t.Fatalf("seed insert: %v", err)
	}

	// The spec the router would hand the handler: resolved through the union.
	viaSpec, ok := reg.GetActionSpec("billing", "order", "confirm")
	if !ok || viaSpec == nil {
		t.Fatal("`confirm` must resolve through the union")
	}
	if viaSpec.RateLimit == nil {
		t.Fatal("the union action must carry the transition's rate_limit")
	}
	if resolveAction(&es, "confirm") != nil {
		t.Fatal("precondition: `confirm` must NOT be reachable via resolveAction (declared actions only)")
	}

	factory := NewHandlerFactory(reg)
	factory.SetResourceRateLimiter(NewResourceRateLimiter())
	factory.SetSpecLookup(func(m, n string) (*spec.EntitySpec, bool) {
		info, ok := reg.GetEntity(m, n)
		if !ok || info.EntitySpec == nil {
			return nil, false
		}
		return info.EntitySpec, true
	})
	factory.SetDispatcher(action.NewDispatcher())

	h := factory.HandleCustomAction("billing", "order", "confirm", *viaSpec, "")

	call := func() *httptest.ResponseRecorder {
		req := httptest.NewRequest(http.MethodPost, "/billing/orders/"+id+"/confirm", nil)
		req.SetPathValue("id", id)
		req = req.WithContext(WithWorkspace(req.Context(), "demo"))
		w := httptest.NewRecorder()
		h(w, req)
		return w
	}

	// First call proceeds past the limiter; second must be 429.
	if first := call(); first.Code == http.StatusTooManyRequests {
		t.Fatalf("first call must not be rate limited, got 429: %s", first.Body.String())
	}
	second := call()
	if second.Code != http.StatusTooManyRequests {
		t.Fatalf("a transition `via`'s rate_limit must be enforced — want 429, got %d: %s",
			second.Code, second.Body.String())
	}
}
