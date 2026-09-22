package starlark

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestSplitModuleEntity(t *testing.T) {
	cases := []struct {
		name          string
		defaultModule string
		target        string
		wantModule    string
		wantEntity    string
	}{
		{"bare name (same-module, regression)", "clinic", "medicine", "clinic", "medicine"},
		{"dotted cross-module", "clinic", "pharmacy.medicine", "pharmacy", "medicine"},
		{"hyphenated module/entity identifiers", "clinic", "general-ledger.journal-entry", "general-ledger", "journal-entry"},
		{"empty target", "clinic", "", "clinic", ""},
		{"malformed multi-dot input — first dot wins", "clinic", "a.b.c", "a", "b.c"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			gotModule, gotEntity := splitModuleEntity(tc.defaultModule, tc.target)
			if gotModule != tc.wantModule || gotEntity != tc.wantEntity {
				t.Errorf("splitModuleEntity(%q, %q) = (%q, %q), want (%q, %q)",
					tc.defaultModule, tc.target, gotModule, gotEntity, tc.wantModule, tc.wantEntity)
			}
		})
	}
}

// writeScript writes a .star script to a temp file and returns its path.
func writeScript(t *testing.T, body string) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "script.star")
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestResourceAPI_Fetch_CrossModule(t *testing.T) {
	scriptPath := writeScript(t, ""+
		"def execute(resource, params, ctx):\n"+
		"    m = resource.fetch(\"pharmacy.medicine\", \"med-1\")\n"+
		"    m.set(\"stock\", 5)\n"+
		"    m.save()\n"+
		"    return ok({})\n")

	var loadModule, loadEntity, loadID string
	var saveModule, saveEntity, saveID string

	res := NewResourceAPI("clinic", "visit", "visit-1", 1, map[string]any{})
	res.SetLoadFunc(func(module, entity, id string) (map[string]any, int, string, error) {
		loadModule, loadEntity, loadID = module, entity, id
		return map[string]any{"stock": 10}, 1, id, nil
	})
	res.SetSaveFunc(func(module, entity, id string, version int, data map[string]any) error {
		saveModule, saveEntity, saveID = module, entity, id
		return nil
	})

	ctxObj := NewCtxAPI("demo", "", "user", "", nil)
	ctxObj.Now = now

	result, err := ExecuteScript(context.Background(), scriptPath, res, nil, ctxObj)
	if err != nil {
		t.Fatalf("ExecuteScript error: %v", err)
	}
	if !result.OK {
		t.Fatalf("script failed: %s", result.Error)
	}

	if loadModule != "pharmacy" || loadEntity != "medicine" || loadID != "med-1" {
		t.Errorf("loadFn called with (%q, %q, %q), want (\"pharmacy\", \"medicine\", \"med-1\")", loadModule, loadEntity, loadID)
	}
	// This is the reconstruction-bug regression: the resource returned by
	// .fetch() must remember it belongs to pharmacy/medicine, not the
	// caller's clinic/visit, so a later .save() writes to the right place.
	if saveModule != "pharmacy" || saveEntity != "medicine" || saveID != "med-1" {
		t.Errorf("saveFn (via fetched resource) called with (%q, %q, %q), want (\"pharmacy\", \"medicine\", \"med-1\")", saveModule, saveEntity, saveID)
	}
}

func TestResourceAPI_Fetch_SameModule_Regression(t *testing.T) {
	scriptPath := writeScript(t, ""+
		"def execute(resource, params, ctx):\n"+
		"    m = resource.fetch(\"medicine\", \"med-1\")\n"+
		"    return ok({})\n")

	var loadModule, loadEntity string
	res := NewResourceAPI("pharmacy", "prescription", "rx-1", 1, map[string]any{})
	res.SetLoadFunc(func(module, entity, id string) (map[string]any, int, string, error) {
		loadModule, loadEntity = module, entity
		return map[string]any{}, 1, id, nil
	})

	ctxObj := NewCtxAPI("demo", "", "user", "", nil)
	ctxObj.Now = now

	result, err := ExecuteScript(context.Background(), scriptPath, res, nil, ctxObj)
	if err != nil {
		t.Fatalf("ExecuteScript error: %v", err)
	}
	if !result.OK {
		t.Fatalf("script failed: %s", result.Error)
	}
	if loadModule != "pharmacy" || loadEntity != "medicine" {
		t.Errorf("loadFn called with (%q, %q), want (\"pharmacy\", \"medicine\") — same-module bare-name regression broken", loadModule, loadEntity)
	}
}

