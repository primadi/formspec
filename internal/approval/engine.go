package approval

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/primadi/formspec/internal/starlark"
	"github.com/primadi/formspec/pkg/spec"
)

// Engine evaluates workflows for state-machine transitions and checks
// approver eligibility (02-core-extended.md §2).
type Engine struct {
	reg *Registry
}

// NewEngine creates a workflow engine bound to the given registry.
func NewEngine(reg *Registry) *Engine {
	return &Engine{reg: reg}
}

// RequiresApproval reports whether the given transition is gated by an approval
// chain. entity is "module.entity" (e.g. "gl.journal-entry"); transition is the
// state machine's `via` name for the transition being executed.
//
// transition is required rather than derived from (from,to): several different
// transitions can share the same target state, so the state pair alone cannot
// identify it. Since the approval is declared ON the transition, the name is the
// only reference there is.
func (e *Engine) RequiresApproval(entity, transition string) bool {
	if e.reg == nil {
		return false
	}
	return len(e.reg.ForTransition(entity, transition)) > 0
}

// ApprovalsFor returns the approval gates holding the given transition.
func (e *Engine) ApprovalsFor(entity, transition string) []*spec.ApprovalSpec {
	if e.reg == nil {
		return nil
	}
	return e.reg.ForTransition(entity, transition)
}

// ApplicableSteps returns the steps of a workflow that apply to the given
// resource data — steps whose `when` condition (FormSpecExpr over `resource`)
// evaluates true are included; steps with a false `when` are skipped without
// holding the transition (02-core-extended.md §2.1).
func (e *Engine) ApplicableSteps(wf *spec.ApprovalSpec, resourceData map[string]any) ([]spec.ApprovalStep, error) {
	if wf == nil {
		return nil, nil
	}
	steps := make([]spec.ApprovalStep, 0, len(wf.Steps))
	for _, step := range wf.Steps {
		if step.When == "" {
			steps = append(steps, step)
			continue
		}
		passed, _, err := starlark.EvaluateGuard(step.When, resourceData)
		if err != nil {
			return nil, fmt.Errorf("workflow step when: %w", err)
		}
		if passed {
			steps = append(steps, step)
		}
	}
	return steps, nil
}

// StepMode returns the effective mode of a step.
//
// An ABSENT mode resolves to "any" — "the declared number decides". It used to
// resolve to "all", which promised something the runtime could not compute: with
// no user directory in hand, "every eligible approver" is not a number a
// manifest implies (see Quorum). Every manifest that omits `mode` has
// `approvers` (default 1), so the effective quorum is unchanged; what changes is
// that the value it reads is one the author actually declared.
func StepMode(step spec.ApprovalStep) string {
	if step.Mode == "" {
		return string(spec.StepModeAny)
	}
	return string(step.Mode)
}

// Quorum returns the number of approvals required for a step.
//
// The number comes from the MANIFEST, never from a guess about who is eligible:
//
//   - "any" (and an absent mode) — `approvers`, default 1;
//   - "sequential" — one approval per role, because the role list IS the chain
//     and each link contributes exactly one signature.
//
// It used to return `len(step.Roles)` for "all", which counted role NAMES as if
// they were people: two roles held by one person promised a quorum of two, which
// that person could never supply (Approve rejects a duplicate for the same step)
// and which stopped meaning anything at all once a step could be gated by a
// permission nobody can enumerate. `mode: all` is therefore refused by
// `formspec validate` rather than kept as a number that is not true.
func Quorum(step spec.ApprovalStep) int {
	switch StepMode(step) {
	case string(spec.StepModeSequential):
		if len(step.Roles) == 0 {
			return 1
		}
		return len(step.Roles)
	default:
		if step.Approvers <= 0 {
			return 1
		}
		return step.Approvers
	}
}

// Approver is the identity information step eligibility needs.
//
// It is a small value rather than the auth.Identity type so this package stays
// independent of the auth package (the same reason ApprovalRow mirrors the
// persistence row instead of importing the renderer).
type Approver struct {
	UserID string
	Roles  []string
	// Can is the permission predicate (`*auth.Identity.HasPermission`). nil means
	// "this caller holds no permissions", which makes a duty-based step
	// unapprovable for them — fail closed, matching the runtime.
	Can func(permission string) bool
}

// holds reports whether the approver has the given permission.
func (a Approver) holds(permission string) bool {
	return permission != "" && a.Can != nil && a.Can(permission)
}

