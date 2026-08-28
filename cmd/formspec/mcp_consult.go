// Consult tools for `formspec mcp-serve` — the write-side of the local MCP
// server (todo 10.1.3–10.1.6, docs/ai/03-formspec-local-mcp.md §2–§5).
//
// Design invariants:
//
//   - Validation gate (§2): propose_spec_file ALWAYS validates — the caller
//     cannot skip it. Validation reuses the same packages as `formspec
//     validate` (internal/manifest engine loader + JSON Schema layer), never
//     a reimplementation (§3).
//   - vendors/ guard (§4): every write tool rejects paths under vendors/ —
//     enforced in code, pointing the caller at Entity Extension / shadow copy.
//   - Undo (02 §4): apply_draft backs up the original file before overwriting.
//   - Server control (§5): restart_server validates first, refuses on invalid.
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"gopkg.in/yaml.v3"

	"github.com/primadi/formspec/internal/manifest"
	"github.com/primadi/formspec/internal/schemaregistry"
)

// ─── Validation (03 §3 — one package, three callers) ───

// SpecProblem is one structural validation finding.
type SpecProblem struct {
	Source  string `json:"source"`
	Layer   string `json:"layer"` // parse | engine | integrator | honesty | schema
	Message string `json:"message"`
}

// SpecValidation is the structured result of validating a spec tree.
type SpecValidation struct {
	OK            bool          `json:"ok"`
	ManifestCount int           `json:"manifest_count"`
	Problems      []SpecProblem `json:"problems"`
	Warnings      []SpecProblem `json:"warnings"` // honesty warnings — advisory
}

// validateSpecTree validates a spec directory in-process using the exact
// same layers as `formspec validate`: engine loader, integrator rules,
// honesty scan, and the JSON Schema layer. No printing, no os.Exit — the
// CLI command keeps its own presentation.
func validateSpecTree(specPath, schemaDirOverride string) (*SpecValidation, error) {
	out := &SpecValidation{Problems: []SpecProblem{}, Warnings: []SpecProblem{}}
	loader := manifest.NewLoader(specPath)

	res, err := loader.LoadAll()
	if err != nil {
		return nil, err
	}
	out.ManifestCount = len(res.Manifests)
	for _, pe := range res.Errors {
		out.Problems = append(out.Problems, SpecProblem{Source: pe.File, Layer: "parse", Message: pe.Error()})
	}

	for _, m := range res.Manifests {
		if err := loader.Validate(m); err != nil {
			out.Problems = append(out.Problems, SpecProblem{Source: m.Source, Layer: "engine", Message: err.Error()})
		}
	}
	for src, msg := range validateIntegrators(res.Manifests) {
		out.Problems = append(out.Problems, SpecProblem{Source: src, Layer: "integrator", Message: msg})
	}
	for _, iss := range scanHonesty(res.Manifests, specPath) {
		p := SpecProblem{Source: iss.Source, Layer: "honesty", Message: iss.Message}
		if iss.Script != "" {
			p.Source = iss.Script
		}
		if iss.Severity == "error" {
			out.Problems = append(out.Problems, p)
		} else {
			out.Warnings = append(out.Warnings, p)
		}
	}

	// JSON Schema layer — local override → ./schemas → registry cache.
	compilers, versionOf, versionErrs, schemaErr := buildSchemaCompilers(res.Manifests, schemaDirOverride)
	if schemaErr != nil {
		return nil, schemaErr
	}
	if compilers != nil {
		for _, m := range res.Manifests {
			if msg, ok := versionErrs[m.Source]; ok {
				out.Problems = append(out.Problems, SpecProblem{Source: m.Source, Layer: "schema", Message: msg})
				continue
			}
			if c, ok := compilers[versionOf[m.Source]]; ok {
				if msg := validateSchema(c, m); msg != "" {
					out.Problems = append(out.Problems, SpecProblem{Source: m.Source, Layer: "schema", Message: msg})
				}
			}
		}
	}

	out.OK = len(out.Problems) == 0
	return out, nil
}

