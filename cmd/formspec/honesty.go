// Command formspec validate — Starlark honesty scan (todo 3.1.1a).
//
// Static analysis of every script referenced by an action/hook impl
// (script_ref / script) against the action's declared `uses:` block:
//
//   - undeclared usage  → ERROR  (script uses ctx.<primitive> /
//     resource.call/fetch/create target / ctx.secrets key that the uses
//     block does not declare — the action would fail USES_VIOLATION in
//     ProdMode/StrictMode)
//   - declared-but-unused → WARNING (uses declares a primitive/resource/
//     secret the script never touches — consent footprint larger than
//     reality; safe to remove)
//   - ctx.environment branching → WARNING (environment-dependent logic is
//     a Control Plane concept; single-server has no environment switch)
//
// --fix removes declared-but-unused entries (shrinks the consent footprint;
// never adds). Adding declarations expands consent and stays manual per the
// repo precedent (see 3.1.2 --fix semantics).
package main

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"go.starlark.net/syntax"
	"gopkg.in/yaml.v3"

	"github.com/primadi/formspec/internal/manifest"
	"github.com/primadi/formspec/pkg/spec"
)

// resolverRoutedPrimitives is the closed set of ctx.* primitives routed
// through the datastore resolver (platform/06-datastore.md §2). config/log
// are separate builtins and not part of uses.primitives enforcement.
var resolverRoutedPrimitives = map[string]bool{
	"db": true, "cache": true, "lock": true, "queue": true,
	"pubsub": true, "storage": true, "kvstore": true,
}

// resourceMethods are ResourceAPI methods whose first string argument is a
// cross-module target ("{module}.{entity}").
var resourceMethods = map[string]bool{
	"resource.call": true, "resource.fetch": true, "resource.create": true,
}

// honestyIssue is one finding of the scan.
type honestyIssue struct {
	Source   string // manifest source (file#doc)
	Script   string // script file path ("" when not applicable)
	Severity string // "error" | "warning"
	Message  string
	// Fix metadata (declared-but-unused only):
	FixKind  string // "" | "primitive" | "resource" | "secret"
	Action   string // action/hook owner name
	Entry    string // declaration to remove
	IsHook   bool   // owner is a hook, not an action
	HookOn   string // hook on: value (for YAML navigation)
	HookActn string // hook action: value
}

// scriptUsage is what one script actually uses.
type scriptUsage struct {
	primitives map[string]bool
	resources  map[string]bool
	secrets    map[string]bool
	envBranch  bool
	parseErr   error
}

// scanHonesty runs the static honesty scan over all manifests. specPath is
// the spec root used for script resolution fallbacks.
func scanHonesty(manifests []manifest.RawManifest, specPath string) []honestyIssue {
	var issues []honestyIssue

	for _, m := range manifests {
		var actions []spec.Action
		var hooks []spec.HookDecl

		sm, ok := m.Spec.(map[string]any)
		if !ok {
			continue
		}
		switch spec.Kind(m.Kind) {
		case spec.KindEntity:
			es, err := manifest.RawSpecToEntitySpec(sm)
			if err != nil {
				continue
			}
			actions = es.Actions
			hooks = es.Hooks
		case spec.KindService:
			ss, err := manifest.RawSpecTo[spec.ServiceSpec](sm)
			if err != nil {
				continue
			}
			actions = ss.Actions
		default:
			continue
		}

		for i := range actions {
			a := actions[i]
			if a.Impl == nil {
				continue
			}
			if a.Impl.Type != spec.ImplScriptRef && a.Impl.Type != spec.ImplScript {
				continue
			}
			// A missing uses block scans as fully undeclared — that is
			// exactly the honesty violation the scan exists to surface.
			uses := a.Uses
			if uses == nil {
				uses = &spec.UsesDecl{}
			}
			path := resolveHonestyScript(specPath, m, a.Impl.Ref)
			usage := parseScriptUsage(path)
			issues = append(issues, compareUses(m.Source, path, a.Name, uses, usage, false, "", "")...)
		}

		for _, h := range hooks {
			if h.Impl == nil || h.Impl.Type != spec.ImplScriptRef && h.Impl.Type != spec.ImplScript {
				continue
			}
			// Hooks inherit the entity's uses? No — HookDecl has no Uses;
			// hooks run under the entity module's own resources. Only the
			// ctx.environment warning applies to hooks today.
			path := resolveHonestyScript(specPath, m, h.Impl.Ref)
			usage := parseScriptUsage(path)
			if usage.envBranch {
				issues = append(issues, honestyIssue{
					Source: m.Source, Script: path, Severity: "warning",
					Message: fmt.Sprintf("hook %s (action %s): branches on ctx.environment — environment-dependent logic is a Control Plane concept; single-server has no environment switch", h.On, h.Action),
					IsHook:  true, HookOn: string(h.On), HookActn: h.Action,
				})
			}
		}
	}

	sort.SliceStable(issues, func(i, j int) bool {
		if issues[i].Source != issues[j].Source {
			return issues[i].Source < issues[j].Source
		}
		return issues[i].Message < issues[j].Message
	})
	return issues
}

