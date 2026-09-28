package api

import (
	"path/filepath"
	"testing"

	"github.com/primadi/formspec/internal/entity"
	"github.com/primadi/formspec/pkg/spec"
	db "github.com/primadi/formspec/renderers/jsonb-persist"
)

// Todo 5.24.3.
//
// `via` is a full action source since L3 (plan via-sebagai-action-penuh.md): a
// transition that declares `impl` gets a route under its `via` name, and three
// of the four sites that decide "which actions exist" were switched to
// `ActionSources()` — `UICustomActionRoutesForEntity`, `generatePrepareRoutes`,
// and the router's custom branch. `GenerateCustomActionRoutes` (the PUBLIC
// `/api/v1` surface) was left reading `es.Actions`, so a transition declared
// ONLY via `via` served `/_ui/entity/…` but nothing on `/api/v1/…`, and
// `formspec generate` — which mirrors the REST surface — emitted no method and
// no params type for an endpoint the UI already had.
//
// These pin the two facts the fix depends on, and the trap it must not fall
// into: `GenerateRoutes` decides whether to emit a generic lifecycle route by
// checking which actions carry an `impl`, so switching only one of the two
// generators makes `POST …/{id}/submit` register TWICE.

func viaRouteRegistry(t *testing.T, expose []spec.ExposeConfig, actions []spec.Action, transitions []spec.TransitionDecl) *entity.Registry {
	t.Helper()
	dir := t.TempDir()
	d, err := db.OpenSQLite(filepath.Join(dir, "via_routes.db"), nil)
	if err != nil {
		t.Fatalf("OpenSQLite: %v", err)
	}
	t.Cleanup(func() { _ = d.Close() })

	reg := entity.NewRegistry(d, db.DriverSQLite, dir)
	es := spec.EntitySpec{
		Version: "v1",
		Plural:  "orders",
		Expose:  expose,
		Fields:  []spec.Field{{Name: "status", Type: spec.FieldString}},
		Actions: actions,
	}
	if len(transitions) > 0 {
		es.StateMachine = &spec.StateMachine{
			Field:   "status",
			Initial: "draft",
			States: []spec.StateDecl{
				{Name: "draft"}, {Name: "posted"}, {Name: "voided"},
			},
			Transitions: transitions,
		}
	}
	registerTestEntity(t, d, reg, "cafe-order", "order", es)
	return reg
}

func implRef(ref string) *spec.ImplDecl {
	return &spec.ImplDecl{Type: spec.ImplScriptRef, Ref: ref}
}

// TestGenerateCustomActionRoutes_IncludesTransitionVia is the reported gap: an
// impl-backed transition must appear on the public REST surface, like it already
// does on the UI surface.
func TestGenerateCustomActionRoutes_IncludesTransitionVia(t *testing.T) {
	reg := viaRouteRegistry(t,
		[]spec.ExposeConfig{{Type: spec.ProtocolREST, Actions: []string{"list", "find"}}},
		nil,
		[]spec.TransitionDecl{{
			From: spec.StateList{"draft"}, To: "posted", Action: "post",
			Impl: implRef("cafe-order/post"),
		}},
	)

	var found *RouteDescriptor
	for _, rd := range GenerateCustomActionRoutes(reg) {
		if rd.Action == "post" {
			r := rd
			found = &r
		}
	}
	if found == nil {
		t.Fatalf("a transition `via: post` with `impl` got no /api/v1 route — the "+
			"public surface disagrees with `/_ui/entity`, and `formspec generate` "+
			"therefore has no method for an endpoint that exists; got %v",
			actionsOfRoutes(GenerateCustomActionRoutes(reg)))
	}
	if want := "/api/v1/cafe-order/orders/{id}/post"; found.Path != want {
		t.Errorf("path = %q, want %q", found.Path, want)
	}
	// A transition declares its gate as `require_permission`; the synthesized
	// action carries none, so the route falls back to the conventional name —
	// the same string `AutoPrefixPermission` produces for the transition's own
	// `require_permission`, which is why the two cannot disagree.
	if want := "cafe-order.orders.post"; found.RequiredPermission != want {
		t.Errorf("permission = %q, want %q", found.RequiredPermission, want)
	}
}

// TestGenerateCustomActionRoutes_SkipsTransitionWithoutImpl keeps the change
// additive: a `via` with no `impl` has no endpoint of its own, so neither
// surface may promise one.
func TestGenerateCustomActionRoutes_SkipsTransitionWithoutImpl(t *testing.T) {
	reg := viaRouteRegistry(t,
		[]spec.ExposeConfig{{Type: spec.ProtocolREST, Actions: []string{"list", "find"}}},
		nil,
		[]spec.TransitionDecl{{
			From: spec.StateList{"draft"}, To: "posted", Action: "start-preparing",
		}},
	)
	for _, rd := range GenerateCustomActionRoutes(reg) {
		if rd.Action == "start-preparing" {
			t.Fatalf("an impl-less transition must not get a route, got %+v", rd)
		}
	}
}

// TestGeneratedRoutes_HaveNoDuplicatePath is the trap guard.
//
// `GenerateRoutes` omits the generic lifecycle route for `submit`/`cancel`/
// `amend` when the action carries an `impl`, because the custom generator emits
// it instead. If only ONE of the two generators is switched to the action-source
// union, a transition declaring `via: submit` + `impl` (with no `actions:` entry)
// is invisible to that check while still being emitted as a custom route — the
// same path registered twice, the second shadowing the first.
//
// A duplicate `(Method, Path)` is wrong on its own terms regardless of the
// generator that produced it, so this asserts the invariant rather than the
// implementation.
func TestGeneratedRoutes_HaveNoDuplicatePath(t *testing.T) {
	reg := viaRouteRegistry(t,
		[]spec.ExposeConfig{{Type: spec.ProtocolREST, Actions: []string{"list", "find", "submit"}}},
		nil,
		[]spec.TransitionDecl{{
			From: spec.StateList{"draft"}, To: "posted", Action: "submit",
			Impl: implRef("cafe-order/submit"),
		}},
	)

	all := append(GenerateRoutes(reg), GenerateCustomActionRoutes(reg)...)
	seen := map[string]string{}
	for _, rd := range all {
		key := rd.Method + " " + rd.Path
		if prev, dup := seen[key]; dup {
			t.Errorf("duplicate route %s: registered as %q and again as %q — the "+
				"second registration shadows the first", key, prev, rd.Handler)
		}
		seen[key] = rd.Handler + ":" + rd.Action
	}

	// And the UI surface, which has the same two-generator shape.
	ui := append(UIRoutesForEntity("cafe-order", "order", specOf(t, reg)), UICustomActionRoutesForEntity("cafe-order", "order", specOf(t, reg))...)
	uiSeen := map[string]string{}
	for _, rd := range ui {
		key := rd.Method + " " + rd.Path
		if prev, dup := uiSeen[key]; dup {
			t.Errorf("duplicate UI route %s: registered as %q and again as %q", key, prev, rd.Handler)
		}
		uiSeen[key] = rd.Handler + ":" + rd.Action
	}
}

func specOf(t *testing.T, reg *entity.Registry) *spec.EntitySpec {
	t.Helper()
	info, ok := reg.GetEntity("cafe-order", "order")
	if !ok || info.EntitySpec == nil {
		t.Fatal("entity not registered")
	}
	return info.EntitySpec
}

func actionsOfRoutes(routes []RouteDescriptor) []string {
	out := make([]string, 0, len(routes))
	for _, rd := range routes {
		out = append(out, rd.Handler+":"+rd.Action)
	}
	return out
}
