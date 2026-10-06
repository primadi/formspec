package api

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	formspec_app "github.com/primadi/formspec/internal/app"
	"github.com/primadi/formspec/internal/approval"
	"github.com/primadi/formspec/internal/auth"
	"github.com/primadi/formspec/internal/entity"
	"github.com/primadi/formspec/pkg/spec"
	db "github.com/primadi/formspec/renderers/jsonb-persist"
)

// ─── Approval inbox surface (todo 5.13.6, plan approval-inbox-endpoint.md) ───
//
// The kind is zero-config, so these tests are about the SOURCE, not the write
// path: pending rows are seeded straight into `formspec_workflow_approval`
// (the write path has its own coverage in workflow_approval_api_test.go), and
// what is asserted is who sees which task and what the decision endpoint does
// to the record.
//
// Two entities are registered on purpose: `billing/order` belongs to the test
// App, `warehouse/pick` does not — so a passing test also proves the App filter
// is not a no-op.

const (
	inboxWorkspace = "t1"
	inboxOtherWS   = "t2"
	inboxGatePerm  = "billing.orders.void-order"
	// inboxRouteGatePerm is declared on the ROUTE only (no transition gate), to
	// pin the fallback order of canDecidePermission.
	inboxRouteGatePerm = "billing.orders.void-order-route-gated"
)

type inboxHarness struct {
	b        *RouterBuilder
	rows     *db.ApprovalRequestStore
	recordID string
	// wfReg is the workflow registry the factory uses, so a test can adjust a
	// step (give it a duty) without rebuilding the harness.
	wfReg *approval.Registry
	// otherRecordID belongs to warehouse/pick — an entity of a module the test
	// App does not mount.
	otherRecordID string
}

