package genjsonschema

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"testing"
)

// pkgPath is the package the generator reads types from — the same default
// `cmd/formspec-gen-schema` uses.
const pkgPath = "github.com/primadi/formspec/pkg/spec"

// TestGeneratedKindSchemas_HaveNoDanglingRefs guards a failure mode that costs
// real debugging time: a kind schema referencing `#/$defs/<Type>` while the root
// schema does not define `<Type>`.
//
// It is silent at generation time — both files are written and generation
// reports success — and only surfaces later as
//
//	schema: internal: compile Entity schema: json-pointer in
//	".../kind-schema.json#/$defs/RebuildSpec" not found
//
// for *every* manifest of that kind, including manifests that never mention the
// missing type. That is what happened to the summary rebuild contract
// (`sources`/`join_key`/`rebuild`): a mismatched pair of generated files took
// down schema validation for all 20+ Entities in the example tree.
//
// The check is direct rather than emergent: walk every `$ref` in every kind
// schema and require a matching root `$defs` entry.
func TestGeneratedKindSchemas_HaveNoDanglingRefs(t *testing.T) {
	converter := New(pkgPath)
	collect, err := converter.Collect()
	if err != nil {
		t.Fatalf("Collect: %v", err)
	}
	result := converter.Generate(collect)

	if len(result.KindSchemas) == 0 {
		t.Fatal("Generate produced no kind schemas")
	}
	rootDefs := rootDefinitionNames(result)
	if len(rootDefs) == 0 {
		t.Fatal("root schema has no $defs — no kind schema could resolve a $ref")
	}

	for _, entry := range KindMapping() {
		kindSchema, ok := result.KindSchemas[entry.Kind]
		if !ok {
			t.Errorf("kind %s: no schema generated", entry.Kind)
			continue
		}
		raw, err := json.Marshal(kindSchema)
		if err != nil {
			t.Fatalf("kind %s: marshal: %v", entry.Kind, err)
		}
		if missing := danglingRefs(raw, rootDefs); len(missing) > 0 {
			t.Errorf("kind %s: refs with no root $defs entry: %s (add the type to `sharedTypes` in generator.go)",
				entry.Kind, strings.Join(missing, ", "))
		}
	}

	// Walk the root schema too. A `$ref` inside a **shared def** (for example
	// `PrintBodyItem.qrcode → #/$defs/PrintQrcode`) never appears in a kind
	// schema file — the kind schema only names the def — so a guard that walks
	// kind schemas alone cannot see it. That blind spot let a missing
	// `sharedTypes` entry reach runtime as
	//
	//	schema: internal: compile Print schema: json-pointer in
	//	"…/kind-schema.json#/$defs/PrintQrcode" not found
	//
	// for every Print manifest in the tree.
	rootRaw, err := json.Marshal(result.RootSchema)
	if err != nil {
		t.Fatalf("marshal root schema: %v", err)
	}
	if missing := danglingRefs(rootRaw, rootDefs); len(missing) > 0 {
		t.Errorf("root $defs: refs with no matching $defs entry: %s (add the type to `sharedTypes` in generator.go)",
			strings.Join(missing, ", "))
	}
}

// TestFormRenderDecl_AcceptsShorthandAndObject pins kafe ledger #37 (item 5.4):
// FormRenderDecl has a custom UnmarshalYAML that accepts both the scalar
// shorthand (`render: drawer`) and the object form (`render: {mode: drawer}`).
// The generated schema must accept both too — otherwise the loader accepts a
// manifest the editor/schema rejects, a divergence that reads as "the spec is
// wrong" when it is the schema that is stale.
func TestFormRenderDecl_AcceptsShorthandAndObject(t *testing.T) {
	converter := New(pkgPath)
	collect, err := converter.Collect()
	if err != nil {
		t.Fatalf("Collect: %v", err)
	}
	result := converter.Generate(collect)

	def, ok := result.RootSchema.Defs["FormRenderDecl"]
	if !ok {
		t.Fatal("root schema has no FormRenderDecl definition")
	}
	if len(def.OneOf) != 2 {
		t.Fatalf("FormRenderDecl should be oneOf [string, object], got %d branches", len(def.OneOf))
	}
	var hasString, hasObject bool
	for _, branch := range def.OneOf {
		switch branch.Type {
		case "string":
			hasString = true
		case "object":
			hasObject = true
		}
	}
	if !hasString {
		t.Error("FormRenderDecl oneOf is missing the string shorthand branch")
	}
	if !hasObject {
		t.Error("FormRenderDecl oneOf is missing the object branch")
	}
}

