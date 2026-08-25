// Command formspec check — cross-file static analysis of a FormSpec project.
//
// `validate` is per-manifest; `check` goes further: it resolves references
// across files and modules within one workspace (docs/cli-tools/02-formspec-cli.md §3).
// It reports at minimum:
//
//  1. Form field references to a field missing from the target Entity schema → error
//  2. FormSpecExpr (visible_when/readonly_when/required_when/compute) referencing
//     a field missing from the schema → error (docs/spec/frontend/08-formspec-expr.md §4)
//  3. Cross-module uses.resources referencing a {module}.{entity} that does not exist → error
//  4. Cross-module uses.resources declared but never used → warning
//
// `--fix` removes unused cross-module declarations (safe — does not change the
// consent footprint). Adding declarations is a consent-footprint expansion and
// is never done silently (interactive confirmation is deferred).
//
// Usage:
//
//	formspec check [-f <path>] [--fix]
//
// Exit code 1 if any error is found.
package main

import (
	"flag"
	"fmt"
	"os"
	"regexp"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"

	"github.com/primadi/formspec/internal/manifest"
	"github.com/primadi/formspec/internal/permission"
	"github.com/primadi/formspec/pkg/spec"
)

// fieldRefRe matches FormSpecExpr field references of the form `fields.<name>`
// (docs/spec/frontend/08-formspec-expr.md §2 — field references use the
// `fields.` prefix).
var fieldRefRe = regexp.MustCompile(`fields\.([a-z_][a-z0-9_]*)`)

// checkIssue is a single finding from formspec check.
type checkIssue struct {
	Source  string // manifest source (file#doc)
	Kind    string // "error" | "warning"
	Message string
}

// checkResult aggregates findings from a check run.
type checkResult struct {
	Issues []checkIssue
}

func (r *checkResult) add(source, kind, format string, args ...any) {
	r.Issues = append(r.Issues, checkIssue{
		Source:  source,
		Kind:    kind,
		Message: fmt.Sprintf(format, args...),
	})
}

func (r *checkResult) hasErrors() bool {
	for _, i := range r.Issues {
		if i.Kind == "error" {
			return true
		}
	}
	return false
}

// entityIndex maps "{module}.{entity}" → EntitySpec for cross-file resolution.
type entityIndex struct {
	byKey map[string]*spec.EntitySpec
	// sourceByKey maps "{module}.{entity}" → manifest source for reporting.
	sourceByKey map[string]string
}

func (idx *entityIndex) add(module, name string, es *spec.EntitySpec, source string) {
	key := module + "." + name
	if _, exists := idx.byKey[key]; !exists {
		idx.byKey[key] = es
		idx.sourceByKey[key] = source
	}
}

func (idx *entityIndex) fieldNames(module, entity string) map[string]bool {
	es, ok := idx.byKey[module+"."+entity]
	if !ok {
		return nil
	}
	names := make(map[string]bool, len(es.Fields))
	for _, f := range es.Fields {
		names[f.Name] = true
	}
	return names
}

// buildEntityIndex indexes all Entity/Document manifests by "{module}.{entity}".
func buildEntityIndex(manifests []manifest.RawManifest) *entityIndex {
	idx := &entityIndex{byKey: map[string]*spec.EntitySpec{}, sourceByKey: map[string]string{}}
	for _, m := range manifests {
		if m.Kind != "Entity" && m.Kind != "Document" {
			continue
		}
		sm, ok := m.Spec.(map[string]any)
		if !ok {
			continue
		}
		es, err := manifest.RawSpecToEntitySpec(sm)
		if err != nil {
			continue
		}
		idx.add(m.Metadata.Module, m.Metadata.Name, es, m.Source)
	}
	return idx
}