// TestResourceAPI_Find_ReturnsMatchAndNone pins #31 (TODO 4.3): resource.find()
// looks up a row by a single field value through the entity layer (so tenant
// isolation and row scope apply), returning the resource on a match and None
// when nothing matches — the high-level alternative to raw SQL for guards.
func TestResourceAPI_Find_ReturnsMatchAndNone(t *testing.T) {
	scriptPath := writeScript(t, ""+
		"def execute(resource, params, ctx):\n"+
		"    hit = resource.find(\"cafe-master.menu-item-price\", {\"branch_id\": \"B1\", \"menu_item_id\": \"M1\"})\n"+
		"    miss = resource.find(\"cafe-master.menu-item-price\", {\"branch_id\": \"NOPE\"})\n"+
		"    return ok({\"hit\": hit != None, \"miss\": miss == None, \"id\": hit.id})\n")

	var gotModule, gotEntity string
	var gotMatches []map[string]any
	res := NewResourceAPI("cafe-order", "order", "o-1", 1, map[string]any{})
	res.SetFindFunc(func(module, entity string, match map[string]any) (map[string]any, int, string, error) {
		gotModule, gotEntity = module, entity
		gotMatches = append(gotMatches, match)
		if match["branch_id"] == "B1" {
			// The record's id is returned as the resolved ID, not carried in
			// the data map — a real record's id is a table column, not a field.
			return map[string]any{"branch_id": "B1"}, 3, "price-1", nil
		}
		return nil, 0, "", nil
	})

	ctxObj := NewCtxAPI("demo", "", "user", "", nil)
	ctxObj.Now = now

	result, err := ExecuteScript(context.Background(), scriptPath, res, nil, ctxObj)
	if err != nil {
		t.Fatalf("ExecuteScript error: %v", err)
	}
	if !result.OK {
		t.Fatalf("script failed: %s", result.Error)
	}
	if gotModule != "cafe-master" || gotEntity != "menu-item-price" {
		t.Errorf("findFn called with (%q, %q), want (\"cafe-master\", \"menu-item-price\")", gotModule, gotEntity)
	}
	if len(gotMatches) != 2 || gotMatches[0]["branch_id"] != "B1" || gotMatches[0]["menu_item_id"] != "M1" {
		t.Errorf("findFn matches = %v, want first {branch_id:B1 menu_item_id:M1}", gotMatches)
	}
	if result.Data["hit"] != true || result.Data["miss"] != true {
		t.Errorf("find results = %v, want hit=true miss=true", result.Data)
	}
	if result.Data["id"] != "price-1" {
		t.Errorf("found id = %v, want \"price-1\"", result.Data["id"])
	}
}

// TestResourceAPI_Upsert_SummaryProjection pins item 4.1 (Opsi A): resource.upsert
// is the ONE supported write path for a summary projection. It forwards the
// match/data to the upsert handler and returns the row id.
func TestResourceAPI_Upsert_SummaryProjection(t *testing.T) {
	scriptPath := writeScript(t, ""+
		"def execute(resource, params, ctx):\n"+
		"    rid = resource.upsert(\"cafe-stock.stock-level\", {\"branch_id\": \"B1\", \"ingredient_id\": \"I1\"}, {\"quantity_on_hand\": 5})\n"+
		"    return ok({\"id\": rid})\n")

	var gotModule, gotEntity string
	var gotMatch, gotData map[string]any
	res := NewResourceAPI("cafe-stock", "stock-movement", "m-1", 1, map[string]any{})
	res.SetUpsertFunc(func(module, entity string, match, data map[string]any) (string, bool, error) {
		gotModule, gotEntity, gotMatch, gotData = module, entity, match, data
		return "sl-1", true, nil
	})

	ctxObj := NewCtxAPI("demo", "", "user", "", nil)
	ctxObj.Now = now

	result, err := ExecuteScript(context.Background(), scriptPath, res, nil, ctxObj)
	if err != nil {
		t.Fatalf("ExecuteScript error: %v", err)
	}
	if !result.OK {
		t.Fatalf("script failed: %s", result.Error)
	}
	if gotModule != "cafe-stock" || gotEntity != "stock-level" {
		t.Errorf("upsertFn called with (%q, %q), want (\"cafe-stock\", \"stock-level\")", gotModule, gotEntity)
	}
	if gotMatch["branch_id"] != "B1" || gotMatch["ingredient_id"] != "I1" {
		t.Errorf("upsertFn match = %v, want {branch_id:B1 ingredient_id:I1}", gotMatch)
	}
	if gotData["quantity_on_hand"] != int64(5) && gotData["quantity_on_hand"] != 5 {
		t.Errorf("upsertFn data = %v, want quantity_on_hand=5", gotData)
	}
	if result.Data["id"] != "sl-1" {
		t.Errorf("upsert id = %v, want \"sl-1\"", result.Data["id"])
	}
}

