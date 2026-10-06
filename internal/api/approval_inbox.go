package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/primadi/formspec/internal/approval"
	"github.com/primadi/formspec/internal/auth"
	entityengine "github.com/primadi/formspec/internal/entity"
	"github.com/primadi/formspec/internal/ui"
	"github.com/primadi/formspec/pkg/spec"
	db "github.com/primadi/formspec/renderers/jsonb-persist"
)

// ─── Approval Inbox surface (kind: ApprovalInbox, frontend/06-page-kinds.md §11) ───
//
// The kind is ZERO-CONFIG: its source is the pending Workflow steps the caller
// may act on, per `docs/spec/frontend/06-page-kinds.md` §11 — not an entity.
// That contract could not be served before, because the rows live in the
// framework table `formspec_workflow_approval`, which is not an Entity and
// therefore had no route at all: the renderer looked for a conventional
// approval entity that does not exist and permanently showed
// "No approval source configured" (todo 5.13.6). These two endpoints are the
// missing source. Plan: docs_internal/plan/approval-inbox-endpoint.md.
//
//	GET  /{ws}/_ui/workflow/approvals?app=<app>  → the caller's pending tasks
//	POST /{ws}/_ui/workflow/approvals/{id}       → {"decision":"approve"|"reject"}
//
// The decision endpoint DELEGATES to handleApproval rather than
// re-implementing the flow: that is where quorum, requester exclusion (7.4.5),
// the signed audit record (7.4.6) and the transition's event emission actually
// live. A second implementation would be a second set of rules.

// approvalInboxField is one value the approver needs in order to decide
// (ApprovalStep.DisplayFields, backend/02-core-extended.md §2.1).
type approvalInboxField struct {
	Field string `json:"field"`
	// Label is the entity field's declared caption; empty when it declares
	// none, so the renderer applies its own caption precedence.
	Label string `json:"label,omitempty"`
	// Type is the entity field's declared type ("money", "datetime", …).
	//
	// It is sent because the renderer cannot infer it: a money value arrives as
	// `{amount, currency}` and `String(value)` printed it as "[object Object]"
	// (measured in the inbox on kafe's Total Amount), while formatting every
	// number as money would mislabel a `decimal` or an `integer`. The type is
	// known here and nowhere else on the wire, so it is decided here — the same
	// reason `cellHintsForField` exists for tables.
	Type  string `json:"type,omitempty"`
	Value any    `json:"value"`
}

// approvalInboxItem is one pending approval task.
//
// `Gate` and `GateModule` keep the wire names `workflow`/`workflow_module`: the
// rename to approval vocabulary is Go-side only, so an already-deployed client
// keeps working.
type approvalInboxItem struct {
	ID            string               `json:"id"`
	Gate          string               `json:"workflow"`
	GateModule    string               `json:"workflow_module"`
	Entity        string               `json:"entity"`
	RecordID      string               `json:"record_id"`
	From          string               `json:"from"`
	To            string               `json:"to"`
	Status        string               `json:"status"`
	ActiveStep    int                  `json:"active_step"`
	TotalSteps    int                  `json:"total_steps"`
	RequesterID   string               `json:"requester_id,omitempty"`
	Title         string               `json:"title,omitempty"`
	Description   string               `json:"description,omitempty"`
	DisplayFields []approvalInboxField `json:"display_fields,omitempty"`
	// CanDecide reports whether the caller may actually run the transition
	// this task approves — i.e. whether it holds the permission that gates the
	// transition's own route. Role eligibility (which decides the LISTING) and
	// the write permission (which decides the ACTION) are different questions,
	// and a task the caller cannot execute is shown as such instead of being
	// offered and then refused with 403.
	CanDecide bool   `json:"can_decide"`
	CreatedAt string `json:"created_at,omitempty"`
}

// HandleApprovalInbox lists the pending approval steps this caller may act
// on, across every entity/module inside the App (zero-config inbox).
func (b *RouterBuilder) HandleApprovalInbox() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		f := b.factory
		if f.approvalRequests == nil || f.approvalReg == nil {
			writeError(w, http.StatusServiceUnavailable, "NOT_CONFIGURED",
				"the workflow approval runtime is not configured")
			return
		}
		identity := IdentityFromContext(r.Context())
		if identity == nil || !identity.IsAuthenticated() {
			writeError(w, http.StatusUnauthorized, "UNAUTHORIZED",
				"authentication required")
			return
		}
		workspaceID := workspaceFromContext(r.Context())

		appCtx, appErr := b.resolveAppContext(r)
		if appErr != "" {
			writeError(w, http.StatusBadRequest, "BAD_REQUEST", appErr)
			return
		}

		rows, err := f.approvalRequests.ListPendingForTenant(r.Context(), workspaceID, approvalInboxLimit)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "WORKFLOW_ERROR", err.Error())
			return
		}

		engine := approval.NewEngine(f.approvalReg)
		items := make([]approvalInboxItem, 0, len(rows))
		for i := range rows {
			item, ok := b.approvalInboxItem(r, identity, appCtx, engine, rows[i])
			if !ok {
				continue
			}
			items = append(items, item)
		}

		writeJSON(w, http.StatusOK, ListResponse{
			Data:  items,
			Meta:  MetaList{Page: 1, PerPage: len(items), Total: len(items), TotalPages: 1},
			Links: ListLinks{},
		})
	}
}

