// Package formspec embeds the generated JSON Schema files so that `formspec init`
// can write them into new projects. This gives the YAML editor (VS Code +
// YAML extension) autocomplete and validation for FormSpec manifests right out
// of the box, via the `yaml.schemas` setting in .vscode/settings.json.
//
// Schemas are generated from pkg/spec Go types with:
//
//	make generate-schema   # -> schemas/formspec.schema.json + schemas/kinds/*.schema.json
//
// and committed to the repo. `formspec init` extracts them into <project>/schemas/.
package formspec

import "embed"

//go:embed schemas/formspec.schema.json schemas/kinds/*.schema.json
var SchemasFS embed.FS
