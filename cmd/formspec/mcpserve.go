// Command `formspec mcp-serve` — local MCP tool server (todo 10.1).
//
//	formspec mcp-serve [--spec <path>] [--schema <dir>] [--project <dir>] [--dev-addr <addr>]
//
// Implements the `formspec-local-mcp` contract (docs/ai/03-formspec-local-mcp.md)
// over the MCP stdio transport, using the official Go SDK. An MCP client
// (formspec-consult, Claude Code, Cursor, ...) spawns this process and talks
// JSON-RPC over stdin/stdout — all diagnostics go to stderr.
//
// Tool catalog (03 §1):
//
//	Grounding (read-only):
//	  list_kind_schemas(kind)            — official per-kind JSON Schema
//	  read_workspace_manifest()          — App manifest(s) of the project
//	  list_installed_modules()           — modules/ + vendors/ inventory
//	  read_module_spec(module,kind,name) — one manifest's source
//	Consult (write, validated):
//	  propose_spec_file(session,path,content) — draft + auto-validate (03 §2)
//	  apply_draft(session,file)               — move draft, guard vendors/ (03 §4)
//	  validate_spec(yaml)                     — same package as `formspec validate` (03 §3)
//	  check_naming_conflict(name)             — module/entity name clashes
//	Server control:
//	  restart_server() / get_server_status() / stop_server() (03 §5)
//	Skills:
//	  list_skills() / read_skill(name) (docs/ai/06-formspec-skill.md)
//
// Everything is local — business specs never leave the machine (01 §2).
package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"gopkg.in/yaml.v3"

	rootformspec "github.com/primadi/formspec"
	"github.com/primadi/formspec/internal/manifest"
	"github.com/primadi/formspec/internal/schemaregistry"
)

// mcpEnv carries the resolved configuration for one mcp-serve process.
type mcpEnv struct {
	specPath    string // spec directory (default "spec")
	schemaDir   string // optional local schemas/ override
	projectRoot string // project root (vendors/, .formspec/)
	devAddr     string // addr of the local `formspec dev` (server control)
}

func runMcpServe(args []string) {
	fs := flag.NewFlagSet("mcp-serve", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	specPath := fs.String("spec", "spec", "path to the spec directory")
	schemaDir := fs.String("schema", "", "local schemas/ dir override (bypasses the registry)")
	projectRoot := fs.String("project", ".", "project root (vendors/, .formspec/)")
	devAddr := fs.String("dev-addr", ":8080", "address of the local `formspec dev` for server-control tools")
	if err := fs.Parse(args); err != nil {
		os.Exit(2)
	}

	env := &mcpEnv{
		specPath:    *specPath,
		schemaDir:   *schemaDir,
		projectRoot: *projectRoot,
		devAddr:     *devAddr,
	}

	srv := mcp.NewServer(&mcp.Implementation{
		Name:    "formspec-local-mcp",
		Version: "0.1.0",
	}, nil)
	env.registerTools(srv)

	// Stdio transport: stdout is the protocol channel — nothing else may
	// write there. Diagnostics go to stderr.
	if err := srv.Run(context.Background(), &mcp.StdioTransport{}); err != nil {
		fmt.Fprintf(os.Stderr, "[formspec-mcp] server error: %v\n", err)
		os.Exit(1)
	}
}

// registerTools wires the full tool catalog (03 §1).
func (e *mcpEnv) registerTools(srv *mcp.Server) {
	// ── Grounding (read-only) ──
	mcp.AddTool(srv, &mcp.Tool{
		Name:        "list_kind_schemas",
		Description: "Return the official JSON Schema for a resource kind (Entity, Form, Table, ...). Call without `kind` to list available kinds. This is the source of truth for authoring — never rely on memory.",
	}, e.listKindSchemas)

	mcp.AddTool(srv, &mcp.Tool{
		Name:        "read_workspace_manifest",
		Description: "Read the project's App manifest(s): active modules, Menu, Auth, Theme. Call this at session start.",
	}, e.readWorkspaceManifest)

	mcp.AddTool(srv, &mcp.Tool{
		Name:        "list_installed_modules",
		Description: "List modules in modules/ and vendors/ with their active status. vendors/ content is READ-ONLY — propose changes via Entity Extension or shadow copy instead.",
	}, e.listInstalledModules)

	mcp.AddTool(srv, &mcp.Tool{
		Name:        "read_module_spec",
		Description: "Read one manifest's YAML source by (module, kind, name).",
	}, e.readModuleSpec)

	// ── Consult (write, validated) ──
	mcp.AddTool(srv, &mcp.Tool{
		Name:        "propose_spec_file",
		Description: "Write a draft manifest to the consult session and validate it automatically (validation gate — the tool always validates, the caller cannot skip it). Path is relative to the spec directory. Paths under vendors/ are rejected.",
	}, e.proposeSpecFile)

	mcp.AddTool(srv, &mcp.Tool{
		Name:        "apply_draft",
		Description: "Move an accepted draft file from the consult session to its real location in the spec tree. The original file is backed up to the session's undo/ directory. Paths under vendors/ are rejected.",
	}, e.applyDraft)

	mcp.AddTool(srv, &mcp.Tool{
		Name:        "validate_spec",
		Description: "Validate one YAML manifest (structural: schema + engine rules). Uses the same validation package as `formspec validate` — not a reimplementation.",
	}, e.validateSpecYAML)

	mcp.AddTool(srv, &mcp.Tool{
		Name:        "check_naming_conflict",
		Description: "Check whether a module/entity name clashes with existing resources in the spec tree.",
	}, e.checkNamingConflict)

	// ── Server control ──
	mcp.AddTool(srv, &mcp.Tool{
		Name:        "restart_server",
		Description: "Restart the local `formspec dev` process. Validates the spec tree first and refuses to restart when invalid. Returns the health status after boot.",
	}, e.restartServer)

	mcp.AddTool(srv, &mcp.Tool{
		Name:        "get_server_status",
		Description: "Report whether the local `formspec dev` process is running and its health endpoint status.",
	}, e.getServerStatus)

	mcp.AddTool(srv, &mcp.Tool{
		Name:        "stop_server",
		Description: "Stop the local `formspec dev` process.",
	}, e.stopServer)

	// ── Skills ──
	mcp.AddTool(srv, &mcp.Tool{
		Name:        "list_skills",
		Description: "List available FormSpec Skills (name + description). Call at session start; read the full skill with read_skill when the topic matches.",
	}, e.listSkills)

	mcp.AddTool(srv, &mcp.Tool{
		Name:        "read_skill",
		Description: "Read the full Markdown content of one FormSpec Skill by name.",
	}, e.readSkill)
}

// ─── Result helpers ───

// textResult returns a successful tool result carrying one JSON text payload.
func textResult(v any) (*mcp.CallToolResult, any, error) {
	b, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return nil, nil, err
	}
	return &mcp.CallToolResult{
		Content: []mcp.Content{&mcp.TextContent{Text: string(b)}},
	}, nil, nil
}

