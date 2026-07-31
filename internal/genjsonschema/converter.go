// Package genjsonschema reads Go struct types from pkg/spec and generates
// JSON Schema (Draft-07) files for every Forma resource kind.
//
// Usage:
//
//	converter := genjsonschema.New("github.com/primadi/forma/pkg/spec")
//	defs := converter.Collect()        // first pass: gather all type info
//	schemas := converter.Generate(defs) // second pass: produce JSON Schema
package genjsonschema

import (
	"encoding/json"
	"fmt"
	"go/ast"
	"go/token"
	"go/types"
	"regexp"
	"strings"

	"golang.org/x/tools/go/packages"
)

// Schema represents a JSON Schema document (Draft-07 subset).
type Schema struct {
	ID                   string             `json:"$id,omitempty"`
	Schema               string             `json:"$schema,omitempty"`
	Ref                  string             `json:"$ref,omitempty"`
	Defs                 map[string]*Schema `json:"$defs,omitempty"`
	Definitions          map[string]*Schema `json:"definitions,omitempty"`
	Title                string             `json:"title,omitempty"`
	Description          string             `json:"description,omitempty"`
	Type                 string             `json:"type,omitempty"`
	Properties           map[string]*Schema `json:"properties,omitempty"`
	Required             []string           `json:"required,omitempty"`
	Items                *Schema            `json:"items,omitempty"`
	AdditionalProperties any                `json:"additionalProperties,omitempty"`
	Enum                 []any              `json:"enum,omitempty"`
	OneOf                []*Schema          `json:"oneOf,omitempty"`
	AnyOf                []*Schema          `json:"anyOf,omitempty"`
	If                   *Schema            `json:"if,omitempty"`
	Then                 *Schema            `json:"then,omitempty"`
	Const                string             `json:"const,omitempty"`
	Default              any                `json:"default,omitempty"`
	Examples             []any              `json:"examples,omitempty"`
	MinLength            *int               `json:"minLength,omitempty"`
	MaxLength            *int               `json:"maxLength,omitempty"`
	Minimum              *float64           `json:"minimum,omitempty"`
	Maximum              *float64           `json:"maximum,omitempty"`
	Pattern              string             `json:"pattern,omitempty"`
	Nullable             bool               `json:"nullable,omitempty"`
	Deprecated           bool               `json:"deprecated,omitempty"`
	MarkdownDescription  string             `json:"markdownDescription,omitempty"`
	MinProperties        *int               `json:"minProperties,omitempty"`
	MaxProperties        *int               `json:"maxProperties,omitempty"`
}

// schemaAnnotation holds extra schema metadata parsed from // @schema comments.
type schemaAnnotation struct {
	Description         string   `json:"description,omitempty"`
	MarkdownDescription string   `json:"markdownDescription,omitempty"`
	Example             string   `json:"example,omitempty"`
	Deprecated          bool     `json:"deprecated,omitempty"`
	MinLength           *int     `json:"minLength,omitempty"`
	MaxLength           *int     `json:"maxLength,omitempty"`
	Pattern             string   `json:"pattern,omitempty"`
	Minimum             *float64 `json:"minimum,omitempty"`
	Maximum             *float64 `json:"maximum,omitempty"`
	Enum                []string `json:"enum,omitempty"`
	Title               string   `json:"title,omitempty"`
}

// TypeDef holds collected information about a Go type.
type TypeDef struct {
	Name        string
	Kind        string     // "struct", "named", "alias", "enum"
	Fields      []FieldDef // for structs
	EnumValues  []string   // for enum types
	Underlying  string     // underlying type name for aliases
	Comment     string     // godoc comment
	Annotation  *schemaAnnotation
	Type        types.Type // original Go type
	PackagePath string
}

// FieldDef holds info about a struct field.
type FieldDef struct {
	Name       string // JSON/YAML field name
	GoName     string // Go field name
	TypeName   string // resolved type name
	TypeKind   string // "basic", "struct", "slice", "map", "pointer", "named", "interface"
	Required   bool   // no omitempty
	Inline     bool   // yaml:",inline"
	OmitEmpty  bool   // yaml:",omitempty"
	Tag        string // full yaml tag
	Comment    string
	Annotation *schemaAnnotation
	GoType     types.Type
	NamedType  *string // for named types, the name

	// MapValueType holds the named value type of a map field (when the value is
	// a struct). Used to emit an additionalProperties $ref for maps of structs
	// (e.g. ConfigSpec.Keys → ConfigKey) instead of collapsing to {}.
	MapValueType *string
}

