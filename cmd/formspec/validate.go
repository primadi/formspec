// Command formspec validate — dry-run validation of a FormSpec spec tree.
//
// Two layers, both reported:
//  1. Engine loader (internal/manifest) — the ground truth of what
//     `formspec dev` / `formspec apply` would accept: YAML parse errors and
//     deep Entity validation (expose, lifecycle, relations, state machine,
//     transaction_date, reserved fields, ...).
//  2. JSON Schema (schemas/formspec.schema.json) — the contract for ALL kinds
//     (App, Module, Form, Workflow, Table, ...) that the engine loader does
//     not yet deep-validate. Catches stale syntax such as the `expose: all`
//     shorthand or a Workflow declared with states/transitions.
//
// Usage:
//
//	formspec validate [--spec <path>] [--schema <dir>] [--no-schema] [--schema-refresh]
//
// The schema version is read from each manifest's apiVersion (formspec.dev/v1)
// and fetched from the schema registry (default https://schemas.formspec.dev),
// then cached locally — a new spec version never requires a CLI reinstall.
// `--schema <dir>` overrides with a local schemas/ dir (no versioning).
// `--schema-refresh` re-fetches from the registry even if already cached.
// Exit code 1 if any manifest fails either layer.
//
// The schema layer is stricter than the engine for constructs whose Go type has
// a scalar/map UnmarshalYAML (e.g. `guard: "expr"` vs `guard: {expression}`,
// `render: drawer` vs `render: {mode: drawer}`) — the generator only expresses
// the object form. Use the object form to pass the schema layer.
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/santhosh-tekuri/jsonschema/v6"

	"github.com/primadi/formspec/internal/manifest"
	"github.com/primadi/formspec/internal/schemaregistry"
	"github.com/primadi/formspec/pkg/spec"
)

