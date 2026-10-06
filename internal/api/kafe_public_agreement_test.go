package api

import (
	"net/http"
	"net/http/httptest"
	"sort"
	"testing"
)

// newRequestForApp builds the request resolveAppContext reads the App from
// (`?app=`), so the test follows the same resolution path the handler does.
func newRequestForApp(workspace, appName string) *http.Request {
	return httptest.NewRequest("GET", "/"+workspace+"/_ui/_meta/ui?app="+appName, nil)
}

// TestKafeQR_BundleAndEndpointAgree closes kafe 10.13.
//
// Before, the bundle's anonymity checker and the router's request-time grant
// were derived from the SAME manifest field through two DIFFERENT code paths
// (internal/ui/publicEntityChecker vs internal/api/publicGrants), so they could
// disagree — a bundle that hides an entity while its endpoint serves it, or the
// reverse. Nothing locked them together.
//
// Now both call `ui.Registry.DerivePublicGrants`, so agreement is structural.
// This test asserts it anyway: a future change that reintroduces a second
// source of truth for the anonymous allowlist should fail here rather than in
// production. Measured on kafe-qr: 5 entities on both sides (was 13 vs 4 in the
// 2026-09-22 incident where the bundle ignored the allowlist).
func TestKafeQR_BundleAndEndpointAgree(t *testing.T) {
	b := kafePublicRouter(t)

	qr := b.apps["kafe-qr"]
	if qr == nil {
		t.Fatal("kafe-qr not resolved")
	}

	// ── Endpoint side: the derived grants, as the router sees them. ──
	decls := b.derivedPublicGrants()["kafe-qr"]
	granted := map[string]bool{}
	for _, d := range decls {
		granted[d.Entity] = true
	}

	// ── Bundle side: what an anonymous caller is shipped. ──
	// Mirror HandleMetaUI's public path exactly: appCtx carries the derived
	// allowlist, and `can` is nil so BuildBundle derives its checker from it.
	appCtx, errMsg := b.resolveAppContext(newRequestForApp("kafe", "kafe-qr"))
	if errMsg != "" {
		t.Fatalf("resolveAppContext: %s", errMsg)
	}
	if appCtx.PublicEntities == nil {
		t.Fatal("appCtx.PublicEntities must be the derived allowlist, not nil")
	}
	bundle := b.uiRegistry.BuildBundle(b.listEntityDescriptors, nil, appCtx)

	shipped := map[string]bool{}
	for _, e := range bundle.Entities {
		shipped[e.Module+"/"+e.Name] = true
	}

	if len(shipped) == 0 {
		t.Fatal("anonymous kafe-qr bundle shipped no entity — the derivation is not wired")
	}

	// Every shipped entity must have a grant, and every granted entity must
	// ship. Either difference is the 10.13 bug in one direction.
	var onlyBundle, onlyGrant []string
	for ref := range shipped {
		if !granted[ref] {
			onlyBundle = append(onlyBundle, ref)
		}
	}
	for ref := range granted {
		if !shipped[ref] {
			onlyGrant = append(onlyGrant, ref)
		}
	}
	sort.Strings(onlyBundle)
	sort.Strings(onlyGrant)

	if len(onlyBundle) > 0 {
		t.Errorf("shipped to an anonymous caller but NOT granted at the endpoint (bundle leaks schema): %v", onlyBundle)
	}
	if len(onlyGrant) > 0 {
		t.Errorf("granted at the endpoint but NOT shipped in the bundle (dead grant): %v", onlyGrant)
	}

	// And pin the actual set, so a regression that widens BOTH sides at once
	// (which the consistency check above would accept) still fails.
	want := []string{
		"cafe-master/dining-table",
		"cafe-master/menu-item",
		"cafe-master/menu-item-price",
		"cafe-order/order",
		"cafe-order/table-session",
	}
	got := make([]string, 0, len(shipped))
	for ref := range shipped {
		got = append(got, ref)
	}
	sort.Strings(got)
	if len(got) != len(want) {
		t.Fatalf("anonymous kafe-qr entity set = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("anonymous kafe-qr entity set = %v, want %v", got, want)
		}
	}

	// The staff entities must be absent from BOTH sides (the GAP-06 set).
	for _, ent := range []string{
		"cafe-master/member", "cafe-master/employee", "cafe-master/branch",
		"cafe-order/shift", "cafe-order/cash-movement", "cafe-order/payment",
	} {
		if shipped[ent] {
			t.Errorf("%s must not ship to an anonymous caller", ent)
		}
		if granted[ent] {
			t.Errorf("%s must not be granted anonymously", ent)
		}
	}
}

// TestKafeKDS_NotPublic confirms a private App contributes nothing, so the
// derivation cannot be the reason data leaks from the kitchen display.
func TestKafeKDS_NotPublic(t *testing.T) {
	b := kafePublicRouter(t)
	if _, ok := b.derivedPublicGrants()["kafe-kds"]; ok {
		t.Error("kafe-kds is private; it must have no derived grant entry")
	}
}