// buildSchemaCompilers resolves the schema layer for the given manifests —
// the flag/./schemas/registry chain from validate.go, without the printing.
func buildSchemaCompilers(manifests []manifest.RawManifest, schemaDirOverride string) (
	map[string]*kindSchemaCompiler, map[string]string, map[string]string, error,
) {
	if schemaDirOverride != "" {
		c, err := newKindSchemaCompiler(schemaDirOverride)
		if err != nil {
			return nil, nil, nil, err
		}
		versionOf := map[string]string{}
		for _, m := range manifests {
			versionOf[m.Source] = "*"
		}
		return map[string]*kindSchemaCompiler{"*": c}, versionOf, map[string]string{}, nil
	}
	if fi, err := os.Stat("schemas/kinds"); err == nil && fi.IsDir() {
		c, err := newKindSchemaCompiler("schemas")
		if err != nil {
			return nil, nil, nil, err
		}
		versionOf := map[string]string{}
		for _, m := range manifests {
			versionOf[m.Source] = "*"
		}
		return map[string]*kindSchemaCompiler{"*": c}, versionOf, map[string]string{}, nil
	}

	reg := schemaregistry.New(schemaRegistryBaseURL())
	compilers := map[string]*kindSchemaCompiler{}
	versionOf := map[string]string{}
	versionErrs := map[string]string{}
	kindsByVer := map[string]map[string]bool{}
	for _, m := range manifests {
		v, err := schemaregistry.ParseVersion(m.APIVersion)
		if err != nil {
			versionErrs[m.Source] = err.Error()
			continue
		}
		versionOf[m.Source] = v
		if kindsByVer[v] == nil {
			kindsByVer[v] = map[string]bool{}
		}
		kindsByVer[v][m.Kind] = true
	}
	for v, kinds := range kindsByVer {
		kindList := make([]string, 0, len(kinds))
		for k := range kinds {
			kindList = append(kindList, k)
		}
		if err := reg.Ensure(v, kindList, false); err != nil {
			return nil, nil, nil, fmt.Errorf("schema %s: %w", v, err)
		}
		dir, err := reg.VersionDir(v)
		if err != nil {
			return nil, nil, nil, fmt.Errorf("schema %s: %w", v, err)
		}
		c, err := newKindSchemaCompiler(dir)
		if err != nil {
			return nil, nil, nil, fmt.Errorf("schema %s: %w", v, err)
		}
		compilers[v] = c
	}
	return compilers, versionOf, versionErrs, nil
}

// ─── validate_spec / check_naming_conflict ───

type validateSpecYAMLArgs struct {
	YAML string `json:"yaml" jsonschema:"full YAML manifest content to validate"`
}

// validateSpecYAML validates one manifest's content structurally: the content
// is overlaid onto a temp copy of the real spec tree so cross-manifest
// references (depends, Entity Extension, shadow copy) are checked too.
func (e *mcpEnv) validateSpecYAML(_ context.Context, _ *mcp.CallToolRequest, args validateSpecYAMLArgs) (*mcp.CallToolResult, any, error) {
	if strings.TrimSpace(args.YAML) == "" {
		return errResult("yaml is empty")
	}
	tmp, err := os.MkdirTemp("", "formspec-validate-*")
	if err != nil {
		return nil, nil, err
	}
	defer os.RemoveAll(tmp)

	if err := copySpecTree(e.specPath, tmp); err != nil {
		return errResult("copy spec tree: " + err.Error())
	}
	// Overlay the content under a probe path inside the copied tree.
	probe := filepath.Join(tmp, "_consult_probe.yaml")
	if err := os.WriteFile(probe, []byte(args.YAML), 0644); err != nil {
		return nil, nil, err
	}

	val, err := validateSpecTree(tmp, e.schemaDir)
	if err != nil {
		return errResult("validation error: " + err.Error())
	}
	// Keep only findings for the probe file — the base tree is assumed clean;
	// if it is not, the caller sees them via propose_spec_file instead.
	var probeProblems, probeWarnings []SpecProblem
	for _, p := range val.Problems {
		if strings.HasSuffix(p.Source, "_consult_probe.yaml") {
			probeProblems = append(probeProblems, p)
		}
	}
	for _, w := range val.Warnings {
		if strings.HasSuffix(w.Source, "_consult_probe.yaml") {
			probeWarnings = append(probeWarnings, w)
		}
	}
	if probeProblems == nil {
		probeProblems = []SpecProblem{}
	}
	if probeWarnings == nil {
		probeWarnings = []SpecProblem{}
	}
	return textResult(map[string]any{
		"ok":       len(probeProblems) == 0,
		"problems": probeProblems,
		"warnings": probeWarnings,
	})
}

type checkNamingConflictArgs struct {
	Name string `json:"name" jsonschema:"module or entity name to check"`
}