// KindMap maps Kind constants to their spec struct names.
type KindEntry struct {
	Kind       string // YAML kind value
	SpecStruct string // Go struct name (e.g. "EntitySpec")
	Deprecated bool   // true if deprecated (e.g. Entity → Document)
	Aliases    []string
}

// Converter loads and converts Go types to JSON Schema.
type Converter struct {
	PkgPath string
}

// New creates a Converter for the given package path.
func New(pkgPath string) *Converter {
	return &Converter{PkgPath: pkgPath}
}

// CollectResult holds all collected type definitions.
type CollectResult struct {
	Structs map[string]*TypeDef
	Enums   map[string]*TypeDef // string-named types with const values
	AllDefs map[string]*TypeDef // all named types
}

// Collect loads the package and gathers all type information.
func (c *Converter) Collect() (*CollectResult, error) {
	cfg := &packages.Config{
		Mode: packages.NeedTypes | packages.NeedTypesInfo | packages.NeedSyntax | packages.NeedDeps |
			packages.NeedCompiledGoFiles | packages.NeedName,
		Dir: "",
	}
	pkgs, err := packages.Load(cfg, c.PkgPath)
	if err != nil {
		return nil, fmt.Errorf("load package %s: %w", c.PkgPath, err)
	}
	if len(pkgs) == 0 {
		return nil, fmt.Errorf("package %s not found", c.PkgPath)
	}
	if packages.PrintErrors(pkgs) > 0 {
		return nil, fmt.Errorf("package %s has errors", c.PkgPath)
	}
	pkg := pkgs[0]

	result := &CollectResult{
		Structs: make(map[string]*TypeDef),
		Enums:   make(map[string]*TypeDef),
		AllDefs: make(map[string]*TypeDef),
	}

	// Build comment map from AST
	comments := buildCommentMap(pkg)

	// First pass: collect all named types and const enums
	scope := pkg.Types.Scope()
	for _, name := range scope.Names() {
		obj := scope.Lookup(name)
		if !obj.Exported() {
			continue
		}

		tname, ok := obj.(*types.TypeName)
		if !ok {
			continue
		}

		namedType, ok := tname.Type().(*types.Named)
		if !ok {
			continue
		}

		typeName := tname.Name()

		// Check for string enum types (type X string with const values)
		if basic, ok := namedType.Underlying().(*types.Basic); ok && basic.Kind() == types.String {
			// This might be an enum type - we'll collect const values separately
			td := &TypeDef{
				Name:       typeName,
				Kind:       "named",
				Underlying: "string",
				Type:       namedType,
				Comment:    extractTypeComment(comments, typeName),
			}
			result.AllDefs[typeName] = td

			// Check if it's also a struct type (e.g. for struct types named like FieldType)
			// Actually, basic underlying means it's NOT a struct
			continue
		}

		// Check for struct types
		structType, ok := namedType.Underlying().(*types.Struct)
		if !ok {
			// Could be a slice, map, etc.
			continue
		}

		td := &TypeDef{
			Name:    typeName,
			Kind:    "struct",
			Type:    namedType,
			Comment: extractTypeComment(comments, typeName),
		}

		// Parse struct fields
		for i := 0; i < structType.NumFields(); i++ {
			field := structType.Field(i)
			if !field.Exported() {
				continue
			}

			tag := structType.Tag(i)
			yamlTag := extractYAMLTags(tag)

			if yamlTag == "-" {
				continue
			}

			fd := FieldDef{
				GoName:   field.Name(),
				Name:     yamlTag,
				Required: !hasOmitEmpty(tag),
				GoType:   field.Type(),
				Tag:      tag,
			}

			if yamlTag == "" {
				fd.Name = field.Name()
			}

			// Check for inline
			if hasInlineFlag(tag) {
				fd.Inline = true
			}
			if hasOmitEmpty(tag) {
				fd.OmitEmpty = true
				fd.Required = false
			}

			// Resolve type info
			resolveFieldType(&fd, result.AllDefs)

			// Get field comment
			fd.Comment = extractFieldComment(comments, typeName, field.Name())
			fd.Annotation = parseSchemaAnnotation(fd.Comment)

			td.Fields = append(td.Fields, fd)
		}

		result.Structs[typeName] = td
		result.AllDefs[typeName] = td

		// Check if this struct field is actually an enum (has const values in the package)
		// Already handled above
	}

	// Parse @schema annotations for types BEFORE enrichEnumValues, so the
	// annotation's enum list can suppress auto-collected const duplicates.
	// Also promote types with @schema {enum: [...]} to enum Kind immediately.
	for _, td := range result.AllDefs {
		td.Annotation = parseSchemaAnnotation(td.Comment)
		if td.Annotation != nil && len(td.Annotation.Enum) > 0 && td.Kind == "named" {
			td.Kind = "enum"
			td.EnumValues = td.Annotation.Enum
			result.Enums[td.Name] = td
		}
	}

	// Second pass: enrich enums with actual values from consts
	enrichEnumValues(pkg, result)

	return result, nil
}