// compareUses diffs one script's actual usage against its declared uses.
func compareUses(source, scriptPath, owner string, uses *spec.UsesDecl, u *scriptUsage, isHook bool, hookOn, hookAction string) []honestyIssue {
	var issues []honestyIssue
	where := fmt.Sprintf("action %q", owner)
	if isHook {
		where = fmt.Sprintf("hook %s (action %s)", hookOn, hookAction)
	}

	if u.parseErr != nil {
		issues = append(issues, honestyIssue{
			Source: source, Script: scriptPath, Severity: "error",
			Message: fmt.Sprintf("%s: script parse failed: %v", where, u.parseErr),
		})
		return issues
	}

	declaredPrims := map[string]bool{}
	for _, p := range uses.Primitives {
		declaredPrims[p] = true
	}
	for p := range u.primitives {
		if !declaredPrims[p] {
			issues = append(issues, honestyIssue{
				Source: source, Script: scriptPath, Severity: "error",
				Message: fmt.Sprintf("%s: uses ctx.%s() but does not declare it — add primitives: [%s] to uses (USES_VIOLATION in strict mode)", where, p, p),
			})
		}
	}
	for _, p := range uses.Primitives {
		if !u.primitives[p] {
			issues = append(issues, honestyIssue{
				Source: source, Script: scriptPath, Severity: "warning",
				Message: fmt.Sprintf("%s: uses declares primitive %q but the script never uses it", where, p),
				FixKind: "primitive", Action: owner, Entry: p, IsHook: isHook,
				HookOn: hookOn, HookActn: hookAction,
			})
		}
	}

	for r := range u.resources {
		// Only cross-module targets (containing "." or "/") require a
		// declaration — same-module bare names resolve implicitly
		// (01-core-basic.md §5, matches runtime uses enforcement).
		if !strings.ContainsAny(r, "./") {
			continue
		}
		if !resourceDeclared(uses.Resources, r) {
			issues = append(issues, honestyIssue{
				Source: source, Script: scriptPath, Severity: "error",
				Message: fmt.Sprintf("%s: accesses resource %q but does not declare it — add resources: [%q] to uses (USES_VIOLATION on cross-module calls)", where, r, r),
			})
		}
	}
	for _, r := range uses.Resources {
		if declaredResourceUnused(u.resources, r) {
			issues = append(issues, honestyIssue{
				Source: source, Script: scriptPath, Severity: "warning",
				Message: fmt.Sprintf("%s: uses declares resource %q but the script never accesses it", where, r),
				FixKind: "resource", Action: owner, Entry: r, IsHook: isHook,
				HookOn: hookOn, HookActn: hookAction,
			})
		}
	}

	declaredSecrets := map[string]bool{}
	for _, s := range uses.Secrets {
		declaredSecrets[s] = true
	}
	for s := range u.secrets {
		if !declaredSecrets[s] {
			issues = append(issues, honestyIssue{
				Source: source, Script: scriptPath, Severity: "error",
				Message: fmt.Sprintf("%s: reads secret %q but does not declare it — add secrets: [%q] to uses", where, s, s),
			})
		}
	}
	for _, s := range uses.Secrets {
		if !u.secrets[s] {
			issues = append(issues, honestyIssue{
				Source: source, Script: scriptPath, Severity: "warning",
				Message: fmt.Sprintf("%s: uses declares secret %q but the script never reads it", where, s),
				FixKind: "secret", Action: owner, Entry: s, IsHook: isHook,
				HookOn: hookOn, HookActn: hookAction,
			})
		}
	}

	if u.envBranch {
		issues = append(issues, honestyIssue{
			Source: source, Script: scriptPath, Severity: "warning",
			Message: fmt.Sprintf("%s: branches on ctx.environment — environment-dependent logic is a Control Plane concept; single-server has no environment switch", where),
		})
	}
	return issues
}