// checkNamingConflict reports every resource whose metadata.name (or module
// name) matches, flagging cross-module entity clashes and duplicate
// (module, kind, name) triples.
func (e *mcpEnv) checkNamingConflict(_ context.Context, _ *mcp.CallToolRequest, args checkNamingConflictArgs) (*mcp.CallToolResult, any, error) {
	loader := manifest.NewLoader(e.specPath)
	res, err := loader.LoadAll()
	if err != nil {
		return errResult("load spec tree: " + err.Error())
	}

	type hit struct {
		Module string `json:"module"`
		Kind   string `json:"kind"`
		Name   string `json:"name"`
		Path   string `json:"path"`
	}
	var matches []hit
	seen := map[string]int{} // "module/kind/name" -> count
	entityModules := map[string]bool{}
	moduleNames := map[string]bool{}
	for _, m := range res.Manifests {
		if m.Kind == "Module" {
			moduleNames[m.Metadata.Name] = true
		}
		if m.Kind == "Entity" {
			entityModules[m.Metadata.Module+"/"+m.Metadata.Name] = true
		}
		key := m.Metadata.Module + "/" + m.Kind + "/" + m.Metadata.Name
		seen[key]++
		if strings.EqualFold(m.Metadata.Name, args.Name) || strings.EqualFold(m.Metadata.Module, args.Name) {
			matches = append(matches, hit{m.Metadata.Module, m.Kind, m.Metadata.Name, m.Source})
		}
	}

	var conflicts []string
	for key, n := range seen {
		if n > 1 {
			conflicts = append(conflicts, fmt.Sprintf("%s declared %d times", key, n))
		}
	}
	// An entity named like the target in multiple modules is a cross-module
	// ambiguity for unqualified references.
	entityHits := map[string]bool{}
	for _, mt := range matches {
		if mt.Kind == "Entity" {
			entityHits[mt.Module] = true
		}
	}
	if len(entityHits) > 1 {
		conflicts = append(conflicts, fmt.Sprintf("entity name %q exists in multiple modules: %v — qualify references with module", args.Name, keysOf(entityHits)))
	}
	if moduleNames[args.Name] {
		conflicts = append(conflicts, fmt.Sprintf("a module named %q already exists", args.Name))
	}
	if matches == nil {
		matches = []hit{}
	}
	if conflicts == nil {
		conflicts = []string{}
	}
	return textResult(map[string]any{
		"name":      args.Name,
		"matches":   matches,
		"conflicts": conflicts,
		"available": len(conflicts) == 0,
	})
}

func keysOf(m map[string]bool) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

// ─── Draft session (03 §2, §4; 02 §3–4) ───

// consultDir returns .formspec/consult under the project root.
func (e *mcpEnv) consultDir() string {
	return filepath.Join(e.projectRoot, ".formspec", "consult")
}

// guardVendors rejects any spec-relative or project-relative path under a
// vendors/ directory (03 §4 — enforced in code, not documentation).
func guardVendors(relPath string) error {
	parts := strings.Split(filepath.ToSlash(relPath), "/")
	for _, part := range parts {
		if part == "vendors" {
			return fmt.Errorf(
				"refused: %q is inside vendors/ which is READ-ONLY by design (checksum/signature integrity, safe version updates). "+
					"For extra fields/validation use an Entity Extension (docs/spec/backend/03-entity-extension.md); "+
					"for presentation customization (Form layout, captions, ordering) use a shadow copy in overrides/ (docs/spec/platform/08-project-layout.md §6.4)", relPath)
		}
	}
	return nil
}

// sanitizeRelPath normalizes a spec-relative draft path and blocks traversal.
func sanitizeRelPath(p string) (string, error) {
	p = strings.TrimPrefix(filepath.ToSlash(p), "/")
	for _, seg := range strings.Split(p, "/") {
		if seg == ".." {
			return "", fmt.Errorf("path traversal rejected: %q", p)
		}
	}
	p = path.Clean(p)
	if p == "" || p == "." {
		return "", fmt.Errorf("empty path")
	}
	if !strings.HasSuffix(p, ".yaml") && !strings.HasSuffix(p, ".yml") {
		return "", fmt.Errorf("draft path must end in .yaml/.yml: %q", p)
	}
	return p, nil
}