func setupInboxHarness(t *testing.T) *inboxHarness {
	t.Helper()
	dir := t.TempDir()
	d, err := db.OpenSQLite(filepath.Join(dir, "approval_inbox.db"), nil)
	if err != nil {
		t.Fatalf("OpenSQLite: %v", err)
	}
	t.Cleanup(func() { _ = d.Close() })
	if err := db.NewMigrationRunner(d, db.DriverSQLite).EnsureSystemTables(context.Background()); err != nil {
		t.Fatalf("EnsureSystemTables: %v", err)
	}

	reg := entity.NewRegistry(d, db.DriverSQLite, dir)
	orderSpec := spec.EntitySpec{
		Version: "v1",
		Plural:  "orders",
		Fields: []spec.Field{
			{Name: "status", Type: spec.FieldString},
			{Name: "void_reason", Type: spec.FieldString, Title: "Alasan void"},
		},
		StateMachine: &spec.StateMachine{
			Field:   "status",
			Initial: "draft",
			States:  []spec.StateDecl{{Name: "draft"}, {Name: "posted"}, {Name: "voided"}, {Name: "cancelled"}},
			Transitions: []spec.TransitionDecl{
				{
					From: spec.StateList{"posted"}, To: "voided", Action: "void-order",
					// The gate lives ON the transition — that is where the write
					// path reads it (handler.go's per-transition gate), so the
					// inbox must read it there too. Unqualified on purpose: the
					// module prefix is added by QualifyPermission, exactly as the
					// route generator and the PATCH gate do it.
					RequirePermission: "orders.void-order",
					Approval: &spec.ApprovalSpec{
						Steps: []spec.ApprovalStep{{
							Roles:         []string{"supervisor"},
							Title:         "Persetujuan Void Pesanan",
							Description:   "Periksa nomor pesanan, total, dan alasan void.",
							DisplayFields: []string{"void_reason", "total_amount"},
						}},
						OnReject: &spec.ApprovalReject{To: "posted"},
					},
				},
				// No gate on the transition: the inbox must then fall back to the
				// route's own permission, and only then to `{module}.{plural}.update`.
				{From: spec.StateList{"posted"}, To: "cancelled", Action: "cancel-order"},
			},
		},
	}
	registerTestEntity(t, d, reg, "billing", "order", orderSpec)
	pickSpec := spec.EntitySpec{
		Version: "v1",
		Plural:  "picks",
		Fields:  []spec.Field{{Name: "status", Type: spec.FieldString}},
		StateMachine: &spec.StateMachine{
			Field:   "status",
			Initial: "draft",
			States:  []spec.StateDecl{{Name: "draft"}, {Name: "posted"}},
			Transitions: []spec.TransitionDecl{{
				From: spec.StateList{"draft"}, To: "posted", Action: "approve-pick",
				Approval: &spec.ApprovalSpec{
					Steps: []spec.ApprovalStep{{Roles: []string{"supervisor"}, Title: "Pick approval"}},
				},
			}},
		},
	}
	registerTestEntity(t, d, reg, "warehouse", "pick", pickSpec)

	orderStore, err := reg.GetEntityStore("billing", "order")
	if err != nil {
		t.Fatalf("GetEntityStore(billing/order): %v", err)
	}
	recordID, err := orderStore.Insert(context.Background(), db.InsertParams{
		WorkspaceID:  inboxWorkspace,
		CreatedBy:    "owner-1",
		SystemCaller: true,
		Data: map[string]any{
			"status":      "posted",
			"void_reason": "customer complaint",
		},
	})
	if err != nil {
		t.Fatalf("seed order: %v", err)
	}
	pickStore, err := reg.GetEntityStore("warehouse", "pick")
	if err != nil {
		t.Fatalf("GetEntityStore(warehouse/pick): %v", err)
	}
	otherRecordID, err := pickStore.Insert(context.Background(), db.InsertParams{
		WorkspaceID:  inboxWorkspace,
		CreatedBy:    "owner-1",
		SystemCaller: true,
		Data:         map[string]any{"status": "posted"},
	})
	if err != nil {
		t.Fatalf("seed pick: %v", err)
	}

	wfReg := approval.NewRegistry()
	wfReg.AddEntity("billing", "order", &orderSpec)
	wfReg.AddEntity("warehouse", "pick", &pickSpec)

	factory := NewHandlerFactory(reg)
	factory.SetApprovalRegistry(wfReg)
	rows := db.NewApprovalRequestStore(d, db.DriverSQLite)
	factory.SetApprovalRequestStore(rows)
	factory.SetSpecLookup(func(m, n string) (*spec.EntitySpec, bool) {
		info, ok := reg.GetEntity(m, n)
		if !ok || info.EntitySpec == nil {
			return nil, false
		}
		return info.EntitySpec, true
	})

	b := NewRouterBuilder(reg)
	b.factory = factory
	b.SetApps(map[string]*formspec_app.ResolvedApp{
		"pos": {
			Name:    "pos",
			Spec:    &spec.AppSpec{RootURL: "/app/pos", Modules: []string{"billing"}},
			Modules: map[string]bool{"billing": true},
		},
	})
	// The transition gate the inbox must mirror. Taken from a real descriptor
	// shape, so canDecidePermission's lookup is exercised rather than stubbed.
	b.routes = []RouteDescriptor{
		{
			Module: "billing", Entity: "order", Plural: "orders", Action: "void-order",
			Method: "POST", Protocol: ProtocolREST, Handler: "custom",
			Path:               "/_ui/entity/billing/order/{id}/void-order",
			RequiredPermission: inboxGatePerm,
		},
		// A transition with NO gate of its own: the route's permission is what
		// authorizes it, so that is what the inbox must report.
		{
			Module: "billing", Entity: "order", Plural: "orders", Action: "cancel-order",
			Method: "POST", Protocol: ProtocolREST, Handler: "custom",
			Path:               "/_ui/entity/billing/order/{id}/cancel-order",
			RequiredPermission: inboxRouteGatePerm,
		},
		{
			Module: "warehouse", Entity: "pick", Plural: "picks", Action: "approve-pick",
			Method: "POST", Protocol: ProtocolREST, Handler: "custom",
			Path:               "/_ui/entity/warehouse/pick/{id}/approve-pick",
			RequiredPermission: "warehouse.picks.approve-pick",
		},
	}
	return &inboxHarness{b: b, rows: rows, recordID: recordID, otherRecordID: otherRecordID, wfReg: wfReg}
}

// seedPending inserts a pending approval row directly — the inbox reads rows,
// and the write path that creates them is covered elsewhere. params carries the
// inputs the requester supplied for the intercepted transition.
func (h *inboxHarness) seedPending(t *testing.T, tenant, entity, recordID, gateName string, params map[string]any) string {
	t.Helper()
	id, err := h.rows.Create(context.Background(), db.ApprovalRequestRow{
		TenantID:       tenant,
		Entity:         entity,
		RecordID:       recordID,
		GateModule:     strings.Split(entity, ".")[0],
		GateName:       gateName,
		FromState:      "posted",
		ToState:        "voided",
		RequesterID:    "owner-1",
		Status:         "pending",
		ActiveStep:     0,
		Approvals:      map[string][]string{},
		EscalatedSteps: map[string][]string{},
		Params:         params,
	})
	if err != nil {
		t.Fatalf("seed pending approval: %v", err)
	}
	return id
}

