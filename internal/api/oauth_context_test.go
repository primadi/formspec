package api

import (
	"net/url"
	"strings"
	"testing"

	"github.com/primadi/formspec/internal/auth"
)

// When an OAuth round-trip ends with the caller holding several session
// contexts, the backend must ASK rather than pick (backend §8.7) — and the
// answer has to come back through the same flow, because a provider redirect has
// no step where a choice could be collected.
//
// The redirect therefore has to carry two things the client cannot derive: the
// choices themselves (a 409 body has them over the API; a redirect has no body)
// and the App, which the client needs to call the authorize endpoint again with
// the choice.
func TestOAuthContextPath_CarriesChoicesAndApp(t *testing.T) {
	b := &RouterBuilder{apps: nil}
	choices := []auth.ContextChoice{
		{ID: "sales@KFE-JKT-01", Role: "sales", Dimension: "branch_id", Value: "KFE-JKT-01"},
		{ID: "admin@KFE-BDG-01", Role: "admin", Dimension: "branch_id", Value: "KFE-BDG-01"},
	}

	got := b.oauthContextPath("kafe", "kafe-pos", "google", choices)

	// The choices ride in the FRAGMENT: it is never sent to the server and never
	// lands in access logs. They are not secrets — they are the caller's own
	// contexts, the same values the API returns in a 409 body.
	i := strings.IndexByte(got, '#')
	if i < 0 {
		t.Fatalf("redirect %q carries no fragment — the choices would be sent to the server", got)
	}
	params, err := url.ParseQuery(got[i+1:])
	if err != nil {
		t.Fatalf("fragment is not a query string: %v", err)
	}
	if params.Get("oauth") != "context_required" {
		t.Errorf("oauth = %q, want context_required", params.Get("oauth"))
	}
	if params.Get("provider") != "google" {
		t.Errorf("provider = %q — without it the flow cannot be resumed", params.Get("provider"))
	}
	if params.Get("app") != "kafe-pos" {
		t.Errorf("app = %q — the authorize endpoint requires it to resume", params.Get("app"))
	}
	// Repeated parameters, not one delimited string: a context VALUE is opaque,
	// so any delimiter we picked could also occur inside one.
	if got := params["c"]; len(got) != 2 || got[0] != "sales@KFE-JKT-01" || got[1] != "admin@KFE-BDG-01" {
		t.Errorf("c = %v, want both choice ids", got)
	}
	// And it lands on a login path, so a caller who would rather start over can.
	if !strings.Contains(got, "/kafe/") || !strings.Contains(got, "/login#") {
		t.Errorf("redirect %q does not target an in-App login path", got)
	}
}

// A value containing the delimiter must survive: this is the reason the choices
// are repeated parameters rather than a comma-joined list.
func TestOAuthContextPath_ValueWithDelimitersSurvives(t *testing.T) {
	b := &RouterBuilder{apps: nil}
	got := b.oauthContextPath("kafe", "app", "google", []auth.ContextChoice{
		{ID: "sales@outlet,north@2026", Role: "sales", Dimension: "branch_id", Value: "outlet,north@2026"},
	})
	i := strings.IndexByte(got, '#')
	params, err := url.ParseQuery(got[i+1:])
	if err != nil {
		t.Fatalf("fragment is not a query string: %v", err)
	}
	if ids := params["c"]; len(ids) != 1 || ids[0] != "sales@outlet,north@2026" {
		t.Fatalf("c = %v, want the id intact", ids)
	}
}
