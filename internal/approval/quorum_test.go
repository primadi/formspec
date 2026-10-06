package approval

import (
	"strings"
	"testing"

	"github.com/primadi/formspec/pkg/spec"
)

// TestQuorum_ComesFromTheManifest is the rule that replaced the fiction.
//
// The quorum used to be `len(step.Roles)` for `mode: all` — a count of role
// NAMES treated as if it were a count of PEOPLE. With two roles held by one
// person it promised two signatures from somebody who can supply only one
// (a duplicate approval for the same step is refused), leaving a step that could
// never be satisfied. The number now comes from what the author declared.
func TestQuorum_ComesFromTheManifest(t *testing.T) {
	cases := []struct {
		name string
		step spec.ApprovalStep
		want int
	}{
		{"absent mode, absent approvers → 1", spec.ApprovalStep{Roles: []string{"a"}}, 1},
		{"absent mode honours approvers", spec.ApprovalStep{Roles: []string{"a"}, Approvers: 3}, 3},
		{"any honours approvers", spec.ApprovalStep{Mode: spec.StepModeAny, Roles: []string{"a"}, Approvers: 2}, 2},
		{"any with no approvers → 1", spec.ApprovalStep{Mode: spec.StepModeAny, Roles: []string{"a"}}, 1},
		// sequential: one signature per link of the chain
		{"sequential counts the chain", spec.ApprovalStep{Mode: spec.StepModeSequential, Roles: []string{"a", "b", "c"}}, 3},
		{"sequential of one", spec.ApprovalStep{Mode: spec.StepModeSequential, Roles: []string{"a"}}, 1},
		// The role COUNT must not leak into the other modes: two roles with
		// `mode: any` is a pool of two, not a quorum of two.
		{"any does not count roles", spec.ApprovalStep{Mode: spec.StepModeAny, Roles: []string{"a", "b", "c", "d"}}, 1},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := Quorum(c.step); got != c.want {
				t.Fatalf("Quorum = %d, want %d", got, c.want)
			}
		})
	}
}

// TestStepMode_AbsentMeansTheNumberDecides pins the default. It used to resolve
// to "all" — a promise the runtime could not compute.
func TestStepMode_AbsentMeansTheNumberDecides(t *testing.T) {
	if got := StepMode(spec.ApprovalStep{}); got != string(spec.StepModeAny) {
		t.Fatalf("absent mode = %q, want %q", got, spec.StepModeAny)
	}
	if got := StepMode(spec.ApprovalStep{Mode: spec.StepModeSequential}); got != string(spec.StepModeSequential) {
		t.Fatalf("explicit mode must be honoured, got %q", got)
	}
}

// sequentialChain is a two-link chain, so the ordering rule has something to
// order.
func sequentialChain() *spec.ApprovalSpec {
	return &spec.ApprovalSpec{
		Steps: []spec.ApprovalStep{{
			Name: "chain", Mode: spec.StepModeSequential, Roles: []string{"gl.clerk", "gl.manager"},
		}},
	}
}

// TestCanApprove_SequentialEnforcesTheOrder is the behaviour the spec promised
// and the engine did not implement: `mode: sequential` set the quorum to 1 and
// let the first eligible role member sign — making it indistinguishable from
// `any` while the documentation described a chain.
func TestCanApprove_SequentialEnforcesTheOrder(t *testing.T) {
	e := NewEngine(NewRegistry())
	wf := sequentialChain()
	a := pendingFor(wf, "gl", "requester-1")

	// Link 1 is up: the clerk may sign, the manager may not yet.
	manager := Approver{UserID: "mgr-1", Roles: []string{"gl.manager"}}
	if ok, reason := e.CanApprove(a, wf.Steps, manager); ok {
		t.Fatal("the second link must not sign before the first")
	} else if !strings.Contains(reason, "gl.clerk") {
		t.Errorf("the refusal should name the role that is up, got %q", reason)
	}

	clerk := Approver{UserID: "clerk-1", Roles: []string{"gl.clerk"}}
	if ok, reason := e.CanApprove(a, wf.Steps, clerk); !ok {
		t.Fatalf("the first link must be able to sign, got: %s", reason)
	}
	if err := a.Approve(wf.Steps, "clerk-1"); err != nil {
		t.Fatalf("Approve: %v", err)
	}

	// Link 2 is up now: the clerk is spent, the manager's turn.
	if ok, _ := e.CanApprove(a, wf.Steps, clerk); ok {
		t.Error("the chain must not accept a second signature from the same link")
	}
	if ok, reason := e.CanApprove(a, wf.Steps, manager); !ok {
		t.Fatalf("the second link must be able to sign now, got: %s", reason)
	}
	if err := a.Approve(wf.Steps, "mgr-1"); err != nil {
		t.Fatalf("Approve: %v", err)
	}
	if !a.StepApproved(wf.Steps, 0) {
		t.Fatal("both links signed, so the step must be approved")
	}
}

