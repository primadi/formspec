// Package config provides the Config manifest registry for the FormSpec
// runtime (01-core-basic.md §10).
//
// Config is a manifest, not a dotenv. Each Config manifest declares a set of
// typed keys (`keys:`), each with a default value and an optional `secret`
// flag. Scripts read non-secret keys via `ctx.config.get("key")` and secret
// keys via `ctx.secrets` (subject to `uses.secrets`).
//
// Single-server mode has no Control Plane environment resolution, so each key
// resolves to its declared default (spec §10: "spec wajib menetapkan default
// standar untuk setiap setting"). Per-environment override is a Control Plane
// concern (deferred to the cloud phase).
package config

import (
	"fmt"
	"strconv"
	"sync"

	"github.com/primadi/formspec/pkg/spec"
)

// Registry resolves Config manifests into flat key→value stores for
// ctx.config (non-secret) and ctx.secrets (secret).
type Registry struct {
	mu      sync.RWMutex
	configs map[string]*spec.ConfigSpec
}

// NewRegistry creates an empty Config registry.
func NewRegistry() *Registry {
	return &Registry{configs: make(map[string]*spec.ConfigSpec)}
}

// Add registers a Config manifest by name. Later registrations with the same
// name overwrite earlier ones (user override wins, matching the manifest
// loader's later-root-wins rule).
func (r *Registry) Add(name string, cs *spec.ConfigSpec) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.configs[name] = cs
}

// NonSecret resolves all non-secret keys across every registered Config
// manifest into a flat map for ctx.config. Secret keys are excluded.
func (r *Registry) NonSecret() map[string]any {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make(map[string]any)
	for _, cs := range r.configs {
		if cs == nil {
			continue
		}
		for key, ck := range cs.Keys {
			if ck.Secret {
				continue
			}
			out[key] = resolveValue(ck)
		}
	}
	return out
}

// Secrets resolves all secret keys across every registered Config manifest
// into a flat map for ctx.secrets. Non-secret keys are excluded.
func (r *Registry) Secrets() map[string]string {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make(map[string]string)
	for _, cs := range r.configs {
		if cs == nil {
			continue
		}
		for key, ck := range cs.Keys {
			if !ck.Secret {
				continue
			}
			out[key] = fmt.Sprintf("%v", resolveValue(ck))
		}
	}
	return out
}

// ResolveKey resolves a single key from a named Config manifest, returning
// its string value and whether it was found. Used by inbound Webhook
// verification (02-core-extended.md §4) to look up the HMAC secret or static
// token referenced via `key: { config: <name> }`. Secret and non-secret keys
// are both resolvable here — the caller decides which it needs.
func (r *Registry) ResolveKey(configName, keyName string) (string, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	cs, ok := r.configs[configName]
	if !ok || cs == nil {
		return "", false
	}
	ck, ok := cs.Keys[keyName]
	if !ok {
		return "", false
	}
	return fmt.Sprintf("%v", resolveValue(ck)), true
}

// resolveValue coerces a ConfigKey's default to the Go type matching its
// declared Type (int|string|bool|decimal|json). Falls back to the raw default
// when the type is unknown or the default is nil.
func resolveValue(ck spec.ConfigKey) any {
	if ck.Default == nil {
		return nil
	}
	switch ck.Type {
	case "int":
		switch v := ck.Default.(type) {
		case int:
			return int64(v)
		case int64:
			return v
		case float64:
			return int64(v)
		case string:
			if i, err := strconv.ParseInt(v, 10, 64); err == nil {
				return i
			}
		}
	case "bool":
		switch v := ck.Default.(type) {
		case bool:
			return v
		case string:
			if b, err := strconv.ParseBool(v); err == nil {
				return b
			}
		}
	case "decimal":
		switch v := ck.Default.(type) {
		case float64:
			return v
		case int:
			return float64(v)
		case int64:
			return float64(v)
		case string:
			if f, err := strconv.ParseFloat(v, 64); err == nil {
				return f
			}
		}
	case "json":
		return ck.Default
	}
	return ck.Default
}
