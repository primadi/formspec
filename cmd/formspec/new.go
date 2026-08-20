// Command `formspec new <kind>` scaffolds boilerplate YAML manifests for a
// given kind (todo 3.1.3). It is the second rung of the four-rung ladder that
// reduces YAML verbosity (docs/cli-tools/02-formspec-cli.md §3):
//
//	formspec new app tokoku        # scaffold App (alias for generate node-app)
//	formspec new module cafe-master # scaffold Module manifest
//	formspec new entity menu-item   # scaffold Entity manifest + basic fields
//
// Scaffolds are written into the standard project layout
// (docs/spec/platform/08-project-layout.md):
//
//	spec/modules/{module}/module.yaml
//	spec/modules/{module}/{characteristic}/{entity}/entity.yaml
package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// validCharacteristics is the closed set of Entity characteristics
// (docs/spec/backend/01-core-basic.md §1).
var validCharacteristics = map[string]bool{
	"master":      true,
	"transaction": true,
	"reference":   true,
	"summary":     true,
}

// runNew dispatches `formspec new <kind>`.
func runNew(args []string) {
	if len(args) < 1 {
		newUsage()
		os.Exit(1)
	}
	switch args[0] {
	case "app":
		// `new app` is an alias for `generate node-app` (existing behavior).
		runGenerateNodeApp(args[1:])
	case "entity":
		runNewEntity(args[1:])
	case "module":
		runNewModule(args[1:])
	default:
		fmt.Fprintf(os.Stderr, "formspec new: unknown kind %q\n", args[0])
		newUsage()
		os.Exit(1)
	}
}

func newUsage() {
	fmt.Fprintf(os.Stderr, "Usage: formspec new <kind> [name] [flags]\n\n")
	fmt.Fprintf(os.Stderr, "Scaffold boilerplate YAML for a kind.\n\n")
	fmt.Fprintf(os.Stderr, "Kinds:\n")
	fmt.Fprintf(os.Stderr, "  app tokoku          Scaffold a TypeScript sidecar app\n")
	fmt.Fprintf(os.Stderr, "  module cafe-master  Scaffold a Module manifest\n")
	fmt.Fprintf(os.Stderr, "  entity menu-item    Scaffold an Entity manifest + basic fields\n")
}

// runNewModule scaffolds a Module manifest at spec/modules/{module}/module.yaml.
func runNewModule(args []string) {
	force := false
	var positional []string
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--force", "-force":
			force = true
		case "--help", "-h":
			fmt.Fprintf(os.Stderr, "Usage: formspec new module <name> [--force]\n\n")
			fmt.Fprintf(os.Stderr, "Scaffold a Module manifest at spec/modules/{name}/module.yaml\n")
			os.Exit(0)
		default:
			positional = append(positional, args[i])
		}
	}

	if len(positional) < 1 {
		fmt.Fprintf(os.Stderr, "Usage: formspec new module <name> [--force]\n")
		os.Exit(1)
	}
	module := toKebabCase(positional[0])
	if module == "" {
		fmt.Fprintf(os.Stderr, "Error: module name cannot be empty\n")
		os.Exit(1)
	}

	dir := filepath.Join("spec", "modules", module)
	if err := os.MkdirAll(dir, 0755); err != nil {
		fmt.Fprintf(os.Stderr, "Error: cannot create directory %s: %v\n", dir, err)
		os.Exit(1)
	}

	path := filepath.Join(dir, "module.yaml")
	if _, err := os.Stat(path); err == nil && !force {
		fmt.Fprintf(os.Stderr, "Error: %s already exists. Use --force to overwrite.\n", path)
		os.Exit(1)
	}

	content := fmt.Sprintf(`apiVersion: formspec.dev/v1
kind: Module
metadata:
  name: %s
  description: "%s module"
spec:
  version: 1.0.0
  vendor: formspec
`, module, titleCase(module))

	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		fmt.Fprintf(os.Stderr, "Error: cannot write %s: %v\n", path, err)
		os.Exit(1)
	}
	fmt.Fprintf(os.Stderr, "  ✓ %s\n", path)
	fmt.Fprintf(os.Stderr, "\nNext: add an entity with:\n")
	fmt.Fprintf(os.Stderr, "  formspec new entity <name> --module %s\n", module)
}

