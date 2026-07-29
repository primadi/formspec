package api

import (
	"context"
	"fmt"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"github.com/primadi/forma/internal/action"
	"github.com/primadi/forma/internal/entity"
	"github.com/primadi/forma/internal/manifest"
	"github.com/primadi/forma/pkg/spec"
	db "github.com/primadi/forma/renderers/jsonbpersist"
)

// registerTestEntity applies migrations for entitySpec and registers it into
// reg via RegisterArtifactManifest — the same non-filesystem registration
// path the Control Plane artifact deployer uses — so GetEntityStore works
// without needing YAML files on disk.
func registerTestEntity(t *testing.T, d db.DB, reg *entity.Registry, module, name string, entitySpec spec.EntitySpec) {
	t.Helper()
	meta := spec.Metadata{Name: name, Module: module}
	runner := db.NewMigrationRunner(d, db.DriverSQLite)
	if _, err := runner.ApplyMigrations(context.Background(), []db.EntityMigration{{Metadata: meta, EntitySpec: entitySpec}}); err != nil {
		t.Fatalf("ApplyMigrations(%s/%s): %v", module, name, err)
	}
	raw := manifest.RawManifest{
		Kind:     "Document",
		Metadata: manifest.RawMetadata{Name: name, Module: module},
		Source:   "test",
	}
	if err := reg.RegisterArtifactManifest(raw, &entitySpec); err != nil {
		t.Fatalf("RegisterArtifactManifest(%s/%s): %v", module, name, err)
	}
}

// TestHandleCustomAction_RollsBackBothWritesOnFailure verifies the core
// TxScope guarantee: a custom action whose native impl performs two
// separate store.Update calls, then fails, rolls back BOTH — proving they
// joined one shared request-scoped transaction instead of each
// self-committing independently (today's HandleCreate/HandleUpdate
// behavior, but not what a multi-write custom action needs).
func TestHandleCustomAction_RollsBackBothWritesOnFailure(t *testing.T) {
	dir := t.TempDir()
	d, err := db.OpenSQLite(filepath.Join(dir, "txscope_rollback.db"), nil)
	if err != nil {
		t.Fatalf("OpenSQLite failed: %v", err)
	}
	defer d.Close()

	reg := entity.NewRegistry(d, db.DriverSQLite, dir)
	orderSpec := spec.EntitySpec{
		Version: "v1",
		Plural:  "orders",
		Fields: []spec.Field{
			{Name: "status", Type: spec.FieldString},
		},
	}
	registerTestEntity(t, d, reg, "billing", "order", orderSpec)

	ctx := context.Background()
	store, err := reg.GetEntityStore("billing", "order")
	if err != nil {
		t.Fatalf("GetEntityStore: %v", err)
	}

	id, err := store.Insert(ctx, db.InsertParams{
		WorkspaceID: "t1",
		CreatedBy:   "tester",
		Data:        map[string]any{"status": "draft"},
	})
	if err != nil {
		t.Fatalf("seed insert: %v", err)
	}

	// Native handler: two writes to the SAME entity, reading its own
	// uncommitted write back in between (read-your-own-writes), then a
	// forced failure.
	nativeEx := action.NewNativeExecutor()
	nativeEx.Register("Test.CheckoutFail", func(ctx context.Context, p action.ExecuteParams) (any, error) {
		rec, err := store.GetByID(ctx, db.GetByIDParams{WorkspaceID: p.WorkspaceID, ID: p.ResourceID})
		if err != nil {
			return nil, fmt.Errorf("load: %w", err)
		}
		if _, err := store.Update(ctx, db.UpdateParams{
			WorkspaceID: p.WorkspaceID, ID: p.ResourceID, Version: rec.Version,
			UpdatedBy: "native", Data: map[string]any{"status": "step1"},
		}); err != nil {
			return nil, fmt.Errorf("update 1: %w", err)
		}

		rec2, err := store.GetByID(ctx, db.GetByIDParams{WorkspaceID: p.WorkspaceID, ID: p.ResourceID})
		if err != nil {
			return nil, fmt.Errorf("reload: %w", err)
		}
		if rec2.Data["status"] != "step1" {
			return nil, fmt.Errorf("read-your-own-writes failed: expected status=step1, got %v", rec2.Data["status"])
		}

		if _, err := store.Update(ctx, db.UpdateParams{
			WorkspaceID: p.WorkspaceID, ID: p.ResourceID, Version: rec2.Version,
			UpdatedBy: "native", Data: map[string]any{"status": "step2"},
		}); err != nil {
			return nil, fmt.Errorf("update 2: %w", err)
		}

		return nil, fmt.Errorf("forced failure after two writes")
	})

	disp := action.NewDispatcher()
	disp.RegisterExecutor(spec.ImplNative, nativeEx)
	disp.SetNativeExecutor(nativeEx)

	factory := NewHandlerFactory(reg)
	factory.SetDispatcher(disp)

	actionSpec := spec.Action{
		Name: "checkout",
		Impl: &spec.ImplDecl{Type: spec.ImplNative, Ref: "Test.CheckoutFail"},
	}
	handlerFunc := factory.HandleCustomAction("billing", "order", "checkout", actionSpec, "")

	req := httptest.NewRequest("POST", "/billing/orders/"+id+"/checkout", nil)
	req.SetPathValue("id", id)
	req = req.WithContext(WithWorkspace(req.Context(), "t1"))
	req = req.WithContext(WithUser(req.Context(), "tester"))
	rr := httptest.NewRecorder()

	handlerFunc(rr, req)

	if rr.Code != 500 {
		t.Fatalf("expected 500 (forced failure), got %d: %s", rr.Code, rr.Body.String())
	}

	final, err := store.GetByID(ctx, db.GetByIDParams{WorkspaceID: "t1", ID: id})
	if err != nil {
		t.Fatalf("final GetByID: %v", err)
	}
	if final.Data["status"] != "draft" {
		t.Fatalf("expected both writes rolled back (status still draft), got %v", final.Data["status"])
	}
}

