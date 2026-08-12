// Package genkinddocs generates human-readable per-kind reference docs
// (docs/kind/) from the same pkg/spec Go types that genjsonschema uses.
//
// Design goals:
//
//   - Zero drift: attribute tables are rendered from Go structs + godoc +
//     // @schema annotations — the same source as schemas/kinds/*.schema.json.
//   - Narrative stays hand-authored: each docs/kind/<group>/<Kind>.md file is
//     the final artifact. Generated regions are delimited by marker comments
//     (<!-- generated:... --> / <!-- /generated:... -->). Only the content
//     between a marker pair is replaced on regenerate; everything else
//     (Kapan Memakai, Contoh Manifest, Gotchas) is preserved verbatim.
//
// Usage (from the formspec repo):
//
//	converter := genjsonschema.New("github.com/primadi/formspec/pkg/spec")
//	collect, _ := converter.Collect()
//	genkinddocs.Generate(collect, "docs/kind")
package genkinddocs

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/primadi/formspec/internal/genjsonschema"
)

// Marker pairs delimiting generated regions inside a kind doc file.
// Content between a marker pair is regenerated on every run; everything
// outside is author-maintained narrative and is left untouched.
const (
	metaStart = "<!-- generated:meta -->"
	metaEnd   = "<!-- /generated:meta -->"
	attrStart = "<!-- generated:attributes -->"
	attrEnd   = "<!-- /generated:attributes -->"
)

// GroupInfo describes which docs/kind sub-folder a kind lives in and which
// plane governs it. Mirrors docs/spec/platform/03-kind-system.md §4.
type GroupInfo struct {
	Group string // curation | data | ui | infra
	Plane string // resource | control
}

var kindGroups = map[string]GroupInfo{
	// Curation — workspace structure
	"App":    {Group: "curation", Plane: "resource"},
	"Module": {Group: "curation", Plane: "resource"},
	// Data — domain model & behavior
	"Entity":         {Group: "data", Plane: "resource"},
	"Service":        {Group: "data", Plane: "resource"},
	"Config":         {Group: "data", Plane: "resource"},
	"Migration":      {Group: "data", Plane: "resource"},
	"Subscription":   {Group: "data", Plane: "resource"},
	"Workflow":       {Group: "data", Plane: "resource"},
	"Api":            {Group: "data", Plane: "resource"},
	"Webhook":        {Group: "data", Plane: "resource"},
	"Mockup":         {Group: "data", Plane: "resource"},
	"Integrator":     {Group: "data", Plane: "resource"},
	"KindDefinition": {Group: "data", Plane: "resource"},
	// UI — visual presentation (tier: page / component)
	"Page":               {Group: "ui", Plane: "resource"},
	"Form":               {Group: "ui", Plane: "resource"},
	"Table":              {Group: "ui", Plane: "resource"},
	"Dashboard":          {Group: "ui", Plane: "resource"},
	"Widget":             {Group: "ui", Plane: "resource"},
	"Report":             {Group: "ui", Plane: "resource"},
	"Wizard":             {Group: "ui", Plane: "resource"},
	"Kanban":             {Group: "ui", Plane: "resource"},
	"Timeline":           {Group: "ui", Plane: "resource"},
	"Calendar":           {Group: "ui", Plane: "resource"},
	"Listing":            {Group: "ui", Plane: "resource"},
	"ApprovalInbox":      {Group: "ui", Plane: "resource"},
	"NotificationCenter": {Group: "ui", Plane: "resource"},
	"Print":              {Group: "ui", Plane: "resource"},
	"Theme":              {Group: "ui", Plane: "resource"},
	// Infra — renderer, storage, control plane
	"Renderer":       {Group: "infra", Plane: "resource"},
	"PersistBackend": {Group: "infra", Plane: "resource"},
	"Environment":    {Group: "infra", Plane: "control"},
	"Policy":         {Group: "infra", Plane: "control"},
	"Datastore":      {Group: "infra", Plane: "control"},
}

// GroupOf returns the docs group + plane for a kind, and whether it has a
// docs/kind page. Meta-kinds without a page (e.g. VisualSpecKind) return ok=false.
func GroupOf(kind string) (GroupInfo, bool) {
	g, ok := kindGroups[kind]
	return g, ok
}

