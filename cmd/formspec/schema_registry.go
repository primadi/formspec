package main

import (
	"os"

	"gopkg.in/yaml.v3"

	"github.com/primadi/formspec/internal/schemaregistry"
)

// schemaRegistryBaseURL resolves the schema registry base URL with priority:
//
//  1. FORMSPEC_SCHEMA_REGISTRY env var
//  2. schema-registry: in formspec-app.yaml (project config)
//  3. default https://schemas.formspec.dev
//
// Used by `formspec validate`, `formspec init`, and `formspec schema`.
func schemaRegistryBaseURL() string {
	if v := os.Getenv("FORMSPEC_SCHEMA_REGISTRY"); v != "" {
		return v
	}
	if p := findConfigFile(); p != "" {
		if data, err := os.ReadFile(p); err == nil {
			var cf configFile
			if err := yaml.Unmarshal(data, &cf); err == nil && cf.SchemaRegistry != nil && *cf.SchemaRegistry != "" {
				return *cf.SchemaRegistry
			}
		}
	}
	return schemaregistry.DefaultBaseURL
}
