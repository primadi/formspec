package api

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/primadi/formspec/pkg/spec"
)

// ─── Anonymous intake challenge (proof-of-work) ───
//
// Plan: docs_internal/plan/intake-challenge-pow.md.
//
// The anonymous intake surface (guests ordering from a QR, no login) is bounded
// today only by `rate_limit: {scope: ip}`. That key is both too weak (IP is not
// an identity) and too blunt (everyone behind one NAT shares, and blocks, the
// same budget). This adds a second, orthogonal cost: a browser solved
// proof-of-work the caller must present before an opted-in action runs.
//
// What it is NOT, stated plainly because the name invites the mistake: this is
// not anti-DDoS. Volumetric floods (L3/L4) never run the script and must be
// absorbed at the edge. It raises the price of a FORGED request to an expensive
// L7 action (here: POST orders = a DB write + an emitted event + a journal),
// and it does nothing about a botnet that really runs JavaScript.
//
// Enforcement point is the handler path, not a global middleware: the global
// chain sees static assets and `_meta/*`, and gating the meta bundle would cut
// the anonymous SPA off before it can render — a chicken-and-egg the gate must
// not create. The gate rides the SAME hook as the rate limit (checkIntakeAction)
// so the two can never drift into different orderings.

// intakeHeader is the request header carrying "<token>:<solution>".
const intakeHeader = "X-Forma-Intake"

// intakeAlg is the hash named in the challenge body. Only sha256 today.
const intakeAlg = "sha256"

// intakeDifficultyCeiling mirrors the validator's cap (32 leading zero bits).
// Past this a "gate" is an outage: 2^32 hashes is minutes of CPU on a phone.
const intakeDifficultyCeiling = 32

// IntakePolicy is the effective, build-time-resolved gate for ONE
// (module, entity, action) pair. Resolution (which App declared what, merged
// across every public App that exposes the action) happens once at router
// build; enforcement only ever reads this struct.
//
// Keying by (module, entity, action) rather than by App is deliberate and
// forced: an anonymous `_ui/entity` request carries no App — AppFromContext is
// empty without an identity, `_ui/entity` sends no `?app=`, and publicGrants()
// already merges every public App into one module/entity map. Letting the
// client name the App would put the gate behind a value it can delete.
type IntakePolicy struct {
	Mode       string  // escalate | always
	ActivateAt float64 // fraction of the global budget that turns the gate on
	GlobalMax  int     // pressure signal: hits per GlobalPer across all anon callers
	GlobalPer  string
	MinBits    int // difficulty when idle
	MaxBits    int // difficulty when saturated
	TTL        time.Duration
	BindIP     bool
}

// intakeDefaults are the effective values for a policy field the manifest left
// out. Written as constants so the defaults are one auditable list rather than
// scattered through the resolver.
const (
	intakeDefaultActivateAt = 0.7
	intakeDefaultMinBits    = 16
	intakeDefaultMaxBits    = 22
	intakeDefaultTTL        = 90 * time.Second
)

// resolveIntakePolicy turns the authored declaration into the effective policy.
// The defaults it fills in are part of the contract, not implementation detail:
// an omitted `difficulty` must not mean "no work" and an omitted `ttl` must not
// mean "forever".
func resolveIntakePolicy(c *spec.IntakeChallenge) IntakePolicy {
	pol := IntakePolicy{
		Mode:       spec.IntakeModeEscalate,
		ActivateAt: intakeDefaultActivateAt,
		MinBits:    intakeDefaultMinBits,
		MaxBits:    intakeDefaultMaxBits,
		TTL:        intakeDefaultTTL,
	}
	if c.Mode != "" {
		pol.Mode = c.Mode
	}
	if c.ActivateAt != nil {
		pol.ActivateAt = *c.ActivateAt
	}
	if c.Global != nil {
		pol.GlobalMax = c.Global.Max
		pol.GlobalPer = c.Global.Per
	}
	if c.Difficulty != nil {
		if c.Difficulty.Min > 0 {
			pol.MinBits = c.Difficulty.Min
		}
		if c.Difficulty.Max > 0 {
			pol.MaxBits = c.Difficulty.Max
		}
	}
	if pol.MaxBits < pol.MinBits {
		pol.MaxBits = pol.MinBits
	}
	if c.TTL != "" {
		if d, err := time.ParseDuration(c.TTL); err == nil && d > 0 {
			pol.TTL = d
		}
	}
	for _, b := range c.Bind {
		if b == "ip" {
			pol.BindIP = true
		}
	}
	return pol
}

