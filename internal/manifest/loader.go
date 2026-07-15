// Package manifest provides YAML manifest loading, parsing, and validation.
//
// It handles:
//   - Multi-document YAML files (--- separator)
//   - Directory scanning and recursive discovery
//   - Schema validation per kind
//   - Registration into the resource registry
//
// The parser mirrors the Core Basic spec §3 ("The Forma Manifest Format").
package manifest

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"

	"github.com/primadi/forma/pkg/spec"
)

// Loader reads Forma manifests from a spec directory.
type Loader struct {
	BasePath string
	Strict   bool // reject unknown kinds / invalid schemas
}

// NewLoader creates a manifest loader for the given base path.
func NewLoader(basePath string) *Loader {
	return &Loader{BasePath: basePath, Strict: true}
}

// Discover walks the spec directory and returns all .yaml / .yml files.
func (l *Loader) Discover() ([]string, error) {
	var files []string

	err := filepath.Walk(l.BasePath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			// Skip hidden directories and non-spec directories
			base := filepath.Base(path)
			if strings.HasPrefix(base, ".") || base == "node_modules" {
				return filepath.SkipDir
			}
			// Skip impl/ directory (build-time Go code, not manifests)
			if base == "impl" {
				return filepath.SkipDir
			}
			return nil
		}

		ext := strings.ToLower(filepath.Ext(path))
		if ext == ".yaml" || ext == ".yml" {
			files = append(files, path)
		}

		return nil
	})

	return files, err
}

// ParseError represents a manifest parsing error with location info.
type ParseError struct {
	File    string
	Line    int
	Message string
}

func (e *ParseError) Error() string {
	if e.Line > 0 {
		return fmt.Sprintf("%s:%d: %s", e.File, e.Line, e.Message)
	}
	return fmt.Sprintf("%s: %s", e.File, e.Message)
}

// LoadResult holds the result of loading manifests.
type LoadResult struct {
	Manifests []RawManifest
	Errors    []ParseError
}

// RawManifest is a parsed but not-yet-validated manifest.
type RawManifest struct {
	APIVersion string         `yaml:"apiVersion"`
	Kind       string         `yaml:"kind"`
	Metadata   RawMetadata    `yaml:"metadata"`
	Spec       map[string]any `yaml:"spec"`
	Source     string         `yaml:"-"` // file path + document index
}

// RawMetadata is the metadata section of a raw manifest.
type RawMetadata struct {
	Name        string            `yaml:"name"`
	Module      string            `yaml:"module,omitempty"`
	Description string            `yaml:"description,omitempty"`
	Labels      map[string]string `yaml:"labels,omitempty"`
	Annotations map[string]string `yaml:"annotations,omitempty"`
}

// LoadAll discovers and parses all manifest files under BasePath.
func (l *Loader) LoadAll() (*LoadResult, error) {
	files, err := l.Discover()
	if err != nil {
		return nil, fmt.Errorf("discover: %w", err)
	}

	result := &LoadResult{}

	for _, file := range files {
		docs, errs := l.parseFile(file)
		result.Manifests = append(result.Manifests, docs...)
		result.Errors = append(result.Errors, errs...)
	}

	return result, nil
}

// ParseBytes parses YAML content from a byte slice, treating it as a single
// file that may contain multiple documents (separated by ---).
// It returns parsed raw manifests and any parse errors.
func (l *Loader) ParseBytes(data []byte, source string) ([]RawManifest, []ParseError) {
	decoder := yaml.NewDecoder(strings.NewReader(string(data)))

	var manifests []RawManifest
	var errs []ParseError
	docIndex := 0

	for {
		var raw RawManifest
		err := decoder.Decode(&raw)
		if err != nil {
			if err.Error() == "EOF" {
				break
			}
			errs = append(errs, ParseError{
				File:    source,
				Message: fmt.Sprintf("yaml decode error: %v", err),
			})
			break
		}

		if raw.APIVersion == "" && raw.Kind == "" {
			docIndex++
			continue
		}

		raw.Source = fmt.Sprintf("%s#%d", source, docIndex)
		manifests = append(manifests, raw)
		docIndex++
	}

	return manifests, errs
}

// parseFile parses a single YAML file that may contain multiple documents.
func (l *Loader) parseFile(path string) ([]RawManifest, []ParseError) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, []ParseError{{File: path, Message: fmt.Sprintf("read error: %v", err)}}
	}

	decoder := yaml.NewDecoder(strings.NewReader(string(data)))

	var manifests []RawManifest
	var errs []ParseError
	docIndex := 0

	for {
		var raw RawManifest
		err := decoder.Decode(&raw)
		if err != nil {
			// End of document stream
			if err.Error() == "EOF" {
				break
			}
			errs = append(errs, ParseError{
				File:    path,
				Message: fmt.Sprintf("yaml decode error: %v", err),
			})
			break
		}

		// Skip empty documents
		if raw.APIVersion == "" && raw.Kind == "" {
			docIndex++
			continue
		}

		raw.Source = fmt.Sprintf("%s#%d", path, docIndex)
		manifests = append(manifests, raw)
		docIndex++
	}

	return manifests, errs
}

