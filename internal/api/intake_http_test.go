package api

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/primadi/formspec/internal/auth"
	"github.com/primadi/formspec/pkg/spec"
)

// Plan: docs_internal/plan/intake-challenge-pow.md.
//
// The unit tests in intake_test.go call `checkChallenge` directly with a
// context they build themselves. That proves the GATE works; it does not prove
// the gate is REACHABLE. Two things have to line up for that, and neither is
// visible from the unit test:
//
//  1. the route must be registered as public (`isPublicAction` → `rd.Public`),
//     or an anonymous caller never gets past the route's permission gate at all;
//  2. `RequirePermissionOrAnonymous` must mark the request
//     (`isPublicGrantAuth`), or `checkChallenge` correctly decides this is not
//     its surface and lets the request through.
//
// This file drives the REAL kafe surface through the REAL HTTP router — the
// same tree public_entities_test.go uses, for the same reason: the derivation
// it depends on walks Pages, Forms and pickers, so a hand-built fixture could
// pass while the actual manifests behave differently.
//
// The policy is injected rather than read from the manifest so the gate is ON
// regardless of live pressure. That is the one piece of state not under test
// here (`intakePolicies_KafeQR` and `intakeActive` cover it): the kafe App runs
// `mode: escalate` with `activate_at: 0.7`, and waiting for real pressure would
// make this test depend on a traffic pattern instead of on the wiring.

const intakeHTTPPath = "/kafe/_ui/entity/cafe-order/order"

func intakeAlwaysOn(t *testing.T) (http.Handler, *RouterBuilder) {
	t.Helper()
	b := kafePublicRouter(t)
	// Routes are generated from the registry union; without this the router
	// answers 404 and every assertion below would pass or fail for the wrong
	// reason.
	b.BuildRoutes()

	// A real JWT validator, as dev and prod both install (resource/formspec.go).
	// Without one AuthMiddleware only records a legacy user string and leaves the
	// identity nil — so a caller WITH a valid token would look anonymous and the
	// "staff are never challenged" case would be untestable (and, in a
	// hypothetical validator-less deployment, actually wrong).
	prev := GetAuthValidator()
	SetAuthValidator(auth.NewJWTValidator("intake-test-secret", "formspec", ""))
	t.Cleanup(func() { SetAuthValidator(prev) })

	// BuildHTTP resolves the policies from the manifests, so the injected policy
	// must come AFTER it — the builder would otherwise overwrite the injection
	// with kafe's own (escalate, currently off). Handlers read the map per
	// request, so replacing it afterwards is what takes effect.
	h := b.BuildHTTP()
	b.factory.SetIntakePolicies(map[string]IntakePolicy{
		intakeKey("cafe-order", "order", "create"): {
			Mode:    spec.IntakeModeAlways,
			MinBits: 8,
			MaxBits: 8,
			TTL:     time.Minute,
		},
	})
	return h, b
}