// approvalInboxLimit caps one inbox page. The escalation worker sweeps 100;
// an inbox deeper than this is a backlog the operator has to see in the task
// list rather than page through, and the response is unpaginated on purpose
// (the renderer shows a count, not a pager).
const approvalInboxLimit = 200

// approvalInboxItem projects one pending approval row into a task, or reports
// false when the caller must not see it at all. Four filters apply, in order:
//
//  1. App scope — the row's module must belong to the resolved App, so a POS
//     session is not handed the KDS's approvals (the same narrowing the menu
//     and the bundle get).
//  2. Workflow still resolvable — a workflow removed from the manifest leaves
//     its row behind; its step labels and eligibility are unknowable, so the
//     task is dropped rather than rendered without a subject.
//  3. Eligibility — CanApprove: the predicate the approve call itself runs.
//     It excludes the requester (7.4.5) and non-holders of the step's roles,
//     which is exactly "approval yang boleh ia tindak".
//  4. Field values — DisplayFields is filled only when the caller may read the
//     entity (`{module}.{plural}.view`). The store read here bypasses the
//     HTTP permission gate, so the values must be gated explicitly; the task
//     itself stays visible (it is the caller's own queue).
func (b *RouterBuilder) approvalInboxItem(
	r *http.Request, identity *auth.Identity, appCtx ui.AppContext,
	engine *approval.Engine, row db.ApprovalRequestRow,
) (approvalInboxItem, bool) {
	f := b.factory
	module, entity := splitEntityRef(row.Entity)
	if module == "" || entity == "" {
		return approvalInboxItem{}, false
	}
	// (1) App scope. An App that resolves but declares no modules (a workspace
	// with no kind: App, or a hand-built context) does not narrow anything —
	// silently returning an empty inbox would be worse than the missing filter.
	if len(appCtx.Modules) > 0 && !appAllowsModule(appCtx, module) {
		return approvalInboxItem{}, false
	}
	// (2) Workflow resolvable.
	wf, ok := f.approvalReg.Get(row.GateModule, row.GateName)
	if !ok || wf == nil {
		return approvalInboxItem{}, false
	}

	// The record the approval is about. It is needed twice: `when` conditions
	// are evaluated against it (ApplicableSteps), and its values fill
	// display_fields. A missing record means the task can never be completed
	// (executeApprovalTransition updates that row), so it is dropped.
	var record *db.EntityRecord
	if store, err := b.registry.GetEntityStore(module, entity); err == nil {
		if rec, getErr := store.GetByID(r.Context(), db.GetByIDParams{
			WorkspaceID: workspaceFromContext(r.Context()),
			ID:          row.RecordID,
		}); getErr == nil && rec != nil {
			record = rec
		}
	}
	if record == nil {
		return approvalInboxItem{}, false
	}

	// (3) Eligibility. Steps are resolved the way the approve path resolves
	// them so the labels match the steps that actually gate the quorum.
	steps, err := engine.ApplicableSteps(wf, record.Data)
	if err != nil || len(steps) == 0 {
		steps = wf.Steps
	}
	// Name first, against the list in force — the row's index was taken from the
	// APPLICABLE list, which `when` can shorten.
	stepIdx := activeStepIndex(steps, row)
	if stepIdx < 0 || stepIdx >= len(steps) {
		return approvalInboxItem{}, false
	}
	step := steps[stepIdx]
	// Eligibility is evaluated against the same step the task will label itself
	// with, so `can_decide` cannot describe a different step than the one shown.
	eligibilityRow := row
	eligibilityRow.ActiveStep = stepIdx

	eligible, _ := engine.CanApprove(workflowApprovalFromRow(&eligibilityRow), steps,
		approval.Approver{
			UserID: identity.UserID,
			Roles:  identity.Roles,
			Can:    identity.HasPermission,
		})
	if !eligible {
		return approvalInboxItem{}, false
	}

	item := approvalInboxItem{
		ID:          row.ID,
		Gate:        row.GateName,
		GateModule:  row.GateModule,
		Entity:      row.Entity,
		RecordID:    row.RecordID,
		From:        row.FromState,
		To:          row.ToState,
		Status:      row.Status,
		ActiveStep:  row.ActiveStep,
		TotalSteps:  len(steps),
		RequesterID: row.RequesterID,
		Title:       step.Title,
		Description: step.Description,
		CanDecide:   b.canRunTransition(identity, module, entity, wf, row),
		CreatedAt:   row.CreatedAt,
	}
	if item.Title == "" {
		item.Title = inboxFallbackTitle(row, wf)
	}

	// (4) Display values, permission-gated.
	if plural, ok := b.entityPlural(module, entity); ok {
		if identity.HasPermission(entityPermission(module, plural, "view")) {
			es, _ := b.entitySpec(module, entity)
			item.DisplayFields = approvalDisplayFields(es, record, row, step.DisplayFields)
		}
	}
	return item, true
}

