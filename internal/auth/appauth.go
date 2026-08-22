package auth

import (
	"fmt"

	"github.com/primadi/formspec/internal/app"
	"github.com/primadi/formspec/pkg/spec"
)

// AuthStrategy is the authentication strategy for an App (platform/02 §3).
// The set is open — new strategies register as artifacts. Single-server
// implements basic-auth; the rest are declared but not yet implemented.
type AuthStrategy string

const (
	StrategyBasicAuth    AuthStrategy = "basic-auth"
	StrategySSO          AuthStrategy = "sso"
	StrategySocialSSO    AuthStrategy = "social-sso"
	StrategyPasswordless AuthStrategy = "passwordless"
	StrategyPasskey      AuthStrategy = "passkey"
)

// knownStrategies is the closed set of recognized strategy names.
var knownStrategies = map[string]bool{
	string(StrategyBasicAuth):    true,
	string(StrategySSO):          true,
	string(StrategySocialSSO):    true,
	string(StrategyPasswordless): true,
	string(StrategyPasskey):      true,
}

// AppAuthConfig is the resolved auth configuration for one App.
type AppAuthConfig struct {
	App      string
	Strategy AuthStrategy
	// Overrides maps a logical auth role (user/session/role) → entity ref
	// ("module/name"), applied via RoleResolver.SetOverride (todo 6.1.4).
	Overrides map[string]string
}

// ResolveAppAuth determines each App's auth strategy from its auth_config_ref
// (defaulting to basic-auth). configs maps Config name → ConfigSpec.
//
// The referenced Config may declare:
//   - `strategy` key → the auth strategy (basic-auth default)
//   - `user_entity` / `session_entity` / `role_entity` keys → entity refs
//     that override which formspec.core entity backs each logical role
//     (user override wins, per the merge strategy).
func ResolveAppAuth(apps map[string]*app.ResolvedApp, configs map[string]*spec.ConfigSpec) ([]AppAuthConfig, []error) {
	var out []AppAuthConfig
	var errs []error

	for name, ra := range apps {
		cfg := AppAuthConfig{App: name, Strategy: StrategyBasicAuth, Overrides: map[string]string{}}
		ref := ra.Spec.AuthConfigRef
		if ref != "" {
			cs, ok := configs[ref]
			if !ok {
				errs = append(errs, fmt.Errorf("app %q: auth_config_ref %q not found", name, ref))
				continue
			}
			if k, ok := cs.Keys["strategy"]; ok {
				if s, ok := k.Default.(string); ok && s != "" {
					if !knownStrategies[s] {
						errs = append(errs, fmt.Errorf("app %q: unknown auth strategy %q", name, s))
						continue
					}
					cfg.Strategy = AuthStrategy(s)
				}
			}
			for role, key := range map[string]string{
				"user": "user_entity", "session": "session_entity", "role": "role_entity",
			} {
				if k, ok := cs.Keys[key]; ok {
					if ref, ok := k.Default.(string); ok && ref != "" {
						cfg.Overrides[role] = ref
					}
				}
			}
		}
		out = append(out, cfg)
	}
	return out, errs
}