func runValidate(args []string) {
	fs := flag.NewFlagSet("validate", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	specPath := fs.String("spec", "spec", "path to the spec directory (default: spec)")
	schemaDir := fs.String("schema", "", "path to a local schemas/ dir (override; bypasses the registry)")
	noSchema := fs.Bool("no-schema", false, "skip JSON Schema validation (engine loader only)")
	refresh := fs.Bool("schema-refresh", false, "re-fetch schema version(s) from the registry even if cached")
	if err := fs.Parse(args); err != nil {
		os.Exit(2)
	}
	if fs.NArg() > 0 {
		fmt.Fprintf(os.Stderr, "formspec validate: unexpected argument %q\n", fs.Arg(0))
		os.Exit(2)
	}

	fails := 0
	loader := manifest.NewLoader(*specPath)

	// ── Layer 1: engine loader (discovery + parse + Entity deep validation) ──
	res, err := loader.LoadAll()
	if err != nil {
		fmt.Fprintf(os.Stderr, "formspec validate: load error: %v\n", err)
		os.Exit(2)
	}
	for _, pe := range res.Errors {
		fails++
		fmt.Printf("[PARSE ERROR] %v\n", pe)
	}

	engineRejects := map[string]string{}
	for _, m := range res.Manifests {
		if err := loader.Validate(m); err != nil {
			engineRejects[m.Source] = err.Error()
		}
	}

	// ── Layer 1.5: cross-manifest integrator validation (todo 7.7.2/7.7.3) ──
	// Every Integrator that makes a side effect from one event must provide a
	// symmetric cancel handler; the target action must be idempotent.
	integratorRejects := validateIntegrators(res.Manifests)

	// ── Layer 2: JSON Schema validation, version-routed ──
	//   * --schema <dir>: one local compiler for every manifest (no versioning).
	//   * default: the schema version comes from each manifest's apiVersion;
	//     fetched from the registry and cached locally when missing.
	var compilers map[string]*kindSchemaCompiler // key: version, or "*" for --schema override
	versionOf := map[string]string{}             // manifest.Source -> version key
	versionErrs := map[string]string{}           // manifest.Source -> apiVersion error
	if !*noSchema {
		reg := schemaregistry.New(schemaRegistryBaseURL())
		compilers = map[string]*kindSchemaCompiler{}
		kindsByVer := map[string]map[string]bool{}

		if *schemaDir != "" {
			c, err := newKindSchemaCompiler(*schemaDir)
			if err != nil {
				fmt.Fprintf(os.Stderr, "formspec validate: schema: %v\n", err)
				os.Exit(2)
			}
			compilers["*"] = c
			for _, m := range res.Manifests {
				versionOf[m.Source] = "*"
			}
			fmt.Printf("schema: %s (local override)\n", *schemaDir)
		} else {
			for _, m := range res.Manifests {
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
			versions := make([]string, 0, len(kindsByVer))
			for v := range kindsByVer {
				versions = append(versions, v)
			}
			sort.Strings(versions)
			for _, v := range versions {
				kinds := make([]string, 0, len(kindsByVer[v]))
				for k := range kindsByVer[v] {
					kinds = append(kinds, k)
				}
				sort.Strings(kinds)
				if err := reg.Ensure(v, kinds, *refresh); err != nil {
					fmt.Fprintf(os.Stderr, "formspec validate: schema %s: %v\n", v, err)
					os.Exit(2)
				}
				dir, err := reg.VersionDir(v)
				if err != nil {
					fmt.Fprintf(os.Stderr, "formspec validate: schema %s: %v\n", v, err)
					os.Exit(2)
				}
				c, err := newKindSchemaCompiler(dir)
				if err != nil {
					fmt.Fprintf(os.Stderr, "formspec validate: schema %s: %v\n", v, err)
					os.Exit(2)
				}
				compilers[v] = c
				fmt.Printf("schema: %s (registry %s, cache %s)\n", v, reg.BaseURL, dir)
			}
		}
	}

	// ── Report per manifest ──
	sorted := append([]manifest.RawManifest(nil), res.Manifests...)
	for _, m := range sorted {
		var msgs []string
		if errMsg, ok := engineRejects[m.Source]; ok {
			msgs = append(msgs, "engine: "+errMsg)
		}
		if errMsg, ok := integratorRejects[m.Source]; ok {
			msgs = append(msgs, "integrator: "+errMsg)
		}
		if compilers != nil {
			if verErr, ok := versionErrs[m.Source]; ok {
				msgs = append(msgs, "schema: "+verErr)
			} else if c, ok := compilers[versionOf[m.Source]]; ok {
				if sErr := validateSchema(c, m); sErr != "" {
					msgs = append(msgs, "schema: "+sErr)
				}
			}
		}
		if len(msgs) > 0 {
			fails++
			fmt.Printf("[FAIL] %s\n", m.Source)
			for _, msg := range msgs {
				fmt.Printf("       %s\n", msg)
			}
		} else {
			fmt.Printf("[OK]   %s\n", m.Source)
		}
	}

	fmt.Printf("\n%d manifest(s) validated, %d problem(s) found\n", len(sorted), fails)
	if fails > 0 {
		os.Exit(1)
	}
}

// validateIntegrators enforces the cross-manifest Integrator rules
// (02-core-extended.md §5, todo 7.7.2/7.7.3):
//
//   - 7.7.2: every Integrator that makes a side effect from one event must
//     provide a symmetric handler for the cancel event of the same resource —
//     otherwise cancel on the source side would be permanently blocked.
//   - 7.7.3: the target action (call.resource + call.action) must be
//     `idempotent: true` for cross-boundary calls.
//
// Returns a map of manifest source → error message.
func validateIntegrators(manifests []manifest.RawManifest) map[string]string {
	rejects := map[string]string{}

	// Collect all integrators by source.
	type itInfo struct {
		source string
		spec   *spec.IntegratorSpec
	}
	var its []itInfo
	for _, m := range manifests {
		if spec.Kind(m.Kind) != spec.KindIntegrator || m.Spec == nil {
			continue
		}
		specMap, ok := m.Spec.(map[string]any)
		if !ok {
			continue
		}
		it, err := manifest.RawSpecToIntegratorSpec(specMap)
		if err != nil {
			rejects[m.Source] = "invalid integrator spec: " + err.Error()
			continue
		}
		its = append(its, itInfo{source: m.Source, spec: it})
	}
	if len(its) == 0 {
		return rejects
	}

	// 7.7.2 — symmetric cancel handler. For each integrator listening to a
	// non-cancel event, there must be another integrator listening to the
	// cancel event of the same resource.
	cancelEvents := map[string]bool{"on_cancel": true, "before_cancel": true}
	hasCancel := map[string]bool{} // "resource" -> has a cancel-listening integrator
	for _, it := range its {
		if it.spec.Listen == nil {
			continue
		}
		if cancelEvents[it.spec.Listen.Event] {
			hasCancel[it.spec.Listen.Resource] = true
		}
	}
	for _, it := range its {
		if it.spec.Listen == nil {
			continue
		}
		if cancelEvents[it.spec.Listen.Event] {
			continue
		}
		if !hasCancel[it.spec.Listen.Resource] {
			rejects[it.source] = fmt.Sprintf(
				"integrator listens to %s.%s but has no symmetric cancel handler for %s (on_cancel/before_cancel) — cancel on the source would be permanently blocked (7.7.2)",
				it.spec.Listen.Resource, it.spec.Listen.Event, it.spec.Listen.Resource)
		}
	}

	// 7.7.3 — target action must be idempotent. Resolve the target entity's
	// action from the manifest set.
	entityActions := map[string]map[string]bool{} // "module.entity" -> action -> idempotent
	for _, m := range manifests {
		if spec.Kind(m.Kind) != spec.KindEntity || m.Spec == nil {
			continue
		}
		specMap, ok := m.Spec.(map[string]any)
		if !ok {
			continue
		}
		es, err := manifest.RawSpecToEntitySpec(specMap)
		if err != nil {
			continue
		}
		key := m.Metadata.Module + "." + m.Metadata.Name
		actions := map[string]bool{}
		for _, a := range es.Actions {
			actions[a.Name] = a.Idempotent
		}
		entityActions[key] = actions
	}
	for _, it := range its {
		if it.spec.Call == nil {
			continue
		}
		actions, ok := entityActions[it.spec.Call.Resource]
		if !ok {
			// Target entity not in this manifest set — can't verify; skip.
			continue
		}
		idempotent, ok := actions[it.spec.Call.Action]
		if !ok {
			rejects[it.source] = fmt.Sprintf(
				"integrator target action %s.%s not found in entity (7.7.3)",
				it.spec.Call.Resource, it.spec.Call.Action)
			continue
		}
		if !idempotent {
			rejects[it.source] = fmt.Sprintf(
				"integrator target action %s.%s must be idempotent: true for cross-boundary calls (7.7.3)",
				it.spec.Call.Resource, it.spec.Call.Action)
		}
	}

	return rejects
}

// kindSchemaCompiler compiles and caches one document schema per kind.
// Each kind schema is the merge of schemas/kinds/<Kind>.schema.json (the spec
// body) with the shared $defs from formspec.schema.json, wrapped in the universal
// manifest envelope (apiVersion / kind / metadata / spec). Validating against
// the specific kind branch keeps error messages precise (the top-level oneOf
// would otherwise report every kind branch at once).
type kindSchemaCompiler struct {
	rootDefs map[string]any
	kindDir  string
	cache    map[string]*jsonschema.Schema
	loaded   map[string][]byte
}

func newKindSchemaCompiler(schemaDir string) (*kindSchemaCompiler, error) {
	c := &kindSchemaCompiler{kindDir: schemaDir, cache: map[string]*jsonschema.Schema{}, loaded: map[string][]byte{}}
	rootData, err := os.ReadFile(filepath.Join(schemaDir, "formspec.schema.json"))
	if err != nil {
		return nil, fmt.Errorf("read formspec.schema.json: %w", err)
	}
	var root map[string]any
	if err := json.Unmarshal(rootData, &root); err != nil {
		return nil, fmt.Errorf("decode formspec.schema.json: %w", err)
	}
	defs, _ := root["$defs"].(map[string]any)
	if defs == nil {
		return nil, fmt.Errorf("formspec.schema.json: missing $defs")
	}
	c.rootDefs = defs
	return c, nil
}

// schemaFor returns the compiled document schema for the given kind.
func (c *kindSchemaCompiler) schemaFor(kind string) (*jsonschema.Schema, error) {
	if s, ok := c.cache[kind]; ok {
		return s, nil
	}
	data, err := c.readKindSchema(kind)
	if err != nil {
		return nil, err
	}
	var kindSpec map[string]any
	if err := json.Unmarshal(data, &kindSpec); err != nil {
		return nil, fmt.Errorf("decode %s.schema.json: %w", kind, err)
	}

	merged := map[string]any{
		"$schema": "http://json-schema.org/draft-07/schema#",
		"$defs":   c.rootDefs,
		"type":    "object",
		"properties": map[string]any{
			"apiVersion": map[string]any{"type": "string"},
			"kind":       map[string]any{"const": kind},
			"metadata":   map[string]any{"$ref": "#/$defs/Metadata"},
			"spec":       kindSpec,
		},
		"required":             []string{"apiVersion", "kind", "metadata", "spec"},
		"additionalProperties": false,
	}

	// Round-trip through JSON so AddResource sees plain JSON types
	// ([]any / map[string]any), not Go []string etc.
	mergedJSON, err := json.Marshal(merged)
	if err != nil {
		return nil, fmt.Errorf("encode %s schema: %w", kind, err)
	}
	var mergedDoc any
	if err := json.Unmarshal(mergedJSON, &mergedDoc); err != nil {
		return nil, fmt.Errorf("decode %s schema: %w", kind, err)
	}

	compiler := jsonschema.NewCompiler()
	if err := compiler.AddResource("kind-schema.json", mergedDoc); err != nil {
		return nil, fmt.Errorf("compile %s schema: %w", kind, err)
	}
	sch, err := compiler.Compile("kind-schema.json")
	if err != nil {
		return nil, fmt.Errorf("compile %s schema: %w", kind, err)
	}
	c.cache[kind] = sch
	return sch, nil
}

func (c *kindSchemaCompiler) readKindSchema(kind string) ([]byte, error) {
	if data, ok := c.loaded[kind]; ok {
		return data, nil
	}
	data, err := os.ReadFile(filepath.Join(c.kindDir, "kinds", kind+".schema.json"))
	if err != nil {
		return nil, fmt.Errorf("read %s.schema.json: %w", kind, err)
	}
	c.loaded[kind] = data
	return data, nil
}

// validateSchema validates one raw manifest against its kind's schema.
// Returns a human-readable error string, or "" if valid.
func validateSchema(ksc *kindSchemaCompiler, m manifest.RawManifest) string {
	sch, err := ksc.schemaFor(m.Kind)
	if err != nil {
		return fmt.Sprintf("internal: %v", err)
	}

	meta := map[string]any{
		"name":        m.Metadata.Name,
		"module":      m.Metadata.Module,
		"description": m.Metadata.Description,
	}
	if len(m.Metadata.Labels) > 0 {
		meta["labels"] = m.Metadata.Labels
	}
	if len(m.Metadata.Annotations) > 0 {
		meta["annotations"] = m.Metadata.Annotations
	}
	doc := map[string]any{
		"apiVersion": m.APIVersion,
		"kind":       m.Kind,
		"metadata":   meta,
		"spec":       m.Spec,
	}

	// Normalize through JSON so the validator sees plain JSON semantics.
	var v any
	b, err := json.Marshal(doc)
	if err != nil {
		return fmt.Sprintf("internal: %v", err)
	}
	if err := json.Unmarshal(b, &v); err != nil {
		return fmt.Sprintf("internal: %v", err)
	}

	if err := sch.Validate(v); err == nil {
		return ""
	} else if ve, ok := err.(*jsonschema.ValidationError); ok {
		return formatValidationError(ve)
	}
	return fmt.Sprintf("validation: %v", err)
}

func formatValidationError(ve *jsonschema.ValidationError) string {
	var sb strings.Builder
	var walk func(u *jsonschema.OutputUnit)
	walk = func(u *jsonschema.OutputUnit) {
		for i := range u.Errors {
			e := &u.Errors[i]
			if len(e.Errors) > 0 {
				walk(e)
				continue
			}
			loc := e.InstanceLocation
			if loc == "" {
				loc = "<root>"
			}
			msg := "<error>"
			if e.Error != nil {
				msg = e.Error.String()
			}
			sb.WriteString(fmt.Sprintf("%s: %s", loc, msg))
			sb.WriteString("; ")
		}
	}
	walk(ve.BasicOutput())
	return strings.TrimSuffix(sb.String(), "; ")
}
