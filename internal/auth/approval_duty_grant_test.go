package auth

import (
	"strings"
	"testing"

	"github.com/primadi/formspec/internal/entity"
	"github.com/primadi/formspec/internal/ui"
	"github.com/primadi/formspec/pkg/spec"
)

// workflowGrant builds the grant an author writes to hand out an approval duty:
//   - { page: "workflow:{gate}", actions: [{ name: "{duty}" }] }
func workflowGrant(gateName, stepName string) []Grant {
	return []Grant{{Page: "workflow:" + gateName, Actions: []ActionGrant{{Name: stepName}}}}
}

// dutyMaterializer builds a Materializer whose approval lookup answers for one
// gate, so these tests are about materialization rather than about loading a
// tree (the real tree is covered by TestKafeSeed_GrantsAllResolve).
func dutyMaterializer(t *testing.T, name, module string, a *spec.ApprovalSpec) *Materializer {
	t.Helper()
	m := NewMaterializer(ui.NewRegistry(), entity.NewRegistry(nil, "", ""))
	m.SetApprovalDuties(func(want string) (string, []spec.DutyRef, bool) {
		if want != name {
			return "", nil, false
		}
		return module, spec.ApprovalDuties(module, name, a), true
	})
	return m
}

// TestMaterialize_ApprovalDutyGrant is the acceptance for Fase 4: a grant naming
// an approval duty resolves to the permission the gate's own declaration
// implies — derived, never transcribed.
func TestMaterialize_ApprovalDutyGrant(t *testing.T) {
	m := dutyMaterializer(t, "order.void-order", "cafe-order", &spec.ApprovalSpec{
		Steps: []spec.ApprovalStep{
			{Name: "supervisor-check", Permission: "supervisor-check", Roles: []string{"supervisor"}},
		},
	})

	perms, problems := m.MaterializeDetailed(workflowGrant("order.void-order", "supervisor-check"))
	if len(problems) > 0 {
		t.Fatalf("a correctly named duty grant must resolve, got %v", problems)
	}
	if len(perms) != 1 {
		t.Fatalf("expected one permission, got %+v", perms)
	}
	want := "workflow.cafe-order.order.void-order.supervisor-check"
	if perms[0].Permission != want {
		t.Fatalf("permission = %q, want %q", perms[0].Permission, want)
	}
	// Deeper than an entity permission ({module}.{plural}.{action}) and prefixed
	// with `workflow.`, so it is not reachable by `cafe-order.*`.
	if n := strings.Count(perms[0].Permission, ".") + 1; n != 5 {
		t.Errorf("duty has %d segments, want 5", n)
	}
}

// TestMaterialize_EscalationDutyIsGrantable is why escalation targets are listed
// as duties at all: the takeover is NOT a step, so without this a grant could
// never name it — and the escalation would point at a permission nobody holds,
// leaving a stalled step nobody can unblock.
func TestMaterialize_EscalationDutyIsGrantable(t *testing.T) {
	m := dutyMaterializer(t, "order.void-order", "cafe-order", &spec.ApprovalSpec{
		Steps: []spec.ApprovalStep{{
			Name:       "supervisor-check",
			Permission: "supervisor-check",
			Escalation: &spec.StepEscalation{After: "4h", Reassign: "manager-check"},
		}},
	})

	perms, problems := m.MaterializeDetailed(workflowGrant("order.void-order", "manager-check"))
	if len(problems) > 0 {
		t.Fatalf("the escalation duty must be grantable, got %v", problems)
	}
	if len(perms) != 1 || perms[0].Permission != "workflow.cafe-order.order.void-order.manager-check" {
		t.Fatalf("escalation duty = %+v, want workflow.cafe-order.order.void-order.manager-check", perms)
	}
}