type proposeSpecFileArgs struct {
	Session string `json:"session" jsonschema:"consult session id (created on first use)"`
	Path    string `json:"path"    jsonschema:"target path relative to the spec directory (e.g. modules/billing/entity/invoice.yaml)"`
	Content string `json:"content" jsonschema:"full YAML manifest content"`
}

// proposeSpecFile writes a draft into the session and validates it
// automatically (03 §2): the validation gate is part of what the tool does,
// not a courtesy the caller may skip. The draft is overlaid onto a copy of
// the real spec tree so cross-manifest references are checked.
func (e *mcpEnv) proposeSpecFile(_ context.Context, _ *mcp.CallToolRequest, args proposeSpecFileArgs) (*mcp.CallToolResult, any, error) {
	if strings.TrimSpace(args.Session) == "" {
		return errResult("session is required")
	}
	if strings.TrimSpace(args.Content) == "" {
		return errResult("content is empty")
	}
	rel, err := sanitizeRelPath(args.Path)
	if err != nil {
		return errResult(err.Error())
	}
	if err := guardVendors(rel); err != nil {
		return errResult(err.Error())
	}

	draftPath := filepath.Join(e.consultDir(), args.Session, "draft", filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(draftPath), 0755); err != nil {
		return nil, nil, err
	}
	if err := os.WriteFile(draftPath, []byte(args.Content), 0644); err != nil {
		return nil, nil, err
	}

	// Validation gate: overlay the draft onto a copy of the spec tree.
	val, err := e.validateWithOverlay(rel, args.Content)
	if err != nil {
		return errResult("validation error: " + err.Error())
	}

	// Re-cek skill relevan (todo 10.6.4, docs/ai/06 §3): pemicu
	// deterministik — bukan inisiatif LLM. Skill yang applies_to_kind-nya
	// mencakup kind draft dikembalikan sebagai name+description; model
	// membaca isinya via read_skill sebelum melanjutkan.
	relevant := relevantSkillsFor(args.Content)

	return textResult(map[string]any{
		"written":         true,
		"draft_path":      draftPath,
		"validation":      val,
		"relevant_skills": relevant,
		"next":            "read relevant skills via read_skill, fix problems if any, then apply_draft(session, path) to accept",
	})
}

// relevantSkillsFor parses the draft's kind and returns the skills whose
// applies_to_kind covers it (empty applies_to_kind = applies to all kinds).
func relevantSkillsFor(draftContent string) []skillMeta {
	var probe struct {
		Kind string `yaml:"kind"`
	}
	_ = yaml.Unmarshal([]byte(draftContent), &probe)
	kind := strings.ToLower(strings.TrimSpace(probe.Kind))

	metas, _, err := embeddedSkills()
	if err != nil {
		return nil
	}
	var out []skillMeta
	for _, m := range metas {
		if len(m.AppliesTo) == 0 {
			out = append(out, m)
			continue
		}
		for _, k := range m.AppliesTo {
			if strings.EqualFold(k, kind) {
				out = append(out, m)
				break
			}
		}
	}
	if out == nil {
		out = []skillMeta{}
	}
	return out
}

// validateWithOverlay copies the spec tree to a temp dir, writes the draft
// content at relPath inside the copy, and runs the full structural validation.
func (e *mcpEnv) validateWithOverlay(relPath, content string) (*SpecValidation, error) {
	tmp, err := os.MkdirTemp("", "formspec-consult-*")
	if err != nil {
		return nil, err
	}
	defer os.RemoveAll(tmp)
	if err := copySpecTree(e.specPath, tmp); err != nil {
		return nil, err
	}
	overlay := filepath.Join(tmp, filepath.FromSlash(relPath))
	if err := os.MkdirAll(filepath.Dir(overlay), 0755); err != nil {
		return nil, err
	}
	if err := os.WriteFile(overlay, []byte(content), 0644); err != nil {
		return nil, err
	}
	return validateSpecTree(tmp, e.schemaDir)
}

// copySpecTree copies all .yaml/.yml/.star files from src to dst preserving
// the relative layout (impl scripts are part of structural honesty checks).
func copySpecTree(src, dst string) error {
	return filepath.WalkDir(src, func(p string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(src, p)
		if err != nil {
			return err
		}
		if d.IsDir() {
			return os.MkdirAll(filepath.Join(dst, rel), 0755)
		}
		ext := strings.ToLower(filepath.Ext(p))
		if ext != ".yaml" && ext != ".yml" && ext != ".star" {
			return nil
		}
		b, err := os.ReadFile(p)
		if err != nil {
			return err
		}
		target := filepath.Join(dst, rel)
		if err := os.MkdirAll(filepath.Dir(target), 0755); err != nil {
			return err
		}
		return os.WriteFile(target, b, 0644)
	})
}

