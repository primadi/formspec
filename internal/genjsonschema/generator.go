package genjsonschema

import (
	"fmt"
	"go/types"
	"sort"
	"strings"
)

// GenerateResult holds all generated JSON Schema documents.
type GenerateResult struct {
	RootSchema  *Schema            // Root discriminator schema
	KindSchemas map[string]*Schema // Per-kind spec schemas
	SharedDefs  map[string]*Schema // Shared type definitions
}

// Generate produces JSON Schema from collected type definitions.
func (c *Converter) Generate(collect *CollectResult) *GenerateResult {
	result := &GenerateResult{
		KindSchemas: make(map[string]*Schema),
		SharedDefs:  make(map[string]*Schema),
	}

	// First pass: build shared $defs for common types
	sharedTypes := []string{
		"Metadata", "Field", "Action", "ImplDecl", "StateMachine",
		"EventDecl", "TransitionDecl", "GuardDecl", "StateDecl",
		"MenuDecl", "MenuItem", "ValidationRule", "RelationDecl",
		"ChildDecl", "ComputedDecl", "AutoFillDecl", "IndexDecl", "UsesDecl",
		"ParamsDecl", "ConditionDecl", "HookDecl", "IdempotencyDecl",
		"EntityAuth", "ExposeConfig", "RateLimitSpec",
		"ActionUIHint", "FilterSpec", "FormSection", "FormField", "FormAction", "FormSubmit", "FormRenderDecl",
		"TableColumn", "TableAction", "BackdatePolicy", "ForwardDatePolicy",
		"SoftDeactivateDecl", "PersistSpec", "ExtendStorage",
		"NaturalKeyRuleDecl", "NaturalKeyPrefix", "StorageSpec", "FieldRef",
		"DeliveryDecl", "PublishDecl", "PayloadDecl", "EventDeliveryDecl", "DeliveryTarget",
		"RetryDecl", "PageBlock", "PageTab", "BlockRef", "DashboardWidget", "WidgetLayout",
		"SectionBlock", "SectionCTA", "SectionItem",
		"ReportParam", "ReportColumn", "ReportGroup", "ReportTotal", "ReportSource", "WizardStep",
		"WizardOnComplete", "WizardSummaryItem", "KanbanColumn", "KanbanCard",
		"PrintOutput", "PrintPaper", "PrintCustomPaper", "PrintHeader", "PrintBodyItem",
		"PrintChildTable", "PrintTotals", "PrintFooter",
		"TimelineDisplay",
		// Kind-specific sub-types referenced via $ref from kind specs
		"ApiGRPCConfig", "ApiRESTConfig",
		"DatastoreAccess", "DatastoreConnection", "DatastoreAccessFilter", "DatastorePermission", "DatastorePool", "DatastorePermissionRule",
		"ConfigKey",                    // map value type for ConfigSpec.Keys
		"Settings", "CurrencySettings", // global settings namespace (spec §10)
		"Dependency",
		"AiIndexDecl", "AppInterface", "AppConsume",
		"EnvironmentPlane", "PolicyApproval",
		"IntegratorCall", "IntegratorListen",
		"KvstoreUseDecl",
		"PageLayout",
		"PageBinds", "BlockBinds",
		"ParamValidation",
		"SlotDecl", "SlotContract",
		"StorageTransform",
		"SubDeliveryDecl",
		"UsesConfigDecl", "UsesDbDecl",
		"WebhookAuth", "WebhookSigConfig", "WebhookKeyRef", "WebhookTokenConfig",
		"WorkflowEscalation", "WorkflowReject", "WorkflowStep", "WorkflowTrigger", "WorkflowTransitionRef", "StepEscalation",
		"CallbackDecl",  // async job callback webhook (todo 7.13)
		"SnapshotField", // financial denormalization snapshot (todo 7.10)
	}

	for _, name := range sharedTypes {
		if td, ok := collect.Structs[name]; ok {
			result.SharedDefs[name] = structToSchema(td, collect, result.SharedDefs)
		}
	}

	// Also add enum types used across kinds as shared defs
	for name, td := range collect.Enums {
		if _, exists := result.SharedDefs[name]; !exists {
			result.SharedDefs[name] = enumToSchema(td)
		}
	}

	// Second pass: generate per-kind schemas
	for _, entry := range KindMapping() {
		specStruct := entry.SpecStruct
		td, ok := collect.Structs[specStruct]
		if !ok {
			// Try to find by matching name
			for _, s := range collect.Structs {
				if s.Name == specStruct {
					td = s
					ok = true
					break
				}
			}
		}
		if !ok {
			// Check if it's defined in another package (like EnvironmentSpec, PolicySpec)
			// For now, create a placeholder
			result.KindSchemas[entry.Kind] = &Schema{
				Title:       entry.Kind + " Spec",
				Description: fmt.Sprintf("Spec for kind: %s (type definition not found in pkg/spec)", entry.Kind),
				Type:        "object",
			}
			continue
		}

		schema := structToSchema(td, collect, result.SharedDefs)
		schema.Title = entry.Kind + " Spec"
		result.KindSchemas[entry.Kind] = schema
	}

	// Add known enum types that might not be in enums map yet
	for name, td := range collect.AllDefs {
		if td.Kind == "enum" {
			result.SharedDefs[name] = enumToSchema(td)
		}
	}

	// Build root discriminator schema
	result.RootSchema = buildRootSchema(collect, result.SharedDefs, result.KindSchemas)

	return result
}

