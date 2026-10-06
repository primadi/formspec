package main

import (
	"strings"
	"testing"

	"github.com/primadi/formspec/internal/manifest"
)

// roleOnlyEntity builds an Entity whose `void-order` transition carries an
// approval step with the given fields, so a test states only what it varies.
func roleOnlyEntity(source string, step map[string]any) manifest.RawManifest {
	return manifest.RawManifest{
		APIVersion: "formspec.dev/v1",
		Kind:       "Entity",
		Source:     source,
		Metadata:   manifest.RawMetadata{Name: "order", Module: "cafe-order"},
		Spec: map[string]any{
			"version":        "v1",
			"characteristic": "transaction",
			"fields":         []any{map[string]any{"name": "status", "type": "string"}},
			"state_machine": map[string]any{
				"field":   "status",
				"initial": "paid",
				"states":  []any{map[string]any{"name": "paid"}, map[string]any{"name": "cancelled"}},
				"transitions": []any{map[string]any{
					"from": []any{"paid"}, "to": "cancelled", "via": "void-order",
					"approval": map[string]any{"steps": []any{step}},
				}},
			},
		},
	}
}

// TestScanApprovalRoleOnlySteps_WarnsWithTheWayForward pins what the deprecation
// has to SAY, not only that it fires: the point is telling an author how to
// convert the step, so the message names the duty form and the grant it needs.
func TestScanApprovalRoleOnlySteps_WarnsWithTheWayForward(t *testing.T) {
	manifests := []manifest.RawManifest{roleOnlyEntity("order.yaml", map[string]any{
		"name": "supervisor-check",
		"roles": []any{
			"supervisor",
		},
	})}

	issues := scanApprovalRoleOnlySteps(manifests)
	if len(issues) != 1 {
		t.Fatalf("issues = %+v, want one for a roles-only step", issues)
	}
	got := issues[0]
	if got.Severity != "warning" {
		t.Errorf("severity = %q — the step WORKS, so this must not fail the run", got.Severity)
	}
	for _, want := range []string{"supervisor-check", "supervisor", "permission:", "workflow:order.void-order"} {
		if !strings.Contains(got.Message, want) {
			t.Errorf("message %q should mention %q", got.Message, want)
		}
	}
	// It must say why the coupling matters, not just that it exists.
	if !strings.Contains(got.Message, "renaming a role") {
		t.Errorf("message %q should name the failure it prevents", got.Message)
	}
}

// TestScanApprovalRoleOnlySteps_DutyIsSilent is the other half: a step that
// declares a duty is the target form and must not be nagged about.
func TestScanApprovalRoleOnlySteps_DutyIsSilent(t *testing.T) {
	manifests := []manifest.RawManifest{roleOnlyEntity("order.yaml", map[string]any{
		"name":       "supervisor-check",
		"permission": "supervisor-check",
	})}
	if issues := scanApprovalRoleOnlySteps(manifests); len(issues) != 0 {
		t.Fatalf("a duty step is the target form, got %+v", issues)
	}

	// A step carrying BOTH keeps the duty, so it is already decoupled.
	both := []manifest.RawManifest{roleOnlyEntity("order.yaml", map[string]any{
		"name":       "supervisor-check",
		"permission": "supervisor-check",
		"roles":      []any{"supervisor"},
	})}
	if issues := scanApprovalRoleOnlySteps(both); len(issues) != 0 {
		t.Fatalf("a step with a duty must be silent even if it also lists roles, got %+v", issues)
	}
}

// TestScanApprovalRoleOnlySteps_SequentialIsExempt is the reason this is a
// deprecation and not a removal: a chain is ORDERED by its roles, and
// `validateApprovalStepMode` refuses `permission` alongside `sequential`. So a
// chain MUST use roles — warning about it would be asking for the impossible.
func TestScanApprovalRoleOnlySteps_SequentialIsExempt(t *testing.T) {
	manifests := []manifest.RawManifest{roleOnlyEntity("order.yaml", map[string]any{
		"name":  "chain",
		"mode":  "sequential",
		"roles": []any{"gl.clerk", "gl.manager"},
	})}
	if issues := scanApprovalRoleOnlySteps(manifests); len(issues) != 0 {
		t.Fatalf("a sequential chain's roles ARE the chain and must be exempt, got %+v", issues)
	}
}

// TestScanApprovalRoleOnlySteps_UnapprovableStepIsNotReportedHere: a step with
// neither roles nor permission is a HARD error raised by
// `ValidateApprovalSteps`; reporting it here too would double-report it as a
// mere warning.
func TestScanApprovalRoleOnlySteps_UnapprovableStepIsNotReportedHere(t *testing.T) {
	manifests := []manifest.RawManifest{roleOnlyEntity("order.yaml", map[string]any{
		"name": "nobody",
	})}
	if issues := scanApprovalRoleOnlySteps(manifests); len(issues) != 0 {
		t.Fatalf("an un-approvable step is an error elsewhere, got %+v", issues)
	}
}
