package main

import (
	"strings"
	"testing"

	"github.com/primadi/formspec/internal/manifest"
)

// eventEntity builds a raw Entity manifest with one event whose deliver list
// names a reliable_event target.
func eventEntity(source, module, name, eventName string, target map[string]any, actions []map[string]any) manifest.RawManifest {
	rawActions := make([]any, 0, len(actions))
	for _, a := range actions {
		rawActions = append(rawActions, a)
	}
	return manifest.RawManifest{
		APIVersion: "formspec.dev/v1",
		Kind:       "Entity",
		Source:     source,
		Metadata:   manifest.RawMetadata{Name: name, Module: module},
		Spec: map[string]any{
			"version": "v1",
			"fields":  []any{map[string]any{"name": "status", "type": "string"}},
			"events": []any{map[string]any{
				"name":    eventName,
				"type":    "async",
				"publish": map[string]any{"durable": true},
				"deliver": []any{map[string]any{
					"channel": "reliable_event",
					"target":  target,
				}},
			}},
			"actions": rawActions,
		},
	}
}

// An event whose deliver target names an action that does not exist validates
// green (no manifest is wrong on its own) and then retries to dead-letter at
// runtime, so the publisher's promised consequence silently never happens.
// This is the exact shape that shipped in kafe: journal-reversed targeted
// gl-balance.reverse while gl-balance only declared `update`.
func TestValidateEventTargets_RejectsUnknownAction(t *testing.T) {
	manifests := []manifest.RawManifest{
		eventEntity("journal-entry.yaml", "gl", "journal-entry", "journal-reversed",
			map[string]any{"resource": "gl.gl-balance", "action": "reverse"}, nil),
		eventEntity("gl-balance.yaml", "gl", "gl-balance", "balance-changed",
			map[string]any{"resource": "gl.account", "action": "update"},
			[]map[string]any{{"name": "update", "idempotent": true}}),
	}

	rejects := validateEventTargets(manifests)
	msg, ok := rejects["journal-entry.yaml"]
	if !ok {
		t.Fatalf("expected a rejection for the unknown target action, got %v", rejects)
	}
	if !strings.Contains(msg, "reverse") || !strings.Contains(msg, "does not exist") {
		t.Errorf("message should name the missing action, got %q", msg)
	}
}

// The corrected shape — both events targeting the one action the summary entity
// actually declares — must stay clean.
func TestValidateEventTargets_AcceptsResolvableTarget(t *testing.T) {
	manifests := []manifest.RawManifest{
		eventEntity("journal-entry.yaml", "gl", "journal-entry", "journal-reversed",
			map[string]any{"resource": "gl.gl-balance", "action": "update"}, nil),
		eventEntity("gl-balance.yaml", "gl", "gl-balance", "balance-changed",
			map[string]any{"resource": "gl.account", "action": "update"},
			[]map[string]any{{"name": "update", "idempotent": true}}),
	}

	if rejects := validateEventTargets(manifests); len(rejects) != 0 {
		t.Fatalf("expected no rejects, got %v", rejects)
	}
}

// A target the outbox retries must be safe to run twice (7.7.3, the same rule
// validateIntegrators applies).
func TestValidateEventTargets_RejectsNonIdempotentAction(t *testing.T) {
	manifests := []manifest.RawManifest{
		eventEntity("journal-entry.yaml", "gl", "journal-entry", "journal-posted",
			map[string]any{"resource": "gl.gl-balance", "action": "update"}, nil),
		eventEntity("gl-balance.yaml", "gl", "gl-balance", "balance-changed",
			map[string]any{"resource": "gl.account", "action": "update"},
			[]map[string]any{{"name": "update", "idempotent": false}}),
	}

	rejects := validateEventTargets(manifests)
	msg, ok := rejects["journal-entry.yaml"]
	if !ok {
		t.Fatalf("expected a rejection for a non-idempotent target, got %v", rejects)
	}
	if !strings.Contains(msg, "idempotent") {
		t.Errorf("message should explain the idempotency requirement, got %q", msg)
	}
}