// structToSchema converts a Go struct TypeDef to a JSON Schema.
// Special types like ValidationRule get custom oneOf handling.
func structToSchema(td *TypeDef, collect *CollectResult, sharedDefs map[string]*Schema) *Schema {
	s := &Schema{
		Type:                 "object",
		Description:          td.Comment,
		AdditionalProperties: false,
	}

	if td.Annotation != nil {
		if td.Annotation.Description != "" {
			s.Description = td.Annotation.Description
		}
		if td.Annotation.MarkdownDescription != "" {
			s.MarkdownDescription = td.Annotation.MarkdownDescription
		}
	}

	// Special handling for ValidationRule which accepts multiple YAML formats
	// via custom UnmarshalYAML: string ("required"), colon ("after:end_date"),
	// map-shorthand ({min_length: 1}), or full ({name: "min_length", value: 1}).
	if td.Name == "ValidationRule" {
		// Full object format: {name: "min_length", value: 1}
		objSchema := &Schema{
			Type:                 "object",
			AdditionalProperties: false,
			Properties: map[string]*Schema{
				"name":  {Type: "string", Description: "Validation rule name"},
				"value": {Description: "Validation rule value (optional)"},
			},
			Required: []string{"name"},
		}

		// String shorthand: "required", "positive", "past"
		strSchema := &Schema{
			Type:        "string",
			Description: "Shorthand rule name — e.g. \"required\", \"positive\", \"past\"",
		}

		// Map shorthand: {min_length: 1}, {max_length: 100}, {pattern: "^..."}
		// Single-property object where key is rule name and value is rule value
		mapSchema := &Schema{
			Type:          "object",
			MinProperties: intPtr(1),
			MaxProperties: intPtr(1),
			AdditionalProperties: map[string]any{
				"description": "Rule value — string, number, or boolean",
			},
		}

		s.OneOf = []*Schema{strSchema, objSchema, mapSchema}
		s.Type = ""
		s.AdditionalProperties = nil
		delete(s.Properties, "name")
		delete(s.Properties, "value")
		s.Required = nil
		return s
	}

	s.Properties = make(map[string]*Schema)
	var required []string

	for _, fd := range td.Fields {
		if fd.Inline {
			// For inline fields, merge the properties from the referenced struct
			mergeInlineSchema(s, fd, collect, sharedDefs)
			continue
		}

		prop := fieldToSchema(fd, collect, sharedDefs)
		if prop.Description == "" && fd.Comment != "" {
			// Use first line of comment as description
			lines := strings.SplitN(fd.Comment, "\n", 2)
			prop.Description = strings.TrimSpace(lines[0])
		}
		s.Properties[fd.Name] = prop

		// Array fields (slices) are not required — FormSpec derives them by
		// default when omitted. Only scalar/object fields enforce required.
		if fd.Required && fd.TypeKind != "slice" {
			required = append(required, fd.Name)
		}
	}

	if len(required) > 0 {
		s.Required = required
	}

	// Special handling for TransitionDecl.from which accepts both string
	// and []string via custom UnmarshalYAML on StateList.
	if td.Name == "TransitionDecl" {
		if fromField, ok := s.Properties["from"]; ok {
			strSchema := &Schema{
				Type:        "string",
				Description: "Single source state name",
			}
			arrSchema := &Schema{
				Type: "array",
				Items: &Schema{
					Type: "string",
				},
				Description: "List of source state names",
			}
			fromField.OneOf = []*Schema{strSchema, arrSchema}
			fromField.Type = ""
			fromField.Items = nil
		}
	}

	return s
}

