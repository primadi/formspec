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
	res.SetLoadFunc(func(module, entity, id string) (map[string]any, int, error) {
		loadModule, loadEntity, loadID = module, entity, id
		return map[string]any{"stock": 10}, 1, nil
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
	res.SetLoadFunc(func(module, entity, id string) (map[string]any, int, error) {
		loadModule, loadEntity = module, entity
		return map[string]any{}, 1, nil
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

func TestResourceAPI_Call_CrossModule(t *testing.T) {
	scriptPath := writeScript(t, ""+
		"def execute(resource, params, ctx):\n"+
		"    resource.call(\"pharmacy.medicine\", \"restock\", {\"qty\": 10})\n"+
		"    return ok({})\n")

	var callModule, callEntity, callAction string
	res := NewResourceAPI("clinic", "visit", "visit-1", 1, map[string]any{})
	res.SetCallFunc(func(module, entity, action string, params map[string]any) (any, error) {
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
	res.SetCallFunc(func(module, entity, action string, params map[string]any) (any, error) {
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