// resourceDeclared reports whether a used target is covered by the declared
// list (exact match or wildcard: "{module}.*", "*").
func resourceDeclared(declared []string, target string) bool {
	for _, d := range declared {
		if d == "*" || d == target {
			return true
		}
		if strings.HasSuffix(d, ".*") && strings.HasPrefix(target+".", strings.TrimSuffix(d, "*")) {
			return true
		}
	}
	return false
}

// declaredResourceUnused reports whether a declared resource entry is truly
// unused: not matched by any actual usage, and not a wildcard whose prefix
// covers an actual usage ("*" is unused only when nothing was used at all).
func declaredResourceUnused(used map[string]bool, decl string) bool {
	if used[decl] {
		return false
	}
	if decl == "*" {
		return len(used) == 0
	}
	if strings.HasSuffix(decl, ".*") {
		prefix := strings.TrimSuffix(decl, "*")
		for u := range used {
			if strings.HasPrefix(u+".", prefix) {
				return false
			}
		}
	}
	return true
}

// resolveHonestyScript mirrors internal/action.resolveScript for validate-time
// resolution: entity-relative first, then spec-root fallbacks.
func resolveHonestyScript(specPath string, m manifest.RawManifest, ref string) string {
	source := m.Source
	if idx := strings.LastIndex(source, "#"); idx >= 0 {
		source = source[:idx]
	}
	specDir := filepath.Dir(source)

	candidates := []string{
		filepath.Join(specDir, "scripts", ref+".star"),
		filepath.Join(specDir, ref+".star"),
	}
	name := ref
	if idx := strings.LastIndex(ref, "/"); idx >= 0 {
		name = ref[idx+1:]
	}
	if m.Metadata.Module != "" {
		candidates = append(candidates,
			filepath.Join(specPath, "modules", m.Metadata.Module, "scripts", name+".star"),
			filepath.Join(specPath, "modules", ref+".star"),
		)
	}
	candidates = append(candidates,
		filepath.Join(specPath, ref+".star"),
		filepath.Join(specPath, "scripts", name+".star"),
	)
	for _, c := range candidates {
		if st, err := os.Stat(c); err == nil && !st.IsDir() {
			return c
		}
	}
	return ""
}

// parseScriptUsage parses a .star file and extracts ctx.*/resource usages.
func parseScriptUsage(path string) *scriptUsage {
	u := &scriptUsage{
		primitives: map[string]bool{},
		resources:  map[string]bool{},
		secrets:    map[string]bool{},
	}
	if path == "" {
		return u
	}
	src, err := os.ReadFile(path)
	if err != nil {
		u.parseErr = err
		return u
	}
	f, err := syntax.Parse(path, src, 0)
	if err != nil {
		u.parseErr = err
		return u
	}
	walkFile(f, func(e syntax.Expr) {
		switch node := e.(type) {
		case *syntax.DotExpr:
			if id, ok := node.X.(*syntax.Ident); ok && id.Name == "ctx" {
				if resolverRoutedPrimitives[node.Name.Name] {
					u.primitives[node.Name.Name] = true
				}
				if node.Name.Name == "environment" {
					u.envBranch = true
				}
			}
		case *syntax.CallExpr:
			fp := dottedPath(node.Fn)
			if resourceMethods[fp] && len(node.Args) > 0 {
				if lit, ok := node.Args[0].(*syntax.Literal); ok && lit.Token == syntax.STRING {
					target := strings.Trim(lit.Raw, "\"'`")
					u.resources[target] = true
				}
			}
			if fp == "ctx.secrets.get" && len(node.Args) > 0 {
				if lit, ok := node.Args[0].(*syntax.Literal); ok && lit.Token == syntax.STRING {
					key := strings.Trim(lit.Raw, "\"'`")
					u.secrets[key] = true
				}
			}
		}
	})
	return u
}

// dottedPath renders a call target as a dotted path ("ctx.secrets.get") for
// DotExpr chains rooted at an Ident; returns "" otherwise.
func dottedPath(e syntax.Expr) string {
	var parts []string
	for {
		switch node := e.(type) {
		case *syntax.DotExpr:
			parts = append([]string{node.Name.Name}, parts...)
			e = node.X
		case *syntax.Ident:
			parts = append([]string{node.Name}, parts...)
			return strings.Join(parts, ".")
		default:
			return ""
		}
	}
}

// walkFile visits every expression in the file (pre-order).
func walkFile(f *syntax.File, visit func(syntax.Expr)) {
	for _, s := range f.Stmts {
		walkStmt(s, visit)
	}
}