// CanApprove checks whether an approver may sign a pending approval. It
// enforces:
//
//   - the step is resolvable in the APPLICABLE step list `a.ActiveStep` indexes;
//   - requester exclusion (7.4.5): the requester can never approve their own
//     request;
//   - step eligibility: the approver holds the step's DUTY permission
//     (`ApprovalStep.permission` — the `resource + action` form AGENTS.md rule 6
//     asks for) OR one of its roles, OR the escalated step's reassignment duty
//     when the step has been escalated;
//   - for `mode: sequential`, that the approver holds the role of the CHAIN LINK
//     that is currently up, not merely any role in the list.
//
// `steps` must be the applicable list (the one `ApplicableSteps` produced and
// `a.ActiveStep` indexes). Passing the authored list instead is how the runtime
// used to read a different step than the one awaiting approval the moment a
// `when` condition skipped one ahead of it — the two lists disagree on every
// index after the first skip. Taking the list as a parameter rather than
// reaching into the spec is what makes the caller state which one it means.
//
// The duty and the roles are alternatives during migration: a workflow that
// declares `permission` becomes grantable without renaming roles, and one that
// still declares only `roles` keeps working unchanged. Once every workflow
// declares a duty the role list is dropped as a hard error — a deprecation that
// is merely announced is one the repo never finishes.
func (e *Engine) CanApprove(a *Approval, steps []spec.ApprovalStep, approver Approver) (bool, string) {
	if a == nil {
		return false, "no approval"
	}
	// Resolve the position the same way every other reader does (name first), so
	// eligibility and the stored history agree about WHICH step is being decided.
	idx := a.ActiveIndex(steps)
	if idx < 0 || idx >= len(steps) {
		return false, "workflow step out of range"
	}
	step := steps[idx]

	// Requester exclusion (7.4.5): the requester can never approve their own
	// request. requesterID is the record's created_by.
	if a.RequesterID != "" && approver.UserID != "" && approver.UserID == a.RequesterID {
		return false, "requester cannot approve their own request"
	}

	escalated := a.escalatedFor(steps, idx)

	// Sequential chains: the role list is an ORDER, so membership in it is not
	// enough — the link that is up is the one after however many signatures the
	// step has already collected. Without this, `mode: sequential` behaved
	// exactly like `any` while the spec promised "the next approver can only act
	// after the previous one".
	if StepMode(step) == string(spec.StepModeSequential) && len(step.Roles) > 0 {
		chainPos := len(a.approvalsFor(steps, idx))
		if chainPos >= len(step.Roles) {
			return false, "the step's approval chain is complete"
		}
		dueRole := step.Roles[chainPos]
		// Who may take THIS turn: the role named for it, or the escalated
		// reassignment — reassignment exists precisely to unblock a stalled link.
		// The step's duty is NOT accepted here: `mode: sequential` may not declare
		// one at all (validate refuses the combination), because a duty has no
		// position in an order. Accepting it here would let one person consume a
		// turn meant for another role.
		eligibleForTurn := hasAnyRole(approver.Roles, []string{dueRole}) ||
			holdsEscalation(approver, escalated)
		if !eligibleForTurn {
			return false, fmt.Sprintf("the approval chain is waiting on role %q (step %d of %d), which you do not hold",
				dueRole, chainPos+1, len(step.Roles))
		}
		return true, ""
	}

	// Duty permission: the `resource + action` gate. Holding it is sufficient.
	duty := spec.StepPermission(a.GateModule, a.GateName, step)
	if approver.holds(duty) {
		return true, ""
	}

	// Escalated reassignment: after the timeout the targeted DUTY gains approval
	// rights, and reassignment exists precisely to unblock a stalled step.
	if holdsEscalation(approver, escalated) {
		return true, ""
	}

	// Role membership (legacy: a step may still rest on roles alone).
	if hasAnyRole(approver.Roles, step.Roles) {
		return true, ""
	}

	if duty != "" {
		return false, "user holds neither the step's duty permission (" + duty + ") nor any of its roles"
	}
	return false, "user does not hold any of the step's required roles"
}

// hasAnyRole reports whether the user's roles intersect the required roles.
func hasAnyRole(userRoles, required []string) bool {
	for _, r := range userRoles {
		for _, req := range required {
			if r == req {
				return true
			}
		}
	}
	return false
}