// entityActionOptedIn reports whether the entity declared `challenge: true` for
// the named action.
//
// Only a DECLARED action can opt in. A transition `via` is a full action in
// every other respect (plan via-sebagai-action-penuh.md), but adding a second
// declaration point for this flag is the exact divergence the design refuses —
// and L4 already rejects redeclaring a `via` under `actions:`, so a via-only
// action cannot opt in yet. That is a recorded gap, not a silent no-op.
func entityActionOptedIn(es *spec.EntitySpec, action string) bool {
	if es == nil {
		return false
	}
	for i := range es.Actions {
		if es.Actions[i].Name == action {
			return es.Actions[i].Challenge
		}
	}
	return false
}

// stricterIntake reports whether a is a stricter gate than b. The order is
// documented because "stricter" is otherwise a judgement call, and an
// undocumented merge rule is how two Apps silently end up disagreeing about the
// same route. Highest wins: mode always > escalate, then the earlier
// activation, then the higher difficulty ceiling.
func stricterIntake(a, b IntakePolicy) bool {
	aAlways := a.Mode == spec.IntakeModeAlways
	bAlways := b.Mode == spec.IntakeModeAlways
	if aAlways != bAlways {
		return aAlways
	}
	if a.ActivateAt != b.ActivateAt {
		return a.ActivateAt < b.ActivateAt
	}
	return a.MaxBits > b.MaxBits
}

// intakeKey names one gated (module, entity, action). The "|" separator keeps
// the action from bleeding into the entity key ("a/b|c" is unambiguous).
func intakeKey(module, entity, action string) string {
	return module + "/" + entity + "|" + action
}

// intakeChallengePayload is the signed body of a challenge. Field names are
// terse because the payload rides in the response body of every rejection.
type intakeChallengePayload struct {
	M string `json:"m"`           // module
	E string `json:"e"`           // entity
	A string `json:"a"`           // action
	D int    `json:"d"`           // difficulty, leading zero bits of sha256
	X int64  `json:"x"`           // expiry, unix seconds
	N string `json:"n"`           // nonce (base64url, 16 random bytes)
	I string `json:"i,omitempty"` // client ip, only when bound
}

// intakeChallengeBody is what the client receives on a 403 CHALLENGE_REQUIRED.
type intakeChallengeBody struct {
	Token      string `json:"token"`
	Difficulty int    `json:"difficulty"`
	Alg        string `json:"alg"`
	TTLSeconds int    `json:"ttl_seconds"`
}

var (
	intakeSecretOnce sync.Once
	intakeSecret     []byte
)

// instanceIntakeSecret returns the per-process HMAC key for challenges. It is
// generated once and never persisted: a stateless challenge needs only that
// the same process can verify what it issued. The consequence is explicit —
// with more than one replica, a challenge issued by replica A is refused by
// replica B, so the client solves again. That is the same single-server caveat
// the in-memory rate limiter and the WebSocket ticket store already carry; a
// shared secret belongs in the Control Plane layer, not in this one.
func instanceIntakeSecret() []byte {
	intakeSecretOnce.Do(func() {
		b := make([]byte, 32)
		// crypto/rand.Read never fails on any supported platform (it panics
		// inside the stdlib otherwise), so there is no partial-value fallback
		// worth writing — and a constant secret would be worse than a crash.
		_, _ = rand.Read(b)
		intakeSecret = b
	})
	return intakeSecret
}

// leadingZeroBits counts the leading zero bits of b. This is the "work" unit:
// a solution is valid when sha256(token + ":" + solution) has at least
// `difficulty` of them.
func leadingZeroBits(b []byte) int {
	n := 0
	for _, c := range b {
		if c == 0 {
			n += 8
			continue
		}
		for i := 7; i >= 0; i-- {
			if c&(1<<uint(i)) == 0 {
				n++
				continue
			}
			return n
		}
		return n
	}
	return n
}

// powSatisfied reports whether solution satisfies difficulty against token.
func powSatisfied(token, solution string, difficulty int) bool {
	sum := sha256.Sum256([]byte(token + ":" + solution))
	return leadingZeroBits(sum[:]) >= difficulty
}