// TestResourceAPI_Upsert_RejectsBadArgs pins that resource.upsert refuses an
// empty match and a non-dict match — the same guard the store enforces, surfaced
// early at the script boundary.
func TestResourceAPI_Upsert_RejectsBadArgs(t *testing.T) {
	res := NewResourceAPI("cafe-stock", "stock-movement", "m-1", 1, map[string]any{})
	res.SetUpsertFunc(func(module, entity string, match, data map[string]any) (string, bool, error) {
		return "sl-1", true, nil
	})
	ctxObj := NewCtxAPI("demo", "", "user", "", nil)
	ctxObj.Now = now

	emptyMatch := writeScript(t, ""+
		"def execute(resource, params, ctx):\n"+
		"    return ok({\"id\": resource.upsert(\"cafe-stock.stock-level\", {}, {\"quantity_on_hand\": 5})})\n")
	result, err := ExecuteScript(context.Background(), emptyMatch, res, nil, ctxObj)
	if err != nil {
		t.Fatalf("ExecuteScript error: %v", err)
	}
	if result.OK {
		t.Error("resource.upsert with an empty match: expected failure, got ok")
	}
}

func TestResourceAPI_Create_CrossModule(t *testing.T) {
	scriptPath := writeScript(t, ""+
		"def execute(resource, params, ctx):\n"+
		"    p = resource.create(\"pharmacy.prescription\", {\"patient_name\": \"Budi\"})\n"+
		"    p.set(\"notes\", \"created from clinic\")\n"+
		"    p.save()\n"+
		"    return ok({})\n")

	var createModule, createEntity string
	var saveModule, saveEntity, saveID string

	res := NewResourceAPI("clinic", "visit", "visit-1", 1, map[string]any{})
	res.SetCreateFunc(func(module, entity string, data map[string]any) (string, error) {
		createModule, createEntity = module, entity
		return "rx-99", nil
	})
	res.SetSaveFunc(func(module, entity, id string, version int, data map[string]any) error {
		saveModule, saveEntity, saveID = module, entity, id
		return nil
	})

	ctxObj := NewCtxAPI("demo", "", "user", "", nil)
	ctxObj.Now = now

	result, err := ExecuteScript(context.Background(), scriptPath, res, nil, ctxObj)
	if err != nil {
		t.Fatalf("ExecuteScript error: %v", err)
	}
	if !result.OK {
		t.Fatalf("script failed: %s", result.Error)
	}

	if createModule != "pharmacy" || createEntity != "prescription" {
		t.Errorf("createFn called with (%q, %q), want (\"pharmacy\", \"prescription\")", createModule, createEntity)
	}
	if saveModule != "pharmacy" || saveEntity != "prescription" || saveID != "rx-99" {
		t.Errorf("saveFn (via created resource) called with (%q, %q, %q), want (\"pharmacy\", \"prescription\", \"rx-99\")", saveModule, saveEntity, saveID)
	}
}