// mergeInlineSchema merges the fields from an inline struct into the parent schema.
func mergeInlineSchema(s *Schema, fd FieldDef, collect *CollectResult, sharedDefs map[string]*Schema) {
	var inlineStruct *TypeDef
	if fd.NamedType != nil {
		inlineStruct = collect.Structs[*fd.NamedType]
	}
	if inlineStruct == nil {
		// Try to find the struct type
		underlying := fd.GoType
		if ptr, ok := underlying.(*types.Pointer); ok {
			underlying = ptr.Elem()
		}
		if named, ok := underlying.(*types.Named); ok {
			name := named.Obj().Name()
			inlineStruct = collect.Structs[name]
		}
	}
	if inlineStruct == nil {
		return
	}

	for _, infd := range inlineStruct.Fields {
		prop := fieldToSchema(infd, collect, sharedDefs)
		if prop.Description == "" && infd.Comment != "" {
			lines := strings.SplitN(infd.Comment, "\n", 2)
			prop.Description = strings.TrimSpace(lines[0])
		}
		s.Properties[infd.Name] = prop
		if infd.Required {
			s.Required = append(s.Required, infd.Name)
		}
	}
}

// fieldToSchema converts a single struct field to a JSON Schema.
func fieldToSchema(fd FieldDef, collect *CollectResult, sharedDefs map[string]*Schema) *Schema {
	s := &Schema{}

	switch fd.TypeKind {
	case "basic":
		s.Type = fd.TypeName

	case "pointer-basic":
		s.Type = fd.TypeName
		s.Nullable = true

	case "pointer-struct", "pointer":
		if fd.NamedType != nil {
			refName := *fd.NamedType
			if refSchema, ok := sharedDefs[refName]; ok && refSchema.Ref != "" {
				s.Ref = refSchema.Ref
			} else if _, ok := collect.Structs[refName]; ok {
				defName := refName
				s.Ref = "#/$defs/" + defName
			} else if td, ok := collect.AllDefs[refName]; ok && td.Kind == "enum" {
				s.Ref = "#/$defs/" + refName
			} else {
				s.Type = "object"
			}
		} else {
			s.Type = "object"
		}
		s.Nullable = true

	case "struct":
		if fd.NamedType != nil {
			refName := *fd.NamedType
			if refSchema, ok := sharedDefs[refName]; ok && refSchema.Ref != "" {
				s.Ref = refSchema.Ref
			} else if _, ok := collect.Structs[refName]; ok {
				s.Ref = "#/$defs/" + refName
			} else {
				s.Type = "object"
			}
		} else {
			s.Type = "object"
		}

	case "named":
		if fd.NamedType != nil {
			n := *fd.NamedType
			if _, ok := collect.Enums[n]; ok {
				s.Ref = "#/$defs/" + n
			} else if _, ok := collect.Structs[n]; ok {
				s.Ref = "#/$defs/" + n
			} else if td, ok := collect.AllDefs[n]; ok && td.Kind == "enum" {
				s.Ref = "#/$defs/" + n
			} else {
				s.Type = "string"
			}
		} else {
			s.Type = "string"
		}

	case "slice":
		s.Type = "array"
		s.Items = fieldItemsSchema(fd, collect, sharedDefs)

	case "map":
		s.Type = "object"
		// When the map's value is a known struct (e.g. ConfigSpec.Keys →
		// ConfigKey), emit a $ref so YAML autocomplete/validation works for
		// the map entries. Otherwise fall back to unconstrained values.
		if fd.MapValueType != nil {
			if _, ok := collect.Structs[*fd.MapValueType]; ok {
				s.AdditionalProperties = map[string]any{
					"$ref": "#/$defs/" + *fd.MapValueType,
				}
				break
			}
		}
		s.AdditionalProperties = map[string]any{}

	case "interface":
		// any type — no constraints

	default:
		s.Type = "string"
	}

	// Apply type-level annotation from the named type definition
	// Only if the schema doesn't already use $ref — $ref + sibling properties
	// violates JSON Schema Draft-7 semantics (siblings are ignored) and
	// confuses validators (e.g. Python jsonschema).
	if s.Ref == "" && fd.NamedType != nil {
		if td, ok := collect.AllDefs[*fd.NamedType]; ok && td.Annotation != nil {
			applyAnnotation(s, td.Annotation)
		}
	}

	// Apply field-level annotation (overrides type-level)
	// Only if not using $ref, for the same reason.
	if s.Ref == "" && fd.Annotation != nil {
		applyAnnotation(s, fd.Annotation)
	}

	// Add comment as description if available and not already set
	if s.Description == "" && fd.Comment != "" {
		lines := strings.SplitN(fd.Comment, "\n", 2)
		desc := strings.TrimSpace(lines[0])
		// Remove @schema annotation from description
		desc = strings.Split(desc, "@schema")[0]
		desc = strings.TrimSpace(desc)
		if desc != "" {
			s.Description = desc
		}
	}

	return s
}