// issueIntakeChallenge mints a stateless, signed challenge for one
// (module, entity, action) at the given difficulty.
func issueIntakeChallenge(p IntakePolicy, module, entity, action, ip string, difficulty int) (string, error) {
	nonce := make([]byte, 16)
	_, _ = rand.Read(nonce)
	pl := intakeChallengePayload{
		M: module,
		E: entity,
		A: action,
		D: difficulty,
		X: time.Now().Add(p.TTL).Unix(),
		N: base64.RawURLEncoding.EncodeToString(nonce),
	}
	if p.BindIP {
		pl.I = ip
	}
	raw, err := json.Marshal(pl)
	if err != nil {
		return "", err
	}
	mac := hmac.New(sha256.New, instanceIntakeSecret())
	mac.Write(raw)
	return base64.RawURLEncoding.EncodeToString(raw) + "." + base64.RawURLEncoding.EncodeToString(mac.Sum(nil)), nil
}

// verifyIntakeSolution checks a presented "<token>:<solution>" against the
// policy. Cheap checks first (shape, signature, expiry, binding), the hash
// search last.
//
// Replay: the challenge is stateless, so ONE solution stays valid until it
// expires. The window is narrowed — not closed — by a short TTL, a signature
// bound to the action (and optionally the client IP), and the rate limit that
// still applies underneath. A single-use nonce would close it completely, at
// the cost of a store that becomes its own target; that trade is deferred
// (see the plan) rather than made silently.
func verifyIntakeSolution(p IntakePolicy, header, module, entity, action, ip string, now time.Time) error {
	token, solution, ok := strings.Cut(header, ":")
	if !ok || token == "" || solution == "" {
		return errors.New("malformed intake header")
	}
	payloadB64, sigB64, ok := strings.Cut(token, ".")
	if !ok {
		return errors.New("malformed challenge token")
	}
	raw, err := base64.RawURLEncoding.DecodeString(payloadB64)
	if err != nil {
		return errors.New("malformed challenge payload")
	}
	sig, err := base64.RawURLEncoding.DecodeString(sigB64)
	if err != nil {
		return errors.New("malformed challenge signature")
	}
	mac := hmac.New(sha256.New, instanceIntakeSecret())
	mac.Write(raw)
	if !hmac.Equal(sig, mac.Sum(nil)) {
		return errors.New("challenge signature does not verify")
	}

	var pl intakeChallengePayload
	if err := json.Unmarshal(raw, &pl); err != nil {
		return errors.New("malformed challenge payload")
	}
	// The challenge names the exact action it was issued for: a solution for a
	// cheap action must not unlock an expensive one.
	if pl.M != module || pl.E != entity || pl.A != action {
		return errors.New("challenge was issued for another action")
	}
	if now.Unix() > pl.X {
		return errors.New("challenge expired")
	}
	if p.BindIP && pl.I != ip {
		return errors.New("challenge is bound to another client")
	}
	if pl.D < 1 || pl.D > intakeDifficultyCeiling {
		return fmt.Errorf("challenge difficulty %d is out of range", pl.D)
	}
	if !powSatisfied(token, solution, pl.D) {
		return errors.New("insufficient proof-of-work")
	}
	return nil
}

// checkIntakeAction is the single gate every entity/service action passes
// through: rate limit first (cheap, and it rejects a flood before any
// challenge is minted), then the intake challenge. Keeping them in one
// function is what stops the two from being reordered by accident.
//
// `action` is the spec the caller already resolved (may be nil — the standard
// CRUD handlers hold none), forwarded only so the rate-limit override keeps
// working exactly as before.
func (f *HandlerFactory) checkIntakeAction(w http.ResponseWriter, r *http.Request, es *spec.EntitySpec, action *spec.Action, module, entity, actionName string) bool {
	if !f.checkRateLimitAction(w, r, es, action, actionName) {
		return false
	}
	return f.checkChallenge(w, r, module, entity, actionName)
}

