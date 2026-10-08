package api

import (
	"crypto/sha256"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/primadi/formspec/internal/auth"
	"github.com/primadi/formspec/pkg/spec"
)

// Plan: docs_internal/plan/intake-challenge-pow.md. These tests pin the parts
// that are easy to get quietly wrong: that a solved challenge actually verifies,
// that every way of NOT solving it is refused, and that the gate neither fires
// for signed-in callers nor stays silent when the pressure signal trips.

// solveIntake brute-forces a solution. Difficulty in tests stays tiny (<= 12
// bits) so this is a few thousand hashes, not a CPU test.
func solveIntake(t *testing.T, token string, difficulty int) string {
	t.Helper()
	for i := 0; i < 1<<22; i++ {
		sol := strconv.Itoa(i)
		if powSatisfied(token, sol, difficulty) {
			return sol
		}
	}
	t.Fatalf("no solution found for difficulty %d", difficulty)
	return ""
}

// solveIntakeBetween finds a solution whose work lands in [min, maxExclusive)
// bits. Used to prove a weaker solution is refused by a harder challenge: just
// taking the first 8-bit solution would satisfy 16 bits about once in 256 runs,
// which would make the test fail intermittently rather than never.
func solveIntakeBetween(t *testing.T, token string, min, maxExclusive int) string {
	t.Helper()
	for i := 0; i < 1<<22; i++ {
		sol := strconv.Itoa(i)
		sum := sha256Sum(token + ":" + sol)
		if bits := leadingZeroBits(sum); bits >= min && bits < maxExclusive {
			return sol
		}
	}
	t.Fatalf("no solution found with %d..%d bits", min, maxExclusive)
	return ""
}

// sha256Sum is a tiny indirection so the test states its intent (hash the same
// string the verifier hashes) without reaching for a different hashing path.
func sha256Sum(s string) []byte {
	sum := sha256.Sum256([]byte(s))
	return sum[:]
}

func TestIntakeChallenge_RoundTrip(t *testing.T) {
	pol := IntakePolicy{Mode: spec.IntakeModeAlways, MinBits: 8, MaxBits: 8, TTL: time.Minute, BindIP: true}
	token, err := issueIntakeChallenge(pol, "cafe-order", "order", "create", "1.2.3.4", 8)
	if err != nil {
		t.Fatalf("issue: %v", err)
	}
	sol := solveIntake(t, token, 8)
	if err := verifyIntakeSolution(pol, token+":"+sol, "cafe-order", "order", "create", "1.2.3.4", time.Now()); err != nil {
		t.Fatalf("a correctly solved challenge must verify, got: %v", err)
	}
}

func TestIntakeChallenge_Refusals(t *testing.T) {
	pol := IntakePolicy{Mode: spec.IntakeModeAlways, MinBits: 8, MaxBits: 8, TTL: time.Minute, BindIP: true}
	token, err := issueIntakeChallenge(pol, "m", "e", "a", "1.2.3.4", 8)
	if err != nil {
		t.Fatalf("issue: %v", err)
	}
	sol := solveIntake(t, token, 8)
	now := time.Now()

	cases := map[string]struct {
		header string
		module string
		entity string
		action string
		ip     string
		at     time.Time
	}{
		"expired":                  {token + ":" + sol, "m", "e", "a", "1.2.3.4", now.Add(2 * time.Minute)},
		"another action":           {token + ":" + sol, "m", "e", "other", "1.2.3.4", now},
		"another entity":           {token + ":" + sol, "m", "other", "a", "1.2.3.4", now},
		"another client (ip bind)": {token + ":" + sol, "m", "e", "a", "9.9.9.9", now},
		"unsolved":                 {token + ":0", "m", "e", "a", "1.2.3.4", now},
		"missing solution":         {token + ":", "m", "e", "a", "1.2.3.4", now},
		"no separator":             {token, "m", "e", "a", "1.2.3.4", now},
		"tampered payload":         {strings.Replace(token, ".", "x.", 1) + ":" + sol, "m", "e", "a", "1.2.3.4", now},
		"empty header":             {"", "m", "e", "a", "1.2.3.4", now},
	}
	for name, c := range cases {
		if err := verifyIntakeSolution(pol, c.header, c.module, c.entity, c.action, c.ip, c.at); err == nil {
			t.Errorf("%s: must be refused", name)
		}
	}
}