// TestMaterialize_ApprovalGrantWithoutRegistryFailsLoudly is the regression for
// the measurement that motivated Fase 4: before `SetApprovalDuties` existed, a
// `workflow:` grant materialized to nothing — the role kept its other
// permissions and the duty was simply absent (kafe: 44 permissions, no
// `workflow.*`). Silent absence is the failure mode, so an unwired lookup must
// REPORT rather than drop.
func TestMaterialize_ApprovalGrantWithoutRegistryFailsLoudly(t *testing.T) {
	m := NewMaterializer(ui.NewRegistry(), entity.NewRegistry(nil, "", ""))

	perms, problems := m.MaterializeDetailed(workflowGrant("order.void-order", "supervisor-check"))
	if len(perms) != 0 {
		t.Fatalf("nothing can resolve without a registry, got %+v", perms)
	}
	if len(problems) != 1 {
		t.Fatalf("expected exactly one problem, got %v", problems)
	}
	if !strings.Contains(problems[0].Reason, "no approval registry is wired") {
		t.Errorf("the problem should say why, got %q", problems[0].Reason)
	}
}

// TestMaterialize_ApprovalWithNoDutyStepFailsLoudly covers the other silent
// no-op: a gate whose every step is role-gated and has no escalation. There is
// nothing to grant, and saying so is what keeps the author from believing the
// grant took effect.
func TestMaterialize_ApprovalWithNoDutyStepFailsLoudly(t *testing.T) {
	m := dutyMaterializer(t, "pick.approve", "warehouse", &spec.ApprovalSpec{
		Steps: []spec.ApprovalStep{{Roles: []string{"demo.manager"}}},
	})

	perms, problems := m.MaterializeDetailed(workflowGrant("pick.approve", "manager-review"))
	if len(perms) != 0 {
		t.Fatalf("expected no permission, got %+v", perms)
	}
	if len(problems) != 1 || !strings.Contains(problems[0].Reason, "declares no grantable duty") {
		t.Fatalf("expected a 'nothing to grant' problem, got %v", problems)
	}
}

// TestMaterialize_UnknownGateAndStepFailLoudly pins the typo cases: an
// unknown gate name, and an action naming a step that declares no duty.
func TestMaterialize_UnknownGateAndStepFailLoudly(t *testing.T) {
	m := dutyMaterializer(t, "order.void-order", "cafe-order", &spec.ApprovalSpec{
		Steps: []spec.ApprovalStep{
			{Name: "supervisor-check", Permission: "supervisor-check"},
			{Name: "role-only", Roles: []string{"x"}},
		},
	})

	t.Run("unknown gate", func(t *testing.T) {
		_, problems := m.MaterializeDetailed(workflowGrant("nope", "supervisor-check"))
		if len(problems) != 1 || !strings.Contains(problems[0].Reason, `unknown approval gate "nope"`) {
			t.Fatalf("got %v", problems)
		}
	})

	t.Run("step that declares no duty", func(t *testing.T) {
		// The step EXISTS but carries no `permission`, so the granted action
		// matches nothing. Reported, not ignored.
		perms, problems := m.MaterializeDetailed(workflowGrant("order.void-order", "role-only"))
		if len(perms) != 0 || len(problems) != 1 {
			t.Fatalf("expected one problem and no permission, got %+v / %v", perms, problems)
		}
	})
}

// TestMaterialize_ApprovalGrantIsScopedToTheNamedStep keeps one grant from
// handing out every duty of a gate: a multi-step chain grants exactly the step
// it names, which is the whole reason the action is the duty name.
func TestMaterialize_ApprovalGrantIsScopedToTheNamedStep(t *testing.T) {
	m := dutyMaterializer(t, "doc.review", "demo", &spec.ApprovalSpec{
		Steps: []spec.ApprovalStep{
			{Name: "first-check", Permission: "first-check"},
			{Name: "second-check", Permission: "second-check"},
		},
	})

	perms, problems := m.MaterializeDetailed(workflowGrant("doc.review", "second-check"))
	if len(problems) > 0 {
		t.Fatalf("unexpected problems: %v", problems)
	}
	if len(perms) != 1 {
		t.Fatalf("expected exactly one permission, got %+v", perms)
	}
	if perms[0].Permission != "workflow.demo.doc.review.second-check" {
		t.Fatalf("got %q — a grant must not hand out the steps it did not name", perms[0].Permission)
	}
}
