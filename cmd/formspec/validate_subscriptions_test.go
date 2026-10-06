package main

import (
	"strings"
	"testing"

	"github.com/primadi/formspec/internal/manifest"
)

// subEntity declares an Entity with one named event, so a subscription can be
// aimed at it.
func subEntity(source, module, name, event string) manifest.RawManifest {
	return manifest.RawManifest{
		APIVersion: "formspec.dev/v1",
		Kind:       "Entity",
		Source:     source,
		Metadata:   manifest.RawMetadata{Name: name, Module: module},
		Spec: map[string]any{
			"version":        "v1",
			"characteristic": "transaction",
			"fields":         []any{map[string]any{"name": "status", "type": "string"}},
			"events":         []any{map[string]any{"name": event, "type": "async"}},
		},
	}
}

// subManifest declares a Subscription listening to the given event names.
func subManifest(source string, events ...string) manifest.RawManifest {
	raw := make([]any, 0, len(events))
	for _, e := range events {
		raw = append(raw, e)
	}
	return manifest.RawManifest{
		APIVersion: "formspec.dev/v1",
		Kind:       "Subscription",
		Source:     source,
		Metadata:   manifest.RawMetadata{Name: "on-paid", Module: "consumer"},
		Spec: map[string]any{
			"events":  raw,
			"handler": map[string]any{"type": "native", "ref": "Consumer.Handle"},
		},
	}
}

// TestValidateSubscriptionEvents_Resolves is the happy path: a fully-qualified
// name matching a declared event is accepted.
func TestValidateSubscriptionEvents_Resolves(t *testing.T) {
	manifests := []manifest.RawManifest{
		subEntity("order.yaml", "billing", "order", "paid"),
		subManifest("sub.yaml", "billing.order.paid"),
	}
	if rejects := validateSubscriptionEvents(manifests); len(rejects) != 0 {
		t.Fatalf("a resolving event must be accepted, got %v", rejects)
	}
}

// TestValidateSubscriptionEvents_ReservedEventIsImplied is why the check cannot
// require an explicit `events:` declaration: every entity has the reserved
// lifecycle events (`on_submit`, `before_cancel`, …) without writing them, so a
// subscription listening to one is legitimate.
func TestValidateSubscriptionEvents_ReservedEventIsImplied(t *testing.T) {
	manifests := []manifest.RawManifest{
		subEntity("order.yaml", "billing", "order", "paid"), // declares only `paid`
		subManifest("sub.yaml", "billing.order.on_submit", "billing.order.before_cancel"),
	}
	if rejects := validateSubscriptionEvents(manifests); len(rejects) != 0 {
		t.Fatalf("reserved lifecycle events are implied and must be accepted, got %v", rejects)
	}
}

// TestValidateSubscriptionEvents_UnknownEventRejected is the gap this closes: a
// typo (or a renamed event) makes the subscription never fire, and nothing used
// to say so.
func TestValidateSubscriptionEvents_UnknownEventRejected(t *testing.T) {
	manifests := []manifest.RawManifest{
		subEntity("order.yaml", "billing", "order", "paid"),
		subManifest("sub.yaml", "billing.order.parid"), // typo for paid
	}
	msg, ok := validateSubscriptionEvents(manifests)["sub.yaml"]
	if !ok {
		t.Fatal("an event the entity does not declare must be rejected")
	}
	for _, want := range []string{"parid", "would never fire", "paid"} {
		if !strings.Contains(msg, want) {
			t.Errorf("error %q should mention %q", msg, want)
		}
	}
}

// TestValidateSubscriptionEvents_ShortNameRejected: a bare/short name cannot be
// resolved to an entity, so accepting it would mean the check silently passes
// over exactly the case it exists for.
func TestValidateSubscriptionEvents_ShortNameRejected(t *testing.T) {
	manifests := []manifest.RawManifest{
		subEntity("order.yaml", "billing", "order", "paid"),
		subManifest("sub.yaml", "paid"),
	}
	msg, ok := validateSubscriptionEvents(manifests)["sub.yaml"]
	if !ok {
		t.Fatal("a short event name must be rejected")
	}
	if !strings.Contains(msg, "fully qualified") {
		t.Errorf("error %q should show the expected form", msg)
	}
}

// TestValidateSubscriptionEvents_EntityOutsideTreeIsSkipped: a subscription may
// listen to an event from a module shipped separately. Its name cannot be
// verified, so it is skipped rather than rejected — the same rule the
// delivery-target check uses.
func TestValidateSubscriptionEvents_EntityOutsideTreeIsSkipped(t *testing.T) {
	manifests := []manifest.RawManifest{
		subEntity("order.yaml", "billing", "order", "paid"),
		subManifest("sub.yaml", "other-module.other-entity.something"),
	}
	if rejects := validateSubscriptionEvents(manifests); len(rejects) != 0 {
		t.Fatalf("an entity outside this tree cannot be verified and must be skipped, got %v", rejects)
	}
}