// A solution for a lower difficulty must not satisfy a higher one — otherwise
// the escalation to a harder puzzle would be decorative.
func TestIntakeChallenge_DifficultyIsEnforced(t *testing.T) {
	pol := IntakePolicy{Mode: spec.IntakeModeAlways, MinBits: 8, MaxBits: 8, TTL: time.Minute}
	token, err := issueIntakeChallenge(pol, "m", "e", "a", "1.2.3.4", 16)
	if err != nil {
		t.Fatalf("issue: %v", err)
	}
	weak := solveIntakeBetween(t, token, 8, 16)
	if err := verifyIntakeSolution(pol, token+":"+weak, "m", "e", "a", "", time.Now()); err == nil {
		t.Fatal("an 8-bit solution must not satisfy a 16-bit challenge")
	}
	strong := solveIntake(t, token, 16)
	if err := verifyIntakeSolution(pol, token+":"+strong, "m", "e", "a", "", time.Now()); err != nil {
		t.Fatalf("a 16-bit solution must satisfy it, got: %v", err)
	}
}

func TestResolveIntakePolicy_Defaults(t *testing.T) {
	pol := resolveIntakePolicy(&spec.IntakeChallenge{})
	if pol.Mode != spec.IntakeModeEscalate {
		t.Errorf("default mode = %q, want escalate", pol.Mode)
	}
	if pol.ActivateAt != intakeDefaultActivateAt {
		t.Errorf("default activate_at = %v, want %v", pol.ActivateAt, intakeDefaultActivateAt)
	}
	if pol.MinBits != intakeDefaultMinBits || pol.MaxBits != intakeDefaultMaxBits {
		t.Errorf("default difficulty = %d..%d, want %d..%d", pol.MinBits, pol.MaxBits, intakeDefaultMinBits, intakeDefaultMaxBits)
	}
	if pol.TTL != intakeDefaultTTL {
		t.Errorf("default ttl = %v, want %v", pol.TTL, intakeDefaultTTL)
	}
	// Binding is opt-in: a phone that changes networks mid-flow must not be
	// asked to solve again for no gain.
	if pol.BindIP {
		t.Error("ip binding must be opt-in, not a default")
	}

	// An explicit zero must survive as zero (activate immediately) rather than
	// being read as "unset" — that is the whole reason the field is a pointer.
	zero := 0.0
	if got := resolveIntakePolicy(&spec.IntakeChallenge{ActivateAt: &zero}); got.ActivateAt != 0 {
		t.Errorf("explicit activate_at: 0 became %v", got.ActivateAt)
	}
	bind := resolveIntakePolicy(&spec.IntakeChallenge{Bind: []string{"ip"}})
	if !bind.BindIP {
		t.Error("bind: [ip] must enable ip binding")
	}
}

func TestStricterIntake(t *testing.T) {
	always := IntakePolicy{Mode: spec.IntakeModeAlways, ActivateAt: 0.9, MaxBits: 18}
	escalate := IntakePolicy{Mode: spec.IntakeModeEscalate, ActivateAt: 0.2, MaxBits: 24}
	if !stricterIntake(always, escalate) {
		t.Error("mode always must outrank a lower activate_at")
	}
	if stricterIntake(escalate, always) {
		t.Error("escalate must not outrank always")
	}
	early := IntakePolicy{Mode: spec.IntakeModeEscalate, ActivateAt: 0.1, MaxBits: 16}
	if !stricterIntake(early, escalate) {
		t.Error("an earlier activation must win at equal mode")
	}
}

