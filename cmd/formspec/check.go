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
//  5. Cross-file NAMES: `spec.entity` and view refs (form/table/component/
//     widget) that no manifest declares → error (todo 10.12)
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
	"strconv"
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

// fieldType resolves a field's declared type on an indexed entity.
func (idx *entityIndex) fieldType(module, entity, field string) (spec.FieldType, bool) {
	es, ok := idx.byKey[module+"."+entity]
	if !ok {
		return "", false
	}
	for _, f := range es.Fields {
		if f.Name == field {
			return f.Type, true
		}
	}
	return "", false
}

// fieldDecl returns the full declaration of a field on an indexed entity.
//
// The cardinality gate needs more than the type name: `multiple` and `options`
// live on the declaration, and a Form `widget:` can only be judged against them
// (see spec.WidgetCardinalityMismatch).
func (idx *entityIndex) fieldDecl(module, entity, field string) (*spec.Field, bool) {
	es, ok := idx.byKey[module+"."+entity]
	if !ok {
		return nil, false
	}
	for i := range es.Fields {
		if es.Fields[i].Name == field {
			return &es.Fields[i], true
		}
	}
	return nil, false
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
		_, _ = fmt.Fprintf(os.Stderr, "formspec check: unexpected argument %q\n", fs.Arg(0))
		os.Exit(2)
	}

	loader := manifest.NewLoader(*specPath)
	res, err := loader.LoadAll()
	if err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "formspec check: load error: %v\n", err)
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

	// Check 2 (menu): MenuItem.When on App/Module — grammar + closed callable
	// set only (no entity schema to resolve field refs against).
	checkMenuExpr(result, res.Manifests)

	// Check 6: aggregate declarations (Report columns/totals, Widget config) —
	// SUM/AVG/MIN/MAX only mean something over a numeric or money field (S7).
	checkAggregates(result, idx, res.Manifests)

	// Check 5.16: renderer registry & resolution (5.16.1), slot-tier
	// validation (5.16.2), stack_family compatibility (5.16.3).
	checkRenderers(result, res.Manifests)
	checkAppWorkspaces(result, res.Manifests)

	// Check 3+4: cross-module uses.resources existence + unused.
	brokenRefs := checkUses(result, idx, res.Manifests)

	// Check 7: cross-file NAMES — `spec.entity` and view refs
	// (form/table/component/widget) that no manifest declares (10.12).
	checkReferences(result, idx, res.Manifests)

	// Check 2.9.4: kind: Datastore driver×serves compatibility +
	// module `spec.datastore` binding targets (platform/06-datastore.md §1.1/§2).
	checkDatastores(result, res.Manifests)

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
				// The Form follows the Entity's cardinality: a widget that picks
				// one value on a set field (or tags on a single-value field) would
				// render a control the data cannot honour. Reported here, at deploy
				// time, rather than as a surprise in the browser.
				// `WidgetCardinalityMismatch` already names the field, so the
				// prefix only says which Form made the claim.
				if f.Field != "" && f.Widget != "" {
					if decl, ok := idx.fieldDecl(module, entity, f.Field); ok {
						if err := spec.WidgetCardinalityMismatch(decl, f.Widget); err != nil {
							result.add(m.Source, "error", "Form %q: %v", name, err)
						}
					}
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
func checkExpr(result *checkResult, source, _, where, expr string, fields map[string]bool) {
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
//
// The expression is scanned with the same rules as the client lexer
// (renderers/react-shadcn/src/lib/formspec-expr/lexer.ts) so that anything
// accepted here can actually be evaluated there. Checking only for balanced
// delimiters was not enough: the kafe promo-form used `== 'percentage'`, which
// balances fine and reported 0 errors, yet every field failed at runtime with
// "unexpected token: '" because the client lexer did not recognise single
// quotes. A gate that passes an expression the renderer cannot parse is a false
// guarantee — §4 says expressions surviving apply are resolvable at runtime.
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

	stack := []rune{}
	runes := []rune(expr)
	for i := 0; i < len(runes); i++ {
		ch := runes[i]

		// Whitespace separates tokens.
		if ch == ' ' || ch == '\t' || ch == '\n' || ch == '\r' {
			continue
		}

		// String literals are opaque: delimiters inside them do not nest
		// (`fields.name != "("` is balanced). Both quote styles are valid
		// Starlark, so both are accepted — see readString in the client lexer.
		if ch == '"' || ch == '\'' {
			quote := ch
			startCol := i + 1
			closed := false
			for i++; i < len(runes); i++ {
				if runes[i] == '\\' {
					i++ // skip the escaped character
					continue
				}
				if runes[i] == quote {
					closed = true
					break
				}
			}
			if !closed {
				return fmt.Sprintf("unterminated string literal starting at column %d", startCol)
			}
			continue
		}

		// Delimiters.
		switch ch {
		case '(', '[':
			stack = append(stack, ch)
			continue
		case ')', ']':
			if len(stack) == 0 {
				return "unbalanced delimiter"
			}
			open := stack[len(stack)-1]
			stack = stack[:len(stack)-1]
			if (ch == ')' && open != '(') || (ch == ']' && open != '[') {
				return "mismatched delimiter"
			}
			continue
		case '{', '}':
			// FormSpecExpr has no dict/set literals or blocks.
			return "construct outside the expression subset (dict/set literals, blocks)"
		case ',':
			continue
		}

		// Operators. A two-character form is preferred, but only when it is a
		// real operator — otherwise the rune stands alone, so `!!x` reads as two
		// unary negations rather than an unknown `!!`.
		if exprOneCharOps[ch] || exprPairFirst[ch] {
			if i+1 < len(runes) {
				if pair := string(runes[i : i+2]); exprTwoCharOps[pair] {
					i++
					continue
				}
			}
			if !exprOneCharOps[ch] {
				return fmt.Sprintf("operator %q is not part of FormSpecExpr", string(ch))
			}
			continue
		}

		// Numbers: digits and '.' (the client lexer's readNumber rule).
		if ch >= '0' && ch <= '9' {
			for i+1 < len(runes) {
				next := runes[i+1]
				if (next >= '0' && next <= '9') || next == '.' {
					i++
					continue
				}
				break
			}
			continue
		}

		// Identifiers, keywords, and member access (`fields.name`).
		if isExprIdentStart(ch) {
			for i+1 < len(runes) && isExprIdentPart(runes[i+1]) {
				i++
			}
			continue
		}

		return fmt.Sprintf("character %q is not part of FormSpecExpr", string(ch))
	}

	if len(stack) > 0 {
		return "unbalanced delimiter"
	}

	// Callable-name gate (closed set). The character scan above cannot see the
	// difference between `len(fields.items)` (valid) and `user.has('x')`
	// (impossible): both are identifiers, dots and parens. The client evaluator
	// resolves a call as `node.callee.name` over an Identifier, so a member
	// callee (`user.has`, `session.x`) lands in its `default:` branch and the
	// expression dies at RUNTIME as a warning — exactly the false guarantee
	// 08-formspec-expr.md §4 forbids. Measured on the live example
	// (`examples/Clinic-UI-Showcase/.../clinic/module.yaml`):
	// `when: "user.has('clinic.settings.update')"` passed this gate and would
	// evaluate to "unknown function: undefined".
	//
	// So the callable identifiers are a closed set, mirroring the way
	// checkAggregates pins aggregateFns. Anything else is a deploy-time error.
	if bad := firstUnknownCallable(expr); bad != "" {
		return fmt.Sprintf("unknown function %q — FormSpecExpr callables are a closed set: %s",
			bad, strings.Join(exprCallables, ", "))
	}
	return ""
}

// exprCallables is the closed set of functions callable from a FormSpecExpr.
//
// Kept in step with the client evaluator's evalCall switch
// (renderers/react-shadcn/src/lib/formspec-expr/eval.ts). Deliberately NOT
// included: the server-side Starlark-only builtins (`sum_line`, `days_ago`,
// `empty`) — those belong to state-machine guards, a different contract
// (internal/starlark.EvaluateGuard), not to the client expression subset.
var exprCallables = []string{"len", "sum", "amount", "currency", "today"}

// firstUnknownCallable returns the first call identifier in expr that is not in
// exprCallables, or "" when every call is known. A member call (`user.has`) is
// reported as its full dotted path so the message says what was actually
// written.
//
// This is a lexical scan, not a parse: it looks for an identifier (optionally
// dotted) immediately followed by `(`, skipping string literals. That is enough
// to catch the class this gate exists for, and it cannot reject a valid
// expression — every valid callable is a bare identifier.
func firstUnknownCallable(expr string) string {
	known := make(map[string]bool, len(exprCallables))
	for _, c := range exprCallables {
		known[c] = true
	}

	runes := []rune(expr)
	for i := 0; i < len(runes); i++ {
		ch := runes[i]

		// String literals are opaque — a `(` inside one is content, not a call.
		// (The delimiter scan above already guarantees the literal is closed.)
		if ch == '"' || ch == '\'' {
			quote := ch
			for i++; i < len(runes); i++ {
				if runes[i] == '\\' {
					i++
					continue
				}
				if runes[i] == quote {
					break
				}
			}
			continue
		}

		if !isExprIdentStart(ch) {
			continue
		}

		// Read the identifier, plus any `.member` chain (`user.has`).
		start := i
		for i+1 < len(runes) && (isExprIdentPart(runes[i+1]) || runes[i+1] == '.') {
			i++
		}
		// `isExprIdentPart` includes '.', so a trailing dot would be swallowed
		// (`fields.` in `fields.(x)`); trim it back so the next loop iteration
		// sees it as a non-identifier and skips it.
		name := string(runes[start : i+1])
		name = strings.TrimRight(name, ".")
		if name == "" {
			continue
		}

		// A call is an identifier immediately followed by `(`, ignoring spaces.
		j := i + 1
		for j < len(runes) && (runes[j] == ' ' || runes[j] == '\t') {
			j++
		}
		if j >= len(runes) || runes[j] != '(' {
			continue
		}
		if !known[name] {
			return name
		}
	}
	return ""
}

// exprOneCharOps is the closed set of single-character operators. A lone `=` is
// deliberately absent: FormSpecExpr uses `==`, and the client lexer treats `=`
// as ILLEGAL.
var exprOneCharOps = map[rune]bool{
	'+': true, '-': true, '*': true, '/': true,
	'<': true, '>': true, '!': true,
}

// exprTwoCharOps is the closed set of two-character operators.
var exprTwoCharOps = map[string]bool{
	"==": true, "!=": true, "<=": true, ">=": true,
	"&&": true, "||": true,
}

// exprPairFirst is the set of characters that could begin a two-character
// operator (or a lone valid `&`/`|`, which FormSpecExpr does not have — hence
// their absence from exprOneCharOps).
var exprPairFirst = map[rune]bool{
	'=': true, '!': true, '<': true, '>': true,
	'&': true, '|': true,
}

func isExprIdentStart(ch rune) bool {
	return (ch >= 'a' && ch <= 'z') || (ch >= 'A' && ch <= 'Z') || ch == '_'
}

func isExprIdentPart(ch rune) bool {
	return isExprIdentStart(ch) || (ch >= '0' && ch <= '9') || ch == '.'
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

// aggregateFns is the closed set of aggregate functions. SUM/AVG/MIN/MAX are
// numeric; COUNT is defined over any field (and over no field at all).
var aggregateFns = map[string]bool{"sum": true, "avg": true, "count": true, "min": true, "max": true}

// checkAggregates verifies that every declared aggregate can mean something
// (S7 / gap #28):
//
//   - the function is one of sum | avg | count | min | max
//   - the target field exists on the report/widget entity
//   - a non-COUNT aggregate targets a numeric or money field
//
// Money is a first-class {amount, currency} value, so aggregating it is
// well-defined only because the engine reads its `.amount` component. A `sum`
// over a text column is a spec bug — and it used to surface as a confident 0 in
// reports, which is precisely the failure mode this gate removes.
func checkAggregates(result *checkResult, idx *entityIndex, manifests []manifest.RawManifest) {
	for _, m := range manifests {
		sm, ok := m.Spec.(map[string]any)
		if !ok {
			continue
		}

		switch m.Kind {
		case "Report":
			rs, err := manifest.RawSpecTo[spec.ReportSpec](sm)
			if err != nil {
				continue
			}
			entityRef := rs.Entity
			if rs.Source != nil && rs.Source.Entity != "" {
				entityRef = rs.Source.Entity
			}
			for _, col := range rs.Columns {
				if col.Aggregate == "" {
					continue
				}
				where := fmt.Sprintf("Report %q column %q", m.Metadata.Name, col.Field)
				checkAggregateDecl(result, idx, m, entityRef, where, string(col.Aggregate), col.Field)
			}
			for _, total := range rs.Totals {
				where := fmt.Sprintf("Report %q total %q", m.Metadata.Name, total.Label)
				checkAggregateDecl(result, idx, m, entityRef, where, total.Fn, total.Field)
			}

		case "Widget":
			ws, err := manifest.RawSpecTo[spec.WidgetSpec](sm)
			if err != nil {
				continue
			}
			fn, _ := ws.Config["aggregate"].(string)
			if fn == "" {
				continue
			}
			field, _ := ws.Config["field"].(string)
			where := fmt.Sprintf("Widget %q config", m.Metadata.Name)
			checkAggregateDecl(result, idx, m, ws.Entity, where, fn, field)
		}
	}
}

// checkAggregateDecl validates one aggregate declaration.
func checkAggregateDecl(
	result *checkResult,
	idx *entityIndex,
	m manifest.RawManifest,
	entityRef, where, fn, field string,
) {
	if entityRef == "" {
		// No entity to resolve against — a missing data source is another
		// check's business.
		return
	}
	if !aggregateFns[fn] {
		result.add(m.Source, "error",
			"%s: aggregate %q is not defined (want sum|avg|count|min|max)", where, fn)
		return
	}
	// count(field) is legal over any field, and count() over none at all.
	if fn == "count" && field == "" {
		return
	}
	if field == "" {
		result.add(m.Source, "error", "%s: aggregate %q needs a field", where, fn)
		return
	}

	module, entity := splitEntityRef(entityRef, m.Metadata.Module)
	if _, ok := idx.byKey[module+"."+entity]; !ok {
		result.add(m.Source, "error",
			"%s: aggregate %q references unknown entity %q", where, fn, entityRef)
		return
	}
	ft, ok := idx.fieldType(module, entity, field)
	if !ok {
		result.add(m.Source, "error",
			"%s: aggregate %q references unknown field %q on %s.%s", where, fn, field, module, entity)
		return
	}
	if fn == "count" {
		return
	}
	if spec.IsNumericField(ft) {
		return
	}
	result.add(m.Source, "error",
		"%s: %s(%s) is not defined — field %q has type %s, which is not numeric (money fields aggregate over their amount)",
		where, fn, field, field, ft)
}

// checkDatastores validates kind: Datastore manifests and module bindings
// (todo 2.9.4, platform/06-datastore.md §1.1/§2):
//   - driver×serves compatibility (§2 table)
//   - module `spec.datastore` binding points to an existing Datastore manifest
func checkDatastores(result *checkResult, manifests []manifest.RawManifest) {
	datastores := map[string]manifest.RawManifest{} // name → manifest
	for _, m := range manifests {
		if spec.Kind(m.Kind) != spec.KindDatastore {
			continue
		}
		sm, ok := m.Spec.(map[string]any)
		if !ok {
			continue
		}
		ds, err := manifest.RawSpecTo[spec.DatastoreSpec](sm)
		if err != nil {
			result.add(m.Source, "error", "datastore %q: %v", m.Metadata.Name, err)
			continue
		}
		if len(ds.Serves) == 0 {
			result.add(m.Source, "error", "datastore %q: spec.serves must list at least one primitive", m.Metadata.Name)
			continue
		}
		compatible := map[spec.PrimitiveType]bool{}
		for _, p := range ds.Driver.Serves() {
			compatible[p] = true
		}
		for _, p := range ds.Serves {
			if !compatible[p] {
				result.add(m.Source, "error", "datastore %q: driver %q cannot serve primitive %q (platform/06-datastore.md §2)", m.Metadata.Name, ds.Driver, p)
			}
		}
		datastores[m.Metadata.Name] = m
	}

	for _, m := range manifests {
		if spec.Kind(m.Kind) != spec.KindModule {
			continue
		}
		sm, ok := m.Spec.(map[string]any)
		if !ok {
			continue
		}
		ms, err := manifest.RawSpecToModuleSpec(sm)
		if err != nil || ms.Datastore == "" {
			continue
		}
		if _, ok := datastores[ms.Datastore]; !ok {
			result.add(m.Source, "error", "module %q binds datastore %q which has no kind: Datastore manifest", m.Metadata.Name, ms.Datastore)
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

// checkAppWorkspaces warns about Apps whose Workspaces allowlist mounts them
// nowhere (explicit `workspaces: []` — a staged App, plan
// docs_internal/plan/named-workspaces.md). This is legal (staging an App
// before binding it to workspaces), but easy to forget — so it surfaces as a
// warning, never an error.
func checkAppWorkspaces(result *checkResult, manifests []manifest.RawManifest) {
	for _, m := range manifests {
		if spec.Kind(m.Kind) != spec.KindApp || m.Spec == nil {
			continue
		}
		specMap, ok := m.Spec.(map[string]any)
		if !ok {
			continue
		}
		appSpec, err := manifest.RawSpecToAppSpec(specMap)
		if err != nil {
			continue
		}
		if appSpec.Workspaces != nil && len(*appSpec.Workspaces) == 0 {
			result.add(m.Source, "warning",
				"app %q is staged — explicit empty `workspaces: []` mounts it in no workspace (intentional? fill the allowlist or remove the field)",
				m.Metadata.Name)
		}
	}
}

// checkMenuExpr validates `MenuItem.When` on every App and Module manifest.
//
// Menu was the one FormSpecExpr site with NO deploy-time gate at all, even
// though 08-formspec-expr.md §4 makes that gate mandatory for every expression.
// The consequence was live, not theoretical: the clinic showcase shipped
// `when: "user.has('clinic.settings.update')"` — a member call the evaluator
// cannot resolve, and identity-based besides, which §3 forbids in FormSpecExpr.
// It validated green and hid nothing.
//
// There is no entity schema to check field references against (a menu condition
// is about the caller and the clock, not a record), so this checks the grammar
// and the closed callable set only — which is precisely what the broken example
// would have failed on.
func checkMenuExpr(result *checkResult, manifests []manifest.RawManifest) {
	// Declared before assignment so the closure can recurse into children.
	var check func(source, kind, label string, items []spec.MenuItem)
	check = func(source, kind, label string, items []spec.MenuItem) {
		for _, it := range items {
			if it.When != "" {
				if err := validateExprGrammar(it.When); err != "" {
					where := fmt.Sprintf("%s %q menu item %q when", kind, label, it.Label)
					result.add(source, "error", "%s: FormSpecExpr %q invalid: %s", where, it.When, err)
				}
			}
			if len(it.Children) > 0 {
				check(source, kind, label, it.Children)
			}
		}
	}

	for _, m := range manifests {
		if m.Spec == nil {
			continue
		}
		specMap, ok := m.Spec.(map[string]any)
		if !ok {
			continue
		}
		switch spec.Kind(m.Kind) {
		case spec.KindApp:
			appSpec, err := manifest.RawSpecToAppSpec(specMap)
			if err != nil {
				continue
			}
			check(m.Source, "App", m.Metadata.Name, appSpec.Menu)
		case spec.KindModule:
			modSpec, err := manifest.RawSpecToModuleSpec(specMap)
			if err != nil {
				continue
			}
			check(m.Source, "Module", m.Metadata.Name, modSpec.Menu)
		}
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
			_, _ = fmt.Fprintf(os.Stderr, "  ⚠️  cannot fix %s action %q: %v\n", k.file, k.action, err)
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

// kindIndex maps "{module}/{name}" → manifest source, per frontend kind. It is
// what makes "does this ref point at something?" answerable at check time.
type kindIndex map[string]map[string]string

func (k kindIndex) add(kind, module, name, source string) {
	if k[kind] == nil {
		k[kind] = map[string]string{}
	}
	k[kind][module+"/"+name] = source
}

func (k kindIndex) has(kind, module, name string) bool {
	_, ok := k[kind][module+"/"+name]
	return ok
}

// hasAnyModule reports whether ANY module declares this kind under this bare
// name.
//
// Why unqualified cross-module refs must resolve: the authoring convention is a
// bare name for a view, and internal/ui resolves it module-locally — but a
// plain name also occurs where the author means "the widget of that name, in
// whichever module has it". Measured on Clinic-UI-Showcase: the `clinic`
// module's dashboard places `pharmacy-queue-count`, declared by the `pharmacy`
// module, with a comment saying so. Treating a bare name as module-local alone
// reported that working dashboard as broken — a false positive, which is worse
// than not checking at all. Only a name NO module declares is an error.
func (k kindIndex) hasAnyModule(kind, name string) bool {
	for key := range k[kind] {
		if i := strings.LastIndex(key, "/"); i >= 0 && key[i+1:] == name {
			return true
		}
	}
	return false
}

// buildKindIndex indexes every ref-able frontend kind by "{module}/{name}".
func buildKindIndex(manifests []manifest.RawManifest) kindIndex {
	idx := kindIndex{}
	for _, m := range manifests {
		switch m.Kind {
		case "Form", "Table", "Page", "Report", "Print", "Widget", "Dashboard",
			"Wizard", "Kanban", "Timeline", "Calendar", "Listing",
			"NotificationCenter", "Component", "ApprovalInbox":
			idx.add(m.Kind, m.Metadata.Module, m.Metadata.Name, m.Source)
		}
	}
	return idx
}

// splitKindRef splits a frontend-kind reference into (module, name). Refs are
// module-local bare names in practice (`r.Forms[ref]` plus a module equality
// check in internal/ui), but the module-qualified spellings "m/name" and
// "m.name" are accepted so a valid cross-module ref is not reported as missing.
func splitKindRef(ref, ownModule string) (string, string) {
	if i := strings.LastIndex(ref, "/"); i > 0 {
		return ref[:i], ref[i+1:]
	}
	if i := strings.LastIndex(ref, "."); i > 0 {
		return ref[:i], ref[i+1:]
	}
	return ownModule, ref
}

// checkReferences verifies that every cross-file NAME a manifest points at
// actually exists (todo 10.12, raised by kafe 10.10).
//
// Why this is needed: a manifest referencing a view, entity or widget that does
// not exist is accepted by `validate` — every check it performs is per-manifest,
// so a name that was never declared anywhere is simply never compared to
// anything. The failure surfaces at runtime as a 404 route or a placeholder,
// long after authoring. Measured on kafe before the fix: `formspec validate`
// was green on a report whose `entity: ledger` never existed, a Table pointing
// at `journal_entry` (the manifest is `journal-entry`), and a dashboard whose
// `recent-journals` widget was never written at all.
//
// Scope is deliberately NAMES, not fields: entity fields are already covered by
// checkForms/checkKanban/checkWizard/checkAggregates, and dotted column paths
// like `customer.name` need relation traversal this check does not do.
func checkReferences(result *checkResult, idx *entityIndex, manifests []manifest.RawManifest) {
	kinds := buildKindIndex(manifests)

	// A `spec.entity` ("module.entity" or bare = the manifest's own module).
	checkEntityRef := func(m manifest.RawManifest, ref, where string) {
		if ref == "" {
			return // legitimate: dashboards, wizards, components carry no entity
		}
		module, entity := splitEntityRef(ref, m.Metadata.Module)
		if _, ok := idx.byKey[module+"."+entity]; !ok {
			result.add(m.Source, "error", "%s references unknown entity %q", where, ref)
		}
	}

	// A view reference to a declared manifest of `kind`. An empty ref or an
	// inline asset replaces the reference, so neither is a dangling name.
	checkKindRef := func(m manifest.RawManifest, kind, ref, asset, where string) {
		if ref == "" || asset != "" {
			return
		}
		module, name := splitKindRef(ref, m.Metadata.Module)
		if kinds.has(kind, module, name) {
			return
		}
		// Unqualified ref: accept any module that declares it (see
		// hasAnyModule). A qualified ref stays strict — the author named the
		// module, so a miss there is a real dangling reference.
		if ref == name && kinds.hasAnyModule(kind, name) {
			return
		}
		result.add(m.Source, "error", "%s references unknown %s %q",
			where, strings.ToLower(kind), ref)
	}

	checkBlocks := func(m manifest.RawManifest, owner string, blocks []spec.PageBlock, tabs []spec.PageTab) {
		for _, blk := range blocks {
			if blk.Form != nil {
				checkKindRef(m, "Form", blk.Form.Ref, blk.Form.Asset, owner+" block form")
			}
			if blk.Table != nil {
				checkKindRef(m, "Table", blk.Table.Ref, blk.Table.Asset, owner+" block table")
			}
			if blk.Component != nil {
				checkKindRef(m, "Component", blk.Component.Ref, blk.Component.Asset, owner+" block component")
			}
			if blk.Widget != nil {
				checkKindRef(m, "Widget", blk.Widget.Ref, blk.Widget.Asset, owner+" block widget")
			}
		}
		for _, tab := range tabs {
			where := owner + " tab " + strconv.Quote(tab.Label)
			if tab.Form != nil {
				checkKindRef(m, "Form", tab.Form.Ref, tab.Form.Asset, where+" form")
			}
			if tab.Table != nil {
				checkKindRef(m, "Table", tab.Table.Ref, tab.Table.Asset, where+" table")
			}
			if tab.Component != nil {
				checkKindRef(m, "Component", tab.Component.Ref, tab.Component.Asset, where+" component")
			}
		}
	}

	for _, m := range manifests {
		sm, ok := m.Spec.(map[string]any)
		if !ok {
			continue
		}
		name := m.Metadata.Name

		switch m.Kind {
		case "Table":
			if ts, err := manifest.RawSpecTo[spec.TableSpec](sm); err == nil {
				checkEntityRef(m, ts.Entity, fmt.Sprintf("Table %q", name))
			}
		case "Form":
			if fs, err := manifest.RawSpecTo[spec.FormSpec](sm); err == nil {
				// An auth form carries `auth_action` and has no entity by design.
				if fs.AuthAction == "" {
					checkEntityRef(m, fs.Entity, fmt.Sprintf("Form %q", name))
				}
			}
		case "Listing":
			if ls, err := manifest.RawSpecTo[spec.ListingSpec](sm); err == nil {
				checkEntityRef(m, ls.Entity, fmt.Sprintf("Listing %q", name))
			}
		case "Kanban":
			if ks, err := manifest.RawSpecTo[spec.KanbanSpec](sm); err == nil {
				checkEntityRef(m, ks.Entity, fmt.Sprintf("Kanban %q", name))
			}
		case "Timeline":
			if ts, err := manifest.RawSpecTo[spec.TimelineSpec](sm); err == nil {
				checkEntityRef(m, ts.Entity, fmt.Sprintf("Timeline %q", name))
			}
		case "Calendar":
			if cs, err := manifest.RawSpecTo[spec.CalendarSpec](sm); err == nil {
				checkEntityRef(m, cs.Entity, fmt.Sprintf("Calendar %q", name))
			}
		case "Print":
			if ps, err := manifest.RawSpecTo[spec.PrintSpec](sm); err == nil {
				checkEntityRef(m, ps.Entity, fmt.Sprintf("Print %q", name))
			}
		case "Report":
			if rs, err := manifest.RawSpecTo[spec.ReportSpec](sm); err == nil {
				ref := rs.Entity
				if rs.Source != nil && rs.Source.Entity != "" {
					ref = rs.Source.Entity
				}
				checkEntityRef(m, ref, fmt.Sprintf("Report %q", name))
			}
		case "Widget":
			if ws, err := manifest.RawSpecTo[spec.WidgetSpec](sm); err == nil {
				checkEntityRef(m, ws.Entity, fmt.Sprintf("Widget %q", name))
			}
		case "Wizard":
			if ws, err := manifest.RawSpecTo[spec.WizardSpec](sm); err == nil {
				checkEntityRef(m, ws.Entity, fmt.Sprintf("Wizard %q", name))
			}
		case "Page":
			if ps, err := manifest.RawSpecTo[spec.PageSpec](sm); err == nil {
				checkBlocks(m, fmt.Sprintf("Page %q", name), ps.Blocks, ps.Tabs)
			}
		case "Dashboard":
			if ds, err := manifest.RawSpecTo[spec.DashboardSpec](sm); err == nil {
				for _, w := range ds.Widgets {
					checkKindRef(m, "Widget", w.Ref, "", fmt.Sprintf("Dashboard %q widget", name))
				}
				// `defaults` names widgets too (the pre-`widgets` spelling) — a
				// stale entry there is the same dangling name.
				for _, d := range ds.Defaults {
					checkKindRef(m, "Widget", d, "", fmt.Sprintf("Dashboard %q default", name))
				}
			}
		}
	}
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