// A target outside this manifest set (a separately shipped module) cannot be
// verified — it must be skipped, not rejected.
func TestValidateEventTargets_SkipsUnresolvableModule(t *testing.T) {
	manifests := []manifest.RawManifest{
		eventEntity("journal-entry.yaml", "gl", "journal-entry", "journal-posted",
			map[string]any{"resource": "other-module.somewhere", "action": "update"}, nil),
	}

	if rejects := validateEventTargets(manifests); len(rejects) != 0 {
		t.Fatalf("an unverifiable target must be skipped, got %v", rejects)
	}
}

// A bare resource name resolves against the publishing entity's own module
// (matching resource.splitResourceRef at runtime), and a reserved action takes
// the idempotency its manifest declares — `create` is not idempotent merely for
// being reserved.
func TestValidateEventTargets_BareNameResolvesToOwnModule(t *testing.T) {
	manifests := []manifest.RawManifest{
		eventEntity("journal-entry.yaml", "gl", "journal-entry", "journal-posted",
			map[string]any{"resource": "journal-entry", "action": "update"},
			[]map[string]any{{"name": "update", "idempotent": true}}),
	}

	if rejects := validateEventTargets(manifests); len(rejects) != 0 {
		t.Fatalf("bare same-module name with an idempotent action must resolve, got %v", rejects)
	}
}

// A reserved action is only retry-safe if the manifest says so: `create` without
// `idempotent: true` is a legitimate rejection, because the outbox retries.
func TestValidateEventTargets_ReservedActionNeedsDeclaredIdempotency(t *testing.T) {
	manifests := []manifest.RawManifest{
		eventEntity("journal-entry.yaml", "gl", "journal-entry", "journal-posted",
			map[string]any{"resource": "journal-entry", "action": "create"}, nil),
	}

	rejects := validateEventTargets(manifests)
	if msg, ok := rejects["journal-entry.yaml"]; !ok {
		t.Fatalf("expected a rejection for a non-idempotent reserved action, got %v", rejects)
	} else if !strings.Contains(msg, "idempotent") {
		t.Errorf("message should explain the idempotency requirement, got %q", msg)
	}

	// With idempotency declared, the same shape is accepted.
	declared := []manifest.RawManifest{
		eventEntity("journal-entry.yaml", "gl", "journal-entry", "journal-posted",
			map[string]any{"resource": "journal-entry", "action": "create"},
			[]map[string]any{{"name": "create", "idempotent": true}}),
	}
	if rejects := validateEventTargets(declared); len(rejects) != 0 {
		t.Fatalf("a declared-idempotent reserved action must resolve, got %v", rejects)
	}
}

// A target naming only one of resource/action is unusable at runtime.
func TestValidateEventTargets_RejectsIncompleteTarget(t *testing.T) {
	manifests := []manifest.RawManifest{
		eventEntity("journal-entry.yaml", "gl", "journal-entry", "journal-posted",
			map[string]any{"resource": "gl.gl-balance"}, nil),
	}

	rejects := validateEventTargets(manifests)
	msg, ok := rejects["journal-entry.yaml"]
	if !ok {
		t.Fatalf("expected a rejection for an incomplete target, got %v", rejects)
	}
	if !strings.Contains(msg, "resource") || !strings.Contains(msg, "action") {
		t.Errorf("message should name both required keys, got %q", msg)
	}
}

// A service action is a legitimate target too.
func TestValidateEventTargets_ResolvesServiceAction(t *testing.T) {
	svc := manifest.RawManifest{
		APIVersion: "formspec.dev/v1",
		Kind:       "Service",
		Source:     "notify.yaml",
		Metadata:   manifest.RawMetadata{Name: "notify", Module: "gl"},
		Spec: map[string]any{
			"version": "v1",
			"actions": []any{map[string]any{"name": "send", "idempotent": true}},
		},
	}
	manifests := []manifest.RawManifest{
		eventEntity("journal-entry.yaml", "gl", "journal-entry", "journal-posted",
			map[string]any{"resource": "gl.notify", "action": "send"}, nil),
		svc,
	}

	if rejects := validateEventTargets(manifests); len(rejects) != 0 {
		t.Fatalf("a declared service action must resolve, got %v", rejects)
	}
}