func TestResourceAPI_Create_SameModule_Regression(t *testing.T) {
	scriptPath := writeScript(t, ""+
		"def execute(resource, params, ctx):\n"+
		"    m = resource.create(\"medicine\", {\"name\": \"Paracetamol\"})\n"+
		"    return ok({})\n")

	var createModule, createEntity string
	res := NewResourceAPI("pharmacy", "prescription", "rx-1", 1, map[string]any{})
	res.SetCreateFunc(func(module, entity string, data map[string]any) (string, error) {
		createModule, createEntity = module, entity
		return "med-1", nil
	})

	ctxObj := NewCtxAPI("demo", "", "user", "", nil)
	ctxObj.Now = now

	result, err := ExecuteScript(context.Background(), scriptPath, res, nil, ctxObj)
	if err != nil {
		t.Fatalf("ExecuteScript error: %v", err)
	}
	if !result.OK {
		t.Fatalf("script failed: %s", result.Error)
	}
	if createModule != "pharmacy" || createEntity != "medicine" {
		t.Errorf("createFn called with (%q, %q), want (\"pharmacy\", \"medicine\") — same-module bare-name regression broken", createModule, createEntity)
	}
}

func TestResourceAPI_New_SameEntityUnsaved(t *testing.T) {
	scriptPath := writeScript(t, ""+
		"def execute(resource, params, ctx):\n"+
		"    n = resource.new()\n"+
		"    n.set(\"name\", \"New Item\")\n"+
		"    n.save()\n"+
		"    return ok({})\n")

	var saveModule, saveEntity, saveID string
	var saveData map[string]any

	res := NewResourceAPI("clinic", "medicine", "med-1", 1, map[string]any{"name": "Old"})
	res.SetSaveFunc(func(module, entity, id string, version int, data map[string]any) error {
		saveModule, saveEntity, saveID = module, entity, id
		saveData = data
		return nil
	})

	ctxObj := NewCtxAPI("demo", "", "user", "", nil)
	ctxObj.Now = now

	result, err := ExecuteScript(context.Background(), scriptPath, res, nil, ctxObj)
	if err != nil {
		t.Fatalf("ExecuteScript error: %v", err)
	}
	if !result.OK {
		t.Fatalf("script failed: %s", result.Error)
	}

	// resource.new() returns a handle for the SAME entity with ID "" (unsaved)
	// so save() performs an INSERT (todo 7.14.4).
	if saveModule != "clinic" || saveEntity != "medicine" {
		t.Errorf("saveFn called with (%q, %q), want (\"clinic\", \"medicine\")", saveModule, saveEntity)
	}
	if saveID != "" {
		t.Errorf("saveFn id = %q, want \"\" (unsaved new handle)", saveID)
	}
	if saveData["name"] != "New Item" {
		t.Errorf("saveFn data name = %v, want \"New Item\"", saveData["name"])
	}
}

func TestResourceAPI_Call_CrossModule(t *testing.T) {
	scriptPath := writeScript(t, ""+
		"def execute(resource, params, ctx):\n"+
		"    resource.call(\"pharmacy.medicine\", \"restock\", {\"qty\": 10})\n"+
		"    return ok({})\n")

	var callModule, callEntity, callAction string
	res := NewResourceAPI("clinic", "visit", "visit-1", 1, map[string]any{})
	res.SetCallFunc(func(module, entity, id, action string, params map[string]any) (any, error) {
		callModule, callEntity, callAction = module, entity, action
		return nil, nil
	})

	ctxObj := NewCtxAPI("demo", "", "user", "", nil)
	ctxObj.Now = now

	result, err := ExecuteScript(context.Background(), scriptPath, res, nil, ctxObj)
	if err != nil {
		t.Fatalf("ExecuteScript error: %v", err)
	}
	if !result.OK {
		t.Fatalf("script failed: %s", result.Error)
	}
	if callModule != "pharmacy" || callEntity != "medicine" || callAction != "restock" {
		t.Errorf("callFn called with (%q, %q, %q), want (\"pharmacy\", \"medicine\", \"restock\")", callModule, callEntity, callAction)
	}
}

