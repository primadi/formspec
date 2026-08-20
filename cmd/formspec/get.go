// Command `formspec get` / `formspec describe` — read-only inspection of
// resources in a spec tree (docs/cli-tools/02-formspec-cli.md §3). The
// Control Plane is deferred, so these operate against local manifests
// (single-server dev mode) rather than a deployed registry:
//
//	formspec get document invoice          # summary: name, kind, version, module
//	formspec get entity invoice --output json
//	formspec describe entity invoice       # detail: fields, actions, state machine, expose
//
// `get` prints a compact summary (or full JSON with --output json);
// `describe` prints a detailed per-kind view.
package main

import (
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strings"

	"github.com/primadi/formspec/internal/manifest"
	"github.com/primadi/formspec/pkg/spec"
)

func runGet(args []string) {
	specPath := "spec"
	output := "table"
	var positional []string
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--spec", "-spec":
			if i+1 < len(args) {
				specPath = args[i+1]
				i++
			}
		case "--output", "-output":
			if i+1 < len(args) {
				output = args[i+1]
				i++
			}
		case "--help", "-h":
			fmt.Fprintf(os.Stderr, "Usage: formspec get <kind> [name] [--spec <path>] [--output table|json]\n")
			os.Exit(0)
		default:
			positional = append(positional, args[i])
		}
	}
	if len(positional) < 1 {
		fmt.Fprintf(os.Stderr, "Usage: formspec get <kind> [name] [--spec <path>] [--output table|json]\n")
		os.Exit(2)
	}
	kind := positional[0]
	name := ""
	if len(positional) >= 2 {
		name = positional[1]
	}

	manifests := loadManifestsOrExit(specPath)

	// Filter by kind (and name if given).
	var matches []manifest.RawManifest
	for _, m := range manifests {
		if !kindMatches(m.Kind, kind) {
			continue
		}
		if name != "" && m.Metadata.Name != name {
			continue
		}
		matches = append(matches, m)
	}

	if len(matches) == 0 {
		fmt.Fprintf(os.Stderr, "No resources found for kind %q", kind)
		if name != "" {
			fmt.Fprintf(os.Stderr, " name %q", name)
		}
		fmt.Fprintf(os.Stderr, " under %s\n", specPath)
		os.Exit(1)
	}

	if output == "json" {
		rows := make([]map[string]any, 0, len(matches))
		for _, m := range matches {
			rows = append(rows, map[string]any{
				"kind":    m.Kind,
				"name":    m.Metadata.Name,
				"module":  m.Metadata.Module,
				"version": specVersionOf(m),
				"source":  m.Source,
			})
		}
		b, _ := json.MarshalIndent(rows, "", "  ")
		fmt.Println(string(b))
		return
	}

	// Table output.
	fmt.Printf("%-12s %-24s %-16s %-8s %s\n", "KIND", "NAME", "MODULE", "VERSION", "SOURCE")
	for _, m := range matches {
		fmt.Printf("%-12s %-24s %-16s %-8s %s\n", m.Kind, m.Metadata.Name, m.Metadata.Module, specVersionOf(m), m.Source)
	}
}

func runDescribe(args []string) {
	specPath := "spec"
	var positional []string
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--spec", "-spec":
			if i+1 < len(args) {
				specPath = args[i+1]
				i++
			}
		case "--help", "-h":
			fmt.Fprintf(os.Stderr, "Usage: formspec describe <kind> <name> [--spec <path>]\n")
			os.Exit(0)
		default:
			positional = append(positional, args[i])
		}
	}
	if len(positional) < 2 {
		fmt.Fprintf(os.Stderr, "Usage: formspec describe <kind> <name> [--spec <path>]\n")
		os.Exit(2)
	}
	kind := positional[0]
	name := positional[1]

	manifests := loadManifestsOrExit(specPath)

	var match *manifest.RawManifest
	for i := range manifests {
		m := &manifests[i]
		if kindMatches(m.Kind, kind) && m.Metadata.Name == name {
			match = m
			break
		}
	}
	if match == nil {
		fmt.Fprintf(os.Stderr, "No resource found for kind %q name %q under %s\n", kind, name, specPath)
		os.Exit(1)
	}

	fmt.Printf("Name:        %s\n", match.Metadata.Name)
	fmt.Printf("Kind:        %s\n", match.Kind)
	fmt.Printf("Module:      %s\n", match.Metadata.Module)
	if match.Metadata.Description != "" {
		fmt.Printf("Description: %s\n", match.Metadata.Description)
	}
	fmt.Printf("Source:      %s\n", match.Source)

	switch match.Kind {
	case "Entity", "Document":
		describeEntity(match)
	default:
		// Generic: print the spec as indented YAML-ish JSON.
		sm, ok := match.Spec.(map[string]any)
		if !ok {
			return
		}
		b, _ := json.MarshalIndent(sm, "  ", "  ")
		fmt.Printf("\nSpec:\n  %s\n", strings.ReplaceAll(string(b), "\n", "\n  "))
	}
}