type applyDraftArgs struct {
	Session string `json:"session" jsonschema:"consult session id"`
	File    string `json:"file"    jsonschema:"draft path relative to the spec directory (as given to propose_spec_file)"`
}

// applyDraft moves an accepted draft to its real location in the spec tree:
// vendors/ guard → backup original to undo/ → move → re-validate the tree.
func (e *mcpEnv) applyDraft(_ context.Context, _ *mcp.CallToolRequest, args applyDraftArgs) (*mcp.CallToolResult, any, error) {
	rel, err := sanitizeRelPath(args.File)
	if err != nil {
		return errResult(err.Error())
	}
	if err := guardVendors(rel); err != nil {
		return errResult(err.Error())
	}

	draftPath := filepath.Join(e.consultDir(), args.Session, "draft", filepath.FromSlash(rel))
	if _, err := os.Stat(draftPath); err != nil {
		return errResult(fmt.Sprintf("draft not found: %s (propose_spec_file first)", rel))
	}

	target := filepath.Join(e.specPath, filepath.FromSlash(rel))
	// Refuse to overwrite a file that moved under vendors/ since the draft
	// was written (guard re-checked against the real target).
	if err := guardVendors(rel); err != nil {
		return errResult(err.Error())
	}

	// Auto-backup the original (02 §4 / 10.7.1).
	undoPath := filepath.Join(e.consultDir(), args.Session, "undo", filepath.FromSlash(rel))
	if orig, err := os.ReadFile(target); err == nil {
		if err := os.MkdirAll(filepath.Dir(undoPath), 0755); err != nil {
			return nil, nil, err
		}
		if err := os.WriteFile(undoPath, orig, 0644); err != nil {
			return nil, nil, err
		}
	}

	if err := os.MkdirAll(filepath.Dir(target), 0755); err != nil {
		return nil, nil, err
	}
	if err := os.Rename(draftPath, target); err != nil {
		return nil, nil, err
	}

	// Post-apply validation of the real tree — boot-time surprises surface
	// here instead of at the next `formspec dev`.
	val, err := validateSpecTree(e.specPath, e.schemaDir)
	if err != nil {
		val = &SpecValidation{OK: false, Problems: []SpecProblem{{
			Source: e.specPath, Layer: "load", Message: err.Error(),
		}}, Warnings: []SpecProblem{}}
	}

	return textResult(map[string]any{
		"applied":    target,
		"backup":     undoPath,
		"validation": val,
	})
}

// ─── Server control (03 §5) ───

type serverControlArgs struct{}

// devPID reads the PID written by `formspec dev` (.formspec/dev.pid).
func (e *mcpEnv) devPID() (int, bool) {
	b, err := os.ReadFile(filepath.Join(e.projectRoot, ".formspec", "dev.pid"))
	if err != nil {
		return 0, false
	}
	var pid int
	if _, err := fmt.Sscanf(strings.TrimSpace(string(b)), "%d", &pid); err != nil {
		return 0, false
	}
	return pid, true
}

// serverHealth GETs the dev server's /health endpoint.
func (e *mcpEnv) serverHealth() (string, error) {
	addr := e.devAddr
	if strings.HasPrefix(addr, ":") {
		addr = "http://localhost" + addr
	} else if !strings.Contains(addr, "://") {
		addr = "http://" + addr
	}
	client := &http.Client{Timeout: 2 * time.Second}
	resp, err := client.Get(addr + "/health")
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	b, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
	return string(b), nil
}

func (e *mcpEnv) getServerStatus(_ context.Context, _ *mcp.CallToolRequest, _ serverControlArgs) (*mcp.CallToolResult, any, error) {
	pid, ok := e.devPID()
	running := ok && processAlive(pid)
	status := map[string]any{"running": running, "pid": pid}
	if running {
		if h, err := e.serverHealth(); err == nil {
			status["health"] = json.RawMessage(h)
		} else {
			status["health"] = "unreachable: " + err.Error()
		}
	}
	return textResult(status)
}

