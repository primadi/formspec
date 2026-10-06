package spec

import (
	"strings"
	"testing"
)

// wfWithSteps builds a minimal approval chain around the given steps.
func wfWithSteps(steps ...ApprovalStep) *ApprovalSpec {
	return &ApprovalSpec{Steps: steps}
}

// TestValidateApprovalSteps_UnapprovableStep is the rule that closes the
// same hole from the other side.
//
// `hasAnyRole` against an EMPTY role list is false for every caller, and a step
// with no `permission` has no duty to hold either — so a step declaring neither
// can never reach quorum. It is not a typo the runtime reports; the approval just
// waits, which reads as "nobody has got to it yet" rather than "this step is
// unapprovable by construction".
func TestValidateApprovalSteps_UnapprovableStep(t *testing.T) {
	err := ValidateApprovalSteps(wfWithSteps(ApprovalStep{Title: "nobody can do this"}))
	if err == nil {
		t.Fatal("a step with neither roles nor permission must be rejected")
	}
	if !strings.Contains(err.Error(), "un-approvable") {
		t.Errorf("error should say the step is un-approvable, got %q", err)
	}

	// Either one satisfies it: a role, or a duty.
	if err := ValidateApprovalSteps(wfWithSteps(ApprovalStep{Roles: []string{"supervisor"}})); err != nil {
		t.Errorf("a role alone must be enough: %v", err)
	}
	if err := ValidateApprovalSteps(wfWithSteps(ApprovalStep{Name: "check", Permission: "check"})); err != nil {
		t.Errorf("a duty alone must be enough: %v", err)
	}
}

// TestValidateApprovalSteps_CompletionRules pins the three obligations a
// step NAME carries, each with its own reason.
func TestValidateApprovalSteps_CompletionRules(t *testing.T) {
	t.Run("a step with escalation must be named", func(t *testing.T) {
		wf := wfWithSteps(ApprovalStep{
			Roles:      []string{"supervisor"},
			Escalation: &StepEscalation{After: "4h", Reassign: "manajer-check"},
		})
		err := ValidateApprovalSteps(wf)
		if err == nil || !strings.Contains(err.Error(), "escalation") {
			t.Fatalf("expected the escalation rule to fire, got %v", err)
		}
	})

	t.Run("a step with a duty must be named", func(t *testing.T) {
		wf := wfWithSteps(ApprovalStep{Roles: []string{"supervisor"}, Permission: "check"})
		err := ValidateApprovalSteps(wf)
		if err == nil || !strings.Contains(err.Error(), "must be nameable") {
			t.Fatalf("expected the duty rule to fire, got %v", err)
		}
	})

	t.Run("names must be unique", func(t *testing.T) {
		wf := wfWithSteps(
			ApprovalStep{Name: "check", Permission: "check"},
			ApprovalStep{Name: "check", Permission: "check"},
		)
		err := ValidateApprovalSteps(wf)
		if err == nil || !strings.Contains(err.Error(), "share the name") {
			t.Fatalf("expected the uniqueness rule to fire, got %v", err)
		}
	})

	t.Run("names must be usable in a permission", func(t *testing.T) {
		// A dotted or spaced name cannot appear in `workflow.a.b.{name}`, which
		// is where a duty lives.
		for _, bad := range []string{"Check", "with_underscore", "with.dot", "with space", "-leading"} {
			wf := wfWithSteps(ApprovalStep{Name: bad, Permission: "x"})
			if err := ValidateApprovalSteps(wf); err == nil {
				t.Errorf("name %q must be rejected as an identifier", bad)
			}
		}
		for _, ok := range []string{"check", "supervisor-check", "step2", "a-2-b"} {
			wf := wfWithSteps(ApprovalStep{Name: ok, Permission: "x"})
			if err := ValidateApprovalSteps(wf); err != nil {
				t.Errorf("name %q should be accepted: %v", ok, err)
			}
		}
	})
}