// holdsEscalation reports whether any stored escalation value grants this
// approver the right to take over a stalled step.
//
// A value is a DUTY permission in the current form — `CanApprove` is reached with
// an approver that can answer `holds`. Rows written before escalation became a
// duty hold ROLE names instead, and refusing to read them would silently drop the
// reassignment of every approval in flight at deploy time, leaving a step nobody
// can unblock. So both are accepted, the same dual-read the step keys use.
func holdsEscalation(approver Approver, escalated []string) bool {
	for _, v := range escalated {
		if approver.holds(v) {
			return true
		}
		if hasAnyRole(approver.Roles, []string{v}) {
			return true
		}
	}
	return false
}

// ApprovalStatus is the lifecycle state of an approval request.
type ApprovalStatus string

const (
	ApprovalPending  ApprovalStatus = "pending"
	ApprovalApproved ApprovalStatus = "approved"
	ApprovalRejected ApprovalStatus = "rejected"
)

// Approval is the runtime state of one approval request for a record.
type Approval struct {
	ID          string         `json:"id"`
	GateModule  string         `json:"workflow_module"`
	GateName    string         `json:"workflow_name"`
	Entity      string         `json:"entity"` // "module.entity"
	RecordID    string         `json:"record_id"`
	From        string         `json:"from"`
	To          string         `json:"to"`
	RequesterID string         `json:"requester_id"`
	Status      ApprovalStatus `json:"status"`
	// ActiveStep is the 0-based index of the step currently awaiting approval.
	ActiveStep int `json:"active_step"`
	// ActiveStepName names that step for steps that declare a name. Consumers that
	// cannot see the record — the escalation worker — resolve the step through the
	// name, because the index is only meaningful against the SAME step list, and
	// `when` makes that list depend on the record.
	ActiveStepName string `json:"active_step_name,omitempty"`
	// Approvals records who approved which step, keyed by StepKey (the step's
	// NAME — see StepKey for why the name and not the index).
	//
	// Legacy rows keyed by the numeric step index are still read: JSON object
	// keys are strings either way, so `{"0": [...]}` lands as key "0", and
	// `approvalsFor` falls back to it. A step name can never be numeric
	// (`validApprovalStepName` requires a leading lowercase letter) and the
	// index fallback key always carries a `#`, so the two spaces cannot collide.
	Approvals map[string][]string `json:"approvals"`
	// RejectedBy records who rejected (and at which step).
	RejectedBy string `json:"rejected_by,omitempty"`
	RejectStep int    `json:"reject_step,omitempty"`
	// EscalatedSteps records which steps were escalated (7.4.4) and the
	// reassign_roles that gained approval rights, keyed by StepKey.
	EscalatedSteps map[string][]string `json:"escalated_steps,omitempty"`
	// Params carries the input values the REQUESTER supplied for the intercepted
	// transition (plan docs_internal/plan/action-input-contract.md, D5).
	//
	// They have to be stored rather than re-supplied: the requesting call returns
	// 202 before writing anything, so the approver's later call is a DIFFERENT
	// request. Without this the values the requester collected were dropped, and
	// `executeApprovalTransition` would write only the state — leaving a voided
	// order with no reason, or, if the transition guarded on that value, failing
	// a check nobody could satisfy.
	Params map[string]any `json:"params,omitempty"`
}

// NewApproval creates a pending approval for a record.
//
// `gateName` is REQUIRED and must be the workflow's manifest name — the same
// one the registry keys it by. It used to be filled in with the spec's pointer
// address (`%p`) and overwritten later by the caller that knew the name. That was
// survivable while the name was only a label; it is not now that the step's DUTY
// permission is derived from it (`spec.StepPermission`): a placeholder name makes
// every duty unholdable, and the symptom is a silent 403 on the approval. A
// caller cannot forget a parameter it has to pass.
func NewApproval(wf *spec.ApprovalSpec, module, gateName, entity, recordID, from, to, requesterID string) *Approval {
	return &Approval{
		GateModule:     module,
		GateName:       gateName,
		Entity:         entity,
		RecordID:       recordID,
		From:           from,
		To:             to,
		RequesterID:    requesterID,
		Status:         ApprovalPending,
		ActiveStep:     0,
		ActiveStepName: stepNameAt(wf, 0),
		Approvals:      make(map[string][]string),
		EscalatedSteps: make(map[string][]string),
	}
}