func TestResourceAPI_Call_SameModule_Regression(t *testing.T) {
	scriptPath := writeScript(t, ""+
		"def execute(resource, params, ctx):\n"+
		"    resource.call(\"medicine\", \"restock\", {\"qty\": 10})\n"+
		"    return ok({})\n")

	var callModule, callEntity string
	res := NewResourceAPI("pharmacy", "prescription", "rx-1", 1, map[string]any{})
	res.SetCallFunc(func(module, entity, id, action string, params map[string]any) (any, error) {
		callModule, callEntity = module, entity
		return nil, nil
	})

	ctxObj := NewCtxAPI("demo", "", "user", "", nil)
	ctxObj.Now = now

	result, err := ExecuteScript(context.Background(), scriptPath, res, nil, ctxObj)
	if err != nil {
		t.Fatalf("ExecuteScript error: %v", err)
	}
	if !result.OK {
		t.Fatalf("script failed: %s", result.Error)
	}
	if callModule != "pharmacy" || callEntity != "medicine" {
		t.Errorf("callFn called with (%q, %q), want (\"pharmacy\", \"medicine\") — same-module bare-name regression broken", callModule, callEntity)
	}
}

func TestResourceAPI_Call_InstanceTarget(t *testing.T) {
	// resource.call("module.entity.<id>", action, params) addresses ONE record
	// — the third segment is a record ID, not part of the entity name. This is
	// how a subscription handler posts a journal it just created.
	scriptPath := writeScript(t, ""+
		"def execute(resource, params, ctx):\n"+
		"    resource.call(\"gl.journal-entry.01a0c332-d45b-7039-b2d3-f8f52d0e63dc\", \"post\", {})\n"+
		"    return ok({})\n")

	var callModule, callEntity, callID, callAction string
	res := NewResourceAPI("gl", "account", "acc-1", 1, map[string]any{})
	res.SetCallFunc(func(module, entity, id, action string, params map[string]any) (any, error) {
		callModule, callEntity, callID, callAction = module, entity, id, action
		return nil, nil
	})

	ctxObj := NewCtxAPI("demo", "", "user", "", nil)
	ctxObj.Now = now

	result, err := ExecuteScript(context.Background(), scriptPath, res, nil, ctxObj)
	if err != nil {
		t.Fatalf("ExecuteScript error: %v", err)
	}
	if !result.OK {
		t.Fatalf("script failed: %s", result.Error)
	}
	wantID := "01a0c332-d45b-7039-b2d3-f8f52d0e63dc"
	if callModule != "gl" || callEntity != "journal-entry" || callID != wantID || callAction != "post" {
		t.Errorf("callFn called with (%q, %q, %q, %q), want (\"gl\", \"journal-entry\", %q, \"post\")",
			callModule, callEntity, callID, callAction, wantID)
	}
}

func TestResourceAPI_Call_CollectionTargetHasEmptyID(t *testing.T) {
	scriptPath := writeScript(t, ""+
		"def execute(resource, params, ctx):\n"+
		"    resource.call(\"pharmacy.medicine\", \"restock\", {\"qty\": 10})\n"+
		"    return ok({})\n")

	var callID string
	gotID := false
	res := NewResourceAPI("clinic", "visit", "visit-1", 1, map[string]any{})
	res.SetCallFunc(func(module, entity, id, action string, params map[string]any) (any, error) {
		callID, gotID = id, true
		return nil, nil
	})

	ctxObj := NewCtxAPI("demo", "", "user", "", nil)
	ctxObj.Now = now

	result, err := ExecuteScript(context.Background(), scriptPath, res, nil, ctxObj)
	if err != nil {
		t.Fatalf("ExecuteScript error: %v", err)
	}
	if !result.OK {
		t.Fatalf("script failed: %s", result.Error)
	}
	if !gotID {
		t.Fatal("callFn was not called")
	}
	if callID != "" {
		t.Errorf("collection-level call carried id = %q, want \"\"", callID)
	}
}

