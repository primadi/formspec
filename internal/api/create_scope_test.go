package api

import (
	"context"
	"net/http"
	"path/filepath"
	"strings"
	"testing"

	"github.com/primadi/formspec/internal/entity"
	"github.com/primadi/formspec/pkg/spec"
	db "github.com/primadi/formspec/renderers/jsonb-persist"
)

// create_scope is the write-side counterpart of `row_scope` (plan
// docs_internal/plan/public-scope-enforcement.md).
//
// The gap cannot be seen on the read path: `row_scope` filters rows that EXIST,
// but a create has no row yet — `db.InsertParams` carries no predicates at all.
// Measured on kafe before this: an anonymous QR guest could POST an order with
// another branch's `branch_id`, because `order.branch_id` declares no
// `required_permission`, so `denyForbiddenFieldWrites` never looked at it.
//
// The fixture drives the REAL kafe tree, and the spec comes from the registry —
// the same wiring the router uses. A hand-built spec would let these tests pass
// while the shipped manifest stayed wrong, which is the failure mode that
// matters here: a security declaration that only exists in a test.
type createScopeFixture struct {
	f         *HandlerFactory
	reg       *entity.Registry
	ws        string
	branchA   string
	branchB   string
	tableA    string
	sessionID string
	orderSpec *spec.EntitySpec
}

// registry exposes the registry to the tests that assert on the SHIPPED
// manifest rather than on a passed-in spec.
func (fx *createScopeFixture) registry() *entity.Registry { return fx.reg }

func newCreateScopeFixture(t *testing.T) *createScopeFixture {
	t.Helper()
	const specPath = "../../examples/kafe/spec"
	const ws = "kafe"

	d, err := db.OpenSQLite(filepath.Join(t.TempDir(), "kafe_create_scope.db"), nil)
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	t.Cleanup(func() { _ = d.Close() })

	reg := entity.NewRegistry(d, db.DriverSQLite, specPath)
	if err := reg.LoadEntities(); err != nil {
		t.Fatalf("load kafe entities: %v", err)
	}
	// The fixture INSERTs rows, so the schema has to exist — LoadEntities only
	// reads manifests.
	if _, err := reg.SyncSchema(context.Background()); err != nil {
		t.Fatalf("sync kafe schema: %v", err)
	}

	ctx := WithWorkspace(context.Background(), ws)

	insert := func(module, name string, data map[string]any) string {
		t.Helper()
		store, err := reg.GetEntityStore(module, name)
		if err != nil {
			t.Fatalf("%s/%s store: %v", module, name, err)
		}
		id, err := store.Insert(ctx, db.InsertParams{WorkspaceID: ws, SystemCaller: true, Data: data})
		if err != nil {
			t.Fatalf("insert %s/%s: %v", module, name, err)
		}
		return id
	}

	branchA := insert("cafe-master", "branch", map[string]any{"code": "B1", "name": "Cabang Satu"})
	branchB := insert("cafe-master", "branch", map[string]any{"code": "B2", "name": "Cabang Dua"})
	tableID := insert("cafe-master", "dining-table",
		map[string]any{"code": "A-01", "branch_id": branchA, "qr_token": "TOK-A01"})
	// The session lives in branch A and is referenced by ID (the shape the
	// client sends), not by its guest token.
	sessionID := insert("cafe-order", "table-session", map[string]any{
		"guest_token": "GUEST-A", "branch_id": branchA,
		"dining_table_id": tableID, "transaction_date": "2026-10-06T10:00:00Z",
	})

	// The spec under test is the one the manifest ships, resolved through the
	// registry — the same call the router's specLookup makes.
	info, ok := reg.GetEntity("cafe-order", "order")
	if !ok || info.EntitySpec == nil {
		t.Fatal("kafe order spec not found — the fixture cannot test what the manifest declares")
	}

	return &createScopeFixture{
		f: NewHandlerFactory(reg), reg: reg, ws: ws,
		branchA: branchA, branchB: branchB, tableA: tableID, sessionID: sessionID,
		orderSpec: info.EntitySpec,
	}
}

