// Package forma embeds the generated JSON Schema files so that `forma init`
// can write them into new projects. This gives the YAML editor (VS Code +
// YAML extension) autocomplete and validation for Forma manifests right out
// of the box, via the `yaml.schemas` setting in .vscode/settings.json.
//
// Schemas are generated from pkg/spec Go types with:
//
//	make generate-schema   # -> schemas/forma.schema.json + schemas/kinds/*.schema.json
//
// and committed to the repo. `forma init` extracts them into <project>/schemas/.
package forma

import "embed"

//go:embed schemas/forma.schema.json schemas/kinds/*.schema.json
var SchemasFS embed.FS