// An anonymous POST that is the guest's real write path must be refused with a
// challenge — through the real middleware chain.
func TestIntakeHTTP_AnonymousCreateIsChallenged(t *testing.T) {
	h, _ := intakeAlwaysOn(t)

	req := httptest.NewRequest(http.MethodPost, intakeHTTPPath, strings.NewReader(`{}`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected 403 CHALLENGE_REQUIRED, got %d: %s", rec.Code, rec.Body.String())
	}
	body := rec.Body.String()
	if !strings.Contains(body, "CHALLENGE_REQUIRED") {
		t.Fatalf("the refusal must name the code: %s", body)
	}
	// The challenge has to actually reach the client — a 403 without a solvable
	// challenge is a locked door with no handle.
	if !strings.Contains(body, `"token"`) || !strings.Contains(body, `"difficulty"`) {
		t.Fatalf("the refusal must carry the challenge: %s", body)
	}
}

// A solved challenge reaches the handler: the request is no longer refused by
// the gate. It may of course fail for other reasons (this POST has no body
// worth accepting) — what matters is that it is not still being challenged.
func TestIntakeHTTP_SolvedChallengeReachesHandler(t *testing.T) {
	h, b := intakeAlwaysOn(t)

	// First request: learn the challenge.
	first := httptest.NewRequest(http.MethodPost, intakeHTTPPath, strings.NewReader(`{}`))
	first.Header.Set("Content-Type", "application/json")
	rec1 := httptest.NewRecorder()
	h.ServeHTTP(rec1, first)
	if rec1.Code != http.StatusForbidden {
		t.Fatalf("precondition: the gate must be on, got %d: %s", rec1.Code, rec1.Body.String())
	}
	pol := b.factory.intakePolicies[intakeKey("cafe-order", "order", "create")]

	// Solve it out-of-band, then present it the way the client does (the header
	// name is the contract with lib/intake/pow.ts).
	token := challengeTokenFrom(t, rec1.Body.String())
	solution := solveIntake(t, token, 8)

	second := httptest.NewRequest(http.MethodPost, intakeHTTPPath, strings.NewReader(`{}`))
	second.Header.Set("Content-Type", "application/json")
	second.Header.Set(intakeHeader, token+":"+solution)
	rec2 := httptest.NewRecorder()
	h.ServeHTTP(rec2, second)

	if rec2.Code == http.StatusForbidden &&
		strings.Contains(rec2.Body.String(), "CHALLENGE_REQUIRED") {
		t.Fatalf("a solved challenge must not be challenged again: %s", rec2.Body.String())
	}
	// The request got past the gate and the TTL policy is intact (guards against
	// a resolver that returns a zero policy, whose TTL would mint dead tokens).
	if pol.TTL <= 0 {
		t.Fatal("the resolved policy must carry a positive TTL")
	}
}

// Staff traffic on the same route is untouched: the token is the difference,
// and gating it would break the POS surface the kafe staff actually use.
func TestIntakeHTTP_AuthenticatedCallerNotChallenged(t *testing.T) {
	h, _ := intakeAlwaysOn(t)

	token := issueKafeToken(t, []string{"cafe-order.orders.create"})
	req := httptest.NewRequest(http.MethodPost, intakeHTTPPath, strings.NewReader(`{}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code == http.StatusForbidden &&
		strings.Contains(rec.Body.String(), "CHALLENGE_REQUIRED") {
		t.Fatalf("an authenticated caller must never be challenged: %s", rec.Body.String())
	}
	// Guard against the test passing for the wrong reason: a rejected token
	// also fails to reach the gate, but it would mean the case was never
	// exercised.
	if rec.Code == http.StatusUnauthorized || rec.Code == http.StatusNotFound {
		t.Fatalf("the token must be accepted for this case to mean anything, got %d: %s", rec.Code, rec.Body.String())
	}
}

// issueKafeToken signs a token scoped to the kafe workspace — `issueToken`
// fixes the workspace to "demo", which AuthMiddleware rejects with 404 when the
// URL slug differs (cross-workspace access is indistinguishable from absence).
func issueKafeToken(t *testing.T, perms []string) string {
	t.Helper()
	issuer := auth.NewTokenIssuer("intake-test-secret", "formspec", "", 0, 0)
	tok, err := issuer.IssueAccessToken(&auth.User{
		ID:          "staff-1",
		Username:    "staff-1",
		WorkspaceID: "kafe",
		Permissions: perms,
	})
	if err != nil {
		t.Fatalf("IssueAccessToken: %v", err)
	}
	return tok
}

// challengeTokenFrom pulls the challenge token out of a refusal body without
// pulling in a JSON dependency: the field is the contract, and a missing one
// must fail loudly rather than yield an empty token that happens to verify
// nothing.
func challengeTokenFrom(t *testing.T, body string) string {
	t.Helper()
	const marker = `"token":"`
	i := strings.Index(body, marker)
	if i < 0 {
		t.Fatalf("no challenge token in %s", body)
	}
	rest := body[i+len(marker):]
	j := strings.Index(rest, `"`)
	if j <= 0 {
		t.Fatalf("malformed challenge token in %s", body)
	}
	return rest[:j]
}