// kafeOrderBody is the payload shape the QR form submits.
func kafeOrderBody(sessionID, branchID string) map[string]any {
	body := map[string]any{
		"channel":          "qr_table",
		"guest_token":      "GUEST-A",
		"table_session_id": sessionID,
	}
	if branchID != "" {
		body["branch_id"] = branchID
	}
	return body
}

// TestCreateScope_ManifestDeclaresItForOrder is the guard that makes the other
// tests meaningful: they call enforceCreateScope with the SHIPPED spec, so an
// empty declaration would make them pass vacuously.
//
// Calibration: removing `create_scope:` from
// examples/kafe/spec/modules/cafe-order/transaction/order/entity.yaml makes this
// test fail.
func TestCreateScope_ManifestDeclaresItForOrder(t *testing.T) {
	fx := newCreateScopeFixture(t)
	if len(fx.orderSpec.CreateScope) == 0 {
		t.Fatal("order must declare create_scope — without it an anonymous QR guest can claim any branch")
	}
	cs := fx.orderSpec.CreateScope[0]
	if cs.Field != "branch_id" || cs.RefField != "table_session_id" ||
		cs.Via != "cafe-order.table-session" || cs.ViaField != "branch_id" {
		t.Fatalf("order create_scope = %#v, want branch_id from table_session_id via cafe-order.table-session.branch_id", cs)
	}
	t.Logf("declaration: %s from %s via %s.%s", cs.Field, cs.RefField, cs.Via, cs.ViaField)
}

// TestCreateScope_RefusesAnotherBranch is the fix under test: the branch of a
// new order is the branch of the table session it references.
func TestCreateScope_RefusesAnotherBranch(t *testing.T) {
	fx := newCreateScopeFixture(t)
	ctx := WithWorkspace(context.Background(), fx.ws)

	status, err := fx.f.enforceCreateScope(ctx, "cafe-order", "order", fx.orderSpec,
		kafeOrderBody(fx.sessionID, fx.branchB))
	if err == nil {
		t.Fatalf("branch B on a session in branch A must be refused (A=%s B=%s)", fx.branchA, fx.branchB)
	}
	// A mismatch is an AUTHORIZATION refusal, not bad input: the caller is
	// asking to write outside their dimension.
	if status != http.StatusForbidden {
		t.Errorf("a mismatch must answer 403, got %d (%v)", status, err)
	}
	// The message must name both values: an operator reading a 403 has to be
	// able to tell which side disagreed.
	if !strings.Contains(err.Error(), fx.branchB) || !strings.Contains(err.Error(), fx.branchA) {
		t.Errorf("refusal must name both the payload value and the referenced one, got: %v", err)
	}
	t.Logf("refused as expected: %v", err)
}

// TestCreateScope_AcceptsTheMatchingBranch pins that the rule does not reject
// correct traffic — a guard that blocks the happy path gets disabled.
func TestCreateScope_AcceptsTheMatchingBranch(t *testing.T) {
	fx := newCreateScopeFixture(t)
	ctx := WithWorkspace(context.Background(), fx.ws)

	if _, err := fx.f.enforceCreateScope(ctx, "cafe-order", "order", fx.orderSpec,
		kafeOrderBody(fx.sessionID, fx.branchA)); err != nil {
		t.Fatalf("a matching branch must be accepted: %v", err)
	}
}

// TestCreateScope_OmittedDimensionIsAccepted pins that the rule is about
// DISAGREEMENT, not absence: the engine resolves an omitted dimension from its
// own defaults, so refusing absence would reject payloads that are correct.
func TestCreateScope_OmittedDimensionIsAccepted(t *testing.T) {
	fx := newCreateScopeFixture(t)
	ctx := WithWorkspace(context.Background(), fx.ws)

	if _, err := fx.f.enforceCreateScope(ctx, "cafe-order", "order", fx.orderSpec,
		kafeOrderBody(fx.sessionID, "")); err != nil {
		t.Fatalf("an omitted dimension must not be refused (the engine fills it): %v", err)
	}
}