// runNewEntity scaffolds an Entity manifest at
// spec/modules/{module}/{characteristic}/{entity}/entity.yaml with basic
// fields (code, name, description) and a default expose block.
func runNewEntity(args []string) {
	module := ""
	characteristic := "master"
	force := false
	var positional []string
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--module", "-module":
			if i+1 < len(args) {
				module = args[i+1]
				i++
			}
		case "--characteristic", "-characteristic":
			if i+1 < len(args) {
				characteristic = args[i+1]
				i++
			}
		case "--force", "-force":
			force = true
		case "--help", "-h":
			fmt.Fprintf(os.Stderr, "Usage: formspec new entity <name> [--module <module>] [--characteristic <kind>] [--force]\n\n")
			fmt.Fprintf(os.Stderr, "Scaffold an Entity manifest + basic fields.\n\n")
			fmt.Fprintf(os.Stderr, "Flags:\n")
			fmt.Fprintf(os.Stderr, "  --module <name>          Owning module (default: detect from CWD)\n")
			fmt.Fprintf(os.Stderr, "  --characteristic <kind>  master|transaction|reference|summary (default: master)\n")
			fmt.Fprintf(os.Stderr, "  --force                  Overwrite existing file\n")
			os.Exit(0)
		default:
			positional = append(positional, args[i])
		}
	}

	if len(positional) < 1 {
		fmt.Fprintf(os.Stderr, "Usage: formspec new entity <name> [--module <module>] [--characteristic <kind>] [--force]\n")
		os.Exit(1)
	}
	entity := toKebabCase(positional[0])
	if entity == "" {
		fmt.Fprintf(os.Stderr, "Error: entity name cannot be empty\n")
		os.Exit(1)
	}

	// Validate characteristic (closed set).
	if !validCharacteristics[characteristic] {
		fmt.Fprintf(os.Stderr, "Error: invalid characteristic %q (must be one of: master, transaction, reference, summary)\n", characteristic)
		os.Exit(1)
	}

	// Resolve module: --module flag, else detect from CWD, else project name.
	mod := module
	if mod == "" {
		mod = detectModule()
	}
	mod = toKebabCase(mod)
	if mod == "" {
		fmt.Fprintf(os.Stderr, "Error: cannot determine module — pass --module <name>\n")
		os.Exit(1)
	}

	dir := filepath.Join("spec", "modules", mod, characteristic, entity)
	if err := os.MkdirAll(dir, 0755); err != nil {
		fmt.Fprintf(os.Stderr, "Error: cannot create directory %s: %v\n", dir, err)
		os.Exit(1)
	}

	path := filepath.Join(dir, "entity.yaml")
	if _, err := os.Stat(path); err == nil && !force {
		fmt.Fprintf(os.Stderr, "Error: %s already exists. Use --force to overwrite.\n", path)
		os.Exit(1)
	}

	plural := entity + "s"
	content := fmt.Sprintf(`apiVersion: formspec.dev/v1
kind: Entity
metadata:
  name: %s
  module: %s
  description: "%s entity"
spec:
  version: v1
  characteristic: %s
  lifecycle: plain_crud
  display_field: name
  plural: %s
  fields:
    - name: code
      type: string
      required: true
      unique: true
      title: "Kode"
      description: "Kode unik, contoh %s-001"
    - name: name
      type: string
      required: true
      title: "Nama"
    - name: description
      type: text
      title: "Deskripsi"
  expose:
    - type: rest
      actions: [list, find, create, update, delete]
`, entity, mod, titleCase(entity), characteristic, plural, strings.ToUpper(entity))

	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		fmt.Fprintf(os.Stderr, "Error: cannot write %s: %v\n", path, err)
		os.Exit(1)
	}
	fmt.Fprintf(os.Stderr, "  ✓ %s\n", path)
	fmt.Fprintf(os.Stderr, "\nNext: edit fields, then validate with:\n")
	fmt.Fprintf(os.Stderr, "  formspec validate --spec spec\n")
}

// detectModule tries to infer the owning module from the current working
// directory. It walks up from CWD looking for a path segment under
// spec/modules/{module}/. Falls back to the base name of the CWD.
func detectModule() string {
	cwd, err := os.Getwd()
	if err != nil {
		return ""
	}
	// Walk up looking for spec/modules/{module}/...
	dir := cwd
	for {
		rel, err := filepath.Rel(dir, cwd)
		if err != nil {
			break
		}
		// If CWD is under spec/modules/{module}/..., the first segment after
		// "modules" is the module name.
		parts := strings.Split(filepath.ToSlash(rel), "/")
		if len(parts) >= 2 && parts[0] == "spec" && parts[1] == "modules" && len(parts) >= 3 {
			return parts[2]
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}
	// Fall back to the base name of CWD (project name).
	return filepath.Base(cwd)
}

// titleCase converts a kebab-case identifier to Title Case for descriptions.
func titleCase(s string) string {
	parts := strings.Split(s, "-")
	for i, p := range parts {
		if p == "" {
			continue
		}
		parts[i] = strings.ToUpper(p[:1]) + p[1:]
	}
	return strings.Join(parts, " ")
}
