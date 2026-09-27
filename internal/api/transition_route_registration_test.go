package api

import (
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"github.com/primadi/formspec/internal/entity"
	"github.com/primadi/formspec/internal/manifest"
	"github.com/primadi/formspec/pkg/spec"
	db "github.com/primadi/formspec/renderers/jsonb-persist"
)

// L3 (plan docs_internal/plan/via-sebagai-action-penuh.md): a transition that
// declares `impl` must get a WORKING route — not merely a RouteDescriptor.
//
// `TestUICustomActionRoutes_TransitionWithImpl` proves the generator emits the
// descriptor. That is necessary but NOT sufficient: the descriptor has to
// survive `registerRouteWithPattern`, which re-resolves the action by name to
// build the handler. While that resolution scanned `EntitySpec.Actions`
// directly, a `via` declared only on a transition resolved to nil, the
// registration was skipped, and the request fell through to the generic file
// handler.
//
// Measured on the live server before the fix:
//
//	POST /kafe/_ui/entity/gl/journal-entry/1/post    → 404
//	  {"code":"NOT_FOUND","message":"no such file field or action: post"}
//	POST /kafe/_ui/entity/gl/journal-entry/1/submit  → 401   (control)
//
// The 404 text names a FILE field, which is the tell: the file handler owned
// the route. The control proves routing was otherwise healthy.
//
// This test asserts the OBSERVABLE outcome — the request reaches the action
// handler, so an unauthenticated call is refused by the permission middleware
// with 401 rather than answered by the router with 404. Both are "not 200", so
// only checking "not 200" would pass even with the bug; the status code is the
// discriminator.
func TestTransitionOnlyAction_RegistersWorkingRoute(t *testing.T) {
	dir := t.TempDir()
	database, err := db.OpenSQLite(filepath.Join(dir, "transition_route.db"), nil)
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	t.Cleanup(func() { _ = database.Close() })

	reg := entity.NewRegistry(database, db.DriverSQLite, dir)

	// `post` exists ONLY as a transition `via` — the whole point of L3. There
	// is deliberately no matching `actions:` entry.
	impl := &spec.ImplDecl{Type: "script_ref", Ref: "gl/journal_post"}
	es := &spec.EntitySpec{
		Plural: "journal-entries",
		Expose: []spec.ExposeConfig{{Type: spec.ProtocolREST, Actions: []string{"list", "find"}}},
		Actions: []spec.Action{
			{Name: "create"},
		},
		StateMachine: &spec.StateMachine{
			Field:   "status",
			Initial: "draft",
			Transitions: []spec.TransitionDecl{
				{
					From: spec.StateList{"draft"}, To: "posted", Action: "post",
					Impl:              impl,
					RequirePermission: "journal-entries.post",
				},
			},
		},
	}
	if err := reg.RegisterArtifactManifest(manifest.RawManifest{
		Kind:     "Entity",
		Source:   "gl/entities/journal-entry.yaml",
		Metadata: manifest.RawMetadata{Name: "journal-entry", Module: "gl"},
	}, es); err != nil {
		t.Fatalf("register: %v", err)
	}

	b := NewRouterBuilder(reg)
	b.BuildRoutes()
	srv := httptest.NewServer(b.BuildHTTP())
	t.Cleanup(srv.Close)

	route := "/kafe/_ui/entity/gl/journal-entry/1/post"

	// Guard: the descriptor must be generated at all. If this fails the test
	// below is measuring the wrong failure.
	found := false
	for _, rd := range b.Routes() {
		if rd.Action == "post" && rd.Path == "/_ui/entity/gl/journal-entry/{id}/post" {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("no RouteDescriptor for the transition-only action `post` — " +
			"the generator, not the registration, is broken")
	}

	resp, err := http.Post(srv.URL+route, "application/json", nil)
	if err != nil {
		t.Fatalf("POST: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()

	// 401 = the route exists and the permission middleware rejected the
	// anonymous caller. 404 = the route was never registered and the request
	// was answered by the file handler instead — the bug this test guards.
	if resp.StatusCode == http.StatusNotFound {
		t.Fatalf("POST %s → 404: the transition's `via` route was not registered. "+
			"A `via` that exists only on a transition must still resolve when the "+
			"router builds its handler (union lookup, not `Actions` scan).", route)
	}
	if resp.StatusCode != http.StatusUnauthorized && resp.StatusCode != http.StatusForbidden {
		t.Fatalf("POST %s → %d, want 401/403 (route live, caller refused)",
			route, resp.StatusCode)
	}
}

// The same defect existed on the `prepare` branch: a server-sourced idempotent
// action declared only as a transition `via` produces a `/{action}/prepare`
// descriptor, and the router resolved that name by scanning `Actions` too.
// Guard the union there as well so the two branches cannot drift apart again.
func TestTransitionOnlyAction_RegistersWorkingPrepareRoute(t *testing.T) {
	dir := t.TempDir()
	database, err := db.OpenSQLite(filepath.Join(dir, "transition_prepare.db"), nil)
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	t.Cleanup(func() { _ = database.Close() })

	reg := entity.NewRegistry(database, db.DriverSQLite, dir)

	// server-sourced idempotency key → generatePrepareRoutes emits a prepare
	// endpoint for this action.
	idem := &spec.IdempotencyDecl{From: "server"}
	es := &spec.EntitySpec{
		Plural:  "journal-entries",
		Expose:  []spec.ExposeConfig{{Type: spec.ProtocolREST, Actions: []string{"list", "find"}}},
		Actions: []spec.Action{{Name: "create"}},
		StateMachine: &spec.StateMachine{
			Field:   "status",
			Initial: "draft",
			Transitions: []spec.TransitionDecl{
				{
					From: spec.StateList{"draft"}, To: "posted", Action: "post",
					Impl:           &spec.ImplDecl{Type: "script_ref", Ref: "gl/journal_post"},
					Idempotent:     true,
					IdempotencyKey: idem,
				},
			},
		},
	}
	if err := reg.RegisterArtifactManifest(manifest.RawManifest{
		Kind:     "Entity",
		Source:   "gl/entities/journal-entry.yaml",
		Metadata: manifest.RawMetadata{Name: "journal-entry", Module: "gl"},
	}, es); err != nil {
		t.Fatalf("register: %v", err)
	}

	b := NewRouterBuilder(reg)
	b.BuildRoutes()
	srv := httptest.NewServer(b.BuildHTTP())
	t.Cleanup(srv.Close)

	route := "/kafe/_ui/entity/gl/journal-entry/post/prepare"

	found := false
	for _, rd := range b.Routes() {
		if rd.Handler == "prepare" && rd.Action == "post" {
			found = true
			break
		}
	}
	if !found {
		t.Skip("prepare descriptor not generated for this transition shape; " +
			"the registration assertion below does not apply")
	}

	resp, err := http.Post(srv.URL+route, "application/json", nil)
	if err != nil {
		t.Fatalf("POST: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode == http.StatusNotFound {
		t.Fatalf("POST %s → 404: prepare route for a transition-only action was "+
			"generated but not registered — the prepare branch must resolve over "+
			"the same union as the custom branch", route)
	}
}
