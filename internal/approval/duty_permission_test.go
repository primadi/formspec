package approval

import (
	"testing"

	"github.com/primadi/formspec/pkg/spec"
)

// `dutyGateName` is the derived gate name for entity `order` + transition
// `void-order`; the duty permission is qualified by it.
const dutyGateName = "order.void-order"

// dutyWorkflow is a step whose gate is a PERMISSION, not a role name — the shape
// AGENTS.md rule 6 asks for ("Permission = resource + action, never hardcoded
// role names dalam YAML").
func dutyWorkflow() *spec.ApprovalSpec {
	return &spec.ApprovalSpec{
		Steps: []spec.ApprovalStep{
			{
				Name:       "supervisor-check",
				Permission: "approve-void", // short form — qualified by the gate's own identity
				Roles:      []string{"supervisor"},
				Title:      "Persetujuan Void Pesanan",
			},
		},
	}
}

// pendingFor builds a pending approval of wf, so a test states only what it
// varies.
func pendingFor(wf *spec.ApprovalSpec, module, requester string) *Approval {
	return NewApproval(wf, module, dutyGateName, "cafe-order.order", "rec-1", "paid", "cancelled", requester)
}

// permissionSet builds an Approver holding exactly the given permissions, to
// keep the assertions about permissions rather than about role data.
func permissionSet(userID string, perms ...string) Approver {
	return Approver{
		UserID: userID,
		Can: func(p string) bool {
			for _, have := range perms {
				if have == p {
					return true
				}
			}
			return false
		},
	}
}

// TestCanApprove_DutyPermissionIsEnough is the point of the whole change: the
// caller holds no role at all and is still eligible, because they hold the
// permission that gates the step.
func TestCanApprove_DutyPermissionIsEnough(t *testing.T) {
	e := NewEngine(NewRegistry())
	wf := dutyWorkflow()

	approver := permissionSet("user-1", spec.StepPermission("cafe-order", dutyGateName, wf.Steps[0]))
	ok, reason := e.CanApprove(pendingFor(wf, "cafe-order", "requester-1"), wf.Steps, approver)
	if !ok {
		t.Fatalf("holding the step's duty permission must be enough, got refused: %s", reason)
	}
}

// TestCanApprove_DutyAndRoleAreAlternatives pins the migration contract: a
// workflow that declares `permission` does not stop working for a caller who
// only has the role, so adopting duties is not a flag day.
func TestCanApprove_DutyAndRoleAreAlternatives(t *testing.T) {
	e := NewEngine(NewRegistry())
	wf := dutyWorkflow()

	// Role only, no permissions.
	ok, reason := e.CanApprove(pendingFor(wf, "cafe-order", "requester-1"), wf.Steps,
		Approver{UserID: "user-1", Roles: []string{"supervisor"}})
	if !ok {
		t.Fatalf("the step's role must still satisfy the step during migration, got: %s", reason)
	}
}

// TestCanApprove_DutyDeniesWithoutEither makes sure adding a duty does not turn
// the step into an open door: no duty, no role means refused, and the message
// names BOTH things the caller lacks.
func TestCanApprove_DutyDeniesWithoutEither(t *testing.T) {
	e := NewEngine(NewRegistry())
	wf := dutyWorkflow()

	ok, reason := e.CanApprove(pendingFor(wf, "cafe-order", "requester-1"), wf.Steps,
		Approver{UserID: "user-1", Roles: []string{"kasir"}})
	if ok {
		t.Fatal("a caller with neither duty nor role must be refused")
	}
	duty := spec.StepPermission("cafe-order", dutyGateName, wf.Steps[0])
	if reason == "" || !contains(reason, duty) {
		t.Fatalf("the refusal should name the missing duty %q, got %q", duty, reason)
	}
}

// TestCanApprove_RoleOnlyStepUnchanged covers a step with no duty at all: the
// role rule applies alone, so nothing written before this change shifts.
func TestCanApprove_RoleOnlyStepUnchanged(t *testing.T) {
	e := NewEngine(NewRegistry())
	wf := &spec.ApprovalSpec{
		Steps: []spec.ApprovalStep{{Roles: []string{"supervisor"}}},
	}

	if ok, _ := e.CanApprove(pendingFor(wf, "gl", "requester-1"), wf.Steps, Approver{UserID: "u", Roles: []string{"supervisor"}}); !ok {
		t.Error("a legacy workflow must keep accepting the step's role")
	}
	if ok, _ := e.CanApprove(pendingFor(wf, "gl", "requester-1"), wf.Steps, Approver{UserID: "u", Roles: []string{"other"}}); ok {
		t.Error("a legacy workflow must still refuse a non-holder")
	}
	// A permission is NOT a substitute when no duty is declared: otherwise any
	// permission would open every legacy step.
	if ok, _ := e.CanApprove(pendingFor(wf, "gl", "requester-1"), wf.Steps, permissionSet("u", "*")); ok {
		t.Error("with no declared duty, holding permissions must not grant approval")
	}
}

