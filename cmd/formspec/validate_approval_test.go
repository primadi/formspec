package main

import (
	"strings"
	"testing"

	"github.com/primadi/formspec/internal/manifest"
)

// approvalEntity builds a raw Entity manifest whose `void-order` transition
// carries an approval gate with the given steps. Approval is declared ON the
// transition now, so a fixture states the steps where a manifest would.
func approvalEntity(source, module, name string, steps ...map[string]any) manifest.RawManifest {
	raw := make([]any, 0, len(steps))
	for _, s := range steps {
		raw = append(raw, s)
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
				"field":   "status",
				"initial": "paid",
				"states": []any{
					map[string]any{"name": "paid"}, map[string]any{"name": "cancelled"},
				},
				"transitions": []any{
					map[string]any{
						"from":     []any{"paid"},
						"to":       "cancelled",
						"via":      "void-order",
						"approval": map[string]any{"steps": raw},
					},
				},
			},
		},
	}
}

// rolesSeedManifest declares role records the way a project does — a `kind:
// Seed` whose entity is the framework's role entity. An approval naming a role
// needs this in the tree, exactly as kafe's `seeds/roles.yaml` provides it.
func rolesSeedManifest(names ...string) manifest.RawManifest {
	records := make([]map[string]any, 0, len(names))
	for _, n := range names {
		records = append(records, map[string]any{"name": n, "grants": []any{}})
	}
	return manifest.RawManifest{
		APIVersion: "formspec.dev/v1",
		Kind:       "Seed",
		Source:     "roles.yaml",
		Metadata:   manifest.RawMetadata{Name: "roles", Module: "formspec.core"},
		Spec: map[string]any{
			"entities": []any{
				map[string]any{"entity": "formspec.core.role", "records": records},
			},
		},
	}
}

// TestValidateApprovals_DisplayFieldsMustExist pins S15 (item 5.3): a step's
// display_fields must name real fields of the entity. A typo would render an
// empty column in the ApprovalInbox — the approver sees nothing to decide on,
// which reads as "no data" rather than "typo".
func TestValidateApprovals_DisplayFieldsMustExist(t *testing.T) {
	stepWith := func(fields ...string) map[string]any {
		raw := make([]any, 0, len(fields))
		for _, f := range fields {
			raw = append(raw, f)
		}
		return map[string]any{
			"roles":          []any{"cafe-order.supervisor"},
			"display_fields": raw,
		}
	}

	// `status` and `transaction_date` are declared on the entity — accepted.
	ok := append([]manifest.RawManifest{
		approvalEntity("order.yaml", "cafe-order", "order", stepWith("status", "transaction_date")),
	}, rolesSeedManifest("cafe-order.supervisor"))
	if rejects := validateApprovals(ok); len(rejects) != 0 {
		t.Fatalf("expected declared display_fields to be accepted, got %v", rejects)
	}

	// `void_reason` is not declared — rejected, naming the field.
	bad := append([]manifest.RawManifest{
		approvalEntity("order.yaml", "cafe-order", "order", stepWith("void_reason")),
	}, rolesSeedManifest("cafe-order.supervisor"))
	rejects := validateApprovals(bad)
	msg, found := rejects["order.yaml"]
	if !found {
		t.Fatalf("expected rejection of an unknown display field, got %v", rejects)
	}
	if !strings.Contains(msg, "void_reason") {
		t.Errorf("error %q should name the unknown field", msg)
	}
}

// TestValidateApprovals_RejectsUnknownRole is the kafe 5.13.8 regression.
//
// The approval named `cafe-order.supervisor` while the seeded role is
// `supervisor`. Nothing checked the name, so validate answered 0 problems and
// the void approval was unreachable forever: `CanApprove` compares names
// literally and refuses, leaving the inbox empty for the only approver who
// exists. The failure was only visible by trying to approve.
func TestValidateApprovals_RejectsUnknownRole(t *testing.T) {
	manifests := append([]manifest.RawManifest{
		approvalEntity("order.yaml", "cafe-order", "order", map[string]any{
			"roles": []any{"cafe-order.supervisor"},
		}),
	}, rolesSeedManifest("supervisor")) // the seeded name — not the one the gate asks for

	msg, ok := validateApprovals(manifests)["order.yaml"]
	if !ok {
		t.Fatal("a step naming a role no role declares must be rejected")
	}
	for _, want := range []string{"cafe-order.supervisor", "no role declares", "supervisor"} {
		if !strings.Contains(msg, want) {
			t.Errorf("error %q should mention %q", msg, want)
		}
	}
}

// TestValidateApprovals_AcceptsDeclaredRole is the other half: the same gate
// with the role actually declared passes, so the check is not simply rejecting
// every approval that names a role.
func TestValidateApprovals_AcceptsDeclaredRole(t *testing.T) {
	manifests := append([]manifest.RawManifest{
		approvalEntity("order.yaml", "cafe-order", "order", map[string]any{
			"roles": []any{"cafe-order.supervisor"},
		}),
	}, rolesSeedManifest("cafe-order.supervisor"))

	if rejects := validateApprovals(manifests); len(rejects) != 0 {
		t.Fatalf("expected the declared role to be accepted, got %v", rejects)
	}
}

// TestValidateApprovals_EscalationDutyIsNotARoleCheck pins the split: the role
// check applies to `roles` (who may approve), while an escalation target is a
// DUTY — a permission, not a role name — so it must NOT be run through the role
// index. Its satisfiability (is anyone granted it?) is a grant question, checked
// by the examples' seed-grant tests.
func TestValidateApprovals_EscalationDutyIsNotARoleCheck(t *testing.T) {
	step := map[string]any{
		"name":       "supervisor-check",
		"roles":      []any{"supervisor"},
		"escalation": map[string]any{"after": "4h", "reassign": "manager-check"},
	}
	manifests := append([]manifest.RawManifest{
		approvalEntity("order.yaml", "cafe-order", "order", step),
	}, rolesSeedManifest("supervisor")) // note: no `manager-check` role exists

	if rejects := validateApprovals(manifests); len(rejects) != 0 {
		t.Fatalf("an escalation duty must not be validated as a role, got %v", rejects)
	}
}

// TestValidateApprovals_AcceptsFrameworkOwnerRole guards the one legitimate way
// to name a role that has no manifest: the owner roles the auth service seeds
// itself. Without them a gate guarding a transition on the workspace owner would
// be rejected as a typo.
func TestValidateApprovals_AcceptsFrameworkOwnerRole(t *testing.T) {
	// No role seed at all: the framework supplies this one.
	manifests := []manifest.RawManifest{
		approvalEntity("order.yaml", "cafe-order", "order", map[string]any{
			"roles": []any{"workspace-owner"},
		}),
	}
	if rejects := validateApprovals(manifests); len(rejects) != 0 {
		t.Fatalf("framework owner role must be accepted, got %v", rejects)
	}
}