// KnownKinds is the catalog of built-in resource kinds across the spec documents
// (Core Basic §4, Core Extended, Frontend, Control Plane). Unknown kinds MUST
// fail validation (Core Basic §4). Third-party kinds are registered via
// KindDefinition; until that mechanism lands, only built-ins are accepted.
// KnownKinds is the catalog of built-in resource kinds across the spec documents
// (Core Basic §4, Core Extended, Frontend, Control Plane). Unknown kinds MUST
// fail validation (Core Basic §4). Third-party kinds are registered via
// KindDefinition; until that mechanism lands, only built-ins are accepted.
var KnownKinds = map[string]bool{
	// Core Basic
	"App": true, "Module": true, "Document": true, "Entity": true, "Service": true,
	"Config": true, "Migration": true, "Subscription": true,
	// Core Extended
	"Workflow": true, "Api": true, "Webhook": true, "Mockup": true, "KindDefinition": true, "Integrator": true,
	// Frontend — no "Menu": navigation lives in App.spec.menu / Module.spec.menu.
	"Page": true, "Form": true, "Table": true, "Dashboard": true, "Widget": true,
	"Report": true, "Wizard": true, "Kanban": true, "Timeline": true,
	"Print": true, "Theme": true,
	// Control Plane
	"Environment": true, "Policy": true, "Datastore": true,
}

// Validate performs basic validation on a raw manifest.
func (l *Loader) Validate(raw RawManifest) error {
	if raw.APIVersion == "" {
		return fmt.Errorf("%s: apiVersion is required", raw.Source)
	}
	if raw.Kind == "" {
		return fmt.Errorf("%s: kind is required", raw.Source)
	}
	if !KnownKinds[raw.Kind] {
		return fmt.Errorf("%s: unknown kind %q", raw.Source, raw.Kind)
	}
	if raw.Metadata.Name == "" {
		return fmt.Errorf("%s: metadata.name is required", raw.Source)
	}

	// Type-specific validation
	if (raw.Kind == "Entity" || raw.Kind == "Document") && raw.Spec != nil {
		entitySpec, err := RawSpecToEntitySpec(raw.Spec)
		if err != nil {
			return fmt.Errorf("%s: invalid spec: %w", raw.Source, err)
		}
		if err := spec.ValidateDocumentSpec(entitySpec); err != nil {
			return fmt.Errorf("%s: %w", raw.Source, err)
		}
	}

	return nil
}

// RawSpecToEntitySpec converts a raw spec map to a typed EntitySpec (DocumentSpec).
// Handles backward compatibility:
//   - `characteristics: [X]` (deprecated array) → `characteristic: X`
func RawSpecToEntitySpec(specMap map[string]any) (*spec.EntitySpec, error) {
	// Backward compat: characteristics (plural array) → characteristic (singular)
	if chars, ok := specMap["characteristics"]; ok {
		switch v := chars.(type) {
		case []any:
			if len(v) > 0 {
				specMap["characteristic"] = v[0]
				delete(specMap, "characteristics")
			}
		}
	}

	// Marshal back to YAML bytes, then unmarshal into typed struct.
	// This ensures proper type conversion via yaml tags.
	b, err := yaml.Marshal(specMap)
	if err != nil {
		return nil, fmt.Errorf("re-marshal spec: %w", err)
	}
	var entitySpec spec.EntitySpec
	if err := yaml.Unmarshal(b, &entitySpec); err != nil {
		return nil, fmt.Errorf("unmarshal entity spec: %w", err)
	}
	return &entitySpec, nil
}

// RawSpecTo converts a raw spec map into any typed kind spec via YAML
// round-trip (same mechanism as RawSpecToEntitySpec). Used for the frontend
// kinds (PageSpec, FormSpec, TableSpec, ...) and any future simple kind.
func RawSpecTo[T any](specMap map[string]any) (*T, error) {
	b, err := yaml.Marshal(specMap)
	if err != nil {
		return nil, fmt.Errorf("re-marshal spec: %w", err)
	}
	var out T
	if err := yaml.Unmarshal(b, &out); err != nil {
		return nil, fmt.Errorf("unmarshal spec: %w", err)
	}
	return &out, nil
}

// RawSpecToAppSpec converts a raw spec map to a typed AppSpec.
func RawSpecToAppSpec(specMap map[string]any) (*spec.AppSpec, error) {
	b, err := yaml.Marshal(specMap)
	if err != nil {
		return nil, fmt.Errorf("re-marshal spec: %w", err)
	}
	var appSpec spec.AppSpec
	if err := yaml.Unmarshal(b, &appSpec); err != nil {
		return nil, fmt.Errorf("unmarshal app spec: %w", err)
	}
	return &appSpec, nil
}

// RawSpecToModuleSpec converts a raw spec map to a typed ModuleSpec.
func RawSpecToModuleSpec(specMap map[string]any) (*spec.ModuleSpec, error) {
	b, err := yaml.Marshal(specMap)
	if err != nil {
		return nil, fmt.Errorf("re-marshal spec: %w", err)
	}
	var modSpec spec.ModuleSpec
	if err := yaml.Unmarshal(b, &modSpec); err != nil {
		return nil, fmt.Errorf("unmarshal module spec: %w", err)
	}
	return &modSpec, nil
}

// RawSpecToServiceSpec converts a raw spec map to a typed ServiceSpec.
func RawSpecToServiceSpec(specMap map[string]any) (*spec.ServiceSpec, error) {
	b, err := yaml.Marshal(specMap)
	if err != nil {
		return nil, fmt.Errorf("re-marshal spec: %w", err)
	}
	var svcSpec spec.ServiceSpec
	if err := yaml.Unmarshal(b, &svcSpec); err != nil {
		return nil, fmt.Errorf("unmarshal service spec: %w", err)
	}
	return &svcSpec, nil
}

// RawSpecToConfigSpec converts a raw spec map to a typed ConfigSpec.
func RawSpecToConfigSpec(specMap map[string]any) (*spec.ConfigSpec, error) {
	b, err := yaml.Marshal(specMap)
	if err != nil {
		return nil, fmt.Errorf("re-marshal spec: %w", err)
	}
	var cfg spec.ConfigSpec
	if err := yaml.Unmarshal(b, &cfg); err != nil {
		return nil, fmt.Errorf("unmarshal config spec: %w", err)
	}
	return &cfg, nil
}