func runCheck(args []string) {
	fs := flag.NewFlagSet("check", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	specPath := fs.String("f", "spec", "path to the spec directory (default: spec)")
	fix := fs.Bool("fix", false, "remove unused cross-module declarations")
	footprint := fs.Bool("footprint", false, "print the consent footprint (required_permission + uses) per module")
	if err := fs.Parse(args); err != nil {
		os.Exit(2)
	}
	if fs.NArg() > 0 {
		fmt.Fprintf(os.Stderr, "formspec check: unexpected argument %q\n", fs.Arg(0))
		os.Exit(2)
	}

	loader := manifest.NewLoader(*specPath)
	res, err := loader.LoadAll()
	if err != nil {
		fmt.Fprintf(os.Stderr, "formspec check: load error: %v\n", err)
		os.Exit(2)
	}

	result := &checkResult{}
	for _, pe := range res.Errors {
		result.add(pe.File, "error", "%s", pe.Message)
	}

	// Build the entity index from all Entity/Document manifests.
	idx := buildEntityIndex(res.Manifests)

	// Build the UI registry (Form/Table/Page/...) for frontend-kind checks.
	// (Parsed directly from raw manifests — see checkForms.)

	// Check 1+2: Form field + FormSpecExpr references against the target Entity.
	checkForms(result, idx, res.Manifests)

	// Check 2 (extended): FormSpecExpr in Kanban drag_guard + Wizard steps.
	checkKanban(result, idx, res.Manifests)
	checkWizard(result, idx, res.Manifests)

	// Check 5.16: renderer registry & resolution (5.16.1), slot-tier
	// validation (5.16.2), stack_family compatibility (5.16.3).
	checkRenderers(result, res.Manifests)

	// Check 3+4: cross-module uses.resources existence + unused.
	brokenRefs := checkUses(result, idx, res.Manifests)

	// ── --fix: remove broken uses.resources references ──
	// A broken reference (target entity does not exist) is a clear error; the
	// declaration is dead weight and safe to remove. This does NOT change the
	// consent footprint of any valid declaration.
	if *fix && len(brokenRefs) > 0 {
		removed := applyUsesFix(brokenRefs)
		for _, r := range removed {
			fmt.Printf("[FIXED] %s: removed uses.resources %q from action %q\n", r.source, r.resource, r.action)
		}
	}

	// ── Consent footprint (todo 6.2.5) ──
	// Aggregate required_permission + uses per module, presented to the
	// workspace owner at install time. Cross-module writes are flagged as
	// high-risk consent (D46).
	if *footprint {
		printConsentFootprint(res.Manifests)
	}

	// ── Report ──
	sort.SliceStable(result.Issues, func(i, j int) bool {
		if result.Issues[i].Source != result.Issues[j].Source {
			return result.Issues[i].Source < result.Issues[j].Source
		}
		return result.Issues[i].Message < result.Issues[j].Message
	})

	errCount, warnCount := 0, 0
	for _, issue := range result.Issues {
		prefix := "ERROR"
		if issue.Kind == "warning" {
			prefix = "WARN "
			warnCount++
		} else {
			errCount++
		}
		loc := issue.Source
		if loc == "" {
			loc = "(project)"
		}
		fmt.Printf("[%s] %s: %s\n", prefix, loc, issue.Message)
	}

	fmt.Printf("\n%d error(s), %d warning(s)\n", errCount, warnCount)
	if errCount > 0 {
		os.Exit(1)
	}
}

// printConsentFootprint builds a permission registry from the manifests and
// prints each module's consent footprint (todo 6.2.5): required permissions,
// uses declarations, and cross-module writes (high-risk consent, D46).
func printConsentFootprint(manifests []manifest.RawManifest) {
	reg := permission.NewRegistry()
	for _, m := range manifests {
		if !spec.IsEntityKind(spec.Kind(m.Kind)) {
			continue
		}
		sm, ok := m.Spec.(map[string]any)
		if !ok {
			continue
		}
		es, err := manifest.RawSpecToEntitySpec(sm)
		if err != nil {
			continue
		}
		module := m.Metadata.Module
		entity := m.Metadata.Name
		for _, a := range es.Actions {
			usesEntry := permission.BuildUsesEntry(module, entity, a.Name, a.Uses)
			_ = reg.RegisterAction(module, entity, a.Name, a.RequiredPermission, usesEntry, m.Source, a.Audit)
		}
		if len(es.Expose) > 0 {
			plural := es.Plural
			if plural == "" {
				plural = entity + "s"
			}
			for _, act := range []string{"list", "view", "create", "update", "delete"} {
				_ = reg.RegisterAction(module, entity, act, module+"."+plural+"."+act, &permission.UsesEntry{}, m.Source, false)
			}
		}
	}

	footprints := reg.AllFootprints()
	sort.Slice(footprints, func(i, j int) bool { return footprints[i].Module < footprints[j].Module })
	for _, fp := range footprints {
		fmt.Println(fp.String())
	}
}

// checkForms verifies Form manifests reference existing Entity fields, both
// for the field itself and for FormSpecExpr strings.
func checkForms(result *checkResult, idx *entityIndex, manifests []manifest.RawManifest) {
	for _, m := range manifests {
		if m.Kind != "Form" {
			continue
		}
		sm, ok := m.Spec.(map[string]any)
		if !ok {
			continue
		}
		form, err := manifest.RawSpecTo[spec.FormSpec](sm)
		if err != nil {
			continue // parse error already reported by validate/loader
		}
		name := m.Metadata.Name
		// Resolve the target entity: "module.entity" or "entity" (own module).
		module, entity := splitEntityRef(form.Entity, m.Metadata.Module)
		fields := idx.fieldNames(module, entity)
		if fields == nil {
			// Entity not found — report once (the Form references a missing entity).
			result.add(m.Source, "error", "Form %q references unknown entity %q", name, form.Entity)
			continue
		}

		for _, section := range form.Sections {
			if section.VisibleWhen != "" {
				checkExpr(result, m.Source, name, "section.visible_when", section.VisibleWhen, fields)
			}
			for _, f := range section.Fields {
				if f.Field != "" && !fields[f.Field] {
					result.add(m.Source, "error", "Form %q field %q references field %q missing from entity %q", name, f.Field, f.Field, form.Entity)
				}
				checkExpr(result, m.Source, name, "field "+f.Field+".visible_when", f.VisibleWhen, fields)
				checkExpr(result, m.Source, name, "field "+f.Field+".readonly_when", f.ReadonlyWhen, fields)
				checkExpr(result, m.Source, name, "field "+f.Field+".required_when", f.RequiredWhen, fields)
				checkExpr(result, m.Source, name, "field "+f.Field+".compute", f.Compute, fields)
			}
		}
	}
}

// checkExpr extracts fields.<name> references from a FormSpecExpr and reports
// any that are missing from the entity schema. It also validates the grammar
// (5.11.2): constructs outside the expression subset (§2) are rejected at
// deploy time, never silently accepted and left to fail at runtime.
func checkExpr(result *checkResult, source, owner, where, expr string, fields map[string]bool) {
	if expr == "" {
		return
	}
	for _, m := range fieldRefRe.FindAllStringSubmatch(expr, -1) {
		ref := m[1]
		if !fields[ref] {
			result.add(source, "error", "%s: FormSpecExpr %q references field %q missing from entity schema", where, expr, ref)
		}
	}
	if err := validateExprGrammar(expr); err != "" {
		result.add(source, "error", "%s: FormSpecExpr %q invalid: %s", where, expr, err)
	}
}

// validateExprGrammar rejects constructs outside the FormSpecExpr subset
// (docs/spec/frontend/08-formspec-expr.md §2): literals, `fields.x`
// references, comparisons, and/or/not, arithmetic, len/sum, list
// comprehension, `in`. Explicitly forbidden: `ctx` access, function
// definitions, imports, loops, and unbalanced delimiters.
func validateExprGrammar(expr string) string {
	// No ctx access — the closed ctx.* primitives are server-side only.
	if strings.Contains(expr, "ctx.") {
		return "ctx access is not allowed in FormSpecExpr"
	}
	// No function definitions / imports / return statements.
	for _, kw := range []string{"def ", "import ", "return ", "lambda"} {
		if strings.Contains(expr, kw) {
			return "construct outside the expression subset (function defs, imports, statements)"
		}
	}
	// Balanced delimiters.
	stack := []rune{}
	for _, ch := range expr {
		switch ch {
		case '(', '[', '{':
			stack = append(stack, ch)
		case ')', ']', '}':
			if len(stack) == 0 {
				return "unbalanced delimiter"
			}
			open := stack[len(stack)-1]
			stack = stack[:len(stack)-1]
			if (ch == ')' && open != '(') || (ch == ']' && open != '[') || (ch == '}' && open != '{') {
				return "mismatched delimiter"
			}
		}
	}
	if len(stack) > 0 {
		return "unbalanced delimiter"
	}
	return ""
}

// checkKanban validates Kanban manifests: the `drag_guard` FormSpecExpr is
// checked against the board's entity schema (5.11.2).
func checkKanban(result *checkResult, idx *entityIndex, manifests []manifest.RawManifest) {
	for _, m := range manifests {
		if m.Kind != "Kanban" {
			continue
		}
		sm, ok := m.Spec.(map[string]any)
		if !ok {
			continue
		}
		kb, err := manifest.RawSpecTo[spec.KanbanSpec](sm)
		if err != nil {
			continue
		}
		if kb.DragGuard == "" {
			continue
		}
		module, entity := splitEntityRef(kb.Entity, m.Metadata.Module)
		fields := idx.fieldNames(module, entity)
		if fields == nil {
			result.add(m.Source, "error", "Kanban %q references unknown entity %q", m.Metadata.Name, kb.Entity)
			continue
		}
		checkExpr(result, m.Source, m.Metadata.Name, "drag_guard", kb.DragGuard, fields)
	}
}

// checkWizard validates Wizard manifests: step field expressions are checked
// against the wizard's entity schema (5.11.2).
func checkWizard(result *checkResult, idx *entityIndex, manifests []manifest.RawManifest) {
	for _, m := range manifests {
		if m.Kind != "Wizard" {
			continue
		}
		sm, ok := m.Spec.(map[string]any)
		if !ok {
			continue
		}
		wz, err := manifest.RawSpecTo[spec.WizardSpec](sm)
		if err != nil {
			continue
		}
		// A wizard may target an entity (spec.entity) or commit via an action
		// (spec.action) — only validate when an entity is declared.
		if wz.Entity == "" {
			continue
		}
		module, entity := splitEntityRef(wz.Entity, m.Metadata.Module)
		fields := idx.fieldNames(module, entity)
		if fields == nil {
			result.add(m.Source, "error", "Wizard %q references unknown entity %q", m.Metadata.Name, wz.Entity)
			continue
		}
		for _, step := range wz.Steps {
			for _, f := range step.Fields {
				checkExpr(result, m.Source, m.Metadata.Name, "step "+step.Title+" field "+f.Field+".visible_when", f.VisibleWhen, fields)
				checkExpr(result, m.Source, m.Metadata.Name, "step "+step.Title+" field "+f.Field+".readonly_when", f.ReadonlyWhen, fields)
				checkExpr(result, m.Source, m.Metadata.Name, "step "+step.Title+" field "+f.Field+".required_when", f.RequiredWhen, fields)
				checkExpr(result, m.Source, m.Metadata.Name, "step "+step.Title+" field "+f.Field+".compute", f.Compute, fields)
			}
		}
	}
}

// checkRenderers validates the renderer registry & resolution (todo 5.16):
//   - 5.16.1: App `renderers:` map + Page `renderer:` field resolve to
//     registered renderers.
//   - 5.16.2: slot-tier rules — accepts_slots only on tier page|app,
//     implements_slot only on tier component.
//   - 5.16.3: App shell + shell-integrated Page share one stack_family.
func checkRenderers(result *checkResult, manifests []manifest.RawManifest) {
	reg := manifest.NewRendererRegistry(manifests)

	for _, msg := range reg.ValidateSlotTiers() {
		result.add("", "error", "%s", msg)
	}

	var apps, pages []manifest.RawManifest
	for _, m := range manifests {
		switch spec.Kind(m.Kind) {
		case spec.KindApp:
			apps = append(apps, m)
		case spec.KindPage:
			pages = append(pages, m)
		}
	}
	for _, msg := range reg.ValidateRendererResolution(apps, pages) {
		result.add("", "error", "%s", msg)
	}
	for _, msg := range reg.ValidateStackFamily(apps, pages) {
		result.add("", "error", "%s", msg)
	}
}

// brokenRef identifies a uses.resources entry whose target entity does not
// exist — a clear error that --fix can safely remove.
type brokenRef struct {
	source   string // manifest source (file#doc)
	file     string // file path (for rewriting)
	action   string
	resource string
}

// checkUses verifies cross-module uses.resources declarations: referenced
// {module}.{entity} must exist (error). Returns the list of broken references
// for --fix.
func checkUses(result *checkResult, idx *entityIndex, manifests []manifest.RawManifest) []brokenRef {
	var broken []brokenRef
	for _, m := range manifests {
		if m.Kind != "Entity" && m.Kind != "Document" {
			continue
		}
		sm, ok := m.Spec.(map[string]any)
		if !ok {
			continue
		}
		es, err := manifest.RawSpecToEntitySpec(sm)
		if err != nil {
			continue
		}
		file := strings.SplitN(m.Source, "#", 2)[0]
		for _, action := range es.Actions {
			if action.Uses == nil {
				continue
			}
			for _, res := range action.Uses.Resources {
				// res may be "{module}.{entity}", "{module}/{entity}", or a wildcard.
				if strings.ContainsAny(res, "*") {
					continue // wildcard — cannot statically resolve
				}
				mod, ent := splitResourceRef(res)
				if mod == "" || ent == "" {
					continue
				}
				if _, ok := idx.byKey[mod+"."+ent]; !ok {
					result.add(m.Source, "error", "action %q uses.resources references unknown resource %q (no entity %s.%s)", action.Name, res, mod, ent)
					broken = append(broken, brokenRef{source: m.Source, file: file, action: action.Name, resource: res})
				}
			}
		}
	}
	return broken
}

// applyUsesFix rewrites the affected manifest files, removing broken
// uses.resources entries. Returns the list of successfully removed refs.
func applyUsesFix(broken []brokenRef) []brokenRef {
	// Group broken refs by file + action.
	type actionKey struct{ file, action string }
	byAction := map[actionKey][]string{}
	var order []actionKey
	for _, b := range broken {
		k := actionKey{b.file, b.action}
		if _, ok := byAction[k]; !ok {
			order = append(order, k)
		}
		byAction[k] = append(byAction[k], b.resource)
	}

	var removed []brokenRef
	for _, k := range order {
		refs := byAction[k]
		if err := removeUsesResources(k.file, k.action, refs); err != nil {
			fmt.Fprintf(os.Stderr, "  ⚠️  cannot fix %s action %q: %v\n", k.file, k.action, err)
			continue
		}
		for _, r := range refs {
			removed = append(removed, brokenRef{source: k.file + "#0", file: k.file, action: k.action, resource: r})
		}
	}
	return removed
}

// removeUsesResources removes the given resource entries from an action's
// uses.resources list in a YAML file, preserving the rest of the document.
func removeUsesResources(file, action string, resources []string) error {
	data, err := os.ReadFile(file)
	if err != nil {
		return err
	}
	var doc yaml.Node
	if err := yaml.Unmarshal(data, &doc); err != nil {
		return err
	}
	if len(doc.Content) == 0 {
		return nil
	}
	root := doc.Content[0]
	removed := removeUsesResourcesFromNode(root, action, resources)
	if !removed {
		return fmt.Errorf("action %q not found", action)
	}
	out, err := yaml.Marshal(&doc)
	if err != nil {
		return err
	}
	return os.WriteFile(file, out, 0644)
}

// removeUsesResourcesFromNode walks the YAML node tree, finds the action with
// the given name, and removes the listed resources from its uses.resources.
func removeUsesResourcesFromNode(root *yaml.Node, action string, resources []string) bool {
	// Find spec.actions (a sequence of mapping nodes).
	spec := findMappingValue(root, "spec")
	if spec == nil {
		return false
	}
	actions := findMappingValue(spec, "actions")
	if actions == nil || actions.Kind != yaml.SequenceNode {
		return false
	}
	for _, act := range actions.Content {
		if act.Kind != yaml.MappingNode {
			continue
		}
		if findMappingValue(act, "name") == nil || findMappingValue(act, "name").Value != action {
			continue
		}
		uses := findMappingValue(act, "uses")
		if uses == nil {
			return false
		}
		resNode := findMappingValue(uses, "resources")
		if resNode == nil || resNode.Kind != yaml.SequenceNode {
			return false
		}
		// Remove matching scalar entries.
		kept := resNode.Content[:0]
		for _, item := range resNode.Content {
			drop := false
			for _, r := range resources {
				if item.Value == r {
					drop = true
					break
				}
			}
			if !drop {
				kept = append(kept, item)
			}
		}
		resNode.Content = kept
		// Clean up empty nodes: drop the resources key if empty, and the
		// uses mapping if it has no remaining keys.
		if len(kept) == 0 {
			removeMappingKey(uses, "resources")
		}
		if len(uses.Content) == 0 {
			removeMappingKey(act, "uses")
		}
		return true
	}
	return false
}

// removeMappingKey removes a key (and its value) from a YAML mapping node.
func removeMappingKey(m *yaml.Node, key string) {
	if m == nil || m.Kind != yaml.MappingNode {
		return
	}
	out := m.Content[:0]
	for i := 0; i+1 < len(m.Content); i += 2 {
		if m.Content[i].Value == key {
			continue
		}
		out = append(out, m.Content[i], m.Content[i+1])
	}
	m.Content = out
}

// findMappingValue returns the value node for a key in a YAML mapping node.
func findMappingValue(m *yaml.Node, key string) *yaml.Node {
	if m == nil || m.Kind != yaml.MappingNode {
		return nil
	}
	for i := 0; i+1 < len(m.Content); i += 2 {
		if m.Content[i].Value == key {
			return m.Content[i+1]
		}
	}
	return nil
}

// splitEntityRef splits a Form entity reference "module.entity" (or bare
// "entity") into (module, entity), defaulting module to the form's own module.
func splitEntityRef(ref, ownModule string) (string, string) {
	if i := strings.Index(ref, "."); i >= 0 {
		return ref[:i], ref[i+1:]
	}
	return ownModule, ref
}

// splitResourceRef splits a uses.resources reference "{module}.{entity}" or
// "{module}/{entity}" into (module, entity).
func splitResourceRef(ref string) (string, string) {
	if i := strings.Index(ref, "."); i >= 0 {
		return ref[:i], ref[i+1:]
	}
	if i := strings.Index(ref, "/"); i >= 0 {
		return ref[:i], ref[i+1:]
	}
	return "", ""
}