// TestCanApprove_EscalatedRolesStillWiden keeps 7.4.4 working: after escalation
// the reassigned roles gain approval rights, whether or not the step declares a
// duty.
func TestCanApprove_EscalatedRolesStillWiden(t *testing.T) {
	e := NewEngine(NewRegistry())
	wf := dutyWorkflow()

	a := pendingFor(wf, "cafe-order", "requester-1")
	a.MarkEscalated(wf.Steps, 0, []string{"manajer"})
	ok, _ := e.CanApprove(a, wf.Steps, Approver{UserID: "u", Roles: []string{"manajer"}})
	if !ok {
		t.Error("an escalated role must gain approval rights")
	}
}

// TestCanApprove_RequesterExcludedEvenWithDuty is the 7.4.5 guarantee under the
// new gate: a permission is not a licence to approve your own request.
func TestCanApprove_RequesterExcludedEvenWithDuty(t *testing.T) {
	e := NewEngine(NewRegistry())
	wf := dutyWorkflow()

	duty := spec.StepPermission("cafe-order", dutyGateName, wf.Steps[0])
	ok, reason := e.CanApprove(pendingFor(wf, "cafe-order", "requester-1"), wf.Steps,
		permissionSet("requester-1", duty))
	if ok {
		t.Fatal("the requester must not approve their own request, even holding the duty permission")
	}
	if !contains(reason, "own request") {
		t.Errorf("refusal should say why, got %q", reason)
	}
}

// TestStepPermission_Qualification pins the derived string and the one rule that
// makes it safe: four segments, so it cannot collide with an entity permission
// and is not reachable by a module wildcard.
func TestStepPermission_Qualification(t *testing.T) {
	short := spec.ApprovalStep{Name: "supervisor-check", Permission: "approve-void"}
	got := spec.StepPermission("cafe-order", dutyGateName, short)
	if got != "workflow.cafe-order.order.void-order.supervisor-check" {
		t.Fatalf("derived duty = %q", got)
	}
	if n := len(splitDots(got)); n != 5 {
		t.Errorf("duty has %d segments, want 5 — it must be deeper than an entity permission", n)
	}

	// A fully qualified value is taken as written.
	full := spec.ApprovalStep{Name: "supervisor-check", Permission: "acme.approvals.void"}
	if got := spec.StepPermission("cafe-order", dutyGateName, full); got != "acme.approvals.void" {
		t.Errorf("explicit duty should pass through, got %q", got)
	}

	// No permission → no duty (eligibility rests on roles alone).
	if got := spec.StepPermission("cafe-order", dutyGateName, spec.ApprovalStep{Name: "x"}); got != "" {
		t.Errorf("a step without `permission` has no duty, got %q", got)
	}
}

// TestStepPermission_NeedsName documents why a duty cannot be derived for an
// unnamed step: the empty answer is what makes the validator reject it, instead
// of persisting a dangling permission nobody can grant.
func TestStepPermission_NeedsName(t *testing.T) {
	unnamed := spec.ApprovalStep{Permission: "approve-void"}
	if got := spec.StepPermission("cafe-order", dutyGateName, unnamed); got != "" {
		t.Fatalf("an unnamed step must yield no duty, got %q", got)
	}
}

func contains(haystack, needle string) bool {
	return len(needle) == 0 || (len(haystack) >= len(needle) && indexOf(haystack, needle) >= 0)
}

func indexOf(h, n string) int {
	for i := 0; i+len(n) <= len(h); i++ {
		if h[i:i+len(n)] == n {
			return i
		}
	}
	return -1
}

func splitDots(s string) []string {
	var out []string
	start := 0
	for i := 0; i < len(s); i++ {
		if s[i] == '.' {
			out = append(out, s[start:i])
			start = i + 1
		}
	}
	return append(out, s[start:])
}