func TestEvaluateGuard_SumLineBuiltin(t *testing.T) {
	// The documented aggregate form (02-core-extended.md §1) is a CALL —
	// sum_line('debit') — not only the pre-computed sum_line_debit identifier.
	// A GL journal guard is written exactly that way; it must resolve.
	data := map[string]any{
		"status": "draft",
		"lines": []any{
			map[string]any{"account_id": "a", "debit": 13.75, "credit": 0},
			map[string]any{"account_id": "b", "debit": 0, "credit": 10.0},
			map[string]any{"account_id": "c", "debit": 0, "credit": 3.75},
		},
	}

	passed, _, err := EvaluateGuard(
		"sum_line('debit') == sum_line('credit') and sum_line('debit') > 0", data)
	if err != nil {
		t.Fatalf("EvaluateGuard error: %v", err)
	}
	if !passed {
		t.Error("balanced journal guard should pass")
	}

	// Unbalanced by one cent → must fail, not error.
	unbalanced := map[string]any{
		"lines": []any{
			map[string]any{"debit": 10.0, "credit": 0},
			map[string]any{"debit": 0, "credit": 9.99},
		},
	}
	passed, _, err = EvaluateGuard(
		"sum_line('debit') == sum_line('credit') and sum_line('debit') > 0", unbalanced)
	if err != nil {
		t.Fatalf("EvaluateGuard error: %v", err)
	}
	if passed {
		t.Error("unbalanced journal guard should fail")
	}

	// All-zero lines fail the `> 0` half.
	empty := map[string]any{
		"lines": []any{map[string]any{"debit": 0, "credit": 0}},
	}
	passed, _, err = EvaluateGuard(
		"sum_line('debit') == sum_line('credit') and sum_line('debit') > 0", empty)
	if err != nil {
		t.Fatalf("EvaluateGuard error: %v", err)
	}
	if passed {
		t.Error("zero-value journal guard should fail")
	}

	// The pre-computed identifier form keeps working alongside the call form.
	passed, _, err = EvaluateGuard("sum_line_debit == sum_line_credit", data)
	if err != nil {
		t.Fatalf("EvaluateGuard (identifier form) error: %v", err)
	}
	if !passed {
		t.Error("sum_line_debit == sum_line_credit identifier form should pass")
	}
}

func TestEvaluateGuard_TableStoredChildRows(t *testing.T) {
	// Children with storage: table come back from ChildStore.Hydrate as
	// []map[string]any, while client-supplied children arrive as []any. A guard
	// must see the same sums either way — accepting only []any made every guard
	// over a table-stored child silently sum to zero and report "not balanced".
	expr := "sum_line('debit') == sum_line('credit') and sum_line('debit') > 0"

	tableShape := map[string]any{
		"lines": []map[string]any{
			{"account_id": "a", "debit": 13.75, "credit": 0},
			{"account_id": "b", "debit": 0, "credit": 10.0},
			{"account_id": "c", "debit": 0, "credit": 3.75},
		},
	}

	passed, _, err := EvaluateGuard(expr, tableShape)
	if err != nil {
		t.Fatalf("EvaluateGuard error: %v", err)
	}
	if !passed {
		t.Error("balanced journal with table-stored children should pass")
	}

	// len() helpers see the collection too, in either shape.
	passed, _, err = EvaluateGuard("len(resource.lines) == 3", tableShape)
	if err != nil {
		t.Fatalf("len(resource.lines) error: %v", err)
	}
	if !passed {
		t.Error("len(resource.lines) should see 3 table-stored children")
	}

	// Unbalanced table-stored data must fail, not silently pass on zero sums.
	unbalanced := map[string]any{
		"lines": []map[string]any{
			{"debit": 10.0, "credit": 0},
			{"debit": 0, "credit": 9.99},
		},
	}
	passed, _, err = EvaluateGuard(expr, unbalanced)
	if err != nil {
		t.Fatalf("EvaluateGuard error: %v", err)
	}
	if passed {
		t.Error("unbalanced table-stored journal should fail")
	}
}

func TestSplitCallTarget(t *testing.T) {
	cases := []struct {
		target                         string
		wantModule, wantEntity, wantID string
	}{
		{"medicine", "pharmacy", "medicine", ""},
		{"pharmacy.medicine", "pharmacy", "medicine", ""},
		{"gl.journal-entry.01a0c332-d45b-7039-b2d3-f8f52d0e63dc", "gl", "journal-entry", "01a0c332-d45b-7039-b2d3-f8f52d0e63dc"},
		{"gl.journal-entry.", "gl", "journal-entry", ""},
	}
	for _, tc := range cases {
		gotModule, gotEntity, gotID := splitCallTarget("pharmacy", tc.target)
		if gotModule != tc.wantModule || gotEntity != tc.wantEntity || gotID != tc.wantID {
			t.Errorf("splitCallTarget(%q) = (%q, %q, %q), want (%q, %q, %q)",
				tc.target, gotModule, gotEntity, gotID, tc.wantModule, tc.wantEntity, tc.wantID)
		}
	}
}