func TestEntityActionOptedIn(t *testing.T) {
	es := &spec.EntitySpec{Actions: []spec.Action{
		{Name: "create", Challenge: true},
		{Name: "delete", Disabled: true},
	}}
	if !entityActionOptedIn(es, "create") {
		t.Error("create declared challenge: true → must opt in")
	}
	if entityActionOptedIn(es, "delete") {
		t.Error("an action without `challenge: true` must not opt in")
	}
	if entityActionOptedIn(es, "nope") {
		t.Error("an undeclared action must not opt in")
	}
	if entityActionOptedIn(nil, "create") {
		t.Error("a nil spec must not opt in")
	}
}

// anonymousRequest builds a request the way the ROUTER actually delivers one:
// an anonymous caller on a public route carries NO grant marker, because
// RequirePermissionOrAnonymous returns early for a nil identity and only marks
// the signed-in-without-permission branch. An earlier version of the gate
// required that marker and therefore never challenged a guest at all — this
// helper exists so the tests exercise the shape the wire really has.
func anonymousRequest(t *testing.T, header string) *http.Request {
	t.Helper()
	req := httptest.NewRequest(http.MethodPost, "/kafe/_ui/entity/cafe-order/order", nil)
	req = req.WithContext(WithWorkspace(req.Context(), "kafe"))
	if header != "" {
		req.Header.Set(intakeHeader, header)
	}
	return req
}

func intakeTestFactory(pol IntakePolicy, module, entity, action string) *HandlerFactory {
	return &HandlerFactory{
		rateLimiter:    NewResourceRateLimiter(),
		intakePolicies: map[string]IntakePolicy{intakeKey(module, entity, action): pol},
	}
}

// mode: always gates every anonymous request to the opted-in action — the
// behaviour an operator gets with activate_at: 0, and the one the E2E check
// relies on.
func TestCheckChallenge_AlwaysMode(t *testing.T) {
	const module, entity, action = "cafe-order", "order", "create"
	f := intakeTestFactory(IntakePolicy{Mode: spec.IntakeModeAlways, MinBits: 8, MaxBits: 8, TTL: time.Minute}, module, entity, action)

	w := httptest.NewRecorder()
	if f.checkChallenge(w, anonymousRequest(t, ""), module, entity, action) {
		t.Fatal("mode: always must gate an anonymous request")
	}
	if w.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d", w.Code)
	}
	if !strings.Contains(w.Body.String(), "CHALLENGE_REQUIRED") || !strings.Contains(w.Body.String(), "\"token\"") {
		t.Fatalf("the 403 must carry the challenge in the envelope, got: %s", w.Body.String())
	}
}

// A signed-in caller is never asked to solve, even on a route a public grant
// also serves. Gating staff would break the POS surface for no security gain.
func TestCheckChallenge_SkipsAuthenticatedCaller(t *testing.T) {
	const module, entity, action = "cafe-order", "order", "create"
	f := intakeTestFactory(IntakePolicy{Mode: spec.IntakeModeAlways, MinBits: 8, MaxBits: 8, TTL: time.Minute}, module, entity, action)

	req := anonymousRequest(t, "")
	req = req.WithContext(WithIdentity(req.Context(), &auth.Identity{UserID: "staff-1", WorkspaceID: "kafe"}))
	w := httptest.NewRecorder()
	if !f.checkChallenge(w, req, module, entity, action) {
		t.Fatalf("an authenticated caller must pass, got %d: %s", w.Code, w.Body.String())
	}
}

// A route that is public but NOT reached through a public grant is not this
// gate's surface; a non-opted-in action on the same entity is not either.
func TestCheckChallenge_SkipsNonGrantAndNonOptedIn(t *testing.T) {
	const module, entity, action = "cafe-order", "order", "create"
	f := intakeTestFactory(IntakePolicy{Mode: spec.IntakeModeAlways, MinBits: 8, MaxBits: 8, TTL: time.Minute}, module, entity, action)

	granted := anonymousRequest(t, "")
	w2 := httptest.NewRecorder()
	if !f.checkChallenge(w2, granted, module, entity, "list") {
		t.Error("an action with no policy must not be gated")
	}

	// A different ENTITY with no policy: the key is (module, entity, action), so
	// a policy for one entity must not leak onto a sibling.
	w3 := httptest.NewRecorder()
	if !f.checkChallenge(w3, anonymousRequest(t, ""), "cafe-order", "table-session", action) {
		t.Error("a policy must not leak onto another entity")
	}
}