// TestHandleCustomAction_CrossStoreErrors verifies the other half of the
// contract: a custom action that mutates two genuinely different stores
// within one execution must error, not silently span two connections.
func TestHandleCustomAction_CrossStoreErrors(t *testing.T) {
	dir := t.TempDir()
	d, err := db.OpenSQLite(filepath.Join(dir, "txscope_cross_a.db"), nil)
	if err != nil {
		t.Fatalf("OpenSQLite failed: %v", err)
	}
	defer d.Close()

	otherDir := t.TempDir()
	otherD, err := db.OpenSQLite(filepath.Join(otherDir, "txscope_cross_b.db"), nil)
	if err != nil {
		t.Fatalf("OpenSQLite failed (other store): %v", err)
	}
	defer otherD.Close()

	reg := entity.NewRegistry(d, db.DriverSQLite, dir)
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

	// A second entity store, deliberately backed by a DIFFERENT db.DB —
	// standing in for a module bound to a different physical Datastore
	// (Fase 2.9, not implemented yet, but this simulates the boundary).
	otherMeta := spec.Metadata{Name: "shipment", Module: "logistics"}
	otherSpec := &spec.EntitySpec{Version: "v1", Plural: "shipments", Fields: []spec.Field{{Name: "status", Type: spec.FieldString}}}
	otherRunner := db.NewMigrationRunner(otherD, db.DriverSQLite)
	if _, err := otherRunner.ApplyMigrations(ctx, []db.EntityMigration{{Metadata: otherMeta, EntitySpec: *otherSpec}}); err != nil {
		t.Fatalf("ApplyMigrations (other store): %v", err)
	}
	otherStore := db.NewEntityStore(otherD, db.DriverSQLite, otherMeta, otherSpec)

	nativeEx := action.NewNativeExecutor()
	nativeEx.Register("Test.CrossStore", func(ctx context.Context, p action.ExecuteParams) (any, error) {
		rec, err := store.GetByID(ctx, db.GetByIDParams{WorkspaceID: p.WorkspaceID, ID: p.ResourceID})
		if err != nil {
			return nil, fmt.Errorf("load: %w", err)
		}
		if _, err := store.Update(ctx, db.UpdateParams{
			WorkspaceID: p.WorkspaceID, ID: p.ResourceID, Version: rec.Version,
			UpdatedBy: "native", Data: map[string]any{"status": "step1"},
		}); err != nil {
			return nil, fmt.Errorf("update local store: %w", err)
		}

		// A different store now attempts to join the same request scope —
		// this must fail with ErrCrossStoreTx.
		if _, err := otherStore.Insert(ctx, db.InsertParams{
			WorkspaceID: p.WorkspaceID, CreatedBy: "native", Data: map[string]any{"status": "new"},
		}); err != nil {
			return nil, err
		}
		return nil, nil
	})

	disp := action.NewDispatcher()
	disp.RegisterExecutor(spec.ImplNative, nativeEx)
	disp.SetNativeExecutor(nativeEx)

	factory := NewHandlerFactory(reg)
	factory.SetDispatcher(disp)

	actionSpec := spec.Action{
		Name: "cross",
		Impl: &spec.ImplDecl{Type: spec.ImplNative, Ref: "Test.CrossStore"},
	}
	handlerFunc := factory.HandleCustomAction("billing", "order", "cross", actionSpec, "")

	req := httptest.NewRequest("POST", "/billing/orders/"+id+"/cross", nil)
	req.SetPathValue("id", id)
	req = req.WithContext(WithWorkspace(req.Context(), "t1"))
	req = req.WithContext(WithUser(req.Context(), "tester"))
	rr := httptest.NewRecorder()

	handlerFunc(rr, req)

	if rr.Code != 500 {
		t.Fatalf("expected 500 (cross-store error), got %d: %s", rr.Code, rr.Body.String())
	}
	if !contains(rr.Body.String(), "CROSS_STORE_TX") {
		t.Fatalf("expected CROSS_STORE_TX error code in response, got: %s", rr.Body.String())
	}

	// The local store's write must also have rolled back.
	final, err := store.GetByID(ctx, db.GetByIDParams{WorkspaceID: "t1", ID: id})
	if err != nil {
		t.Fatalf("final GetByID: %v", err)
	}
	if final.Data["status"] != "draft" {
		t.Fatalf("expected local write rolled back after cross-store error, got %v", final.Data["status"])
	}
}