// TestCreateScope_ConditionalOnTheReference pins the deliberate exception: a
// cashier walk-in order carries no `table_session_id`, so there is nothing to
// derive from and the rule must not fire — otherwise the declaration would
// quietly mean "every create needs a table session".
func TestCreateScope_ConditionalOnTheReference(t *testing.T) {
	fx := newCreateScopeFixture(t)
	ctx := WithWorkspace(context.Background(), fx.ws)

	body := map[string]any{"channel": "cashier", "branch_id": fx.branchB}
	if _, err := fx.f.enforceCreateScope(ctx, "cafe-order", "order", fx.orderSpec, body); err != nil {
		t.Fatalf("a create with no reference must not be blocked: %v", err)
	}
}

// TestCreateScope_DanglingReferenceFailsClosed pins fail-closed: a reference the
// server cannot resolve must not degrade into "no rule", because that is exactly
// how a scope becomes decorative.
//
// It also pins the CLASSIFICATION, which was a real defect: answering 403 here
// mislabelled bad input as a permission problem and broke the 422 that a broken
// relation had always produced (measured on
// TestKafe_ClientFaults_BrokenRelationIsStillValidationError).
func TestCreateScope_DanglingReferenceFailsClosed(t *testing.T) {
	fx := newCreateScopeFixture(t)
	ctx := WithWorkspace(context.Background(), fx.ws)

	status, err := fx.f.enforceCreateScope(ctx, "cafe-order", "order", fx.orderSpec,
		kafeOrderBody("00000000-0000-7000-8000-000000000000", fx.branchB))
	if err == nil {
		t.Fatal("a reference that does not resolve must be refused, not skipped")
	}
	if status != http.StatusUnprocessableEntity {
		t.Errorf("a dangling reference is bad input, must answer 422, got %d (%v)", status, err)
	}
	t.Logf("refused as expected: %v", err)
}

// TestCreateScope_TableSessionPinsTheBranch is the twin of the order hole, in
// the create that comes FIRST in the QR flow.
//
// `table-open-form.yaml` fills `branch_id` from `{table.branch_id}`, which reads
// like the server deriving it — but the value arrives in the PAYLOAD, so without
// this declaration an anonymous guest could open a table session in any branch
// by editing the request. Same rule, same enforcement: the branch of a session
// is the branch of its table.
func TestCreateScope_TableSessionPinsTheBranch(t *testing.T) {
	fx := newCreateScopeFixture(t)

	info, ok := fx.registry().GetEntity("cafe-order", "table-session")
	if !ok || info.EntitySpec == nil {
		t.Fatal("kafe table-session spec not found")
	}
	if len(info.EntitySpec.CreateScope) == 0 {
		t.Fatal("table-session must declare create_scope — otherwise an anonymous guest can open a session in any branch")
	}
	cs := info.EntitySpec.CreateScope[0]
	if cs.Field != "branch_id" || cs.RefField != "dining_table_id" ||
		cs.Via != "cafe-master.dining-table" || cs.ViaField != "branch_id" {
		t.Fatalf("table-session create_scope = %#v, want branch_id from dining_table_id via cafe-master.dining-table.branch_id", cs)
	}

	ctx := WithWorkspace(context.Background(), fx.ws)
	body := func(branch string) map[string]any {
		return map[string]any{
			"guest_token":      "GUEST-NEW",
			"transaction_date": "2026-10-06T10:00:00Z",
			"dining_table_id":  fx.tableA,
			"branch_id":        branch,
		}
	}

	// Another branch on a table in branch A ⇒ refused.
	if _, err := fx.f.enforceCreateScope(ctx, "cafe-order", "table-session", info.EntitySpec, body(fx.branchB)); err == nil {
		t.Fatal("a session in branch B on a table in branch A must be refused")
	}
	// The table's own branch ⇒ accepted.
	if _, err := fx.f.enforceCreateScope(ctx, "cafe-order", "table-session", info.EntitySpec, body(fx.branchA)); err != nil {
		t.Fatalf("the table's own branch must be accepted: %v", err)
	}
}