// enrichEnumValues finds all const declarations for string-based types.
func enrichEnumValues(pkg *packages.Package, result *CollectResult) {
	for _, file := range pkg.Syntax {
		for _, decl := range file.Decls {
			genDecl, ok := decl.(*ast.GenDecl)
			if !ok || genDecl.Tok != token.CONST {
				continue
			}
			for _, spec := range genDecl.Specs {
				valueSpec, ok := spec.(*ast.ValueSpec)
				if !ok {
					continue
				}
				if len(valueSpec.Names) == 0 || !valueSpec.Names[0].IsExported() {
					continue
				}
				if valueSpec.Type == nil {
					continue
				}

				typeExpr := valueSpec.Type
				typeName, ok := typeNameFromExpr(typeExpr)
				if !ok {
					continue
				}

				td, exists := result.AllDefs[typeName]
				if !exists {
					continue
				}

				// Skip if type already has @schema {enum: [...]} annotation — the
				// annotation is the authoritative source and overrides auto-collected
				// const values to avoid duplicates.
				if td.Annotation != nil && len(td.Annotation.Enum) > 0 {
					continue
				}

				for i, name := range valueSpec.Names {
					if !name.IsExported() {
						continue
					}
					var val string
					if i < len(valueSpec.Values) {
						val = valueSpec.Values[i].(*ast.BasicLit).Value
						val = strings.Trim(val, "\"")
					}
					td.EnumValues = append(td.EnumValues, val)
				}
			}
		}
	}

	// Mark string enums that have values
	for name, td := range result.AllDefs {
		if len(td.EnumValues) > 0 && td.Underlying == "string" {
			td.Kind = "enum"
			result.Enums[name] = td
		}
	}
}

// typeNameFromExpr extracts the type name from an AST expression.
func typeNameFromExpr(expr ast.Expr) (string, bool) {
	switch e := expr.(type) {
	case *ast.Ident:
		return e.Name, true
	case *ast.SelectorExpr:
		if id, ok := e.X.(*ast.Ident); ok {
			return id.Name + "." + e.Sel.Name, true
		}
		return "", false
	default:
		return "", false
	}
}

// resolveFieldType determines the JSON Schema type info for a field.
func resolveFieldType(fd *FieldDef, _ map[string]*TypeDef) {
	t := fd.GoType
	if t == nil {
		return
	}

	switch typ := t.(type) {
	case *types.Basic:
		fd.TypeKind = "basic"
		fd.TypeName = basicTypeName(typ)
	case *types.Alias:
		// Go 1.22+: `any` is represented as *types.Alias
		// Check if the RHS is interface{} (empty interface)
		if iface, ok := typ.Rhs().(*types.Interface); ok && iface.Empty() {
			fd.TypeKind = "interface"
			fd.TypeName = "any"
		} else {
			fd.TypeKind = "interface"
			fd.TypeName = "any"
		}
	case *types.Named:
		fd.TypeName = typ.Obj().Name()
		fd.NamedType = &fd.TypeName
		// Check underlying
		switch typ.Underlying().(type) {
		case *types.Struct:
			fd.TypeKind = "struct"
		case *types.Basic:
			// Named basic types (like Characteristic = string) stay as "named"
			// so we can resolve enums properly
			fd.TypeKind = "named"
		case *types.Slice:
			fd.TypeKind = "slice"
		case *types.Map:
			fd.TypeKind = "map"
		default:
			fd.TypeKind = "named"
		}
	case *types.Pointer:
		fd.TypeKind = "pointer"
		// Resolve underlying
		switch elem := typ.Elem().(type) {
		case *types.Named:
			fd.TypeName = elem.Obj().Name()
			fd.NamedType = &fd.TypeName
			switch elem.Underlying().(type) {
			case *types.Struct:
				fd.TypeKind = "pointer-struct"
			case *types.Basic:
				fd.TypeKind = "pointer-basic"
			}
		case *types.Basic:
			fd.TypeName = basicTypeName(elem)
			fd.TypeKind = "pointer-basic"
		}
	case *types.Slice:
		fd.TypeKind = "slice"
		if elem, ok := typ.Elem().(*types.Named); ok {
			fd.TypeName = "[]" + elem.Obj().Name()
		} else if elem, ok := typ.Elem().(*types.Basic); ok {
			fd.TypeName = "[]" + basicTypeName(elem)
		} else {
			fd.TypeName = "[]?"
		}
	case *types.Map:
		fd.TypeKind = "map"
		fd.TypeName = "map"
		// Capture the map's value type so fieldToSchema can emit a $ref when
		// the value is a known struct (e.g. ConfigSpec.Keys → ConfigKey).
		if elem, ok := typ.Elem().(*types.Named); ok {
			name := elem.Obj().Name()
			fd.MapValueType = &name
		}
	case *types.Interface:
		fd.TypeKind = "interface"
		fd.TypeName = "any"
	default:
		fd.TypeKind = "unknown"
		fd.TypeName = fmt.Sprintf("%T", t)
	}
}