// approvalDisplayFields projects the step's declared display_fields off the
// record, falling back to the values the REQUESTER supplied for the intercepted
// transition.
//
// The fallback is not a nicety. An intercepted transition writes NOTHING until
// the approval completes — that is the whole point of interception — so a field
// the requester filled in (kafe's `void_reason`) is empty on the record and
// lives only on the approval row (`params`, carried across the boundary per the
// action-input contract). Reading the record alone rendered exactly the field
// the approver needs to decide as empty, which `02-core-extended.md` §2.1 names
// as the reason `display_fields` exists.
//
// A field absent from both is still reported, with a nil value: "declared but
// empty" and "not declared" are different answers to the approver's question
// "what am I looking at".
func approvalDisplayFields(es *spec.EntitySpec, record *db.EntityRecord, row db.ApprovalRequestRow, fields []string) []approvalInboxField {
	if len(fields) == 0 {
		return nil
	}
	out := make([]approvalInboxField, 0, len(fields))
	for _, name := range fields {
		var value any
		if record != nil {
			value = record.Data[name]
		}
		if value == nil {
			if supplied, ok := row.Params[name]; ok && supplied != nil {
				value = supplied
			}
		}
		item := approvalInboxField{Field: name, Value: value}
		// Label + type come from the entity's own declaration, so the renderer
		// formats the value the way the rest of the UI does.
		if es != nil {
			for i := range es.Fields {
				if es.Fields[i].Name != name {
					continue
				}
				item.Label = es.Fields[i].Title
				item.Type = string(es.Fields[i].Type)
				break
			}
		}
		out = append(out, item)
	}
	return out
}