// StepKey returns the canonical key under which a step's state is stored:
// its NAME when it declares one, else "#{index}".
//
// The name is the key because the index is not stable across a manifest edit:
// inserting, removing or re-ordering a step moves every index after it, so
// `approvals[0]` recorded yesterday can describe a different step today — the
// same silent re-pointing that `active_step_name` removed for the ACTIVE step,
// and that `when` reintroduces even without an edit, since the applicable list
// is not the authored one. A name is what the manifest author chose as the
// step's permanent identity.
//
// "#{index}" is the fallback for a step that declares no name. The `#` is
// deliberate: step names are validated as `[a-z][a-z0-9-]*`, so no name can look
// like an index, and no legacy numeric key can look like this.
func StepKey(steps []spec.ApprovalStep, idx int) string {
	if idx >= 0 && idx < len(steps) {
		if name := strings.TrimSpace(steps[idx].Name); name != "" {
			return name
		}
	}
	return "#" + strconv.Itoa(idx)
}

// ActiveIndex resolves which position in `steps` this approval is waiting on.
//
// The stored NAME wins, with the stored index as the fallback for rows written
// before the column existed. That order matters because the two can disagree:
// `active_step` indexes the step list IN FORCE (the applicable one), while the
// stored value was written against whatever list existed at the time, so an
// index alone can name a step that is not awaiting a decision.
func (a *Approval) ActiveIndex(steps []spec.ApprovalStep) int {
	if a == nil {
		return 0
	}
	if idx, found := NameForStep(steps, a.ActiveStepName); found {
		return idx
	}
	return a.ActiveStep
}

// approvalsFor returns the approvals recorded for the step at idx, accepting the
// legacy numeric-index key for rows written before keys were names.
func (a *Approval) approvalsFor(steps []spec.ApprovalStep, idx int) []string {
	return lookupStepBucket(a.Approvals, steps, idx)
}

// escalatedFor returns the escalated reassign_roles for the step at idx, with the
// same legacy fallback as approvalsFor.
func (a *Approval) escalatedFor(steps []spec.ApprovalStep, idx int) []string {
	return lookupStepBucket(a.EscalatedSteps, steps, idx)
}

// lookupStepBucket reads one step's bucket out of a StepKey-keyed map, falling
// back to the legacy numeric key.
//
// The fallback exists so an in-flight approval survives the deploy that changed
// the key shape: its history was written as `{"0": [...]}`, and refusing to read
// that would silently discard recorded signatures — turning "two people already
// signed" into "nobody has", which the next approver would have to redo.
func lookupStepBucket(m map[string][]string, steps []spec.ApprovalStep, idx int) []string {
	if len(m) == 0 {
		return nil
	}
	if v, ok := m[StepKey(steps, idx)]; ok {
		return v
	}
	if v, ok := m[strconv.Itoa(idx)]; ok {
		return v
	}
	return nil
}

// mergeLegacyBucket moves a step's legacy numeric-index bucket onto its name key,
// so a write does not split one step's history across two keys (which would make
// the legacy half invisible to every subsequent read).
func (a *Approval) mergeLegacyBucket(m map[string][]string, steps []spec.ApprovalStep, idx int) map[string][]string {
	if m == nil {
		m = make(map[string][]string)
	}
	key := StepKey(steps, idx)
	legacy := strconv.Itoa(idx)
	if _, hasPrimary := m[key]; hasPrimary {
		return m
	}
	if old, ok := m[legacy]; ok && legacy != key {
		m[key] = old
		delete(m, legacy)
	}
	return m
}

// stepNameAt returns the name of the approval's step at index i, or "" when the
// index is out of range or the step declares no name.
func stepNameAt(wf *spec.ApprovalSpec, i int) string {
	if wf == nil || i < 0 || i >= len(wf.Steps) {
		return ""
	}
	return wf.Steps[i].Name
}

// NameForStep resolves a step NAME back to its index in the given step list, and
// reports whether it was found. ok=false means the caller must fall back to the
// index it already has (a step without a name, or a workflow since edited).
func NameForStep(steps []spec.ApprovalStep, name string) (int, bool) {
	if name == "" {
		return 0, false
	}
	for i, s := range steps {
		if s.Name == name {
			return i, true
		}
	}
	return 0, false
}

// StepApproved reports whether the step at idx has reached its quorum.
func (a *Approval) StepApproved(steps []spec.ApprovalStep, idx int) bool {
	return len(a.approvalsFor(steps, idx)) >= Quorum(steps[idx])
}