// errResult returns a tool-level error result (protocol-level success, but
// the tool refused — e.g. the vendors/ guard). The message guides the caller
// to the correct mechanism.
func errResult(msg string) (*mcp.CallToolResult, any, error) {
	return &mcp.CallToolResult{
		Content: []mcp.Content{&mcp.TextContent{Text: msg}},
		IsError: true,
	}, nil, nil
}

// ─── Grounding tools ───

type listKindSchemasArgs struct {
	Kind string `json:"kind,omitempty" jsonschema:"kind name (e.g. Entity, Form). Omit to list available kinds."`
}

// listKindSchemas returns the per-kind JSON Schema. Schema source resolution:
// --schema flag → ./schemas → registry cache (same chain as `formspec validate`).
func (e *mcpEnv) listKindSchemas(_ context.Context, _ *mcp.CallToolRequest, args listKindSchemasArgs) (*mcp.CallToolResult, any, error) {
	dir, err := e.resolveSchemaDir()
	if err != nil {
		return errResult("schema source unavailable: " + err.Error())
	}
	kindsDir := filepath.Join(dir, "kinds")
	entries, err := os.ReadDir(kindsDir)
	if err != nil {
		return errResult(fmt.Sprintf("read %s: %v", kindsDir, err))
	}
	var available []string
	for _, ent := range entries {
		if name := ent.Name(); strings.HasSuffix(name, ".schema.json") {
			available = append(available, strings.TrimSuffix(name, ".schema.json"))
		}
	}
	sort.Strings(available)

	if args.Kind == "" {
		return textResult(map[string]any{"kinds": available})
	}
	for _, k := range available {
		if strings.EqualFold(k, args.Kind) {
			b, err := os.ReadFile(filepath.Join(kindsDir, k+".schema.json"))
			if err != nil {
				return errResult(fmt.Sprintf("read schema %s: %v", k, err))
			}
			return textResult(map[string]any{"kind": k, "schema": json.RawMessage(b)})
		}
	}
	return errResult(fmt.Sprintf("unknown kind %q — available: %s", args.Kind, strings.Join(available, ", ")))
}