// setupCrossModuleSameStore registers TWO entities in DIFFERENT modules
// ("billing/order" and "pharmacy/medicine") but backed by the SAME
// underlying db.DB — the normal, single-Datastore-per-App reality this
// codebase runs in today. Returns both stores and their seeded record ids.
func setupCrossModuleSameStore(t *testing.T) (reg *entity.Registry, orderStore, medStore *db.EntityStore, orderID, medID string) {
	t.Helper()
	dir := t.TempDir()
	d, err := db.OpenSQLite(filepath.Join(dir, "txscope_cross_module.db"), nil)
	if err != nil {
		t.Fatalf("OpenSQLite failed: %v", err)
	}
	t.Cleanup(func() { d.Close() })

	reg = entity.NewRegistry(d, db.DriverSQLite, dir)
	registerTestEntity(t, d, reg, "billing", "order", spec.EntitySpec{
		Version: "v1", Plural: "orders",
		Fields: []spec.Field{{Name: "status", Type: spec.FieldString}},
	})
	registerTestEntity(t, d, reg, "pharmacy", "medicine", spec.EntitySpec{
		Version: "v1", Plural: "medicines",
		Fields: []spec.Field{{Name: "status", Type: spec.FieldString}},
	})

	ctx := context.Background()
	orderStore, err = reg.GetEntityStore("billing", "order")
	if err != nil {
		t.Fatalf("GetEntityStore(billing/order): %v", err)
	}
	medStore, err = reg.GetEntityStore("pharmacy", "medicine")
	if err != nil {
		t.Fatalf("GetEntityStore(pharmacy/medicine): %v", err)
	}

	orderID, err = orderStore.Insert(ctx, db.InsertParams{WorkspaceID: "t1", CreatedBy: "tester", Data: map[string]any{"status": "draft"}})
	if err != nil {
		t.Fatalf("seed order insert: %v", err)
	}
	medID, err = medStore.Insert(ctx, db.InsertParams{WorkspaceID: "t1", CreatedBy: "tester", Data: map[string]any{"status": "in-stock"}})
	if err != nil {
		t.Fatalf("seed medicine insert: %v", err)
	}
	return reg, orderStore, medStore, orderID, medID
}