func (e *mcpEnv) stopServer(_ context.Context, _ *mcp.CallToolRequest, _ serverControlArgs) (*mcp.CallToolResult, any, error) {
	pid, ok := e.devPID()
	if !ok || !processAlive(pid) {
		return textResult(map[string]any{"running": false, "stopped": false})
	}
	proc, err := os.FindProcess(pid)
	if err == nil {
		proc.Signal(syscallSIGTERM())
		deadline := time.Now().Add(8 * time.Second)
		for time.Now().Before(deadline) && processAlive(pid) {
			time.Sleep(150 * time.Millisecond)
		}
		if processAlive(pid) {
			killDescendants(pid)
			proc.Signal(os.Kill)
		}
	}
	os.Remove(filepath.Join(e.projectRoot, ".formspec", "dev.pid"))
	return textResult(map[string]any{"running": false, "stopped": true, "pid": pid})
}

type restartServerArgs struct{}

// restartServer is a composite tool (03 §5): validate the spec tree first
// and refuse to restart when invalid; then stop → spawn `formspec dev`
// detached (log → .formspec/consult/server.log) → poll /health.
func (e *mcpEnv) restartServer(_ context.Context, _ *mcp.CallToolRequest, _ restartServerArgs) (*mcp.CallToolResult, any, error) {
	val, err := validateSpecTree(e.specPath, e.schemaDir)
	if err != nil {
		return errResult("validation error: " + err.Error())
	}
	if !val.OK {
		return textResult(map[string]any{
			"restarted":  false,
			"reason":     "spec tree invalid — fix problems before restarting",
			"validation": val,
		})
	}

	// Stop the current instance (if any).
	if pid, ok := e.devPID(); ok && processAlive(pid) {
		if proc, err := os.FindProcess(pid); err == nil {
			proc.Signal(syscallSIGTERM())
			deadline := time.Now().Add(8 * time.Second)
			for time.Now().Before(deadline) && processAlive(pid) {
				time.Sleep(150 * time.Millisecond)
			}
			if processAlive(pid) {
				killDescendants(pid)
				proc.Signal(os.Kill)
			}
		}
		os.Remove(filepath.Join(e.projectRoot, ".formspec", "dev.pid"))
	}

	// Spawn `formspec dev` detached, logs captured for boot-failure reporting.
	exe, err := os.Executable()
	if err != nil {
		return nil, nil, err
	}
	logPath := filepath.Join(e.consultDir(), "server.log")
	_ = os.MkdirAll(filepath.Dir(logPath), 0755)
	logFile, err := os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0644)
	if err != nil {
		return nil, nil, err
	}
	cmd := exec.Command(exe, "dev", "--spec", e.specPath)
	cmd.Stdout = logFile
	cmd.Stderr = logFile
	cmd.Dir = e.projectRoot
	cmd.SysProcAttr = detachedSysProcAttr()
	if err := cmd.Start(); err != nil {
		logFile.Close()
		return errResult("start formspec dev: " + err.Error())
	}
	go cmd.Wait() // reap when it exits; the PID file tracks liveness

	// Poll /health until the server answers (or the boot window expires).
	deadline := time.Now().Add(30 * time.Second)
	var health string
	var healthErr error
	for time.Now().Before(deadline) {
		if !processAlive(cmd.Process.Pid) && !pidFilePresent(e) {
			break // exited before writing the PID file — boot failure
		}
		if h, err := e.serverHealth(); err == nil {
			health = h
			healthErr = nil
			break
		} else {
			healthErr = err
		}
		time.Sleep(500 * time.Millisecond)
	}

	result := map[string]any{
		"restarted":  health != "",
		"pid":        cmd.Process.Pid,
		"log":        logPath,
		"validation": val,
	}
	if health != "" {
		result["health"] = json.RawMessage(health)
	} else {
		// Boot failure — surface the tail of the server log (03 §5 open
		// question resolved as: ringkasan error saja, bukan streaming).
		result["health_error"] = fmt.Sprintf("%v", healthErr)
		result["log_tail"] = tailFile(logPath, 40)
	}
	return textResult(result)
}

func pidFilePresent(e *mcpEnv) bool {
	_, ok := e.devPID()
	return ok
}

// tailFile returns the last n lines of a file ("" when unreadable).
func tailFile(path string, n int) string {
	b, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	lines := strings.Split(strings.TrimRight(string(b), "\n"), "\n")
	if len(lines) > n {
		lines = lines[len(lines)-n:]
	}
	return strings.Join(lines, "\n")
}