// Generate writes (or updates in place) one Markdown file per kind at
// outDir/<group>/<Kind>.md. Existing files keep their narrative sections;
// only the generated regions are refreshed. Kinds without a docs/kind page
// (per GroupOf) are skipped.
func Generate(collect *genjsonschema.CollectResult, outDir string) (int, error) {
	written := 0
	for _, entry := range genjsonschema.KindMapping() {
		group, ok := kindGroups[entry.Kind]
		if !ok {
			continue // meta-kind without a docs/kind page
		}
		td := collect.Structs[entry.SpecStruct]
		if td == nil {
			fmt.Printf("   ⚠️  spec struct %s for kind %s not found\n", entry.SpecStruct, entry.Kind)
			continue
		}

		dir := filepath.Join(outDir, group.Group)
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return written, fmt.Errorf("create output dir %s: %w", dir, err)
		}

		path := filepath.Join(dir, entry.Kind+".md")
		existing := ""
		if data, err := os.ReadFile(path); err == nil {
			existing = string(data)
		}

		meta := renderMeta(entry, group)
		attrs := renderAttributes(td, collect)

		var doc string
		if existing == "" {
			doc = renderFresh(entry, meta, attrs)
		} else {
			doc = merge(entry, existing, meta, attrs)
		}

		if err := os.WriteFile(path, []byte(doc), 0o644); err != nil {
			return written, fmt.Errorf("write %s: %w", path, err)
		}
		fmt.Printf("   ✅ %s\n", path)
		written++
	}
	return written, nil
}

// merge refreshes the generated regions of an existing doc while preserving
// all narrative outside the markers. If a required marker is missing, the
// whole file is regenerated (with a warning) since it is no longer valid
// template+output.
func merge(entry genjsonschema.KindEntry, existing, meta, attrs string) string {
	out := replaceRegion(existing, metaStart, metaEnd, meta)
	if out == "" {
		fmt.Printf("   ⚠️  missing meta marker, regenerating fresh\n")
		return renderFresh(entry, meta, attrs)
	}
	out = replaceRegion(out, attrStart, attrEnd, attrs)
	if out == "" {
		fmt.Printf("   ⚠️  missing attributes marker, regenerating fresh\n")
		return renderFresh(entry, meta, attrs)
	}
	return out
}

// replaceRegion swaps the content between start and end markers with new
// content. Returns "" if either marker is absent.
func replaceRegion(doc, start, end, content string) string {
	si := strings.Index(doc, start)
	ei := strings.Index(doc, end)
	if si < 0 || ei < 0 {
		return ""
	}
	ei += len(end)
	return doc[:si] + start + "\n" + content + "\n" + end + doc[ei:]
}

// renderFresh builds a brand-new doc with generated regions populated and
// narrative sections left as TODO placeholders for the author to fill.
func renderFresh(entry genjsonschema.KindEntry, meta, attrs string) string {
	var b strings.Builder
	b.WriteString("# " + entry.Kind + "\n\n")
	b.WriteString("<!-- generated:meta -->\n")
	b.WriteString(meta)
	b.WriteString("<!-- /generated:meta -->\n\n")
	b.WriteString("## Kapan Memakai\n\n")
	b.WriteString("_TODO: tulis kapan kind ini dipakai, kapan TIDAK dipakai, dan pola desain yang relevan._\n\n")
	b.WriteString("## Contoh Manifest\n\n")
	b.WriteString("```yaml\n")
	b.WriteString("# TODO: tulis contoh YAML valid untuk kind " + entry.Kind + "\n")
	b.WriteString("```\n\n")
	b.WriteString("## Atribut\n\n")
	b.WriteString("<!-- generated:attributes -->\n")
	b.WriteString(attrs)
	b.WriteString("<!-- /generated:attributes -->\n\n")
	b.WriteString("## Gotchas\n\n")
	b.WriteString("_TODO: tulis gotchas, pitfalls, dan cross-reference ke `docs/spec/` + `ai_skills/formspec-kinds`._\n")
	return b.String()
}

// renderMeta builds the generated profile block (group, plane, spec struct).
func renderMeta(entry genjsonschema.KindEntry, group GroupInfo) string {
	var b strings.Builder
	b.WriteString("| | |\n|---|---|\n")
	b.WriteString(fmt.Sprintf("| Grup | `%s` |\n", group.Group))
	b.WriteString(fmt.Sprintf("| Plane | `%s` |\n", group.Plane))
	b.WriteString(fmt.Sprintf("| Spec struct | `%s` |\n", entry.SpecStruct))
	if len(entry.Aliases) > 0 {
		b.WriteString(fmt.Sprintf("| Alias | `%s` |\n", strings.Join(entry.Aliases, "`, `")))
	}
	if entry.Deprecated {
		b.WriteString("| Deprecated | ✅ |\n")
	}
	return b.String()
}