// basicTypeName maps Go basic types to JSON Schema type names.
func basicTypeName(b *types.Basic) string {
	switch b.Kind() {
	case types.String:
		return "string"
	case types.Int, types.Int8, types.Int16, types.Int32, types.Int64,
		types.Uint, types.Uint8, types.Uint16, types.Uint32, types.Uint64:
		return "integer"
	case types.Float32, types.Float64:
		return "number"
	case types.Bool:
		return "boolean"
	case types.UntypedInt:
		return "integer"
	case types.UntypedFloat:
		return "number"
	case types.UntypedString:
		return "string"
	default:
		return "string"
	}
}

// extractYAMLTags parses the yaml tag from a struct tag string.
func extractYAMLTags(tag string) string {
	tag = strings.Trim(tag, "`")
	// Parse Go struct tag format
	re := regexp.MustCompile(`yaml:"([^"]*)"`)
	matches := re.FindStringSubmatch(tag)
	if len(matches) < 2 {
		return ""
	}
	val := matches[1]
	// Remove options like omitempty, flow, etc.
	if idx := strings.Index(val, ","); idx >= 0 {
		val = val[:idx]
	}
	return val
}

// hasOmitEmpty checks if the struct tag has omitempty.
func hasOmitEmpty(tag string) bool {
	tag = strings.Trim(tag, "`")
	re := regexp.MustCompile(`yaml:"([^"]*)"`)
	matches := re.FindStringSubmatch(tag)
	if len(matches) < 2 {
		return false
	}
	return strings.Contains(matches[1], "omitempty")
}

// hasInlineFlag checks if the struct tag has ,inline.
func hasInlineFlag(tag string) bool {
	tag = strings.Trim(tag, "`")
	re := regexp.MustCompile(`yaml:"([^"]*)"`)
	matches := re.FindStringSubmatch(tag)
	if len(matches) < 2 {
		return false
	}
	return strings.Contains(matches[1], "inline")
}

// buildCommentMap builds a map from type name and field name to comments.
func buildCommentMap(pkg *packages.Package) map[string]map[string]string {
	result := make(map[string]map[string]string)

	for _, file := range pkg.Syntax {
		for _, decl := range file.Decls {
			genDecl, ok := decl.(*ast.GenDecl)
			if !ok {
				continue
			}
			for _, spec := range genDecl.Specs {
				typeSpec, ok := spec.(*ast.TypeSpec)
				if !ok {
					continue
				}
				typeName := typeSpec.Name.Name
				if !typeSpec.Name.IsExported() {
					continue
				}

				// Store type-level comment
				if genDecl.Doc != nil {
					if result[typeName] == nil {
						result[typeName] = make(map[string]string)
					}
					result[typeName][""] = genDecl.Doc.Text()
				}

				structType, ok := typeSpec.Type.(*ast.StructType)
				if !ok {
					continue
				}

				for _, field := range structType.Fields.List {
					if len(field.Names) == 0 {
						continue
					}
					for _, name := range field.Names {
						if !name.IsExported() {
							continue
						}
						fieldName := name.Name
						if result[typeName] == nil {
							result[typeName] = make(map[string]string)
						}
						if field.Doc != nil {
							result[typeName][fieldName] = field.Doc.Text()
						} else if field.Comment != nil {
							result[typeName][fieldName] = field.Comment.Text()
						}
					}
				}
			}
		}
	}

	return result
}

// extractTypeComment gets the type-level comment.
func extractTypeComment(comments map[string]map[string]string, typeName string) string {
	if m, ok := comments[typeName]; ok {
		return m[""]
	}
	return ""
}

// extractFieldComment gets the comment for a specific field.
func extractFieldComment(comments map[string]map[string]string, typeName, fieldName string) string {
	if m, ok := comments[typeName]; ok {
		return m[fieldName]
	}
	return ""
}