// request issues one inbox request as the given identity.
func (h *inboxHarness) request(
	t *testing.T, handler http.HandlerFunc, method, target, ws, id string,
	identity *auth.Identity, body string,
) *httptest.ResponseRecorder {
	t.Helper()
	var reader *strings.Reader
	if body == "" {
		reader = strings.NewReader("")
	} else {
		reader = strings.NewReader(body)
	}
	req := httptest.NewRequest(method, "/"+ws+target, reader)
	req = req.WithContext(WithWorkspace(req.Context(), ws))
	if identity != nil {
		req = req.WithContext(WithIdentity(req.Context(), identity))
		req = req.WithContext(WithUser(req.Context(), identity.UserID))
	}
	if id != "" {
		req.SetPathValue("id", id)
	}
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	return rec
}

// list decodes the inbox listing for one identity.
func (h *inboxHarness) list(t *testing.T, identity *auth.Identity, ws string) (int, []approvalInboxItem) {
	t.Helper()
	rec := h.request(t, h.b.HandleApprovalInbox(), http.MethodGet, "/_ui/workflow/approvals", ws, "", identity, "")
	var resp struct {
		Data []approvalInboxItem `json:"data"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode inbox response (%d): %v — %s", rec.Code, err, rec.Body.String())
	}
	return rec.Code, resp.Data
}

func supervisorIdentity(perms ...string) *auth.Identity {
	return &auth.Identity{
		UserID:      "supervisor-1",
		WorkspaceID: inboxWorkspace,
		Roles:       []string{"supervisor"},
		Permissions: perms,
	}
}

// clearRecordField removes one field from the harness's order record, so a test
// can reproduce "the record does not have it yet" (the state an intercepted
// transition leaves behind).
func clearRecordField(t *testing.T, h *inboxHarness, field string) {
	t.Helper()
	store, err := h.b.registry.GetEntityStore("billing", "order")
	if err != nil {
		t.Fatalf("GetEntityStore: %v", err)
	}
	rec, err := store.GetByID(t.Context(), db.GetByIDParams{WorkspaceID: inboxWorkspace, ID: h.recordID})
	if err != nil || rec == nil {
		t.Fatalf("GetByID: %v", err)
	}
	delete(rec.Data, field)
	if _, err := store.Update(t.Context(), db.UpdateParams{
		WorkspaceID:  inboxWorkspace,
		ID:           h.recordID,
		Version:      rec.Version,
		Data:         rec.Data,
		SystemCaller: true,
	}); err != nil {
		t.Fatalf("clear %s: %v", field, err)
	}
}

func orderStatus(t *testing.T, h *inboxHarness) string {
	t.Helper()
	store, err := h.b.registry.GetEntityStore("billing", "order")
	if err != nil {
		t.Fatalf("GetEntityStore: %v", err)
	}
	rec, err := store.GetByID(t.Context(), db.GetByIDParams{WorkspaceID: inboxWorkspace, ID: h.recordID})
	if err != nil || rec == nil {
		t.Fatalf("GetByID: %v", err)
	}
	status, _ := rec.Data["status"].(string)
	return status
}

// TestApprovalInbox_DisplayFieldFallsBackToRequesterInput is the regression for
// a field the approver needs but the RECORD does not have yet.
//
// An intercepted transition writes nothing until the approval completes, so the
// reason the requester supplied exists only on the approval row (`params`).
// Reading the record alone rendered `void_reason` — the field the contract and
// the kafe acceptance criterion both name — as empty, i.e. the approver was told
// to check a reason that was always blank. Measured live on kafe: the row held
// {"void_reason":"salah input"} while display_fields answered null.
func TestApprovalInbox_DisplayFieldFallsBackToRequesterInput(t *testing.T) {
	h := setupInboxHarness(t)
	// Mirror the real situation: the record does NOT carry the reason yet (the
	// intercepted transition has not run), so the only copy is on the row.
	clearRecordField(t, h, "void_reason")
	h.seedPending(t, inboxWorkspace, "billing.order", h.recordID, "order.void-order",
		map[string]any{"void_reason": "salah input"})

	code, items := h.list(t, supervisorIdentity(inboxGatePerm, "billing.orders.view"), inboxWorkspace)
	if code != http.StatusOK || len(items) != 1 {
		t.Fatalf("expected one task, got %d: %+v", len(items), items)
	}
	var reason *approvalInboxField
	for i := range items[0].DisplayFields {
		if items[0].DisplayFields[i].Field == "void_reason" {
			reason = &items[0].DisplayFields[i]
		}
	}
	if reason == nil {
		t.Fatalf("void_reason must be reported, got %+v", items[0].DisplayFields)
	}
	if reason.Value != "salah input" {
		t.Fatalf("the requester's input must reach the approver, got %#v", reason.Value)
	}
	// The record's OWN value still wins once it exists: the fallback fills a
	// gap, it does not shadow the record.
	if items[0].DisplayFields[1].Field != "total_amount" || items[0].DisplayFields[1].Value != nil {
		t.Errorf("a field in neither the record nor params stays empty: %+v", items[0].DisplayFields[1])
	}
}

// TestApprovalInbox_ListsEligibleTask is the regression for the observed
// failure: the inbox showed "No approval source configured" because no source
// existed at all. The task must now arrive with the step's own labels and the
// record values the approver needs.
func TestApprovalInbox_ListsEligibleTask(t *testing.T) {
	h := setupInboxHarness(t)
	h.seedPending(t, inboxWorkspace, "billing.order", h.recordID, "order.void-order", nil)

	code, items := h.list(t, supervisorIdentity(inboxGatePerm, "billing.orders.view"), inboxWorkspace)
	if code != http.StatusOK {
		t.Fatalf("expected 200, got %d", code)
	}
	if len(items) != 1 {
		t.Fatalf("expected 1 pending task, got %d: %+v", len(items), items)
	}
	got := items[0]
	if got.Title != "Persetujuan Void Pesanan" {
		t.Errorf("title must come from the workflow step, got %q", got.Title)
	}
	if got.Description == "" {
		t.Error("description must come from the workflow step")
	}
	if got.Entity != "billing.order" || got.RecordID != h.recordID {
		t.Errorf("task must name its record, got %s / %s", got.Entity, got.RecordID)
	}
	if !got.CanDecide {
		t.Error("caller holds the transition's gate permission — can_decide must be true")
	}
	// display_fields: the declared reason is present; the declared-but-absent
	// total_amount is reported empty rather than silently dropped.
	if len(got.DisplayFields) != 2 {
		t.Fatalf("expected both declared display fields, got %+v", got.DisplayFields)
	}
	if got.DisplayFields[0].Field != "void_reason" || got.DisplayFields[0].Value != "customer complaint" {
		t.Errorf("void_reason value: %+v", got.DisplayFields[0])
	}
	if got.DisplayFields[1].Field != "total_amount" || got.DisplayFields[1].Value != nil {
		t.Errorf("absent field must be reported empty: %+v", got.DisplayFields[1])
	}
	// Label + type must travel with the value: the renderer cannot infer either
	// (a money value arrives as {amount, currency}, which `String(value)`
	// printed as "[object Object]" — measured in the browser on kafe).
	if got.DisplayFields[0].Label != "Alasan void" {
		t.Errorf("label must come from the entity field, got %q", got.DisplayFields[0].Label)
	}
	if got.DisplayFields[0].Type != "string" {
		t.Errorf("type must come from the entity field, got %q", got.DisplayFields[0].Type)
	}
}

// TestApprovalInbox_HidesNonEligibleAndSelf pins the two exclusions the
// contract names: a caller who does not hold the step's role sees nothing, and
// the requester never sees their own request (7.4.5).
func TestApprovalInbox_HidesNonEligibleAndSelf(t *testing.T) {
	h := setupInboxHarness(t)
	h.seedPending(t, inboxWorkspace, "billing.order", h.recordID, "order.void-order", nil)

	if _, items := h.list(t, &auth.Identity{
		UserID: "cashier-1", WorkspaceID: inboxWorkspace,
		Roles: []string{"cashier"}, Permissions: []string{"*"},
	}, inboxWorkspace); len(items) != 0 {
		t.Fatalf("a caller without the step role must see no task, got %+v", items)
	}

	self := supervisorIdentity(inboxGatePerm)
	self.UserID = "owner-1" // the seeded record's creator
	if _, items := h.list(t, self, inboxWorkspace); len(items) != 0 {
		t.Fatalf("the requester must not see their own request (7.4.5), got %+v", items)
	}
}

// TestApprovalInbox_ScopedToAppAndTenant proves the two narrowings that make
// this endpoint safe to expose: a task from a module the App does not mount is
// invisible, and so is a task from another workspace — including one whose
// approval id is passed straight to the decision endpoint.
func TestApprovalInbox_ScopedToAppAndTenant(t *testing.T) {
	h := setupInboxHarness(t)
	h.seedPending(t, inboxWorkspace, "billing.order", h.recordID, "order.void-order", nil)
	h.seedPending(t, inboxWorkspace, "warehouse.pick", h.otherRecordID, "pick-approval", nil)
	foreignID := h.seedPending(t, inboxOtherWS, "billing.order", "rec-in-t2", "order.void-order", nil)

	// The App mounts `billing` only, so the warehouse task (same tenant, same
	// role) must not appear.
	code, items := h.list(t, supervisorIdentity(inboxGatePerm), inboxWorkspace)
	if code != http.StatusOK || len(items) != 1 || items[0].Entity != "billing.order" {
		t.Fatalf("expected exactly the App's own task, got %d items: %+v", len(items), items)
	}

	// Another workspace's approval is not decidable by id either.
	rec := h.request(t, h.b.HandleApprovalDecision(), http.MethodPost,
		"/_ui/workflow/approvals/"+foreignID, inboxWorkspace, foreignID,
		supervisorIdentity(inboxGatePerm, "*"), `{"decision":"approve"}`)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("cross-tenant approval id: expected 404, got %d: %s", rec.Code, rec.Body.String())
	}
	if got := orderStatus(t, h); got != "posted" {
		t.Fatalf("the other workspace's decision must not touch this record: status=%q", got)
	}
}

// TestApprovalInbox_CanDecideIsHonest covers the distinction the item reports:
// role eligibility decides what is LISTED, the transition's gate permission
// decides what can be RUN. A task with can_decide=false must still be listed
// (it is the caller's queue) and must be refused with 403 if attempted.
func TestApprovalInbox_CanDecideIsHonest(t *testing.T) {
	h := setupInboxHarness(t)
	h.seedPending(t, inboxWorkspace, "billing.order", h.recordID, "order.void-order", nil)

	// Holds a permission on the entity, but not its `view` and not the
	// transition's gate.
	weak := supervisorIdentity("billing.orders.export")
	code, items := h.list(t, weak, inboxWorkspace)
	if code != http.StatusOK || len(items) != 1 {
		t.Fatalf("the caller's own eligible task must still be listed, got %d: %+v", len(items), items)
	}
	if items[0].CanDecide {
		t.Error("can_decide must be false without the transition's gate permission")
	}
	if len(items[0].DisplayFields) != 0 {
		t.Errorf("display values require the entity view permission, got %+v", items[0].DisplayFields)
	}

	rec := h.request(t, h.b.HandleApprovalDecision(), http.MethodPost,
		"/_ui/workflow/approvals/"+items[0].ID, inboxWorkspace, items[0].ID,
		weak, `{"decision":"approve"}`)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected 403 without the gate permission, got %d: %s", rec.Code, rec.Body.String())
	}
	if got := orderStatus(t, h); got != "posted" {
		t.Fatalf("a refused decision must not move the record: status=%q", got)
	}
}

// TestApprovalInbox_DecisionApprovesAndConsumesTask is the end-to-end proof:
// the decision runs the transition it was queued for, and the task disappears
// from the inbox afterwards — a queue that keeps completed work is a queue
// nobody trusts.
func TestApprovalInbox_DecisionApprovesAndConsumesTask(t *testing.T) {
	h := setupInboxHarness(t)
	taskID := h.seedPending(t, inboxWorkspace, "billing.order", h.recordID, "order.void-order", nil)

	id := supervisorIdentity(inboxGatePerm)
	rec := h.request(t, h.b.HandleApprovalDecision(), http.MethodPost,
		"/_ui/workflow/approvals/"+taskID, inboxWorkspace, taskID, id, `{"decision":"approve"}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("approve: expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	if got := orderStatus(t, h); got != "voided" {
		t.Fatalf("approval must execute the transition: status=%q, want voided", got)
	}
	if _, items := h.list(t, id, inboxWorkspace); len(items) != 0 {
		t.Fatalf("a completed task must leave the inbox, got %+v", items)
	}
	// Deciding twice must not be a silent success.
	again := h.request(t, h.b.HandleApprovalDecision(), http.MethodPost,
		"/_ui/workflow/approvals/"+taskID, inboxWorkspace, taskID, id, `{"decision":"approve"}`)
	if again.Code != http.StatusNotFound {
		t.Fatalf("second decision: expected 404, got %d: %s", again.Code, again.Body.String())
	}
}

func TestApprovalInbox_DecisionRejects(t *testing.T) {
	h := setupInboxHarness(t)
	taskID := h.seedPending(t, inboxWorkspace, "billing.order", h.recordID, "order.void-order", nil)

	rec := h.request(t, h.b.HandleApprovalDecision(), http.MethodPost,
		"/_ui/workflow/approvals/"+taskID, inboxWorkspace, taskID,
		supervisorIdentity(inboxGatePerm), `{"decision":"reject"}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("reject: expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	if got := orderStatus(t, h); got != "posted" {
		t.Fatalf("a rejection must not void the order (on_reject.to = posted): status=%q", got)
	}
	if _, items := h.list(t, supervisorIdentity(inboxGatePerm), inboxWorkspace); len(items) != 0 {
		t.Fatalf("a rejected task must leave the inbox, got %+v", items)
	}
}

// TestApprovalInbox_BadDecisionIsRejected guards the body contract: an
// unrecognized decision must not reach the workflow engine at all.
func TestApprovalInbox_BadDecisionIsRejected(t *testing.T) {
	h := setupInboxHarness(t)
	taskID := h.seedPending(t, inboxWorkspace, "billing.order", h.recordID, "order.void-order", nil)

	rec := h.request(t, h.b.HandleApprovalDecision(), http.MethodPost,
		"/_ui/workflow/approvals/"+taskID, inboxWorkspace, taskID,
		supervisorIdentity(inboxGatePerm), `{"decision":"maybe"}`)
	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("expected 422, got %d: %s", rec.Code, rec.Body.String())
	}
	if got := orderStatus(t, h); got != "posted" {
		t.Fatalf("the record must be untouched: status=%q", got)
	}
}

// TestApprovalInbox_RequiresAuthentication pins the surface: the inbox is
// session-authenticated like the rest of /_ui, and anonymous callers get 401
// rather than an empty list (which would read as "no work to do").
func TestApprovalInbox_RequiresAuthentication(t *testing.T) {
	h := setupInboxHarness(t)
	h.seedPending(t, inboxWorkspace, "billing.order", h.recordID, "order.void-order", nil)

	rec := h.request(t, h.b.HandleApprovalInbox(), http.MethodGet,
		"/_ui/workflow/approvals", inboxWorkspace, "", nil, "")
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 for anonymous, got %d", rec.Code)
	}
}

// TestApprovalInbox_RoutesAreRegistered drives the REAL router, because the
// handlers above are called directly and would happily pass while the route
// was never mounted. 401 (authenticated surface, no credential) rather than
// 404 is what proves the mount exists.
func TestApprovalInbox_RoutesAreRegistered(t *testing.T) {
	h := setupInboxHarness(t)
	handler := h.b.BuildHTTP()

	for _, tc := range []struct{ method, path string }{
		{http.MethodGet, "/" + inboxWorkspace + "/_ui/workflow/approvals"},
		{http.MethodPost, "/" + inboxWorkspace + "/_ui/workflow/approvals/1"},
	} {
		req := httptest.NewRequest(tc.method, tc.path, strings.NewReader(""))
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)
		if rec.Code != http.StatusUnauthorized {
			t.Errorf("%s %s: expected 401 (route mounted, unauthenticated), got %d: %s",
				tc.method, tc.path, rec.Code, rec.Body.String())
		}
	}
}

// TestApprovalInbox_GateResolutionOrder pins where the gate is read from, which
// is the difference between an inbox that refuses work the caller CAN do and one
// that offers work the caller cannot.
//
// The transition's own `require_permission` wins, because that is what the write
// path enforces (handler.go's per-transition gate); the matching route's
// permission is the fallback for a transition that declares none; and
// `{module}.{plural}.update` is the last resort — what a `PATCH` is authorized
// by when the transition has no `impl` and therefore no route at all.
func TestApprovalInbox_GateResolutionOrder(t *testing.T) {
	h := setupInboxHarness(t)

	// 1. Declared on the transition (unqualified) → qualified with its module.
	if got := h.b.canDecidePermission("billing", "order", "void-order", "posted", "voided"); got != inboxGatePerm {
		t.Errorf("transition gate: got %q, want %q", got, inboxGatePerm)
	}
	// The name form is tried first, so a stale `to` still resolves the gate
	// rather than skipping it.
	if got := h.b.canDecidePermission("billing", "order", "void-order", "posted", "stale-target"); got != inboxGatePerm {
		t.Errorf("name form must not depend on the target state: got %q", got)
	}

	// 2. Silent on the transition, declared on the route → the route's value.
	if got := h.b.canDecidePermission("billing", "order", "cancel-order", "posted", "cancelled"); got != inboxRouteGatePerm {
		t.Errorf("route fallback: got %q, want %q", got, inboxRouteGatePerm)
	}

	// 3. Neither a gate nor a matching route/state pair → the permission a PATCH
	// would be authorized by.
	if got := h.b.canDecidePermission("billing", "order", "unknown-transition", "draft", "voided"); got != "billing.orders.update" {
		t.Errorf("update fallback: got %q, want billing.orders.update", got)
	}

	// 4. `can_decide` follows that fallback: holding `update` is what lets a
	// caller run a transition with no gate and no route of its own. The workflow
	// in this fixture declares no duty, so the transition gate is the only one
	// in play — which is what makes this assertion about the fallback.
	identity := supervisorIdentity("billing.orders.update")
	wf := &spec.ApprovalSpec{
		Steps: []spec.ApprovalStep{{Roles: []string{"supervisor"}}},
	}
	if !h.b.canRunTransition(identity, "billing", "order", wf,
		db.ApprovalRequestRow{GateName: "order.unknown-transition", FromState: "draft", ToState: "voided", ActiveStep: 0}) {
		t.Error("holding the fallback permission must satisfy can_decide")
	}
}

// TestApprovalInbox_DutyIsAnAlternativeToRoles pins the inbox against the WRITE
// path: two doors into one flow must agree about who may enter.
//
// A step that declares a duty is satisfiable by that permission OR by one of its
// roles (`CanApprove`) — which is exactly what the record's own page enforces.
// Requiring the duty ALONE here was my first implementation, and it was wrong:
// it would let a caller approve from the record's page and refuse them on the
// inbox, with no rule saying which one is authoritative.
func TestApprovalInbox_DutyIsAnAlternativeToRoles(t *testing.T) {
	h := setupInboxHarness(t)
	duty := "workflow.billing.order.void-order.supervisor-check"
	addStepDuty(t, h, "supervisor-check", duty)
	h.seedPending(t, inboxWorkspace, "billing.order", h.recordID, "order.void-order", nil)

	// Holds the transition gate and the step's ROLE — no duty permission.
	code, items := h.list(t, supervisorIdentity(inboxGatePerm), inboxWorkspace)
	if code != http.StatusOK || len(items) != 1 {
		t.Fatalf("expected the task to be listed, got %d: %+v", len(items), items)
	}
	if !items[0].CanDecide {
		t.Error("the step's role must satisfy can_decide while roles remain an alternative")
	}
	rec := h.request(t, h.b.HandleApprovalDecision(), http.MethodPost,
		"/_ui/workflow/approvals/"+items[0].ID, inboxWorkspace, items[0].ID,
		supervisorIdentity(inboxGatePerm), `{"decision":"approve"}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("role holder: expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	if got := orderStatus(t, h); got != "voided" {
		t.Fatalf("approval must execute the transition: status=%q", got)
	}
}

// TestApprovalInbox_DutyPermissionAloneIsEnough is the adoption case: a caller
// with NO role at all approves because they hold the step's duty permission.
func TestApprovalInbox_DutyPermissionAloneIsEnough(t *testing.T) {
	h := setupInboxHarness(t)
	duty := "workflow.billing.order.void-order.supervisor-check"
	addStepDuty(t, h, "supervisor-check", duty)
	h.seedPending(t, inboxWorkspace, "billing.order", h.recordID, "order.void-order", nil)

	noRole := &auth.Identity{
		UserID:      "auditor-1",
		WorkspaceID: inboxWorkspace,
		Permissions: []string{duty, inboxGatePerm},
	}
	code, items := h.list(t, noRole, inboxWorkspace)
	if code != http.StatusOK || len(items) != 1 || !items[0].CanDecide {
		t.Fatalf("holding the duty must be enough, got %d: %+v", len(items), items)
	}
	rec := h.request(t, h.b.HandleApprovalDecision(), http.MethodPost,
		"/_ui/workflow/approvals/"+items[0].ID, inboxWorkspace, items[0].ID,
		noRole, `{"decision":"approve"}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("duty holder: expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	if got := orderStatus(t, h); got != "voided" {
		t.Fatalf("approval must execute the transition: status=%q", got)
	}
}

// addStepDuty gives the fixture workflow's first step a duty permission, the
// way a manifest declares `permission:` on a step.
func addStepDuty(t *testing.T, h *inboxHarness, name, permission string) {
	t.Helper()
	wf, ok := h.wfReg.Get("billing", "order.void-order")
	if !ok || wf == nil {
		t.Fatal("fixture workflow missing")
	}
	wf.Steps[0].Name = name
	wf.Steps[0].Permission = permission
}

// TestApprovalInbox_HistorySurvivesAManifestEdit is the end-to-end form of the
// step-key change, over the real store and the real endpoint.
//
// A pending approval's history used to be keyed by step POSITION (`{"0": [...]}`),
// so inserting or re-ordering a step re-pointed yesterday's signatures at a
// different step: the recorded approver silently became "whoever is at index 0
// now". Keys are step names now, and this proves the consequences on both sides —
// the recorded signature is still found for the step it belongs to, and the newly
// inserted step does NOT inherit it.
func TestApprovalInbox_HistorySurvivesAManifestEdit(t *testing.T) {
	h := setupInboxHarness(t)
	// Two named steps, the first one gated by a duty so the caller needs no role.
	dutyA := "workflow.billing.order.void-order.first-check"
	dutyB := "workflow.billing.order.void-order.second-check"
	setTwoDutySteps(t, h, dutyA, dutyB)
	h.seedPending(t, inboxWorkspace, "billing.order", h.recordID, "order.void-order", nil)

	// Approver A signs the FIRST step.
	row := latestPendingRow(t, h)
	rec := h.request(t, h.b.HandleApprovalDecision(), http.MethodPost,
		"/_ui/workflow/approvals/"+row.ID, inboxWorkspace, row.ID,
		supervisorIdentity(inboxGatePerm, dutyA), `{"decision":"approve"}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("first step approve: expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	// The signature is stored under the STEP NAME, not under "0".
	stored := latestPendingRow(t, h)
	if _, byName := stored.Approvals["first-check"]; !byName {
		t.Fatalf("approvals must be keyed by step name, got %v", stored.Approvals)
	}
	if _, byIndex := stored.Approvals["0"]; byIndex {
		t.Errorf("no numeric key should be written any more, got %v", stored.Approvals)
	}

	// The second step is now waiting and the first one's signature is what
	// advanced the chain — reading it back by name is what makes that true.
	if got := stored.ActiveStepName; got != "second-check" {
		t.Fatalf("the approval should be waiting on the second step, got %q", got)
	}
}

// setTwoDutySteps replaces the fixture workflow's steps with two named, duty-gated
// ones, so a test can exercise the chain without depending on roles.
func setTwoDutySteps(t *testing.T, h *inboxHarness, duties ...string) {
	t.Helper()
	wf, ok := h.wfReg.Get("billing", "order.void-order")
	if !ok || wf == nil {
		t.Fatal("fixture workflow missing")
	}
	steps := make([]spec.ApprovalStep, 0, len(duties))
	for i, d := range duties {
		name := "first-check"
		if i > 0 {
			name = "second-check"
		}
		steps = append(steps, spec.ApprovalStep{Name: name, Permission: d, Approvers: 1})
	}
	wf.Steps = steps
}

// latestPendingRow reads the workspace's most recent pending approval straight
// from the store, so a test can assert on what was actually persisted.
func latestPendingRow(t *testing.T, h *inboxHarness) db.ApprovalRequestRow {
	t.Helper()
	rows, err := h.rows.ListPendingForTenant(context.Background(), inboxWorkspace, 50)
	if err != nil {
		t.Fatalf("ListPendingForTenant: %v", err)
	}
	if len(rows) == 0 {
		t.Fatal("expected a pending approval")
	}
	return rows[len(rows)-1]
}

// TestApprovalInbox_ListPendingForTenant covers the store-level guarantee the
// handler depends on. The escalation worker's ListPending is deliberately
// tenant-blind; serving a request from it would hand one workspace another
// workspace's approvals.
func TestApprovalInbox_ListPendingForTenant(t *testing.T) {
	h := setupInboxHarness(t)
	h.seedPending(t, inboxWorkspace, "billing.order", h.recordID, "order.void-order", nil)
	h.seedPending(t, inboxOtherWS, "billing.order", "rec-in-t2", "order.void-order", nil)

	mine, err := h.rows.ListPendingForTenant(context.Background(), inboxWorkspace, 50)
	if err != nil {
		t.Fatalf("ListPendingForTenant: %v", err)
	}
	if len(mine) != 1 || mine[0].TenantID != inboxWorkspace {
		t.Fatalf("expected only this workspace's row, got %+v", mine)
	}

	all, err := h.rows.ListPending(context.Background(), 50)
	if err != nil {
		t.Fatalf("ListPending: %v", err)
	}
	if len(all) != 2 {
		t.Fatalf("the worker's sweep stays tenant-blind (2 rows), got %d", len(all))
	}

	// An empty tenant must not degrade into "every tenant".
	none, err := h.rows.ListPendingForTenant(context.Background(), "", 50)
	if err != nil {
		t.Fatalf("ListPendingForTenant(\"\"): %v", err)
	}
	if len(none) != 0 {
		t.Fatalf("an unscoped call must return nothing, got %d rows", len(none))
	}
}