// TestDanglingRefs_DetectsMissingDefinition proves the checker above can fail —
// a guard that cannot fail is not a guard.
func TestDanglingRefs_DetectsMissingDefinition(t *testing.T) {
	schema := []byte(`{"properties":{"a":{"$ref":"#/$defs/Present"},"b":{"$ref":"#/$defs/Absent"}}}`)

	missing := danglingRefs(schema, map[string]bool{"Present": true})
	if len(missing) != 1 || missing[0] != "Absent" {
		t.Fatalf("danglingRefs: got %v, want [Absent]", missing)
	}

	if got := danglingRefs(schema, map[string]bool{"Present": true, "Absent": true}); len(got) != 0 {
		t.Errorf("danglingRefs: got %v, want none when every $defs entry exists", got)
	}
}

// danglingRefs returns the sorted, de-duplicated "#/$defs/X" targets referenced
// anywhere in raw that rootDefs does not define.
func danglingRefs(raw []byte, rootDefs map[string]bool) []string {
	var doc any
	if err := json.Unmarshal(raw, &doc); err != nil {
		return []string{"<unparsable schema>"}
	}

	seen := map[string]bool{}
	var walk func(v any)
	walk = func(v any) {
		switch node := v.(type) {
		case map[string]any:
			if ref, ok := node["$ref"].(string); ok {
				seen[ref] = true
			}
			for _, child := range node {
				walk(child)
			}
		case []any:
			for _, child := range node {
				walk(child)
			}
		}
	}
	walk(doc)

	var missing []string
	const prefix = "#/$defs/"
	for ref := range seen {
		if !strings.HasPrefix(ref, prefix) {
			continue // external or fragment-less refs are out of scope here
		}
		if name := strings.TrimPrefix(ref, prefix); !rootDefs[name] {
			missing = append(missing, name)
		}
	}
	sort.Strings(missing)
	return missing
}

// rootDefinitionNames extracts the set of names defined in the generated root
// schema's "$defs".
func rootDefinitionNames(result *GenerateResult) map[string]bool {
	names := map[string]bool{}
	if result.RootSchema == nil {
		return names
	}
	for name := range result.RootSchema.Defs {
		names[name] = true
	}
	return names
}

// TestGeneratedSchemas_MatchOnDisk catches the other half of the same failure
// mode: pkg/spec changes and the committed schemas/ are never regenerated.
//
// The generated files are committed and consumed by `formspec validate
// --schema schemas` and by the YAML editor, so a stale pair is not harmless —
// it is a wrong contract. Comparing the in-memory generation against what is on
// disk names the exact stale file instead of letting it surface later as a
// compile error in an unrelated manifest.
func TestGeneratedSchemas_MatchOnDisk(t *testing.T) {
	converter := New(pkgPath)
	collect, err := converter.Collect()
	if err != nil {
		t.Fatalf("Collect: %v", err)
	}
	result := converter.Generate(collect)

	compareWithOnDisk(t, filepath.Join("..", "..", "schemas", "formspec.schema.json"), result.RootSchema)

	for _, entry := range KindMapping() {
		kindSchema, ok := result.KindSchemas[entry.Kind]
		if !ok {
			continue
		}
		path := filepath.Join("..", "..", "schemas", "kinds", entry.Kind+".schema.json")
		compareWithOnDisk(t, path, kindSchema)
	}
}

// compareWithOnDisk fails when the committed schema at path differs
// semantically from want.
func compareWithOnDisk(t *testing.T, path string, want any) {
	t.Helper()

	onDiskRaw, err := os.ReadFile(path)
	if err != nil {
		t.Errorf("read %s: %v", path, err)
		return
	}
	wantRaw, err := json.Marshal(want)
	if err != nil {
		t.Fatalf("marshal generated schema for %s: %v", path, err)
	}

	var onDisk, generated any
	if err := json.Unmarshal(onDiskRaw, &onDisk); err != nil {
		t.Errorf("%s: unparsable: %v", path, err)
		return
	}
	if err := json.Unmarshal(wantRaw, &generated); err != nil {
		t.Fatalf("unmarshal generated schema for %s: %v", path, err)
	}

	if !reflect.DeepEqual(onDisk, generated) {
		t.Errorf("%s is stale — run `make generate-schema` (pkg/spec changed but the committed schema was not regenerated)", path)
	}
}