// renderAttributes builds the generated attribute table for a spec struct.
func renderAttributes(td *genjsonschema.TypeDef, collect *genjsonschema.CollectResult) string {
	var b strings.Builder
	b.WriteString("| Atribut | Tipe | Wajib | Contoh | Deskripsi |\n")
	b.WriteString("|---|---|---|---|---|\n")

	for _, fd := range td.Fields {
		if fd.Inline {
			// Inline fields are merged into the parent object; represented as
			// a nested block instead of a flat row.
			continue
		}
		req := "—"
		// Slice fields are never required — FormSpec derives empty arrays.
		if fd.Required && fd.TypeKind != "slice" {
			req = "✅"
		}
		ex, desc := fieldText(fd)
		b.WriteString(fmt.Sprintf("| `%s` | %s | %s | %s | %s |\n",
			fd.Name, fieldType(fd, collect), req, ex, desc))
	}
	return b.String()
}

// fieldText extracts the example + description for a field from its
// @schema annotation, falling back to the first line of the godoc comment
// (with the @schema annotation itself stripped out).
func fieldText(fd genjsonschema.FieldDef) (example, description string) {
	if fd.Annotation != nil {
		example = fd.Annotation.Example
		description = fd.Annotation.Description
	}
	if description == "" && fd.Comment != "" {
		clean := stripSchemaAnnotation(fd.Comment)
		clean = strings.TrimSpace(clean)
		if clean != "" {
			lines := strings.SplitN(clean, "\n", 2)
			description = strings.TrimSpace(lines[0])
		}
	}
	example = strings.ReplaceAll(example, "|", "\\|")
	description = strings.ReplaceAll(description, "|", "\\|")
	description = strings.ReplaceAll(description, "\n", " ")
	return example, description
}

// stripSchemaAnnotation removes any comment line containing the @schema
// annotation so it is not mistaken for a human-readable description. Line-based
// (not regex-based) so annotation values with nested braces never leak through.
func stripSchemaAnnotation(comment string) string {
	lines := strings.Split(comment, "\n")
	out := make([]string, 0, len(lines))
	for _, l := range lines {
		if strings.Contains(l, "@schema") {
			continue
		}
		out = append(out, l)
	}
	return strings.Join(out, "\n")
}

// fieldType renders a human-readable type for a field, expanding named
// structs to `Name`, enum types to their value list, and plain-scalar fields
// constrained by an @schema {enum} annotation to "enum (a · b · c)".
func fieldType(fd genjsonschema.FieldDef, collect *genjsonschema.CollectResult) string {
	switch fd.TypeKind {
	case "slice":
		elem := strings.TrimPrefix(fd.TypeName, "[]")
		return "[]" + namedType(elem, collect)
	case "map":
		if fd.MapValueType != nil {
			return "map[string]" + namedType(*fd.MapValueType, collect)
		}
		return "map"
	case "interface":
		return "any"
	default:
		// Plain scalar types. If the field carries an @schema {enum} annotation
		// even though its Go type is just `string` (e.g. EntitySpec.lifecycle,
		// FormSpec.mode, WebhookSpec.method), surface the allowed values as an
		// enum instead of a bare type. Fields whose Go type is already a named
		// enum (e.g. Characteristic) are handled by namedType below.
		if fd.Annotation != nil && len(fd.Annotation.Enum) > 0 {
			if td, ok := collect.AllDefs[fd.TypeName]; !ok || td.Kind != "enum" {
				vals := fd.Annotation.Enum
				if len(vals) > 8 {
					vals = append(vals[:8], "…")
				}
				return "enum (" + strings.Join(vals, " · ") + ")"
			}
		}
		return namedType(fd.TypeName, collect)
	}
}

// namedType renders a type name, inlining enum values when the name refers
// to a known string enum. Rendering is uniform with annotation-based enums
// (see fieldType): the Go type name is omitted — it never appears in YAML,
// so it would only add noise for manifest authors.
func namedType(name string, collect *genjsonschema.CollectResult) string {
	if name == "" {
		return "—"
	}
	if td, ok := collect.AllDefs[name]; ok && td.Kind == "enum" && len(td.EnumValues) > 0 {
		vals := td.EnumValues
		if len(vals) > 8 {
			vals = append(vals[:8], "…")
		}
		return "enum (" + strings.Join(vals, " · ") + ")"
	}
	return "`" + name + "`"
}