// TestCanApprove_SequentialEscalationUnblocksTheChain keeps 7.4.4 working on a
// chain: reassignment exists to unblock a stalled step, so an escalated role may
// take the link that is up without waiting for the role the manifest named.
func TestCanApprove_SequentialEscalationUnblocksTheChain(t *testing.T) {
	e := NewEngine(NewRegistry())
	wf := sequentialChain()
	a := pendingFor(wf, "gl", "requester-1")
	a.MarkEscalated(wf.Steps, 0, []string{"gl.head"})

	head := Approver{UserID: "head-1", Roles: []string{"gl.head"}}
	if ok, reason := e.CanApprove(a, wf.Steps, head); !ok {
		t.Fatalf("an escalated role must be able to take the current link, got: %s", reason)
	}
}

// TestCanApprove_SequentialIgnoresDuty is defense in depth for a combination the
// validator refuses: `mode: sequential` with `permission`. The engine does not
// let a duty holder take a turn that belongs to another role, because doing so
// would let one person consume a turn meant for someone else — and the reason the
// combination is refused (rather than given a rule) is that neither answer is
// obviously right: a chain is ordered by roles, a duty has no position in it.
func TestCanApprove_SequentialIgnoresDuty(t *testing.T) {
	e := NewEngine(NewRegistry())
	wf := sequentialChain()
	wf.Steps[0].Permission = "approve-chain" // invalid per validate; tested for behaviour
	a := pendingFor(wf, "gl", "requester-1")
	a.GateName = "wf"

	duty := spec.StepPermission("gl", "wf", wf.Steps[0])
	// Holding the duty is not a way into the chain: the due role still decides.
	if ok, reason := e.CanApprove(a, wf.Steps, permissionSet("u2", duty)); ok {
		t.Errorf("a duty must not open a turn that belongs to another role; reason was %q", reason)
	}
	// The link's own role works, as always.
	if ok, reason := e.CanApprove(a, wf.Steps, Approver{UserID: "u1", Roles: []string{"gl.clerk"}}); !ok {
		t.Fatalf("the link's role must be able to sign, got: %s", reason)
	}
}

// TestCanApprove_ResolvesByStepNameNotPosition is the regression for the third
// instance of one defect: the approve path took the step from `ApplicableSteps`
// (the list in force) but asked `CanApprove` about the stored INDEX against
// `wf.Steps` (the authored list). One skipped step makes the two lists disagree
// on every index, so eligibility was decided for a step that was not awaiting
// approval — and with `when` that can happen without anyone editing anything.
//
// The fix is that a step's IDENTITY is its name: both lists resolve the same
// approval to the same step and the same history bucket, whatever position that
// step happens to occupy in each.
func TestCanApprove_ResolvesByStepNameNotPosition(t *testing.T) {
	e := NewEngine(NewRegistry())
	// Authored: step 0 requires `nobody` (skipped in the applicable list);
	// step 1 requires `clerk`.
	wf := &spec.ApprovalSpec{
		Steps: []spec.ApprovalStep{
			{Name: "skipped", Roles: []string{"gl.nobody"}},
			{Name: "real", Roles: []string{"gl.clerk"}},
		},
	}
	a := pendingFor(wf, "gl", "requester-1")
	a.ActiveStep = 1 // as written when the applicable list was [real]
	a.ActiveStepName = "real"

	applicable := []spec.ApprovalStep{wf.Steps[1]}

	// The name resolves to each list's OWN position for that step…
	if got := a.ActiveIndex(wf.Steps); got != 1 {
		t.Errorf("in the authored list `real` is at 1, got %d", got)
	}
	if got := a.ActiveIndex(applicable); got != 0 {
		t.Errorf("in the applicable list `real` is at 0, got %d", got)
	}
	// …and to the SAME history bucket, which is the property that makes an
	// in-flight approval survive a manifest edit (or a `when` that flips).
	if StepKey(wf.Steps, 1) != StepKey(applicable, 0) {
		t.Errorf("the same step must have one key: %q vs %q",
			StepKey(wf.Steps, 1), StepKey(applicable, 0))
	}

	// Eligibility follows the same resolution, so both lists agree about who may
	// sign — they cannot disagree, because neither answer depends on position.
	clerk := Approver{UserID: "clerk-1", Roles: []string{"gl.clerk"}}
	if ok, reason := e.CanApprove(a, applicable, clerk); !ok {
		t.Fatalf("the applicable list decides who may sign, got: %s", reason)
	}
	if ok, reason := e.CanApprove(a, wf.Steps, clerk); !ok {
		t.Fatalf("the authored list must resolve to the same step, got: %s", reason)
	}
	// The skipped step's role still decides nothing: `gl.nobody` is not eligible
	// for `real`, whichever list is passed.
	if ok, _ := e.CanApprove(a, applicable, Approver{UserID: "x", Roles: []string{"gl.nobody"}}); ok {
		t.Error("a role that belongs to another step must not be eligible here")
	}
}