// fieldItemsSchema creates the items schema for an array field.
func fieldItemsSchema(fd FieldDef, collect *CollectResult, _ map[string]*Schema) *Schema {
	items := &Schema{}

	// Determine element type from GoType
	switch t := fd.GoType.(type) {
	case *types.Slice:
		elem := t.Elem()
		switch e := elem.(type) {
		case *types.Named:
			name := e.Obj().Name()
			if _, ok := collect.Enums[name]; ok {
				items.Ref = "#/$defs/" + name
			} else if _, ok := collect.Structs[name]; ok {
				items.Ref = "#/$defs/" + name
			} else {
				items.Type = "string"
			}
		case *types.Basic:
			items.Type = basicTypeName(e)
		case *types.Pointer:
			if named, ok := e.Elem().(*types.Named); ok {
				name := named.Obj().Name()
				if _, ok := collect.Structs[name]; ok {
					items.Ref = "#/$defs/" + name
				} else {
					items.Type = "string"
				}
			} else {
				items.Type = "string"
			}
		case *types.Interface:
			// any type
		default:
			items.Type = "string"
		}
	case *types.Named:
		// Named slice type like []string
		items.Type = "string"
	}

	return items
}

// enumToSchema converts an enum type definition to a JSON Schema.
func enumToSchema(td *TypeDef) *Schema {
	s := &Schema{
		Type:        "string",
		Description: td.Comment,
	}
	hasAnnotationEnum := false
	if td.Annotation != nil {
		applyAnnotation(s, td.Annotation)
		hasAnnotationEnum = len(td.Annotation.Enum) > 0
	}
	// Only add auto-collected const values if no annotation enum was applied,
	// to avoid duplicates.
	if !hasAnnotationEnum {
		for _, v := range td.EnumValues {
			s.Enum = append(s.Enum, v)
		}
	}
	return s
}

// applyAnnotation applies annotation metadata to a schema.
func applyAnnotation(s *Schema, ann *SchemaAnnotation) {
	if ann.Description != "" {
		s.Description = ann.Description
	}
	if ann.MarkdownDescription != "" {
		s.MarkdownDescription = ann.MarkdownDescription
	}
	if ann.Title != "" {
		s.Title = ann.Title
	}
	if ann.Deprecated {
		s.Deprecated = true
	}
	if ann.MinLength != nil {
		s.MinLength = ann.MinLength
	}
	if ann.MaxLength != nil {
		s.MaxLength = ann.MaxLength
	}
	if ann.Pattern != "" {
		s.Pattern = ann.Pattern
	}
	if ann.Minimum != nil {
		s.Minimum = ann.Minimum
	}
	if ann.Maximum != nil {
		s.Maximum = ann.Maximum
	}
	if ann.Example != "" {
		s.Examples = append(s.Examples, ann.Example)
	}
	if len(ann.Enum) > 0 {
		for _, v := range ann.Enum {
			s.Enum = append(s.Enum, v)
		}
	}
}

