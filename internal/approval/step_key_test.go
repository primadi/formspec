package approval

import (
	"encoding/json"
	"testing"

	"github.com/primadi/formspec/pkg/spec"
)

// twoNamedSteps is the shape whose HISTORY used to be keyed by position.
func twoNamedSteps() []spec.ApprovalStep {
	return []spec.ApprovalStep{
		{Name: "first-check", Roles: []string{"gl.clerk"}},
		{Name: "second-check", Roles: []string{"gl.manager"}},
	}
}

// TestStepKey_PrefersNameAndReservesTheIndexForm pins the two spaces apart, which
// is what lets a legacy numeric key and a name key live in one map without
// ambiguity.
func TestStepKey_PrefersNameAndReservesTheIndexForm(t *testing.T) {
	steps := twoNamedSteps()
	if got := StepKey(steps, 0); got != "first-check" {
		t.Errorf("named step: got %q", got)
	}
	// An unnamed step falls back to the reserved `#{index}` form. `#` cannot
	// start a valid step name (`[a-z]…`), so this key can never collide with a
	// real name, and it cannot be mistaken for a legacy numeric index either.
	unnamed := []spec.ApprovalStep{{Roles: []string{"x"}}}
	if got := StepKey(unnamed, 0); got != "#0" {
		t.Errorf("unnamed step: got %q", got)
	}
	if got := StepKey(steps, 99); got != "#99" {
		t.Errorf("out-of-range index must still yield a stable key, got %q", got)
	}
}

// TestApprovals_LegacyNumericKeysStillRead is the compatibility guarantee: rows
// written before keys were names carry `{"0": [...]}`, and JSON object keys are
// strings either way, so they land as key "0". Refusing to read them would
// silently discard recorded signatures — turning "two people already signed"
// into "nobody has", which the next approver would have to redo.
func TestApprovals_LegacyNumericKeysStillRead(t *testing.T) {
	steps := twoNamedSteps()
	a := pendingFor(&spec.ApprovalSpec{Steps: steps}, "gl", "requester-1")

	// Exactly the JSON a pre-change row holds.
	var back Approval
	if err := json.Unmarshal([]byte(`{"approvals":{"0":["u1"],"1":["u2"]}}`), &back); err != nil {
		t.Fatalf("unmarshal legacy row: %v", err)
	}
	a.Approvals = back.Approvals

	if got := a.approvalsFor(steps, 0); len(got) != 1 || got[0] != "u1" {
		t.Fatalf("step 0 must read the legacy bucket, got %v", got)
	}
	if got := a.approvalsFor(steps, 1); len(got) != 1 || got[0] != "u2" {
		t.Fatalf("step 1 must read the legacy bucket, got %v", got)
	}
	// A step the legacy row says nothing about is simply unapproved.
	if got := a.approvalsFor(steps, 99); got != nil {
		t.Fatalf("unknown step: got %v", got)
	}
}

// TestApprove_MigratesLegacyBucketWithoutLosingSignatures covers the trap in a
// dual-read design: if a write started a fresh name-keyed bucket while the
// legacy one still held signatures, `approvalsFor` (name first) would return only
// the new bucket and the old approvals would vanish from the count.
func TestApprove_MigratesLegacyBucketWithoutLosingSignatures(t *testing.T) {
	steps := twoNamedSteps()
	wf := &spec.ApprovalSpec{Steps: steps}

	a := pendingFor(wf, "gl", "requester-1")
	a.Approvals = map[string][]string{"0": {"u1"}} // legacy: step 0 signed by u1

	if err := a.Approve(steps, "u2"); err != nil {
		t.Fatalf("Approve: %v", err)
	}

	got := a.approvalsFor(steps, 0)
	if len(got) != 2 {
		t.Fatalf("the legacy signature must survive the write, got %v", got)
	}
	if _, stale := a.Approvals["0"]; stale {
		t.Error("the legacy bucket should have been migrated onto the step key, not left behind")
	}
	if len(a.Approvals["first-check"]) != 2 {
		t.Errorf("both signatures belong under the step name, got %v", a.Approvals)
	}

	// And the duplicate guard still works across the migration: u1 already
	// signed this step, whichever key its signature arrived under.
	if err := a.Approve(steps, "u1"); err == nil {
		t.Error("a second signature from the same user must be refused")
	}
}

// TestApprovals_NamesMakeHistorySurviveReordering is the reason the key is a name
// at all: inserting a step must not re-point yesterday's signatures.
func TestApprovals_NamesMakeHistorySurviveReordering(t *testing.T) {
	before := twoNamedSteps()
	wfBefore := &spec.ApprovalSpec{Steps: before}

	a := pendingFor(wfBefore, "gl", "requester-1")
	if err := a.Approve(before, "u1"); err != nil {
		t.Fatalf("Approve: %v", err)
	}

	// The manifest gains a step in front, and the approval row is unchanged.
	after := append([]spec.ApprovalStep{{Name: "pre-check", Roles: []string{"gl.head"}}}, before...)

	// `first-check` is now at index 1, but its signature is still found by name.
	if got := a.approvalsFor(after, 1); len(got) != 1 || got[0] != "u1" {
		t.Fatalf("history must follow the step, not the position, got %v", got)
	}
	// The newly inserted step has no history — it must not inherit one merely
	// because index 0 used to mean `first-check`.
	if got := a.approvalsFor(after, 0); got != nil {
		t.Fatalf("the new step must start unapproved, got %v", got)
	}
}

// TestReject_EscalationAndKeysAgree ties the two maps together: escalation is
// written through MarkEscalated and read by CanApprove, so both must name the
// same step.
func TestReject_EscalationAndKeysAgree(t *testing.T) {
	steps := twoNamedSteps()
	wf := &spec.ApprovalSpec{Steps: steps}

	a := pendingFor(wf, "gl", "requester-1")
	a.ActiveStepName = "second-check" // waiting on the second step
	a.MarkEscalated(steps, 1, []string{"gl.head"})

	if _, ok := a.EscalatedSteps["second-check"]; !ok {
		t.Fatalf("escalation must be keyed by the step name, got %v", a.EscalatedSteps)
	}
	// Eligibility sees the escalated role for THAT step…
	head := Approver{UserID: "h", Roles: []string{"gl.head"}}
	if ok, reason := e().CanApprove(a, steps, head); !ok {
		t.Fatalf("the escalated role must be eligible, got: %s", reason)
	}
	// …and not for the other one.
	a.ActiveStepName = "first-check"
	if ok, _ := e().CanApprove(a, steps, head); ok {
		t.Error("an escalation recorded for step 2 must not widen step 1")
	}
}

// e is a tiny helper so the test above reads as one sentence.
func e() *Engine { return NewEngine(NewRegistry()) }