// The escalation path: under pressure the gate turns on by itself, and a
// solved challenge then lets the caller through. This is the behaviour that
// makes "escalate" a real mode rather than a synonym for "off".
func TestCheckChallenge_EscalatesUnderPressureThenAcceptsSolution(t *testing.T) {
	const module, entity, action = "cafe-order", "order", "create"
	half := 0.5
	pol := IntakePolicy{
		Mode:       spec.IntakeModeEscalate,
		ActivateAt: half,
		GlobalMax:  10,
		GlobalPer:  "1m",
		MinBits:    8,
		MaxBits:    8,
		TTL:        time.Minute,
	}
	f := intakeTestFactory(pol, module, entity, action)

	// Each call records one hit, so utilization climbs 0.1, 0.2, ... The gate
	// must stay OFF below the threshold: a low-traffic cafe never shows a
	// puzzle to a guest.
	var lastToken string
	for i := 1; i <= 4; i++ {
		if !f.checkChallenge(httptest.NewRecorder(), anonymousRequest(t, ""), module, entity, action) {
			t.Fatalf("call %d: gate must stay off below activate_at", i)
		}
	}
	// Fifth hit reaches the threshold → 403 with a challenge. The exact call is
	// not pinned: the bucket refills between calls, so the threshold is crossed
	// within the budget rather than on a specific index.
	var w *httptest.ResponseRecorder
	for i := 0; i < 4; i++ {
		w = httptest.NewRecorder()
		if !f.checkChallenge(w, anonymousRequest(t, ""), module, entity, action) {
			break
		}
	}
	if w.Code != http.StatusForbidden {
		t.Fatalf("the gate must turn on under pressure, got %d: %s", w.Code, w.Body.String())
	}
	body := w.Body.String()
	start := strings.Index(body, `"token":"`) + len(`"token":"`)
	end := strings.Index(body[start:], `"`)
	if start < len(`"token":"`) || end <= 0 {
		t.Fatalf("challenge token missing from: %s", body)
	}
	lastToken = body[start : start+end]

	// A correctly solved challenge is accepted even while the gate is on.
	sol := solveIntake(t, lastToken, 8)
	w2 := httptest.NewRecorder()
	if !f.checkChallenge(w2, anonymousRequest(t, lastToken+":"+sol), module, entity, action) {
		t.Fatalf("a solved challenge must pass, got %d: %s", w2.Code, w2.Body.String())
	}
}

func TestParsePerAndUtilizationSignal(t *testing.T) {
	rl := NewResourceRateLimiter()
	rs := &spec.RateLimitSpec{Max: 4, Per: "1m"}
	// Observe counts the hit it is reporting on, so the first call already
	// consumes one of the four: utilization is "how full", not "how full before".
	if got := rl.Observe(rs, "k"); got != 0.25 {
		t.Errorf("first hit of a 4-budget bucket = %v, want 0.25", got)
	}
	var last float64
	for i := 0; i < 8; i++ {
		last = rl.Observe(rs, "k")
	}
	if last != 1 {
		t.Errorf("an exhausted bucket must report 1.0 utilization, got %v", last)
	}
	// nil / unparseable specs report no pressure instead of panicking.
	if got := rl.Observe(nil, "k"); got != 0 {
		t.Errorf("nil spec must report 0, got %v", got)
	}
	if got := rl.Observe(&spec.RateLimitSpec{Max: 5, Per: ""}, "k"); got != 0 {
		t.Errorf("unparseable per must report 0, got %v", got)
	}
}