// buildRootSchema creates the root discriminator schema that selects the
// appropriate kind-specific schema based on the `kind` field.
func buildRootSchema(collect *CollectResult, sharedDefs map[string]*Schema, kindSchemas map[string]*Schema) *Schema {
	root := &Schema{
		Schema: "http://json-schema.org/draft-07/schema#",
		Title:  "FormSpec Manifest",
		Description: "JSON Schema for FormSpec YAML manifests (formspec.dev/v1).\n" +
			"Validates the apiVersion/kind/metadata/spec structure and routes to kind-specific specs.",
		Type: "object",
		Properties: map[string]*Schema{
			"apiVersion": {
				Type:        "string",
				Const:       "formspec.dev/v1",
				Description: "FormSpec API version — must be formspec.dev/v1",
			},
			"kind": {
				Type:        "string",
				Description: "Resource kind — determines the structure of `spec`",
			},
			"metadata": {
				Ref: "#/$defs/Metadata",
			},
			"spec": {
				Description: "Kind-specific specification — structure depends on `kind`",
			},
		},
		Required: []string{"apiVersion", "kind", "metadata", "spec"},
		Defs:     make(map[string]*Schema),
	}

	// Add shared defs to root schema
	// Sort for deterministic output
	var defNames []string
	for name := range sharedDefs {
		defNames = append(defNames, name)
	}
	sort.Strings(defNames)

	for _, name := range defNames {
		root.Defs[name] = sharedDefs[name]
	}

	// Also populate `definitions` (Draft-4 alias) for validators that don't
	// support $defs or have issues resolving $ref across nested schemas.
	root.Definitions = root.Defs

	// Also add Metadata definition if not already present
	if _, ok := sharedDefs["Metadata"]; !ok {
		if md, ok := collect.Structs["Metadata"]; ok {
			root.Defs["Metadata"] = structToSchema(md, collect, sharedDefs)
		}
	}

	// Build enum list for kind field
	var kindVals []any
	var kindNames []string
	for _, entry := range KindMapping() {
		if entry.Deprecated {
			continue
		}
		kindNames = append(kindNames, entry.Kind)
	}
	sort.Strings(kindNames)
	for _, name := range kindNames {
		kindVals = append(kindVals, name)
	}
	// Add deprecated kinds too
	for _, entry := range KindMapping() {
		if entry.Deprecated {
			kindVals = append(kindVals, entry.Kind)
		}
	}

	root.Properties["kind"].Enum = kindVals

	// Build oneOf for kind-based discrimination
	// Handle aliases: Entity → Document share the same oneOf entry
	var oneOfs []*Schema
	// Get Metadata schema to inline in oneOf entries (avoids $ref resolution issues)
	metadataSchema := root.Defs["Metadata"]

	for _, entry := range KindMapping() {
		if entry.Deprecated {
			// Skip deprecated entries; they share a oneOf with their alias target
			continue
		}

		specSchema, ok := kindSchemas[entry.Kind]
		if !ok {
			continue
		}

		// Collect all kind names that share this spec struct (aliases)
		kindValues := []any{entry.Kind}
		for _, other := range KindMapping() {
			if other.SpecStruct == entry.SpecStruct && other.Kind != entry.Kind {
				kindValues = append(kindValues, other.Kind)
			}
		}

		kindSchema := &Schema{
			Type: "string",
		}
		if len(kindValues) == 1 {
			kindSchema.Const = entry.Kind
		} else {
			kindSchema.Enum = kindValues
		}

		oneOf := &Schema{
			Properties: map[string]*Schema{
				"apiVersion": {
					Type:  "string",
					Const: "formspec.dev/v1",
				},
				"kind":     kindSchema,
				"metadata": metadataSchema, // inlined, not $ref, to avoid $ref resolution issues in some validators
				"spec":     specSchema,
			},
			Required: []string{"apiVersion", "kind", "metadata", "spec"},
		}
		oneOfs = append(oneOfs, oneOf)
	}

	root.OneOf = oneOfs

	return root
}

func intPtr(n int) *int {
	return &n
}