// HandleApprovalDecision records an approve/reject decision for one
// pending task.
//
// The caller must hold the permission that gates the transition's own route —
// the inbox must never be a way to run a transition the caller cannot run from
// the record's own page. Eligibility (the step's roles, requester exclusion) is
// then enforced by handleApproval, which also applies quorum, the audit
// record, and the transition's event emission.
func (b *RouterBuilder) HandleApprovalDecision() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		f := b.factory
		if f.approvalRequests == nil || f.approvalReg == nil {
			writeError(w, http.StatusServiceUnavailable, "NOT_CONFIGURED",
				"the workflow approval runtime is not configured")
			return
		}
		identity := IdentityFromContext(r.Context())
		if identity == nil || !identity.IsAuthenticated() {
			writeError(w, http.StatusUnauthorized, "UNAUTHORIZED",
				"authentication required")
			return
		}
		workspaceID := workspaceFromContext(r.Context())
		id := r.PathValue("id")
		if id == "" {
			writeError(w, http.StatusBadRequest, "BAD_REQUEST", "missing approval id")
			return
		}

		var body struct {
			Decision string `json:"decision"`
		}
		if r.Body != nil && r.ContentLength > 0 {
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
				writeError(w, http.StatusUnprocessableEntity, "VALIDATION_ERROR",
					"invalid JSON: "+err.Error())
				return
			}
		}
		decision := strings.TrimSpace(body.Decision)
		if decision != "approve" && decision != "reject" {
			writeError(w, http.StatusUnprocessableEntity, "VALIDATION_ERROR",
				`decision must be "approve" or "reject"`)
			return
		}

		// The row is located by id WITHIN the workspace: an id from another
		// workspace is indistinguishable from a missing one (anti-enumeration,
		// and tenant isolation by construction rather than by convention).
		row, err := f.approvalRequests.GetPendingByID(r.Context(), workspaceID, id)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "WORKFLOW_ERROR", err.Error())
			return
		}
		if row == nil {
			writeError(w, http.StatusNotFound, "NOT_FOUND", "approval not found")
			return
		}

		module, entity := splitEntityRef(row.Entity)
		wf, ok := f.approvalReg.Get(row.GateModule, row.GateName)
		if !ok || wf == nil {
			writeError(w, http.StatusNotFound, "NOT_FOUND",
				fmt.Sprintf("workflow %s/%s is no longer registered", row.GateModule, row.GateName))
			return
		}

		if appCtx, appErr := b.resolveAppContext(r); appErr != "" {
			writeError(w, http.StatusBadRequest, "BAD_REQUEST", appErr)
			return
		} else if len(appCtx.Modules) > 0 && !appAllowsModule(appCtx, module) {
			writeError(w, http.StatusNotFound, "NOT_FOUND", "approval not found")
			return
		}

		if perm := b.canDecidePermission(module, entity, transitionNameOf(*row), row.FromState, row.ToState); perm != "" &&
			!identity.HasPermission(perm) {
			writeError(w, http.StatusForbidden, "FORBIDDEN",
				"you do not hold "+perm+", which gates this transition")
			return
		}

		store, err := b.registry.GetEntityStore(module, entity)
		if err != nil {
			writeError(w, http.StatusNotFound, "NOT_FOUND",
				fmt.Sprintf("entity %q not found", row.Entity))
			return
		}
		rec, err := store.GetByID(r.Context(), db.GetByIDParams{
			WorkspaceID: workspaceID,
			ID:          row.RecordID,
		})
		if err != nil || rec == nil {
			writeError(w, http.StatusNotFound, "NOT_FOUND", "approved record not found")
			return
		}

		f.handleApproval(w, r, r.Context(), module, entity, row.RecordID,
			transitionNameOf(*row), row.FromState, row.ToState,
			rec.Data, rec.Version, identity.UserID, workspaceID,
			map[string]any{"decision": decision}, approval.NewEngine(f.approvalReg))
	}
}

// canRunTransition reports whether the caller may RUN the transition a task
// approves — the same permission the record's own page enforces.
//
// It is deliberately NOT a second eligibility check: eligibility (the step's
// duty or roles, requester exclusion) already filtered this task into the list
// via `Engine.CanApprove`, and the decision endpoint re-enforces it through
// `handleApproval`. Asking the same question twice here would be a
// second place for the two answers to drift — which is exactly what happened
// when an earlier version ANDed a duty check in, refusing callers the record's
// own page would have accepted.
func (b *RouterBuilder) canRunTransition(identity *auth.Identity, module, entity string, wf *spec.ApprovalSpec, row db.ApprovalRequestRow) bool {
	perm := b.canDecidePermission(module, entity, transitionNameOf(row), row.FromState, row.ToState)
	return perm == "" || identity.HasPermission(perm)
}

// activeStepIndex resolves which step a row is waiting on, within `steps` — the
// APPLICABLE list.
//
// The stored NAME wins over the stored index, because the index is only
// meaningful against the list it was taken from: `when` is evaluated against the
// record, so a list that was one step shorter when the row was written (or one
// longer now) makes the same index name a different step. The name is the stable
// reference; the index is the fallback for rows written before the column
// existed.
func activeStepIndex(steps []spec.ApprovalStep, row db.ApprovalRequestRow) int {
	if idx, found := approval.NameForStep(steps, row.ActiveStepName); found {
		return idx
	}
	return row.ActiveStep
}

// appAllowsModule mirrors ui.AppContext's own rule (which is unexported): the
// framework's own modules always ship, because the auth screens and access
// management live in them and every App needs them.
func appAllowsModule(appCtx ui.AppContext, module string) bool {
	if appCtx.Modules == nil || module == "core" || module == "formspec.core" {
		return true
	}
	return appCtx.Modules[module]
}