// TestValidateApprovalSteps_QuorumModes pins the three refusals that replaced a
// number the engine could not compute.
//
// `mode: all` asked for "every eligible approver" — a count no manifest implies,
// since a role list is not a list of people and a duty's holders cannot be
// enumerated. The runtime answered `len(roles)`, which could demand two
// signatures from one person (a duplicate approval for a step is refused), so the
// step could never be satisfied. Refusing the declaration is the honest end of
// that, and every manifest that omits `mode` behaves exactly as before.
func TestValidateApprovalSteps_QuorumModes(t *testing.T) {
	t.Run("all is refused with a way forward", func(t *testing.T) {
		wf := wfWithSteps(ApprovalStep{Roles: []string{"gl.supervisor"}, Mode: StepModeAll})
		err := ValidateApprovalSteps(wf)
		if err == nil {
			t.Fatal("`mode: all` cannot be computed and must be refused")
		}
		for _, want := range []string{"cannot be derived", "approvers", "sequential"} {
			if !strings.Contains(err.Error(), want) {
				t.Errorf("the refusal should point at %q, got %q", want, err)
			}
		}
	})

	t.Run("sequential needs the roles that order it", func(t *testing.T) {
		wf := wfWithSteps(ApprovalStep{Mode: StepModeSequential, Permission: "x", Name: "chain"})
		err := ValidateApprovalSteps(wf)
		if err == nil || !strings.Contains(err.Error(), "the role list IS the chain") {
			t.Fatalf("expected the roles rule, got %v", err)
		}
	})

	t.Run("sequential does not take a second count", func(t *testing.T) {
		wf := wfWithSteps(ApprovalStep{
			Mode: StepModeSequential, Roles: []string{"a", "b"}, Approvers: 2,
		})
		err := ValidateApprovalSteps(wf)
		if err == nil || !strings.Contains(err.Error(), "no effect") {
			t.Fatalf("expected the approvers rule, got %v", err)
		}
	})

	t.Run("sequential and a duty cannot be combined", func(t *testing.T) {
		wf := wfWithSteps(ApprovalStep{
			Name: "chain", Mode: StepModeSequential, Roles: []string{"a", "b"}, Permission: "chain",
		})
		err := ValidateApprovalSteps(wf)
		if err == nil || !strings.Contains(err.Error(), "no position in that order") {
			t.Fatalf("expected the duty/chain rule, got %v", err)
		}
	})

	t.Run("the usable shapes pass", func(t *testing.T) {
		for _, step := range []ApprovalStep{
			{Roles: []string{"a"}}, // absent mode → approvers (default 1)
			{Mode: StepModeAny, Roles: []string{"a"}, Approvers: 2},
			{Name: "c", Permission: "c", Approvers: 1},            // duty + explicit number
			{Mode: StepModeSequential, Roles: []string{"a", "b"}}, // a real chain
		} {
			if err := ValidateApprovalSteps(wfWithSteps(step)); err != nil {
				t.Errorf("step %+v should be accepted: %v", step, err)
			}
		}
	})
}

// TestValidateApprovalSteps_StepEscalationIsAccepted pins that step-level
// escalation — the only escalation effect the engine implements (reassignment
// after a timeout) — is the supported form.
func TestValidateApprovalSteps_StepEscalationIsAccepted(t *testing.T) {
	ok := wfWithSteps(ApprovalStep{
		Name: "check", Permission: "check",
		Escalation: &StepEscalation{After: "48h", Reassign: "head-check"},
	})
	if err := ValidateApprovalSteps(ok); err != nil {
		t.Fatalf("step-level escalation must be accepted: %v", err)
	}
}