// TestHandleCustomAction_CrossModuleSameStore_CommitsAtomically proves the
// user's first correction to the initial TxScope design: a transaction MAY
// span multiple modules as long as they share one physical store — it is
// NOT an error, and both writes commit together.
func TestHandleCustomAction_CrossModuleSameStore_CommitsAtomically(t *testing.T) {
	reg, orderStore, medStore, orderID, medID := setupCrossModuleSameStore(t)
	ctx := context.Background()

	nativeEx := action.NewNativeExecutor()
	nativeEx.Register("Test.CrossModuleOK", func(ctx context.Context, p action.ExecuteParams) (any, error) {
		orderRec, err := orderStore.GetByID(ctx, db.GetByIDParams{WorkspaceID: p.WorkspaceID, ID: orderID})
		if err != nil {
			return nil, fmt.Errorf("load order: %w", err)
		}
		if _, err := orderStore.Update(ctx, db.UpdateParams{
			WorkspaceID: p.WorkspaceID, ID: orderID, Version: orderRec.Version,
			UpdatedBy: "native", Data: map[string]any{"status": "confirmed"},
		}); err != nil {
			return nil, fmt.Errorf("update order (billing module): %w", err)
		}

		medRec, err := medStore.GetByID(ctx, db.GetByIDParams{WorkspaceID: p.WorkspaceID, ID: medID})
		if err != nil {
			return nil, fmt.Errorf("load medicine: %w", err)
		}
		if _, err := medStore.Update(ctx, db.UpdateParams{
			WorkspaceID: p.WorkspaceID, ID: medID, Version: medRec.Version,
			UpdatedBy: "native", Data: map[string]any{"status": "reserved"},
		}); err != nil {
			return nil, fmt.Errorf("update medicine (pharmacy module): %w", err)
		}
		return nil, nil
	})

	disp := action.NewDispatcher()
	disp.RegisterExecutor(spec.ImplNative, nativeEx)
	disp.SetNativeExecutor(nativeEx)

	factory := NewHandlerFactory(reg)
	factory.SetDispatcher(disp)

	actionSpec := spec.Action{Name: "confirm", Impl: &spec.ImplDecl{Type: spec.ImplNative, Ref: "Test.CrossModuleOK"}}
	handlerFunc := factory.HandleCustomAction("billing", "order", "confirm", actionSpec, "")

	req := httptest.NewRequest("POST", "/billing/orders/"+orderID+"/confirm", nil)
	req.SetPathValue("id", orderID)
	req = req.WithContext(WithWorkspace(req.Context(), "t1"))
	req = req.WithContext(WithUser(req.Context(), "tester"))
	rr := httptest.NewRecorder()
	handlerFunc(rr, req)

	if rr.Code != 200 {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}

	orderFinal, err := orderStore.GetByID(ctx, db.GetByIDParams{WorkspaceID: "t1", ID: orderID})
	if err != nil {
		t.Fatalf("final order GetByID: %v", err)
	}
	if orderFinal.Data["status"] != "confirmed" {
		t.Fatalf("expected order status=confirmed, got %v", orderFinal.Data["status"])
	}
	medFinal, err := medStore.GetByID(ctx, db.GetByIDParams{WorkspaceID: "t1", ID: medID})
	if err != nil {
		t.Fatalf("final medicine GetByID: %v", err)
	}
	if medFinal.Data["status"] != "reserved" {
		t.Fatalf("expected medicine status=reserved, got %v", medFinal.Data["status"])
	}
}