// checkChallenge enforces the intake gate for one (module, entity, action).
// It returns true when the request may proceed; when false a response has
// already been written.
//
// It never applies to an authenticated caller or an API-key caller: the gate
// exists for the anonymous surface, and putting it in front of the staff,
// integration, or non-browser clients would break them for no gain. What makes
// a request "the anonymous surface" is the PRESENCE of a policy for its
// (module, entity, action) — that map is built from the App's derived public
// grants, so it contains exactly the actions an anonymous caller can reach.
func (f *HandlerFactory) checkChallenge(w http.ResponseWriter, r *http.Request, module, entity, action string) bool {
	if len(f.intakePolicies) == 0 {
		return true
	}
	pol, ok := f.intakePolicies[intakeKey(module, entity, action)]
	if !ok {
		return true
	}
	ctx := r.Context()
	// Signed in (including an API-key caller — it sets an identity) → never
	// asked. Gating staff would break the POS surface for no security gain.
	if IdentityFromContext(ctx) != nil {
		return true
	}
	// Anonymous. Note this deliberately does NOT require `isPublicGrantAuth`:
	// RequirePermissionOrAnonymous returns EARLY for a nil identity (comment
	// „Anonymous — the public grant is its authorization“) and only marks the
	// request in the SIGNED-IN-without-permission branch. Requiring the marker
	// here therefore disabled the gate for the exact caller it exists for, and
	// silently — measured through the real router: an anonymous POST to the
	// kafe order route was never challenged. It is not needed either: this map
	// is populated from the App's DERIVED public grants, so the existence of a
	// policy for (module, entity, action) already states that the action is
	// anonymously reachable, and nothing else reaches this far anonymously.
	ip := clientIP(r)
	difficulty, active := f.intakeActive(pol, module, entity, action)
	if !active {
		return true
	}
	if h := r.Header.Get(intakeHeader); h != "" {
		if err := verifyIntakeSolution(pol, h, module, entity, action, ip, time.Now()); err == nil {
			return true
		}
		// An invalid, expired, or bound-elsewhere solution falls through to a
		// fresh challenge rather than an error the client cannot act on.
	}
	token, err := issueIntakeChallenge(pol, module, entity, action, ip, difficulty)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "INTERNAL", "could not issue intake challenge")
		return false
	}
	writeChallengeRequired(w, token, difficulty, pol)
	return false
}

// intakeActive reports whether the gate is on right now, and at what
// difficulty.
//
// Under mode: escalate the signal is GLOBAL per action, not per IP. A
// distributed flood keeps every single IP under its own threshold, so per-IP
// pressure would report calm during exactly the attack the gate is for. The
// one hit is recorded on every anonymous request (Observe), because a signal
// that only counts while the gate is already on could never turn it on.
func (f *HandlerFactory) intakeActive(pol IntakePolicy, module, entity, action string) (int, bool) {
	mid := (pol.MinBits + pol.MaxBits + 1) / 2
	if pol.Mode == spec.IntakeModeAlways {
		return mid, true
	}
	if pol.GlobalMax <= 0 || f.rateLimiter == nil {
		// A policy present with no pressure signal cannot escalate. Validation
		// refuses this combination for a declared gate, so reaching here means
		// a policy was injected by a test or a hand-built factory.
		return 0, false
	}
	rs := &spec.RateLimitSpec{Max: pol.GlobalMax, Per: pol.GlobalPer, Scope: "global"}
	u := f.rateLimiter.Observe(rs, "intake:"+intakeKey(module, entity, action))
	if u < pol.ActivateAt {
		return 0, false
	}
	// Scale min..max across the remaining headroom. A zero-width band
	// (min == max, or activate_at == 1) means "always at max" — the honest
	// reading of a band with no width.
	d := pol.MaxBits
	if pol.MaxBits > pol.MinBits && pol.ActivateAt < 1 {
		frac := (u - pol.ActivateAt) / (1 - pol.ActivateAt)
		if frac < 0 {
			frac = 0
		}
		if frac > 1 {
			frac = 1
		}
		d = pol.MinBits + int(frac*float64(pol.MaxBits-pol.MinBits)+0.5)
	}
	if d < pol.MinBits {
		d = pol.MinBits
	}
	if d > pol.MaxBits {
		d = pol.MaxBits
	}
	return d, true
}

// writeChallengeRequired writes the 403 that carries a fresh challenge. The
// challenge rides in the error envelope rather than a separate issue endpoint:
// one fewer public endpoint to protect, and the client can solve without a
// round-trip. The code — not the status — is the contract the client matches
// on.
func writeChallengeRequired(w http.ResponseWriter, token string, difficulty int, pol IntakePolicy) {
	writeJSON(w, http.StatusForbidden, ErrorResponse{
		Error: ErrorDetail{
			Code:    "CHALLENGE_REQUIRED",
			Message: "prove this request comes from a browser: solve the challenge and resend with the " + intakeHeader + " header",
			Challenge: &intakeChallengeBody{
				Token:      token,
				Difficulty: difficulty,
				Alg:        intakeAlg,
				TTLSeconds: int(pol.TTL.Seconds()),
			},
		},
		Meta: MetaSingle{Timestamp: time.Now().UTC().Format(time.RFC3339)},
	})
}