// TestValidateApprovalSteps_EscalationMustDoSomething pins the three refusals
// that keep an escalation from being a declaration with no effect.
func TestValidateApprovalSteps_EscalationMustDoSomething(t *testing.T) {
	base := func(mut func(*StepEscalation)) ApprovalStep {
		e := &StepEscalation{After: "4h", Reassign: "manager-check"}
		mut(e)
		return ApprovalStep{Name: "supervisor-check", Permission: "supervisor-check", Escalation: e}
	}

	t.Run("after without reassign", func(t *testing.T) {
		err := ValidateApprovalSteps(wfWithSteps(base(func(e *StepEscalation) { e.Reassign = "" })))
		if err == nil || !strings.Contains(err.Error(), "reassign") {
			t.Fatalf("expected the missing-reassign rule, got %v", err)
		}
	})

	t.Run("reassign without after", func(t *testing.T) {
		err := ValidateApprovalSteps(wfWithSteps(base(func(e *StepEscalation) { e.After = "" })))
		if err == nil || !strings.Contains(err.Error(), "without `after`") {
			t.Fatalf("expected the missing-after rule, got %v", err)
		}
	})

	t.Run("reassign to the step's own duty changes nothing", func(t *testing.T) {
		step := base(func(e *StepEscalation) { e.Reassign = "supervisor-check" })
		err := ValidateApprovalSteps(wfWithSteps(step))
		if err == nil || !strings.Contains(err.Error(), "own duty") {
			t.Fatalf("expected the self-reference rule, got %v", err)
		}
	})

	t.Run("reassign name must be usable in a permission", func(t *testing.T) {
		err := ValidateApprovalSteps(wfWithSteps(base(func(e *StepEscalation) { e.Reassign = "Manager Check" })))
		if err == nil || !strings.Contains(err.Error(), "not a usable duty name") {
			t.Fatalf("expected the name-shape rule, got %v", err)
		}
	})
}

// TestEscalationPermission_IsQualifiedLikeAStepDuty pins that the takeover duty
// is derived by the same rule as a step's, so a grant points at it the same way.
func TestEscalationPermission_IsQualifiedLikeAStepDuty(t *testing.T) {
	esc := &StepEscalation{After: "4h", Reassign: "manager-check"}
	if got := EscalationPermission("cafe-order", "order.void-order", esc); got != "workflow.cafe-order.order.void-order.manager-check" {
		t.Fatalf("escalation duty = %q", got)
	}
	// A fully qualified value is taken as written.
	full := &StepEscalation{After: "4h", Reassign: "acme.approvals.head"}
	if got := EscalationPermission("cafe-order", "order.void-order", full); got != "acme.approvals.head" {
		t.Fatalf("explicit escalation duty should pass through, got %q", got)
	}
	if got := EscalationPermission("x", "y", nil); got != "" {
		t.Fatalf("no escalation has no duty, got %q", got)
	}
}

// TestApprovalDuties_CoversStepsAndEscalation pins the grant surface: both a
// step's duty and its escalation target are grantable by name.
func TestApprovalDuties_CoversStepsAndEscalation(t *testing.T) {
	a := &ApprovalSpec{
		Steps: []ApprovalStep{
			{Name: "supervisor-check", Permission: "supervisor-check",
				Escalation: &StepEscalation{After: "4h", Reassign: "manager-check"}},
			{Name: "role-only", Roles: []string{"x"}},
		},
	}
	duties := ApprovalDuties("cafe-order", "order.void-order", a)
	if len(duties) != 2 {
		t.Fatalf("duties = %+v, want the step duty and the escalation duty", duties)
	}
	want := map[string]string{
		"supervisor-check": "workflow.cafe-order.order.void-order.supervisor-check",
		"manager-check":    "workflow.cafe-order.order.void-order.manager-check",
	}
	for _, d := range duties {
		if want[d.Name] != d.Permission {
			t.Errorf("duty %q = %q, want %q", d.Name, d.Permission, want[d.Name])
		}
	}
}

// TestStepPermission_StaysOutOfReach is the security property of the derived
// duty, stated as a test rather than as a comment: the string carries the
// `workflow.` namespace and is deeper than an entity permission, so it is not
// covered by a module wildcard like `cafe-order.*`.
func TestStepPermission_StaysOutOfReach(t *testing.T) {
	step := ApprovalStep{Name: "supervisor-check", Permission: "supervisor-check"}
	got := StepPermission("cafe-order", "order.void-order", step)

	if !strings.HasPrefix(got, "workflow.cafe-order.") {
		t.Fatalf("%q must live under `workflow.{module}.`, so a module wildcard cannot reach it", got)
	}
	// workflow . module . entity . transition . step — five segments, one deeper
	// than an entity permission ({module}.{plural}.{action}).
	if n := strings.Count(got, ".") + 1; n < 5 {
		t.Errorf("%q has %d segments, want at least 5", got, n)
	}
}