// parseSchemaAnnotation parses // @schema { ... } from a comment string.
// Supports both strict JSON and Go-like key: value format.
func parseSchemaAnnotation(comment string) *schemaAnnotation {
	if comment == "" {
		return nil
	}

	re := regexp.MustCompile(`@schema\s*\{([^}]*)\}`)
	matches := re.FindStringSubmatch(comment)
	if len(matches) < 2 {
		return nil
	}

	content := strings.TrimSpace(matches[1])

	// Try strict JSON first
	jsonStr := "{" + content + "}"
	var ann schemaAnnotation
	if err := json.Unmarshal([]byte(jsonStr), &ann); err == nil {
		return &ann
	}

	// Fallback: manual parse for Go-like key: value format (unquoted keys)
	ann = schemaAnnotation{}
	parts := splitAnnotationParts(content)
	for _, part := range parts {
		colonIdx := strings.Index(part, ":")
		if colonIdx < 0 {
			continue
		}
		key := strings.TrimSpace(part[:colonIdx])
		rawVal := strings.TrimSpace(part[colonIdx+1:])

		// Unquote string value if needed
		val := strings.Trim(rawVal, "\"'")

		switch key {
		case "description":
			ann.Description = val
		case "markdownDescription":
			ann.MarkdownDescription = val
		case "title":
			ann.Title = val
		case "example":
			ann.Example = val
		case "deprecated":
			ann.Deprecated = val == "true"
		case "pattern":
			ann.Pattern = val
		case "minLength":
			if v, err := parseIntPtr(val); err == nil {
				ann.MinLength = v
			}
		case "maxLength":
			if v, err := parseIntPtr(val); err == nil {
				ann.MaxLength = v
			}
		case "minimum":
			if v, err := parseFloat64Ptr(val); err == nil {
				ann.Minimum = v
			}
		case "maximum":
			if v, err := parseFloat64Ptr(val); err == nil {
				ann.Maximum = v
			}
		case "enum":
			// Format: [val1, val2, val3]
			vals := parseEnumValues(val)
			ann.Enum = vals
		}
	}
	return &ann
}

// splitAnnotationParts splits "a: 1, b: \"hello\", c: [x, y]" by comma
// respecting quoted strings.
func splitAnnotationParts(s string) []string {
	var parts []string
	depth := 0
	inQuote := false
	quoteChar := byte(0)
	start := 0

	for i := 0; i < len(s); i++ {
		ch := s[i]
		if inQuote {
			if ch == quoteChar && (i == 0 || s[i-1] != '\\') {
				inQuote = false
			}
			continue
		}
		switch ch {
		case '"', '\'':
			inQuote = true
			quoteChar = ch
		case '{', '[':
			depth++
		case '}', ']':
			depth--
		case ',':
			if depth == 0 {
				parts = append(parts, strings.TrimSpace(s[start:i]))
				start = i + 1
			}
		}
	}
	if start < len(s) {
		parts = append(parts, strings.TrimSpace(s[start:]))
	}
	return parts
}

// parseEnumValues parses "[val1, val2, val3]" into a string slice.
func parseEnumValues(s string) []string {
	s = strings.TrimSpace(s)
	s = strings.Trim(s, "[]")
	if s == "" {
		return nil
	}
	var vals []string
	for _, part := range splitEnumParts(s) {
		vals = append(vals, strings.Trim(strings.TrimSpace(part), "\"'"))
	}
	return vals
}

// splitEnumParts splits by comma respecting quotes.
func splitEnumParts(s string) []string {
	var parts []string
	inQuote := false
	quoteChar := byte(0)
	start := 0
	for i := 0; i < len(s); i++ {
		ch := s[i]
		if inQuote {
			if ch == quoteChar {
				inQuote = false
			}
			continue
		}
		switch ch {
		case '"', '\'':
			inQuote = true
			quoteChar = ch
		case ',':
			parts = append(parts, strings.TrimSpace(s[start:i]))
			start = i + 1
		}
	}
	if start < len(s) {
		parts = append(parts, strings.TrimSpace(s[start:]))
	}
	return parts
}

func parseIntPtr(s string) (*int, error) {
	s = strings.TrimSpace(s)
	var v int
	if _, err := fmt.Sscanf(s, "%d", &v); err != nil {
		return nil, err
	}
	return &v, nil
}

func parseFloat64Ptr(s string) (*float64, error) {
	s = strings.TrimSpace(s)
	var v float64
	if _, err := fmt.Sscanf(s, "%f", &v); err != nil {
		return nil, err
	}
	return &v, nil
}

// ToJSON marshals a Schema to indented JSON.
func (s *Schema) ToJSON() ([]byte, error) {
	return json.MarshalIndent(s, "", "  ")
}
