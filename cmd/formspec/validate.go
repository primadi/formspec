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
//	formspec validate [--spec <path>] [--schema <dir>] [--no-schema]
//
// The JSON Schema is auto-detected: a local `schemas/` dir next to the spec
// dir (<spec>/../schemas) is preferred, then `./schemas` (cwd), then the
// schema embedded into the binary. `--schema` overrides detection explicitly.
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
	"strings"

	"github.com/santhosh-tekuri/jsonschema/v6"

	formspecroot "github.com/primadi/formspec"
	"github.com/primadi/formspec/internal/manifest"
)

func runValidate(args []string) {
	fs := flag.NewFlagSet("validate", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	specPath := fs.String("spec", "spec", "path to the spec directory (default: spec)")
	schemaDir := fs.String("schema", "", "path to a schemas/ dir (default: auto-detect <spec>/../schemas, then ./schemas, then embedded schema)")
	noSchema := fs.Bool("no-schema", false, "skip JSON Schema validation (engine loader only)")
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

	// ── Layer 2: JSON Schema validation per kind ──
	var ksc *kindSchemaCompiler
	if !*noSchema {
		used := resolveSchemaDir(*specPath, *schemaDir)
		if used != "" {
			fmt.Printf("schema: %s (local)\n", used)
		} else {
			fmt.Printf("schema: embedded\n")
		}
		var err error
		ksc, err = newKindSchemaCompiler(used)
		if err != nil {
			fmt.Fprintf(os.Stderr, "formspec validate: schema: %v\n", err)
			os.Exit(2)
		}
	}

	// ── Report per manifest ──
	sorted := append([]manifest.RawManifest(nil), res.Manifests...)
	for _, m := range sorted {
		var msgs []string
		if errMsg, ok := engineRejects[m.Source]; ok {
			msgs = append(msgs, "engine: "+errMsg)
		}
		if ksc != nil {
			if sErr := validateSchema(ksc, m); sErr != "" {
				msgs = append(msgs, "schema: "+sErr)
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

// resolveSchemaDir decides which schema directory to use:
//  1. an explicit --schema dir (used as-is; broken dir → error downstream)
//  2. a `schemas/` folder next to the spec dir (<spec>/../schemas)
//  3. a `./schemas` folder in the current working directory
//  4. "" → fall back to the schema embedded into the binary
func resolveSchemaDir(specPath, explicit string) string {
	if explicit != "" {
		return explicit
	}
	candidates := []string{
		filepath.Join(filepath.Dir(specPath), "schemas"),
		filepath.Join(".", "schemas"),
	}
	for _, c := range candidates {
		if info, err := os.Stat(c); err == nil && info.IsDir() {
			return c
		}
	}
	return ""
}

// kindSchemaCompiler compiles and caches one document schema per kind.
// Each kind schema is the merge of schemas/kinds/<Kind>.schema.json (the spec
// body) with the shared $defs from formspec.schema.json, wrapped in the universal
// manifest envelope (apiVersion / kind / metadata / spec). Validating against
// the specific kind branch keeps error messages precise (the top-level oneOf
// would otherwise report every kind branch at once).
type kindSchemaCompiler struct {
	rootDefs map[string]any
	kindDir  string // "" = embedded
	cache    map[string]*jsonschema.Schema
	loaded   map[string][]byte
}

func newKindSchemaCompiler(schemaDir string) (*kindSchemaCompiler, error) {
	c := &kindSchemaCompiler{kindDir: schemaDir, cache: map[string]*jsonschema.Schema{}, loaded: map[string][]byte{}}
	var rootData []byte
	var err error
	if schemaDir != "" {
		rootData, err = os.ReadFile(schemaDir + "/formspec.schema.json")
	} else {
		rootData, err = formspecroot.SchemasFS.ReadFile("schemas/formspec.schema.json")
	}
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
	var data []byte
	var err error
	if c.kindDir != "" {
		data, err = os.ReadFile(c.kindDir + "/kinds/" + kind + ".schema.json")
	} else {
		data, err = formspecroot.SchemasFS.ReadFile("schemas/kinds/" + kind + ".schema.json")
	}
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
