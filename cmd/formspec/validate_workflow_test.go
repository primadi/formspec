package main

import (
	"strings"
	"testing"

	"github.com/primadi/formspec/internal/manifest"
)

// wfManifest builds a raw Workflow manifest as far as validateWorkflows reads it.
func wfManifest(source, module, name, entity string, transition map[string]any) manifest.RawManifest {
	return manifest.RawManifest{
		APIVersion: "formspec.dev/v1",
		Kind:       "Workflow",
		Source:     source,
		Metadata:   manifest.RawMetadata{Name: name, Module: module},
		Spec: map[string]any{
			"entity": entity,
			"on":     map[string]any{"transition": transition},
			"steps":  []any{map[string]any{"roles": []any{module + ".supervisor"}}},
		},
	}
}

// entityManifest builds a raw Entity manifest carrying a state machine.
func entityManifest(source, module, name string, transitions []map[string]any) manifest.RawManifest {
	raw := make([]any, 0, len(transitions))
	for _, t := range transitions {
		raw = append(raw, t)
	}
	return manifest.RawManifest{
		APIVersion: "formspec.dev/v1",
		Kind:       "Entity",
		Source:     source,
		Metadata:   manifest.RawMetadata{Name: name, Module: module},
		Spec: map[string]any{
			"version": "v1",
			"fields": []any{
				map[string]any{"name": "status", "type": "string"},
				map[string]any{"name": "transaction_date", "type": "date"},
			},
			"state_machine": map[string]any{
				"field": "status",
				"states": []any{
					map[string]any{"name": "draft"}, map[string]any{"name": "paid"},
					map[string]any{"name": "in_kitchen"}, map[string]any{"name": "ready"},
					map[string]any{"name": "served"}, map[string]any{"name": "cancelled"},
				},
				"initial":     "draft",
				"transitions": raw,
			},
		},
	}
}

// multiOriginEntity is the kafe `order` shape: `void-order` reachable from four
// states, plus a single-origin transition that shares the same target state.
func multiOriginEntity() manifest.RawManifest {
	return entityManifest("order.yaml", "cafe-order", "order", []map[string]any{
		{"from": "paid", "to": "in_kitchen", "via": "start-preparing"},
		{"from": []any{"paid", "in_kitchen", "ready", "served"}, "to": "cancelled", "via": "void-order"},
		{"from": "draft", "to": "cancelled", "via": "abandon"},
	})
}

// TestValidateWorkflows_ByNameCoversEveryOriginState is the S9 acceptance: one
// workflow naming the transition intercepts it from all four origin states.
func TestValidateWorkflows_ByNameCoversEveryOriginState(t *testing.T) {
	manifests := []manifest.RawManifest{
		multiOriginEntity(),
		wfManifest("void.yaml", "cafe-order", "order-void-approval", "cafe-order.order",
			map[string]any{"name": "void-order"}),
	}

	if rejects := validateWorkflows(manifests); len(rejects) != 0 {
		t.Fatalf("expected a clean name-form workflow, got %v", rejects)
	}
}

// TestValidateWorkflows_RejectsPartialStatePair is the hole this check exists to
// close: `from: paid` on a four-origin transition previously validated green
// while leaving three origins unguarded.
func TestValidateWorkflows_RejectsPartialStatePair(t *testing.T) {
	manifests := []manifest.RawManifest{
		multiOriginEntity(),
		wfManifest("void.yaml", "cafe-order", "order-void-approval", "cafe-order.order",
			map[string]any{"from": "paid", "to": "cancelled"}),
	}

	rejects := validateWorkflows(manifests)
	msg, ok := rejects["void.yaml"]
	if !ok {
		t.Fatalf("expected rejection of a partial state pair, got %v", rejects)
	}
	for _, want := range []string{"void-order", "in_kitchen", "name:"} {
		if !strings.Contains(msg, want) {
			t.Errorf("error %q should mention %q", msg, want)
		}
	}
}