func walkStmt(s syntax.Stmt, visit func(syntax.Expr)) {
	switch node := s.(type) {
	case *syntax.ExprStmt:
		walkExpr(node.X, visit)
	case *syntax.AssignStmt:
		// Covers both `x = y` and augmented assignments (`x += y`) — this
		// starlark-go version has no separate AugAssignStmt type.
		walkExpr(node.LHS, visit)
		walkExpr(node.RHS, visit)
	case *syntax.DefStmt:
		for _, p := range node.Params {
			walkExpr(p, visit)
		}
		for _, st := range node.Body {
			walkStmt(st, visit)
		}
	case *syntax.IfStmt:
		walkExpr(node.Cond, visit)
		for _, st := range node.True {
			walkStmt(st, visit)
		}
		for _, st := range node.False {
			walkStmt(st, visit)
		}
	case *syntax.ForStmt:
		walkExpr(node.Vars, visit)
		walkExpr(node.X, visit)
		for _, st := range node.Body {
			walkStmt(st, visit)
		}
	case *syntax.WhileStmt:
		walkExpr(node.Cond, visit)
		for _, st := range node.Body {
			walkStmt(st, visit)
		}
	case *syntax.ReturnStmt:
		if node.Result != nil {
			walkExpr(node.Result, visit)
		}
	case *syntax.LoadStmt:
		// load() imports carry no ctx/resource usage.
	default:
	}
}

func walkExpr(e syntax.Expr, visit func(syntax.Expr)) {
	if e == nil {
		return
	}
	visit(e)
	switch node := e.(type) {
	case *syntax.CallExpr:
		walkExpr(node.Fn, visit)
		for _, a := range node.Args {
			walkExpr(a, visit)
		}
	case *syntax.BinaryExpr:
		walkExpr(node.X, visit)
		walkExpr(node.Y, visit)
	case *syntax.UnaryExpr:
		walkExpr(node.X, visit)
	case *syntax.CondExpr:
		walkExpr(node.Cond, visit)
		walkExpr(node.True, visit)
		walkExpr(node.False, visit)
	case *syntax.DotExpr:
		walkExpr(node.X, visit)
	case *syntax.IndexExpr:
		walkExpr(node.X, visit)
		walkExpr(node.Y, visit)
	case *syntax.SliceExpr:
		if node.X != nil {
			walkExpr(node.X, visit)
		}
		if node.Lo != nil {
			walkExpr(node.Lo, visit)
		}
		if node.Hi != nil {
			walkExpr(node.Hi, visit)
		}
		if node.Step != nil {
			walkExpr(node.Step, visit)
		}
	case *syntax.ListExpr:
		for _, item := range node.List {
			walkExpr(item, visit)
		}
	case *syntax.TupleExpr:
		for _, item := range node.List {
			walkExpr(item, visit)
		}
	case *syntax.DictExpr:
		for _, entry := range node.List {
			if de, ok := entry.(*syntax.DictEntry); ok {
				walkExpr(de.Key, visit)
				walkExpr(de.Value, visit)
			}
		}
	case *syntax.ParenExpr:
		walkExpr(node.X, visit)
	case *syntax.LambdaExpr:
		for _, p := range node.Params {
			walkExpr(p, visit)
		}
		walkExpr(node.Body, visit)
	case *syntax.Comprehension:
		walkExpr(node.Body, visit)
		for _, cl := range node.Clauses {
			switch c := cl.(type) {
			case *syntax.ForClause:
				walkExpr(c.Vars, visit)
				walkExpr(c.X, visit)
			case *syntax.IfClause:
				walkExpr(c.Cond, visit)
			}
		}
	case *syntax.Literal, *syntax.Ident:
		// leaves
	default:
	}
}

// applyHonestyFix removes declared-but-unused entries from the manifest YAML
// files. It NEVER adds declarations (consent expansion stays manual).
func applyHonestyFix(manifests []manifest.RawManifest, issues []honestyIssue) (removed int) {
	bySource := map[string][]honestyIssue{}
	for _, iss := range issues {
		if iss.FixKind == "" {
			continue
		}
		bySource[iss.Source] = append(bySource[iss.Source], iss)
	}

	sources := make([]string, 0, len(bySource))
	for s := range bySource {
		sources = append(sources, s)
	}
	sort.Strings(sources)

	for _, src := range sources {
		file := src
		docIdx := 0
		if idx := strings.LastIndex(src, "#"); idx >= 0 {
			file = src[:idx]
			fmt.Sscanf(src[idx+1:], "%d", &docIdx)
		}
		n, err := fixManifestFile(file, docIdx, bySource[src])
		if err != nil {
			fmt.Fprintf(os.Stderr, "[FIX FAILED] %s: %v\n", src, err)
			continue
		}
		removed += n
	}
	return removed
}