// TestHandleCustomAction_CrossModuleSameStore_RollsBackAtomically is the
// failure-mode counterpart: two writes to two DIFFERENT modules, same
// store, then a forced failure — BOTH must roll back together.
func TestHandleCustomAction_CrossModuleSameStore_RollsBackAtomically(t *testing.T) {
	reg, orderStore, medStore, orderID, medID := setupCrossModuleSameStore(t)
	ctx := context.Background()

	nativeEx := action.NewNativeExecutor()
	nativeEx.Register("Test.CrossModuleFail", func(ctx context.Context, p action.ExecuteParams) (any, error) {
		orderRec, err := orderStore.GetByID(ctx, db.GetByIDParams{WorkspaceID: p.WorkspaceID, ID: orderID})
		if err != nil {
			return nil, fmt.Errorf("load order: %w", err)
		}
		if _, err := orderStore.Update(ctx, db.UpdateParams{
			WorkspaceID: p.WorkspaceID, ID: orderID, Version: orderRec.Version,
			UpdatedBy: "native", Data: map[string]any{"status": "confirmed"},
		}); err != nil {
			return nil, fmt.Errorf("update order: %w", err)
		}

		medRec, err := medStore.GetByID(ctx, db.GetByIDParams{WorkspaceID: p.WorkspaceID, ID: medID})
		if err != nil {
			return nil, fmt.Errorf("load medicine: %w", err)
		}
		if _, err := medStore.Update(ctx, db.UpdateParams{
			WorkspaceID: p.WorkspaceID, ID: medID, Version: medRec.Version,
			UpdatedBy: "native", Data: map[string]any{"status": "reserved"},
		}); err != nil {
			return nil, fmt.Errorf("update medicine: %w", err)
		}

		return nil, fmt.Errorf("forced failure after both cross-module writes")
	})

	disp := action.NewDispatcher()
	disp.RegisterExecutor(spec.ImplNative, nativeEx)
	disp.SetNativeExecutor(nativeEx)

	factory := NewHandlerFactory(reg)
	factory.SetDispatcher(disp)

	actionSpec := spec.Action{Name: "confirm", Impl: &spec.ImplDecl{Type: spec.ImplNative, Ref: "Test.CrossModuleFail"}}
	handlerFunc := factory.HandleCustomAction("billing", "order", "confirm", actionSpec, "")

	req := httptest.NewRequest("POST", "/billing/orders/"+orderID+"/confirm", nil)
	req.SetPathValue("id", orderID)
	req = req.WithContext(WithWorkspace(req.Context(), "t1"))
	req = req.WithContext(WithUser(req.Context(), "tester"))
	rr := httptest.NewRecorder()
	handlerFunc(rr, req)

	if rr.Code != 500 {
		t.Fatalf("expected 500 (forced failure), got %d: %s", rr.Code, rr.Body.String())
	}

	orderFinal, err := orderStore.GetByID(ctx, db.GetByIDParams{WorkspaceID: "t1", ID: orderID})
	if err != nil {
		t.Fatalf("final order GetByID: %v", err)
	}
	if orderFinal.Data["status"] != "draft" {
		t.Fatalf("expected order write rolled back (still draft), got %v", orderFinal.Data["status"])
	}
	medFinal, err := medStore.GetByID(ctx, db.GetByIDParams{WorkspaceID: "t1", ID: medID})
	if err != nil {
		t.Fatalf("final medicine GetByID: %v", err)
	}
	if medFinal.Data["status"] != "in-stock" {
		t.Fatalf("expected medicine write rolled back too (still in-stock) — proves cross-module writes shared one transaction, got %v", medFinal.Data["status"])
	}
}
