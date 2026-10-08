package api

import (
	"testing"
	"time"

	"github.com/primadi/formspec/pkg/spec"
)

// Plan: docs_internal/plan/intake-challenge-pow.md.
//
// These tests drive the REAL kafe tree through the router builder, the same way
// public_entities_test.go does, and for the same reason: intake resolution walks
// the App's DERIVED public grants ("which actions does this App actually expose
// anonymously?"), so a synthetic fixture would have to reproduce the whole
// surface — and could pass while the real manifests expose something else.
//
// The kafe-qr App declares an `intake` policy and the order entity opts `create`
// in; everything the guest surface touches must therefore resolve to a gate,
// and everything else (list, the staff Apps' actions) must not.

func TestIntakePolicies_KafeQR(t *testing.T) {
	b := kafePublicRouter(t)
	pols := b.intakePolicies()

	key := intakeKey("cafe-order", "order", "create")
	pol, ok := pols[key]
	if !ok {
		t.Fatalf("`create` opted in and kafe-qr declares a policy → %s must be gated; got keys %v", key, keysOf(pols))
	}

	// The policy is the App's, resolved with the documented defaults for
	// anything the manifest left out. Asserting the values (not just presence)
	// is what catches a resolver that reads the wrong field or silently drops
	// the block on the YAML round-trip.
	if pol.Mode != spec.IntakeModeEscalate {
		t.Errorf("mode = %q, want escalate", pol.Mode)
	}
	if pol.ActivateAt != 0.7 {
		t.Errorf("activate_at = %v, want 0.7", pol.ActivateAt)
	}
	if pol.GlobalMax != 300 || pol.GlobalPer != "1m" {
		t.Errorf("global = %d/%s, want 300/1m", pol.GlobalMax, pol.GlobalPer)
	}
	if pol.MinBits != 16 || pol.MaxBits != 22 {
		t.Errorf("difficulty = %d..%d, want 16..22", pol.MinBits, pol.MaxBits)
	}
	if pol.TTL != 90*time.Second {
		t.Errorf("ttl = %v, want 90s", pol.TTL)
	}
	if !pol.BindIP {
		t.Error("bind: [ip] must set BindIP")
	}
}

// The status page reads with the same anonymous grant that writes. Gating the
// READ would mean a guest cannot see the order they just placed — a worse
// outcome than the abuse it prevents — so `list` must stay ungated.
func TestIntakePolicies_OnlyOptedInActions(t *testing.T) {
	b := kafePublicRouter(t)
	pols := b.intakePolicies()

	if _, ok := pols[intakeKey("cafe-order", "order", "list")]; ok {
		t.Error("`list` did not opt in → it must not be gated")
	}
	// The very action is exposed only to staff Apps, which declare no intake
	// policy: an admin patch must never be gated.
	if _, ok := pols[intakeKey("cafe-order", "order", "update")]; ok {
		t.Error("`update` is not anonymously exposed → it must not be gated")
	}
	// A public App with no policy contributes nothing, so an entity in another
	// module stays clean.
	if _, ok := pols[intakeKey("cafe-master", "menu-item", "list")]; ok {
		t.Error("an entity whose App declares no intake policy must not be gated")
	}
}

// A public App that declares no policy at all (kafe has exactly one, but the
// builder must not assume that) resolves to no gates rather than the zero-value
// policy — a zero policy would be "gated at difficulty 0", i.e. a gate that
// passes everything while looking configured.
func TestIntakePolicies_NoPolicyNoGates(t *testing.T) {
	if got := (&RouterBuilder{}).intakePolicies(); len(got) != 0 {
		t.Errorf("a builder with no Apps must resolve no gates, got %v", got)
	}
}

func keysOf(m map[string]IntakePolicy) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	return out
}