// describeEntity prints the detailed view for an Entity: fields, actions,
// state machine, and expose (permission) surfaces.
func describeEntity(m *manifest.RawManifest) {
	sm, ok := m.Spec.(map[string]any)
	if !ok {
		return
	}
	es, err := manifest.RawSpecToEntitySpec(sm)
	if err != nil {
		fmt.Fprintf(os.Stderr, "  (cannot parse entity spec: %v)\n", err)
		return
	}

	fmt.Printf("Version:       %s\n", es.Version)
	if es.Plural != "" {
		fmt.Printf("Plural:        %s\n", es.Plural)
	}
	fmt.Printf("Characteristic: %s\n", es.Characteristic)
	if es.Lifecycle != "" {
		fmt.Printf("Lifecycle:     %s\n", es.Lifecycle)
	}
	if es.DisplayField != "" {
		fmt.Printf("DisplayField:  %s\n", es.DisplayField)
	}

	// Fields.
	fmt.Printf("\nFields (%d):\n", len(es.Fields))
	fmt.Printf("  %-24s %-10s %-8s %s\n", "NAME", "TYPE", "REQUIRED", "DESCRIPTION")
	for _, f := range es.Fields {
		req := ""
		if fieldIsRequired(f) {
			req = "yes"
		}
		desc := strings.ReplaceAll(f.Description, "\n", " ")
		if len(desc) > 48 {
			desc = desc[:48] + "..."
		}
		fmt.Printf("  %-24s %-10s %-8s %s\n", f.Name, f.Type, req, desc)
	}

	// Actions.
	fmt.Printf("\nActions (%d):\n", len(es.Actions))
	for _, a := range es.Actions {
		perm := a.RequiredPermission
		if perm == "" {
			perm = "(default)"
		}
		impl := ""
		if a.Impl != nil {
			impl = string(a.Impl.Type)
			if a.Impl.Ref != "" {
				impl += ":" + a.Impl.Ref
			}
		}
		fmt.Printf("  %-20s perm=%-40s impl=%s\n", a.Name, perm, impl)
	}

	// State machine.
	if es.StateMachine != nil {
		sm := es.StateMachine
		fmt.Printf("\nState machine (field=%s, initial=%s):\n", sm.Field, sm.Initial)
		fmt.Printf("  States: %s\n", strings.Join(stateNames(sm.States), ", "))
		fmt.Printf("  Transitions:\n")
		for _, tr := range sm.Transitions {
			from := strings.Join(tr.From, ", ")
			if from == "" {
				from = "*"
			}
			via := ""
			if tr.Action != "" {
				via = fmt.Sprintf(" via %s", tr.Action)
			}
			fmt.Printf("    %s → %s%s\n", from, tr.To, via)
		}
	}

	// Expose (permission surfaces).
	fmt.Printf("\nExpose:\n")
	if len(es.Expose) == 0 {
		fmt.Printf("  (none — internal only)\n")
	}
	for _, e := range es.Expose {
		fmt.Printf("  %-6s actions=[%s]\n", e.Type, strings.Join(e.Actions, ", "))
	}
}

func stateNames(states []spec.StateDecl) []string {
	names := make([]string, len(states))
	for i, s := range states {
		names[i] = s.Name
	}
	return names
}

// specVersionOf extracts the spec.version from a raw manifest (Entity) or
// falls back to the apiVersion segment.
func specVersionOf(m manifest.RawManifest) string {
	if sm, ok := m.Spec.(map[string]any); ok {
		if v, ok := sm["version"].(string); ok && v != "" {
			return v
		}
	}
	return strings.TrimPrefix(m.APIVersion, "formspec.dev/")
}

// kindMatches reports whether a manifest kind matches the requested kind,
// treating "document" as an alias for "entity" (deprecated rename).
func kindMatches(actual, requested string) bool {
	if strings.EqualFold(actual, requested) {
		return true
	}
	if strings.EqualFold(requested, "document") && (actual == "Entity" || actual == "Document") {
		return true
	}
	return false
}

// loadManifestsOrExit loads all manifests under specPath, exiting on error.
func loadManifestsOrExit(specPath string) []manifest.RawManifest {
	loader := manifest.NewLoader(specPath)
	res, err := loader.LoadAll()
	if err != nil {
		fmt.Fprintf(os.Stderr, "formspec: load error: %v\n", err)
		os.Exit(2)
	}
	manifests := res.Manifests
	sort.SliceStable(manifests, func(i, j int) bool {
		if manifests[i].Kind != manifests[j].Kind {
			return manifests[i].Kind < manifests[j].Kind
		}
		return manifests[i].Metadata.Name < manifests[j].Metadata.Name
	})
	return manifests
}