// resolveSchemaDir finds a local schemas/ directory: explicit flag → ./schemas
// → the registry cache for the spec tree's apiVersion (offline after first fetch).
func (e *mcpEnv) resolveSchemaDir() (string, error) {
	if e.schemaDir != "" {
		if fi, err := os.Stat(filepath.Join(e.schemaDir, "kinds")); err == nil && fi.IsDir() {
			return e.schemaDir, nil
		}
		return "", fmt.Errorf("--schema %s has no kinds/ subdir", e.schemaDir)
	}
	if fi, err := os.Stat("schemas/kinds"); err == nil && fi.IsDir() {
		return "schemas", nil
	}
	// Fall back to the registry cache for the versions present in the tree.
	loader := manifest.NewLoader(e.specPath)
	res, err := loader.LoadAll()
	if err != nil {
		return "", err
	}
	versions := map[string]bool{}
	for _, m := range res.Manifests {
		if v, err := schemaregistry.ParseVersion(m.APIVersion); err == nil {
			versions[v] = true
		}
	}
	reg := schemaregistry.New(schemaRegistryBaseURL())
	for v := range versions {
		if err := reg.Ensure(v, nil, false); err != nil {
			continue
		}
		if dir, err := reg.VersionDir(v); err == nil {
			return dir, nil
		}
	}
	return "", fmt.Errorf("no local schemas/ dir and registry cache is empty — run `formspec validate` once to populate")
}

type readWorkspaceManifestArgs struct{}

// readWorkspaceManifest returns the raw YAML of every kind: App manifest in
// the spec tree (the activation list / Menu / Auth / Theme entry points).
func (e *mcpEnv) readWorkspaceManifest(_ context.Context, _ *mcp.CallToolRequest, _ readWorkspaceManifestArgs) (*mcp.CallToolResult, any, error) {
	loader := manifest.NewLoader(e.specPath)
	res, err := loader.LoadAll()
	if err != nil {
		return errResult("load spec tree: " + err.Error())
	}
	var apps []map[string]any
	for _, m := range res.Manifests {
		if strings.EqualFold(m.Kind, "App") {
			b, err := os.ReadFile(m.Source)
			if err != nil {
				continue
			}
			apps = append(apps, map[string]any{
				"path":    m.Source,
				"content": string(b),
			})
		}
	}
	if apps == nil {
		return errResult("no kind: App manifest found in " + e.specPath)
	}
	return textResult(map[string]any{"apps": apps})
}

type listInstalledModulesArgs struct{}

// listInstalledModules inventories modules/ + vendors/ at the project root
// (layout 08-project-layout.md §6.1) and, when present, merges the active
// status from formspec.lock (best-effort — the lockfile lands with Fase 13).
func (e *mcpEnv) listInstalledModules(_ context.Context, _ *mcp.CallToolRequest, _ listInstalledModulesArgs) (*mcp.CallToolResult, any, error) {
	type modInfo struct {
		Name     string `json:"name"`
		Source   string `json:"source"` // "modules" | "vendors"
		Active   bool   `json:"active"`
		ReadOnly bool   `json:"read_only"`
	}
	var out []modInfo
	for _, src := range []struct{ dir, label string }{
		{"modules", "modules"},
		{"vendors", "vendors"},
	} {
		abs := filepath.Join(e.projectRoot, src.dir)
		entries, err := os.ReadDir(abs)
		if err != nil {
			continue
		}
		for _, ent := range entries {
			if ent.IsDir() {
				out = append(out, modInfo{
					Name:     ent.Name(),
					Source:   src.label,
					Active:   true, // activation list lands with Fase 13 (formspec.lock)
					ReadOnly: src.label == "vendors",
				})
			}
		}
	}
	if out == nil {
		out = []modInfo{}
	}
	// Best-effort lockfile status (Fase 13 will define the real format).
	lockPath := filepath.Join(e.projectRoot, "formspec.lock")
	lockPresent := false
	if _, err := os.Stat(lockPath); err == nil {
		lockPresent = true
	}
	return textResult(map[string]any{
		"modules":         out,
		"formspec_lock":   lockPresent,
		"activation_note": "active=true is the default until formspec.lock lands (Fase 13)",
	})
}

type readModuleSpecArgs struct {
	Module string `json:"module" jsonschema:"module name (metadata.module)"`
	Kind   string `json:"kind"   jsonschema:"resource kind (e.g. Entity, Form)"`
	Name   string `json:"name"   jsonschema:"resource name (metadata.name)"`
}

