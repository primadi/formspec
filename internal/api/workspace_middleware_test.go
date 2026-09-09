package api

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

// fakeWorkspaceResolver is an in-memory WorkspaceResolver for tests.
type fakeWorkspaceResolver struct {
	registered map[string]bool
	fail       bool
}

func (f *fakeWorkspaceResolver) Registered(_ context.Context, slug string) (bool, error) {
	if f.fail {
		return false, context.DeadlineExceeded
	}
	return f.registered[slug], nil
}

func setupWorkspaceResolver(t *testing.T, r WorkspaceResolver) {
	t.Helper()
	prev := GetWorkspaceResolver()
	SetWorkspaceResolver(r)
	t.Cleanup(func() { SetWorkspaceResolver(prev) })
}

// TestWorkspaceMiddleware_UnregisteredSlug404 verifies the registry check
// (plan named-workspaces.md): an unregistered slug is a 404 — indistinguishable
// from a missing resource (§15.2 anti-enumeration).
func TestWorkspaceMiddleware_UnregisteredSlug404(t *testing.T) {
	setupWorkspaceResolver(t, &fakeWorkspaceResolver{registered: map[string]bool{
		"default": true, "cafe": true,
	}})
	handler := WorkspaceMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	// Registered slugs pass.
	req := httptest.NewRequest("GET", "/cafe/app/kafe", nil)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("registered workspace: expected 200, got %d", rr.Code)
	}

	// Unregistered slug → 404 with WORKSPACE_NOT_FOUND.
	req = httptest.NewRequest("GET", "/unknown-ws/app/kafe", nil)
	rr = httptest.NewRecorder()
	handler.ServeHTTP(rr, req)
	if rr.Code != http.StatusNotFound {
		t.Fatalf("unregistered workspace: expected 404, got %d", rr.Code)
	}

	// Missing slug falls back to "default" (registered) → 200.
	req = httptest.NewRequest("GET", "/", nil)
	rr = httptest.NewRecorder()
	handler.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("default fallback: expected 200, got %d", rr.Code)
	}
}

// TestWorkspaceMiddleware_NilResolverPassthrough verifies backward
// compatibility: without a resolver (embedded use/tests), any slug passes.
func TestWorkspaceMiddleware_NilResolverPassthrough(t *testing.T) {
	if GetWorkspaceResolver() != nil {
		t.Fatal("test requires nil resolver")
	}
	handler := WorkspaceMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	req := httptest.NewRequest("GET", "/anything-goes/app", nil)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("nil resolver: expected passthrough 200, got %d", rr.Code)
	}
}

// TestWorkspaceMiddleware_ReservedSegmentPassthrough verifies that root-level
// router surfaces (static assets, favicon, health) are not treated as
// workspace slugs — chi middlewares run before routing, so without this skip
// the registry check would swallow /assets/* as 404 WORKSPACE_NOT_FOUND and
// Vite absolute asset URLs would break (bug 2026-09-08).
func TestWorkspaceMiddleware_ReservedSegmentPassthrough(t *testing.T) {
	setupWorkspaceResolver(t, &fakeWorkspaceResolver{registered: map[string]bool{
		"default": true, // "assets" dkk deliberately NOT registered
	}})
	var seenWorkspace string
	handler := WorkspaceMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		seenWorkspace = GetWorkspace(r.Context())
		w.WriteHeader(http.StatusOK)
	}))

	for _, path := range []string{
		"/assets/index-abc123.js",
		"/favicon.svg",
		"/health",
	} {
		req := httptest.NewRequest("GET", path, nil)
		rr := httptest.NewRecorder()
		handler.ServeHTTP(rr, req)
		if rr.Code != http.StatusOK {
			t.Fatalf("reserved segment %s: expected passthrough 200, got %d", path, rr.Code)
		}
		if seenWorkspace != "default" {
			t.Fatalf("reserved segment %s: expected default workspace in context, got %q", path, seenWorkspace)
		}
	}
}

// TestWorkspaceMiddleware_ResolverError500 verifies that a registry failure
// surfaces as 500 — never as a silent pass-through or a 404.
func TestWorkspaceMiddleware_ResolverError500(t *testing.T) {
	setupWorkspaceResolver(t, &fakeWorkspaceResolver{fail: true})
	handler := WorkspaceMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	req := httptest.NewRequest("GET", "/cafe/app/kafe", nil)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)
	if rr.Code != http.StatusInternalServerError {
		t.Fatalf("resolver error: expected 500, got %d", rr.Code)
	}
}