// TestValidateWorkflows_DisplayFieldsMustExist pins S15 (item 5.3): a step's
// display_fields must name real fields of the workflow's entity. A typo would
// render an empty column in the ApprovalInbox — the approver sees nothing to
// decide on, which reads as "no data" rather than "typo".
func TestValidateWorkflows_DisplayFieldsMustExist(t *testing.T) {
	withDisplay := func(fields ...string) manifest.RawManifest {
		raw := make([]any, 0, len(fields))
		for _, f := range fields {
			raw = append(raw, f)
		}
		return manifest.RawManifest{
			APIVersion: "formspec.dev/v1",
			Kind:       "Workflow",
			Source:     "void.yaml",
			Metadata:   manifest.RawMetadata{Name: "order-void-approval", Module: "cafe-order"},
			Spec: map[string]any{
				"entity": "cafe-order.order",
				"on":     map[string]any{"transition": map[string]any{"name": "void-order"}},
				"steps": []any{map[string]any{
					"roles":          []any{"cafe-order.supervisor"},
					"display_fields": raw,
				}},
			},
		}
	}

	// `status` and `transaction_date` are declared on the entity — accepted.
	ok := []manifest.RawManifest{multiOriginEntity(), withDisplay("status", "transaction_date")}
	if rejects := validateWorkflows(ok); len(rejects) != 0 {
		t.Fatalf("expected declared display_fields to be accepted, got %v", rejects)
	}

	// `void_reason` is not declared — rejected, naming the field.
	bad := []manifest.RawManifest{multiOriginEntity(), withDisplay("void_reason")}
	rejects := validateWorkflows(bad)
	msg, found := rejects["void.yaml"]
	if !found {
		t.Fatalf("expected rejection of an unknown display field, got %v", rejects)
	}
	if !strings.Contains(msg, "void_reason") {
		t.Errorf("error %q should name the unknown field", msg)
	}
}

// TestValidateWorkflows_AcceptsSingleOriginStatePair guards against
// over-rejection: the majority of existing workflows use from/to legitimately.
func TestValidateWorkflows_AcceptsSingleOriginStatePair(t *testing.T) {
	manifests := []manifest.RawManifest{
		multiOriginEntity(),
		wfManifest("prepare.yaml", "cafe-order", "prepare-approval", "cafe-order.order",
			map[string]any{"from": "paid", "to": "in_kitchen"}),
	}

	if rejects := validateWorkflows(manifests); len(rejects) != 0 {
		t.Fatalf("expected a single-origin pair to be accepted, got %v", rejects)
	}
}

// TestValidateWorkflows_Detects the failure modes that used to be silent: a
// mistyped transition name, a state pair that does not exist, a wrong entity,
// and a qualified name that disagrees with spec.entity.
func TestValidateWorkflows_Detects(t *testing.T) {
	cases := []struct {
		name       string
		transition map[string]any
		entity     string
		wantMsg    string
	}{
		{
			name:       "mistyped transition name",
			transition: map[string]any{"name": "void-oder"},
			entity:     "cafe-order.order",
			wantMsg:    "does not exist",
		},
		{
			name:       "state pair that is not a transition",
			transition: map[string]any{"from": "draft", "to": "served"},
			entity:     "cafe-order.order",
			wantMsg:    "not a transition",
		},
		{
			name:       "entity without a state machine",
			transition: map[string]any{"name": "void-order"},
			entity:     "cafe-order.nope",
			wantMsg:    "has no state machine",
		},
		{
			name:       "qualified name disagreeing with spec.entity",
			transition: map[string]any{"name": "other-module.other-entity.void-order"},
			entity:     "cafe-order.order",
			wantMsg:    "pick one form",
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			manifests := []manifest.RawManifest{
				multiOriginEntity(),
				wfManifest("wf.yaml", "cafe-order", "wf", c.entity, c.transition),
			}
			msg, ok := validateWorkflows(manifests)["wf.yaml"]
			if !ok {
				t.Fatalf("expected rejection for %s", c.name)
			}
			if !strings.Contains(msg, c.wantMsg) {
				t.Errorf("error %q should contain %q", msg, c.wantMsg)
			}
		})
	}
}

// TestValidateWorkflows_QualifiedNameMatchingEntityIsAccepted keeps the
// qualified spelling usable when it agrees.
func TestValidateWorkflows_QualifiedNameMatchingEntityIsAccepted(t *testing.T) {
	manifests := []manifest.RawManifest{
		multiOriginEntity(),
		wfManifest("wf.yaml", "cafe-order", "wf", "cafe-order.order",
			map[string]any{"name": "cafe-order.order.void-order"}),
	}

	if rejects := validateWorkflows(manifests); len(rejects) != 0 {
		t.Fatalf("expected a matching qualified name to be accepted, got %v", rejects)
	}
}
