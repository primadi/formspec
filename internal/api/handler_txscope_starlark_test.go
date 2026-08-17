package api

import (
	"context"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/primadi/formspec/internal/action"
	"github.com/primadi/formspec/internal/auth"
	"github.com/primadi/formspec/internal/entity"
	"github.com/primadi/formspec/pkg/spec"
	db "github.com/primadi/formspec/renderers/jsonb-persist"
)

// TestHandleCustomAction_StarlarkScript_RollsBackAcrossSaveAndCreate is the
// gap this session's earlier tests left open: everything before this test
// exercised TxScope via a NATIVE handler, but ctx only reaches
// resource.save()/create() through a real Starlark script if the
// ctx-threading fix (internal/starlark/executor.go, internal/action/script.go,
// resource/formspec.go) actually works end-to-end. This runs a real .star
// script through HandleCustomAction: it saves the bound resource, creates a
// second record via resource.create(), then fails — both writes must roll
// back together, proving the script path shares the same TxScope as the
// native path already verified.
func TestHandleCustomAction_StarlarkScript_RollsBackAcrossSaveAndCreate(t *testing.T) {
	specDir := t.TempDir()
	scriptDir := filepath.Join(specDir, "modules", "billing", "scripts")
	if err := os.MkdirAll(scriptDir, 0o755); err != nil {
		t.Fatalf("mkdir scripts dir: %v", err)
	}
	scriptSrc := `
def execute(resource, params, ctx):
    resource.set("status", "step1")
    resource.save()
    resource.create("order", {"status": "created-by-script"})
    return fail("forced failure after save + create")
`
	if err := os.WriteFile(filepath.Join(scriptDir, "checkout_fail.star"), []byte(scriptSrc), 0o644); err != nil {
		t.Fatalf("write script: %v", err)
	}

	dbDir := t.TempDir()
	d, err := db.OpenSQLite(filepath.Join(dbDir, "txscope_starlark.db"), nil)
	if err != nil {
		t.Fatalf("OpenSQLite failed: %v", err)
	}
	defer d.Close()

	reg := entity.NewRegistry(d, db.DriverSQLite, dbDir)
	orderSpec := spec.EntitySpec{
		Version: "v1",
		Plural:  "orders",
		Fields:  []spec.Field{{Name: "status", Type: spec.FieldString}},
	}
	registerTestEntity(t, d, reg, "billing", "order", orderSpec)

	ctx := context.Background()
	store, err := reg.GetEntityStore("billing", "order")
	if err != nil {
		t.Fatalf("GetEntityStore: %v", err)
	}
	id, err := store.Insert(ctx, db.InsertParams{WorkspaceID: "t1", CreatedBy: "tester", Data: map[string]any{"status": "draft"}})
	if err != nil {
		t.Fatalf("seed insert: %v", err)
	}
	before, err := store.List(ctx, db.ListParams{WorkspaceID: "t1", PerPage: 100})
	if err != nil {
		t.Fatalf("list before: %v", err)
	}

	// Wire the script executor exactly like resource/formspec.go's
	// newDispatcher does — the same Set*Handler closures, pointed at this
	// test's registry instead of the real app's.
	scriptEx := action.NewScriptExecutor(specDir)
	scriptEx.SetSaveHandler(func(ctx context.Context, workspaceID, module, entityName, id string, version int, data map[string]any) error {
		s, err := reg.GetEntityStore(module, entityName)
		if err != nil {
			return err
		}
		_, err = s.Update(ctx, db.UpdateParams{
			WorkspaceID: workspaceID,
			ID:          id,
			Version:     version,
			UpdatedBy:   "script",
			Data:        data,
			Permissions: auth.PermissionsFromContext(ctx),
		})
		return err
	})
	scriptEx.SetCreateHandler(func(ctx context.Context, workspaceID, fromModule, module, entityName string, data map[string]any, callerResources []string) (string, error) {
		s, err := reg.GetEntityStore(module, entityName)
		if err != nil {
			return "", err
		}
		return s.Insert(ctx, db.InsertParams{WorkspaceID: workspaceID, CreatedBy: "script", Data: data})
	})

	disp := action.NewDispatcher()
	disp.RegisterExecutor(spec.ImplScript, scriptEx)
	disp.RegisterExecutor(spec.ImplScriptRef, scriptEx)

	factory := NewHandlerFactory(reg)
	factory.SetDispatcher(disp)

	actionSpec := spec.Action{
		Name: "checkout",
		Impl: &spec.ImplDecl{Type: spec.ImplScript, Ref: "billing/checkout_fail"},
	}
	handlerFunc := factory.HandleCustomAction("billing", "order", "checkout", actionSpec, "")

	req := httptest.NewRequest("POST", "/billing/orders/"+id+"/checkout", nil)
	req.SetPathValue("id", id)
	req = req.WithContext(WithWorkspace(req.Context(), "t1"))
	req = req.WithContext(WithUser(req.Context(), "tester"))
	rr := httptest.NewRecorder()

	handlerFunc(rr, req)

	if rr.Code != 500 {
		t.Fatalf("expected 500 (script fail()), got %d: %s", rr.Code, rr.Body.String())
	}

	final, err := store.GetByID(ctx, db.GetByIDParams{WorkspaceID: "t1", ID: id})
	if err != nil {
		t.Fatalf("final GetByID: %v", err)
	}
	if final.Data["status"] != "draft" {
		t.Fatalf("expected resource.save() rolled back (status still draft), got %v", final.Data["status"])
	}

	after, err := store.List(ctx, db.ListParams{WorkspaceID: "t1", PerPage: 100})
	if err != nil {
		t.Fatalf("list after: %v", err)
	}
	if after.Total != before.Total {
		t.Fatalf("expected resource.create() rolled back too (record count unchanged: %d), got %d — proves script's save+create shared one transaction", before.Total, after.Total)
	}
}