// readModuleSpec returns the YAML source of one manifest, resolved through
// the same loader the engine uses — never by guessing file paths.
func (e *mcpEnv) readModuleSpec(_ context.Context, _ *mcp.CallToolRequest, args readModuleSpecArgs) (*mcp.CallToolResult, any, error) {
	loader := manifest.NewLoader(e.specPath)
	res, err := loader.LoadAll()
	if err != nil {
		return errResult("load spec tree: " + err.Error())
	}
	for _, m := range res.Manifests {
		if strings.EqualFold(m.Metadata.Module, args.Module) &&
			strings.EqualFold(m.Kind, args.Kind) &&
			strings.EqualFold(m.Metadata.Name, args.Name) {
			b, err := os.ReadFile(m.Source)
			if err != nil {
				return errResult(fmt.Sprintf("read %s: %v", m.Source, err))
			}
			return textResult(map[string]any{
				"path":    m.Source,
				"kind":    m.Kind,
				"module":  m.Metadata.Module,
				"name":    m.Metadata.Name,
				"content": string(b),
			})
		}
	}
	return errResult(fmt.Sprintf("no manifest %s/%s/%s found in %s", args.Module, args.Kind, args.Name, e.specPath))
}

// ─── Skills (docs/ai/06-formspec-skill.md) ───

type listSkillsArgs struct{}

// skillMeta is the frontmatter of a SKILL.md (docs/ai/06-formspec-skill.md §2).
type skillMeta struct {
	Name        string   `yaml:"name"        json:"name"`
	Description string   `yaml:"description" json:"description"`
	AppliesTo   []string `yaml:"applies_to_kind" json:"applies_to_kind,omitempty"`
	MinCoreVer  string   `yaml:"min_core_spec_version" json:"min_core_spec_version,omitempty"`
}

// embeddedSkills returns every embedded skill's metadata + raw content.
func embeddedSkills() ([]skillMeta, map[string]string, error) {
	var metas []skillMeta
	bodies := map[string]string{}
	err := fs.WalkDir(rootformspec.AISkillsFS, ".", func(p string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() || !strings.HasSuffix(p, "SKILL.md") {
			return err
		}
		b, err := fs.ReadFile(rootformspec.AISkillsFS, p)
		if err != nil {
			return err
		}
		meta, body, perr := splitSkillFrontmatter(string(b))
		if perr != nil {
			return perr
		}
		if meta.Name == "" {
			meta.Name = filepath.Base(filepath.Dir(p))
		}
		metas = append(metas, *meta)
		bodies[meta.Name] = body
		return nil
	})
	if err != nil {
		return nil, nil, err
	}
	sort.Slice(metas, func(i, j int) bool { return metas[i].Name < metas[j].Name })
	return metas, bodies, nil
}

// splitSkillFrontmatter splits YAML frontmatter from the Markdown body.
func splitSkillFrontmatter(content string) (*skillMeta, string, error) {
	meta := &skillMeta{}
	body := content
	if strings.HasPrefix(content, "---\n") {
		rest := content[4:]
		if idx := strings.Index(rest, "\n---"); idx >= 0 {
			fm := rest[:idx]
			body = strings.TrimLeft(rest[idx+4:], "\n")
			if err := yaml.Unmarshal([]byte(fm), meta); err != nil {
				return nil, "", fmt.Errorf("parse skill frontmatter: %w", err)
			}
		}
	}
	return meta, body, nil
}

func (e *mcpEnv) listSkills(_ context.Context, _ *mcp.CallToolRequest, _ listSkillsArgs) (*mcp.CallToolResult, any, error) {
	metas, _, err := embeddedSkills()
	if err != nil {
		return errResult("read embedded skills: " + err.Error())
	}
	return textResult(map[string]any{"skills": metas})
}

type readSkillArgs struct {
	Name string `json:"name" jsonschema:"skill name from list_skills"`
}

// readSkill returns the skill's full Markdown body — raw natural-language
// instructions for the LLM, deliberately not JSON-wrapped (06 §2).
func (e *mcpEnv) readSkill(_ context.Context, _ *mcp.CallToolRequest, args readSkillArgs) (*mcp.CallToolResult, any, error) {
	_, bodies, err := embeddedSkills()
	if err != nil {
		return errResult("read embedded skills: " + err.Error())
	}
	body, ok := bodies[args.Name]
	if !ok {
		return errResult(fmt.Sprintf("unknown skill %q", args.Name))
	}
	return &mcp.CallToolResult{
		Content: []mcp.Content{&mcp.TextContent{Text: body}},
	}, nil, nil
}