// fixManifestFile edits one YAML document: removes unused primitives/
// resources/secrets entries from the matching action/hook uses block.
func fixManifestFile(file string, docIdx int, issues []honestyIssue) (int, error) {
	raw, err := os.ReadFile(file)
	if err != nil {
		return 0, err
	}
	var docs yaml.Node
	if err := yaml.Unmarshal(raw, &docs); err != nil {
		return 0, err
	}
	if docs.Kind != yaml.DocumentNode {
		return 0, fmt.Errorf("not a YAML document")
	}

	// Collect all documents (multi-doc files supported).
	var targets []*yaml.Node
	if len(docs.Content) == 1 && docs.Content[0].Kind == yaml.MappingNode {
		targets = append(targets, docs.Content[0])
	} else {
		for _, d := range docs.Content {
			if d.Kind == yaml.DocumentNode && len(d.Content) > 0 {
				targets = append(targets, d.Content[0])
			} else if d.Kind == yaml.MappingNode {
				targets = append(targets, d)
			}
		}
	}
	if docIdx >= len(targets) {
		return 0, fmt.Errorf("document index %d out of range", docIdx)
	}
	root := targets[docIdx]

	removed := 0
	for _, iss := range issues {
		if fixOneUse(root, iss) {
			removed++
		}
	}
	if removed == 0 {
		return 0, nil
	}

	out, err := yaml.Marshal(root)
	if err != nil {
		return 0, err
	}
	if err := os.WriteFile(file, out, 0o644); err != nil {
		return 0, err
	}
	return removed, nil
}

// fixOneUse locates the action/hook uses block in the document node and
// removes the declared entry. Returns true when removed.
func fixOneUse(root *yaml.Node, iss honestyIssue) bool {
	specNode := mappingValue(root, "spec")
	if specNode == nil {
		return false
	}
	var ownerNode *yaml.Node
	if iss.IsHook {
		hooks := mappingValue(specNode, "hooks")
		if hooks == nil {
			return false
		}
		for _, h := range sequenceItems(hooks) {
			on := mappingValue(h, "on")
			act := mappingValue(h, "action")
			if on != nil && act != nil &&
				nodeString(on) == iss.HookOn && nodeString(act) == iss.HookActn {
				ownerNode = h
				break
			}
		}
	} else {
		actions := mappingValue(specNode, "actions")
		for _, a := range sequenceItems(actions) {
			n := mappingValue(a, "name")
			if n != nil && nodeString(n) == iss.Action {
				ownerNode = a
				break
			}
		}
	}
	if ownerNode == nil {
		return false
	}
	uses := mappingValue(ownerNode, "uses")
	if uses == nil {
		return false
	}
	key := map[string]string{
		"primitive": "primitives",
		"resource":  "resources",
		"secret":    "secrets",
	}[iss.FixKind]
	list := mappingValue(uses, key)
	if list == nil || list.Kind != yaml.SequenceNode {
		return false
	}
	kept := list.Content[:0]
	found := false
	for _, item := range list.Content {
		if nodeString(item) == iss.Entry && !found {
			found = true
			continue
		}
		kept = append(kept, item)
	}
	if !found {
		return false
	}
	list.Content = kept
	if len(list.Content) == 0 {
		removeMappingKey(uses, key)
	}
	// Prune an emptied uses block entirely.
	if len(uses.Content) == 0 {
		removeMappingKey(ownerNode, "uses")
	}
	return true
}

// ─── small yaml.Node helpers ───

// sequenceItems returns the items of a YAML sequence node (nil-safe).
func sequenceItems(n *yaml.Node) []*yaml.Node {
	if n == nil || n.Kind != yaml.SequenceNode {
		return nil
	}
	return n.Content
}

func mappingValue(m *yaml.Node, key string) *yaml.Node {
	if m == nil || m.Kind != yaml.MappingNode {
		return nil
	}
	for i := 0; i+1 < len(m.Content); i += 2 {
		if nodeString(m.Content[i]) == key {
			return m.Content[i+1]
		}
	}
	return nil
}

func nodeString(n *yaml.Node) string {
	if n == nil {
		return ""
	}
	return n.Value
}
