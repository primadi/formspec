package main

import (
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// ─── Unit tests: path guards & skill frontmatter ───

func TestGuardVendors(t *testing.T) {
	for _, p := range []string{
		"vendors/stripe/module.yaml",
		"modules/billing/entity/invoice.yaml", // not vendors — allowed
	} {
		if err := guardVendors(p); (err == nil) != !strings.Contains(p, "vendors") {
			t.Errorf("guardVendors(%q) = %v", p, err)
		}
	}
	err := guardVendors("vendors/stripe/module.yaml")
	if err == nil || !strings.Contains(err.Error(), "Entity Extension") {
		t.Errorf("guard message must point to Entity Extension / shadow copy, got: %v", err)
	}
}

func TestSanitizeRelPath(t *testing.T) {
	if p, err := sanitizeRelPath("modules/b/entity.yaml"); err != nil || p != "modules/b/entity.yaml" {
		t.Errorf("clean path mangled: %q %v", p, err)
	}
	for _, bad := range []string{"", ".", "../evil.yaml", "a/../../evil.yaml", "no-ext", "dir/"} {
		if _, err := sanitizeRelPath(bad); err == nil {
			t.Errorf("sanitizeRelPath(%q) accepted", bad)
		}
	}
}

func TestSplitSkillFrontmatter(t *testing.T) {
	content := "---\nname: entity-authoring\ndescription: How to author entities\n---\n\n# Body\n"
	meta, body, err := splitSkillFrontmatter(content)
	if err != nil {
		t.Fatal(err)
	}
	if meta.Name != "entity-authoring" || meta.Description != "How to author entities" {
		t.Errorf("meta = %+v", meta)
	}
	if !strings.HasPrefix(body, "# Body") {
		t.Errorf("body = %q", body)
	}
	// No frontmatter → whole content is body.
	meta, body, err = splitSkillFrontmatter("# Just markdown\n")
	if err != nil || meta.Name != "" || !strings.HasPrefix(body, "# Just") {
		t.Errorf("no-frontmatter case: %+v %q %v", meta, body, err)
	}
}

// ─── validateSpecTree (03 §3 — same packages as `formspec validate`) ───

const fixtureEntity = `apiVersion: formspec.dev/v1
kind: Entity
metadata:
  name: product
  module: shop
  description: "Fixture product"
spec:
  version: v1
  characteristic: master
  lifecycle: plain_crud
  display_field: name
  plural: products
  fields:
    - name: name
      type: string
      required: true
      title: "Name"
`

func writeFixtureTree(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	p := filepath.Join(dir, "modules", "shop", "entity")
	if err := os.MkdirAll(p, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(p, "product.yaml"), []byte(fixtureEntity), 0644); err != nil {
		t.Fatal(err)
	}
	return dir
}

func TestValidateSpecTree_OK(t *testing.T) {
	tree := writeFixtureTree(t)
	val, err := validateSpecTree(tree, "../../schemas")
	if err != nil {
		t.Fatalf("validateSpecTree: %v", err)
	}
	if !val.OK {
		t.Fatalf("expected OK, got problems: %+v", val.Problems)
	}
	if val.ManifestCount != 1 {
		t.Errorf("manifest count = %d, want 1", val.ManifestCount)
	}
}

func TestValidateSpecTree_RejectsBroken(t *testing.T) {
	tree := writeFixtureTree(t)
	broken := filepath.Join(tree, "modules", "shop", "entity", "broken.yaml")
	if err := os.WriteFile(broken, []byte("apiVersion: formspec.dev/v1\nkind: Entity\nmetadata: {name: broken, module: shop}\nspec: {fields: []}\n"), 0644); err != nil {
		t.Fatal(err)
	}
	val, err := validateSpecTree(tree, "../../schemas")
	if err != nil {
		t.Fatalf("validateSpecTree: %v", err)
	}
	if val.OK {
		t.Fatal("expected problems for broken manifest")
	}
}

// ─── End-to-end: MCP client ↔ `formspec mcp-serve` over stdio ───

// TestMain lets the test binary re-exec itself as a real MCP server process.
func TestMain(m *testing.M) {
	if os.Getenv("FORMSPEC_MCP_HELPER") == "1" {
		runMcpServe(strings.Fields(os.Getenv("FORMSPEC_MCP_ARGS")))
		os.Exit(0)
	}
	os.Exit(m.Run())
}

func startMCPServer(t *testing.T, specDir, projectDir string) *mcp.ClientSession {
	t.Helper()
	exe, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command(exe, "-test.run=TestMain") // re-exec self; TestMain branches on env
	cmd.Env = append(os.Environ(),
		"FORMSPEC_MCP_HELPER=1",
		"FORMSPEC_MCP_ARGS=--spec "+specDir+" --schema ../../schemas --project "+projectDir,
	)
	client := mcp.NewClient(&mcp.Implementation{Name: "test-client", Version: "0.0.0"}, nil)
	session, err := client.Connect(context.Background(), &mcp.CommandTransport{Command: cmd}, nil)
	if err != nil {
		t.Fatalf("connect MCP server: %v", err)
	}
	t.Cleanup(func() { session.Close() })
	return session
}

func callTool(t *testing.T, s *mcp.ClientSession, name string, args map[string]any) map[string]any {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	res, err := s.CallTool(ctx, &mcp.CallToolParams{Name: name, Arguments: args})
	if err != nil {
		t.Fatalf("CallTool %s: %v", name, err)
	}
	if res.IsError {
		t.Fatalf("tool %s returned error: %v", name, res.Content)
	}
	if len(res.Content) == 0 {
		t.Fatalf("tool %s returned no content", name)
	}
	tc, ok := res.Content[0].(*mcp.TextContent)
	if !ok {
		t.Fatalf("tool %s content[0] is %T", name, res.Content[0])
	}
	var out map[string]any
	if err := json.Unmarshal([]byte(tc.Text), &out); err != nil {
		t.Fatalf("tool %s output not JSON: %v\n%s", name, err, tc.Text)
	}
	return out
}

func TestMCPServerEndToEnd(t *testing.T) {
	specDir := writeFixtureTree(t)
	projectDir := t.TempDir()
	session := startMCPServer(t, specDir, projectDir)

	// list_skills — embedded skills are visible (06 §2).
	out := callTool(t, session, "list_skills", map[string]any{})
	skills, _ := out["skills"].([]any)
	if len(skills) == 0 {
		t.Fatal("list_skills returned no skills")
	}

	// read_skill — raw markdown body (06 §2: deliberately not JSON-wrapped).
	res, err := session.CallTool(context.Background(), &mcp.CallToolParams{
		Name: "read_skill",
		Arguments: map[string]any{
			"name": skills[0].(map[string]any)["name"],
		},
	})
	if err != nil || res.IsError {
		t.Fatalf("read_skill failed: %v %v", err, res)
	}
	tc, ok := res.Content[0].(*mcp.TextContent)
	if !ok || !strings.HasPrefix(tc.Text, "#") {
		t.Fatalf("read_skill must return raw markdown, got %T: %q", res.Content[0], tc.Text)
	}

	// check_naming_conflict — existing name found, unknown name available.
	out = callTool(t, session, "check_naming_conflict", map[string]any{"name": "product"})
	if m, _ := out["matches"].([]any); len(m) == 0 {
		t.Error("expected a match for existing entity 'product'")
	}
	out = callTool(t, session, "check_naming_conflict", map[string]any{"name": "brand-new-thing"})
	if avail, _ := out["available"].(bool); !avail {
		t.Errorf("expected available=true for unused name, got %v", out)
	}

	// propose_spec_file — valid draft → written + validation ok.
	draft := strings.Replace(fixtureEntity, "name: product", "name: customer", 1)
	draft = strings.Replace(draft, "plural: products", "plural: customers", 1)
	out = callTool(t, session, "propose_spec_file", map[string]any{
		"session": "s1",
		"path":    "modules/shop/entity/customer.yaml",
		"content": draft,
	})
	if w, _ := out["written"].(bool); !w {
		t.Fatalf("propose_spec_file not written: %v", out)
	}
	val, _ := out["validation"].(map[string]any)
	if ok, _ := val["ok"].(bool); !ok {
		t.Fatalf("validation not ok: %v", val)
	}

	// propose_spec_file — vendors/ guard rejects (03 §4).
	res, err = session.CallTool(context.Background(), &mcp.CallToolParams{
		Name: "propose_spec_file",
		Arguments: map[string]any{
			"session": "s1",
			"path":    "vendors/stripe/module.yaml",
			"content": draft,
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if !res.IsError {
		t.Fatal("vendors/ path must be rejected")
	}

	// apply_draft — moves the draft into the real spec tree.
	out = callTool(t, session, "apply_draft", map[string]any{
		"session": "s1",
		"file":    "modules/shop/entity/customer.yaml",
	})
	if applied, _ := out["applied"].(string); !strings.HasSuffix(applied, "customer.yaml") {
		t.Fatalf("apply_draft applied = %v", out)
	}
	if _, err := os.Stat(filepath.Join(specDir, "modules", "shop", "entity", "customer.yaml")); err != nil {
		t.Fatalf("draft not moved into spec tree: %v", err)
	}

	// validate_spec — inline YAML validated against the tree.
	out = callTool(t, session, "validate_spec", map[string]any{"yaml": fixtureEntity})
	if ok, _ := out["ok"].(bool); !ok {
		t.Fatalf("validate_spec not ok: %v", out)
	}

	// get_server_status — no dev server running in the fixture.
	out = callTool(t, session, "get_server_status", map[string]any{})
	if running, _ := out["running"].(bool); running {
		t.Error("expected running=false in fixture project")
	}
}