// canDecidePermission resolves the permission that gates a transition, from the
// SAME source the write path is authorized with.
//
// The transition's OWN `require_permission` comes first, because that is what
// the path that actually applies the move enforces: `PATCH` (the only route a
// transition without `impl` can take) reads it off the transition and checks it
// against the caller (handler.go, per-transition gate). Reading the state
// machine rather than the route table is what makes the inbox gate independent
// of whether the transition happens to declare an `impl`.
//
// State-pair resolution is used ONLY when the task carries a resolvable pair,
// and the name form is tried first: a task knows the transition's name and the
// record's CURRENT state, so resolving by (from,to) would depend on the record
// never moving — a stale row would match no pair and its gate would be skipped.
//
// Two further fallbacks keep the answer honest for a transition that declares no
// gate at all: a matching UI route's permission if one exists, else
// `{module}.{plural}.update` (what a PATCH is authorized by).
func (b *RouterBuilder) canDecidePermission(module, entity, transition, fromState, toState string) string {
	es, _ := b.entitySpec(module, entity)
	if es != nil && es.StateMachine != nil {
		smEngine := entityengine.NewStateMachineEngine()
		trans := smEngine.FindTransitionByName(es, transition, fromState)
		if trans == nil {
			trans = smEngine.FindTransitionByStates(es, fromState, toState)
		}
		if trans != nil {
			if perm := spec.TransitionPermission(*trans); perm != "" {
				return spec.QualifyPermission(perm, module)
			}
			// The transition resolves but declares no gate of its own. That is
			// NOT automatically "ungated": a transition with an `impl` is
			// reached through its own route (which carries a permission), and
			// one without is applied by PATCH. Both are answered below.
		}
	}

	for _, rd := range b.routes {
		if rd.Module != module || rd.Entity != entity || rd.Action != transition {
			continue
		}
		if rd.Protocol != ProtocolREST || !strings.HasPrefix(rd.Path, "/_ui/entity") {
			continue
		}
		return rd.RequiredPermission
	}

	plural, ok := b.entityPlural(module, entity)
	if !ok {
		return ""
	}
	return entityPermission(module, plural, "update")
}

// entitySpec resolves an entity's spec, through the registry (which is the
// source both the handlers and the router share).
func (b *RouterBuilder) entitySpec(module, entity string) (*spec.EntitySpec, bool) {
	if info, ok := b.registry.GetEntity(module, entity); ok && info.EntitySpec != nil {
		return info.EntitySpec, true
	}
	if b.factory.specLookup != nil {
		return b.factory.specLookup(module, entity)
	}
	return nil, false
}

// entityPlural resolves an entity's plural (the segment every generated
// permission is named after), falling back to `{name}s` as the generator does.
func (b *RouterBuilder) entityPlural(module, entity string) (string, bool) {
	if b.factory.specLookup != nil {
		if es, ok := b.factory.specLookup(module, entity); ok && es != nil {
			if es.Plural != "" {
				return es.Plural, true
			}
			return entity + "s", true
		}
	}
	if info, ok := b.registry.GetEntity(module, entity); ok && info.EntitySpec != nil {
		if info.EntitySpec.Plural != "" {
			return info.EntitySpec.Plural, true
		}
		return entity + "s", true
	}
	return "", false
}

// entityPermission builds a fully qualified `{module}.{plural}.{action}`
// permission — the convention every generated route and materialized grant
// uses (generator.go).
func entityPermission(module, plural, action string) string {
	return module + "." + plural + "." + action
}

// transitionNameOf returns the transition's `via` name for an approval row. The
// row stores the gate as "{entity}.{transition}"; the transition is the segment
// after the last dot (entity names never contain dots).
func transitionNameOf(row db.ApprovalRequestRow) string {
	if i := strings.LastIndex(row.GateName, "."); i >= 0 {
		return row.GateName[i+1:]
	}
	return row.GateName
}

// inboxFallbackTitle labels a task whose step declares no `title`. It prefers
// the humanised target state ("Void requested" for a `→ voided` transition)
// over an internal name, and never falls back to an empty string — a task
// without a subject is what made the old inbox unusable.
func inboxFallbackTitle(row db.ApprovalRequestRow, wf *spec.ApprovalSpec) string {
	if name := transitionNameOf(row); name != "" {
		return humaniseIdentifier(name)
	}
	if row.ToState != "" {
		return humaniseIdentifier(row.ToState) + " requested"
	}
	return row.GateName
}

// humaniseIdentifier turns "void-order" into "Void order".
func humaniseIdentifier(s string) string {
	parts := strings.FieldsFunc(s, func(r rune) bool { return r == '-' || r == '_' })
	if len(parts) == 0 {
		return s
	}
	out := strings.Join(parts, " ")
	return strings.ToUpper(out[:1]) + out[1:]
}

// splitEntityRef splits "{module}.{entity}" at the LAST dot, so a dotted module
// name ("formspec.core.user") keeps its module intact (same rule as
// internal/ui/registry.go and print.go).
func splitEntityRef(ref string) (module, entity string) {
	i := strings.LastIndexByte(ref, '.')
	if i <= 0 || i == len(ref)-1 {
		return "", ""
	}
	return ref[:i], ref[i+1:]
}
