package config

import (
	"testing"

	"github.com/primadi/formspec/pkg/spec"
)

// TestRegistry_ResolveKeyAny covers the by-bare-name lookup used for
// framework-level keys (`core.idempotency_retention`, todo 2.1.6): the caller
// does not know which module declared the key, so every registered manifest is
// searched instead of one by name.
func TestRegistry_ResolveKeyAny(t *testing.T) {
	reg := NewRegistry()
	reg.Add("workspace", &spec.ConfigSpec{
		Keys: map[string]spec.ConfigKey{
			"currency": {Type: "string", Default: "IDR"},
		},
	})
	reg.Add("core", &spec.ConfigSpec{
		Keys: map[string]spec.ConfigKey{
			"idempotency_retention": {Type: "string", Default: "7d"},
		},
	})

	got, ok := reg.ResolveKeyAny("idempotency_retention")
	if !ok {
		t.Fatal("ResolveKeyAny(idempotency_retention) not found, want found")
	}
	if got != "7d" {
		t.Errorf("ResolveKeyAny(idempotency_retention) = %q, want %q", got, "7d")
	}

	// A key only some manifest declares is still reachable.
	if got, ok := reg.ResolveKeyAny("currency"); !ok || got != "IDR" {
		t.Errorf("ResolveKeyAny(currency) = (%q, %v), want (IDR, true)", got, ok)
	}

	// Unknown key is reported as absent, not as an empty string — the caller
	// must be able to tell "not declared" from "declared empty".
	if got, ok := reg.ResolveKeyAny("nope"); ok {
		t.Errorf("ResolveKeyAny(nope) = (%q, true), want (_, false)", got)
	}

	// nil registry field guard: a Config manifest with no keys must not panic.
	reg.Add("empty", &spec.ConfigSpec{})
	if _, ok := reg.ResolveKeyAny("still-nope"); ok {
		t.Error("ResolveKeyAny on registry with an empty manifest should not find keys")
	}
}
