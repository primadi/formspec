package auth

import (
	"testing"

	"github.com/primadi/formspec/internal/app"
	"github.com/primadi/formspec/pkg/spec"
)

func TestResolveAppAuth_DefaultBasicAuth(t *testing.T) {
	apps := map[string]*app.ResolvedApp{
		"erp": {Name: "erp", Spec: &spec.AppSpec{RootURL: "/app/erp"}},
	}
	auths, errs := ResolveAppAuth(apps, map[string]*spec.ConfigSpec{})
	if len(errs) != 0 {
		t.Fatalf("unexpected errors: %v", errs)
	}
	if len(auths) != 1 || auths[0].Strategy != StrategyBasicAuth {
		t.Fatalf("expected default basic-auth, got %+v", auths)
	}
}

func TestResolveAppAuth_CustomStrategyAndOverrides(t *testing.T) {
	apps := map[string]*app.ResolvedApp{
		"erp": {Name: "erp", Spec: &spec.AppSpec{RootURL: "/app/erp", AuthConfigRef: "auth"}},
	}
	configs := map[string]*spec.ConfigSpec{
		"auth": {Keys: map[string]spec.ConfigKey{
			"strategy":       {Type: "string", Default: "sso"},
			"user_entity":    {Type: "string", Default: "acme/custom-user"},
			"session_entity": {Type: "string", Default: "acme/custom-session"},
		}},
	}
	auths, errs := ResolveAppAuth(apps, configs)
	if len(errs) != 0 {
		t.Fatalf("unexpected errors: %v", errs)
	}
	ac := auths[0]
	if ac.Strategy != StrategySSO {
		t.Fatalf("expected sso, got %s", ac.Strategy)
	}
	if ac.Overrides["user"] != "acme/custom-user" {
		t.Errorf("expected user override, got %q", ac.Overrides["user"])
	}
	if ac.Overrides["session"] != "acme/custom-session" {
		t.Errorf("expected session override, got %q", ac.Overrides["session"])
	}
}

func TestResolveAppAuth_Errors(t *testing.T) {
	// Missing config.
	apps := map[string]*app.ResolvedApp{
		"erp": {Name: "erp", Spec: &spec.AppSpec{RootURL: "/app/erp", AuthConfigRef: "nope"}},
	}
	_, errs := ResolveAppAuth(apps, map[string]*spec.ConfigSpec{})
	if len(errs) != 1 {
		t.Fatalf("expected 1 error for missing config, got %v", errs)
	}

	// Unknown strategy.
	apps2 := map[string]*app.ResolvedApp{
		"erp": {Name: "erp", Spec: &spec.AppSpec{RootURL: "/app/erp", AuthConfigRef: "auth"}},
	}
	configs := map[string]*spec.ConfigSpec{
		"auth": {Keys: map[string]spec.ConfigKey{"strategy": {Type: "string", Default: "magic"}}},
	}
	_, errs2 := ResolveAppAuth(apps2, configs)
	if len(errs2) != 1 {
		t.Fatalf("expected 1 error for unknown strategy, got %v", errs2)
	}
}