// AllStepsApproved reports whether every applicable step has been approved.
//
// The active position is resolved the same way every other reader resolves it,
// so "all steps done" cannot disagree with "which step is waiting".
func (a *Approval) AllStepsApproved(steps []spec.ApprovalStep) bool {
	return a.ActiveIndex(steps) >= len(steps)
}

// Approve records an approval by userID for the active step. Returns an error
// if the user already approved this step.
func (a *Approval) Approve(steps []spec.ApprovalStep, userID string) error {
	idx := a.ActiveIndex(steps)
	key := StepKey(steps, idx)
	a.Approvals = a.mergeLegacyBucket(a.Approvals, steps, idx)
	for _, u := range a.Approvals[key] {
		if u == userID {
			return fmt.Errorf("user %s already approved step %q", userID, key)
		}
	}
	a.Approvals[key] = append(a.Approvals[key], userID)
	return nil
}

// Reject records a rejection by userID at the active step.
func (a *Approval) Reject(steps []spec.ApprovalStep, userID string) {
	a.Status = ApprovalRejected
	a.RejectedBy = userID
	// Recorded as the RESOLVED position: the stored index may point elsewhere
	// after a manifest edit, and this value exists to be read back.
	a.RejectStep = a.ActiveIndex(steps)
}

// MarkEscalated records that a step gained extra approvers (7.4.4), keyed the
// same way as approvals.
func (a *Approval) MarkEscalated(steps []spec.ApprovalStep, idx int, roles []string) {
	a.EscalatedSteps = a.mergeLegacyBucket(a.EscalatedSteps, steps, idx)
	a.EscalatedSteps[StepKey(steps, idx)] = roles
}

// Advance moves to the next step (called when the active step reaches quorum).
// Running off the end is normal (the caller tracks the step count).
//
// steps is the APPLICABLE list — the one the caller has been indexing all along
// — so the recorded name matches the recorded index instead of the authored
// array, which `when` may have shortened.
func (a *Approval) Advance(steps []spec.ApprovalStep) {
	a.ActiveStep = a.ActiveIndex(steps) + 1
	a.ActiveStepName = stepNameAt(&spec.ApprovalSpec{Steps: steps}, a.ActiveStep)
}

// ToRow converts an Approval into a persistence row. The row's ID/tenant/
// timestamps are left for the store to manage.
func (a *Approval) ToRow() *ApprovalRow {
	return &ApprovalRow{
		Entity:         a.Entity,
		RecordID:       a.RecordID,
		GateModule:     a.GateModule,
		GateName:       a.GateName,
		FromState:      a.From,
		ToState:        a.To,
		RequesterID:    a.RequesterID,
		Status:         string(a.Status),
		ActiveStep:     a.ActiveStep,
		ActiveStepName: a.ActiveStepName,
		Approvals:      a.Approvals,
		RejectedBy:     a.RejectedBy,
		RejectStep:     a.RejectStep,
		EscalatedSteps: a.EscalatedSteps,
	}
}

// FromRow converts a persistence row back into an Approval.
func FromRow(row *ApprovalRow) *Approval {
	if row == nil {
		return nil
	}
	return &Approval{
		ID:             row.ID,
		GateModule:     row.GateModule,
		GateName:       row.GateName,
		Entity:         row.Entity,
		RecordID:       row.RecordID,
		From:           row.FromState,
		To:             row.ToState,
		RequesterID:    row.RequesterID,
		Status:         ApprovalStatus(row.Status),
		ActiveStep:     row.ActiveStep,
		ActiveStepName: row.ActiveStepName,
		Approvals:      row.Approvals,
		RejectedBy:     row.RejectedBy,
		RejectStep:     row.RejectStep,
		EscalatedSteps: row.EscalatedSteps,
	}
}

// ApprovalRow mirrors db.ApprovalRequestRow without importing the renderer
// package (avoids an internal → renderer dependency for the workflow engine).
type ApprovalRow struct {
	ID          string
	TenantID    string
	Entity      string
	RecordID    string
	GateModule  string
	GateName    string
	FromState   string
	ToState     string
	RequesterID string
	Status      string
	ActiveStep  int
	// ActiveStepName names the active step, for steps that declare one. Empty for
	// rows written before the column existed — consumers fall back to the index.
	ActiveStepName string
	Approvals      map[string][]string
	RejectedBy     string
	RejectStep     int
	EscalatedSteps map[string][]string
	CreatedAt      string
	UpdatedAt      string
}
