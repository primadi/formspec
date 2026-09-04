// Command `formspec init` scaffolds a new FormSpec project with the standard
// layout defined in docs/spec/platform/08-project-layout.md.
//
// It also extracts embedded AI skills (ai_skills/*) into .agents/skills/
// so that AI coding agents can assist with FormSpec app development, and writes
// the JSON Schema files (schemas/) + .vscode/settings.json (yaml.schemas)
// so the YAML editor gets autocomplete and validation for FormSpec manifests.
//
// Agent instructions are written to AGENTS.md (tool-agnostic standard, read by
// Copilot, Codex, Cursor, Gemini CLI, etc.) plus a thin pointer at
// .github/copilot-instructions.md for older Copilot versions.
//
// Usage:
//
//	formspec init [project-name] [flags]
//
// Flags:
//
//	--module        Module name (default: project name, kebab-case)
//	--with-sidecar  Include app/ sidecar directory for custom handlers
//	--force         Overwrite existing directory without prompt
package main

import (
	"flag"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	formspec "github.com/primadi/formspec"
	"github.com/primadi/formspec/internal/schemaregistry"
)

// bt is a placeholder for backtick (`) in raw string literals.
// Replaced with actual backtick before writing files.
const bt = "\x60"

func runInit(args []string) {
	fs := flag.NewFlagSet("init", flag.ExitOnError)
	moduleName := fs.String("module", "", "Module name (default: project name, kebab-case)")
	withSidecar := fs.Bool("with-sidecar", false, "Include app/ sidecar directory for custom TypeScript handlers")
	force := fs.Bool("force", false, "Overwrite existing directory without prompt")
	fs.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage: formspec init [project-name] [flags]\n\n")
		fmt.Fprintf(os.Stderr, "Scaffold a new FormSpec project with the standard layout.\n\n")
		fmt.Fprintf(os.Stderr, "Without arguments, initializes the current directory.\n")
		fmt.Fprintf(os.Stderr, "With a project name, creates a new subdirectory.\n\n")
		fmt.Fprintf(os.Stderr, "The project includes:\n")
		fmt.Fprintf(os.Stderr, "  - Standard directory structure (spec/)\n")
		fmt.Fprintf(os.Stderr, "  - formspec-app.yaml configuration\n")
		fmt.Fprintf(os.Stderr, "  - schemas/ with JSON Schema for YAML editor validation\n")
		fmt.Fprintf(os.Stderr, "  - .vscode/settings.json registering yaml.schemas\n")
		fmt.Fprintf(os.Stderr, "  - .agents/skills/ with AI skills for coding agents\n")
		fmt.Fprintf(os.Stderr, "  - AGENTS.md (+ .github/copilot-instructions.md pointer)\n\n")
		fmt.Fprintf(os.Stderr, "Flags:\n")
		fs.PrintDefaults()
	}
	fs.Parse(args)

	cwd, err := os.Getwd()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: cannot determine current directory: %v\n", err)
		os.Exit(1)
	}

	var projectName string
	var targetDir string

	if fs.NArg() >= 1 {
		// formspec init my-project → create subdirectory
		projectName = fs.Arg(0)
		targetDir = filepath.Join(cwd, projectName)
	} else {
		// formspec init → initialize current directory
		projectName = filepath.Base(cwd)
		targetDir = cwd
	}

	modName := *moduleName
	if modName == "" {
		modName = toKebabCase(projectName)
	}

	// Check if directory exists (only for subdirectory case — cwd always exists)
	if targetDir != cwd {
		if info, err := os.Stat(targetDir); err == nil {
			if !info.IsDir() {
				fmt.Fprintf(os.Stderr, "Error: %s exists and is not a directory\n", projectName)
				os.Exit(1)
			}
			if !*force {
				fmt.Fprintf(os.Stderr, "Directory %s already exists. Use --force to overwrite.\n", projectName)
				os.Exit(1)
			}
			fmt.Fprintf(os.Stderr, "Overwriting existing directory %s (--force)\n", projectName)
		}
	}

	// ── Create directory structure ──────────────────────────────────
	dirs := []string{
		filepath.Join(targetDir, "spec", "apps"),
		filepath.Join(targetDir, "spec", "modules"),
		filepath.Join(targetDir, ".agents", "skills"),
		filepath.Join(targetDir, ".vscode"),
		filepath.Join(targetDir, ".github"),
	}

	if *withSidecar {
		dirs = append(dirs,
			filepath.Join(targetDir, "app", "src"),
		)
	}

	for _, d := range dirs {
		if err := os.MkdirAll(d, 0755); err != nil {
			fmt.Fprintf(os.Stderr, "Error: cannot create directory %s: %v\n", d, err)
			os.Exit(1)
		}
	}

	// ── Write files ─────────────────────────────────────────────────
	writeFile := func(path, content string) {
		content = strings.ReplaceAll(content, "{BT}", bt)
		fullPath := filepath.Join(targetDir, path)
		if err := os.WriteFile(fullPath, []byte(content), 0644); err != nil {
			fmt.Fprintf(os.Stderr, "Error: cannot write %s: %v\n", path, err)
			os.Exit(1)
		}
	}

	// formspec-app.yaml
	writeFile("formspec-app.yaml", fmt.Sprintf(`# ── FormSpec Dev Config ──
# This file configures {BT}formspec dev{BT}, {BT}formspec serve{BT}, and the
# schema registry. It is NOT a kind: Config manifest — it is CLI tooling config only.
#
# See docs/spec/platform/08-project-layout.md for the full reference.

spec: spec
dsn: sqlite:.formspec/%s.db
# schema-registry: https://schemas.formspec.dev   # override dengan FORMSPEC_SCHEMA_REGISTRY
# runtime: node          # uncomment if you have an app/ sidecar
# app-dir: app
# app-entrypoint: src/app.ts
# dev: true
`, modName))

	// .gitignore
	writeFile(".gitignore", `# FormSpec runtime data
.formspec/

# Binaries
bin/
obj/
*.exe

# Dependencies
node_modules/

# Build output
dist/
build/

# IDE
.idea/
*.swp
*.swo
*~

# OS
.DS_Store
Thumbs.db
`)

	// AGENTS.md — the single source of agent instructions (tool-agnostic).
	writeFile("AGENTS.md", makeAgentsInstructions(projectName))

	// .github/copilot-instructions.md — thin pointer for Copilot versions that
	// do not read AGENTS.md yet. Never clobber an existing file.
	ciPath := filepath.Join(targetDir, ".github", "copilot-instructions.md")
	if _, err := os.Stat(ciPath); os.IsNotExist(err) {
		writeFile(".github/copilot-instructions.md", fmt.Sprintf(
			"All agent instructions for this project live in {BT}AGENTS.md{BT}.\n"+
				"Read and follow {BT}AGENTS.md{BT} at the repository root.\n"))
	}

	// Write embedded AI skills to .agents/skills/
	fmt.Fprintf(os.Stderr, "Extracting AI skills...\n")
	if err := extractSkills(targetDir); err != nil {
		fmt.Fprintf(os.Stderr, "Error: cannot extract skills: %v\n", err)
		os.Exit(1)
	}

	// Fetch JSON Schema files from the registry into schemas/ (editor autocomplete).
	fmt.Fprintf(os.Stderr, "Fetching JSON Schema (registry)...\n")
	if err := fetchSchemas(targetDir); err != nil {
		fmt.Fprintf(os.Stderr, "  ⚠️  cannot fetch schemas: %v\n", err)
		fmt.Fprintf(os.Stderr, "     Project tetap ter-scaffold — jalankan ulang saat online atau\n")
		fmt.Fprintf(os.Stderr, "     gunakan \"formspec schema fetch v1\" untuk autocomplete editor (schemas/ dilewati).\n")
	}

	// Register schemas for the YAML editor (.vscode/settings.json).
	// Only write if missing — never clobber existing editor settings.
	settingsPath := filepath.Join(targetDir, ".vscode", "settings.json")
	if _, err := os.Stat(settingsPath); os.IsNotExist(err) {
		writeFile(".vscode/settings.json", `{
  "yaml.schemas": {
    "schemas/formspec.schema.json": ["spec/**/*.yaml", "spec/**/*.yml"]
  }
}
`)
		fmt.Fprintf(os.Stderr, "  ✓ .vscode/settings.json (yaml.schemas)\n")
	} else {
		fmt.Fprintf(os.Stderr, "  ⚠️  .vscode/settings.json already exists — add yaml.schemas manually:\n")
		fmt.Fprintf(os.Stderr, "     \"yaml.schemas\": {\"schemas/formspec.schema.json\": [\"spec/**/*.yaml\", \"spec/**/*.yml\"]}\n")
	}

	// Optional sidecar files
	if *withSidecar {
		writeFile("app/package.json", fmt.Sprintf(`{
  "name": "%s",
  "version": "1.0.0",
  "private": true,
  "type": "module",
  "scripts": {
    "dev": "tsx --watch src/app.ts",
    "build": "tsc",
    "start": "node dist/app.js"
  },
  "dependencies": {
    "@formspec/client": "*"
  },
  "devDependencies": {
    "tsx": "^4.0.0",
    "typescript": "^5.0.0"
  }
}
`, modName))

		writeFile("app/tsconfig.json", `{
  "compilerOptions": {
    "target": "ES2022",
    "module": "ESNext",
    "moduleResolution": "bundler",
    "outDir": "dist",
    "rootDir": "src",
    "strict": true,
    "esModuleInterop": true,
    "skipLibCheck": true
  },
  "include": ["src"]
}
`)

		writeFile("app/src/app.ts", `// FormSpec sidecar entrypoint.
// This file is loaded by the FormSpec runtime when formspec-app.yaml has:
//   runtime: node
//   app-entrypoint: src/app.ts
//
// Handlers registered here can be referenced from YAML manifests as:
//   handler: { type: sidecar, ref: "handlerName" }

import { formspec } from "@formspec/client";

// Example: register a custom action handler
// formspec.register("calculateTotal", async (ctx, input) => {
//   const { items } = input;
//   const total = items.reduce((sum, item) => sum + item.amount, 0);
//   return { total };
// });

console.log("FormSpec sidecar ready: " + formspec.moduleName);
`)
	}

	// ── Success ─────────────────────────────────────────────────────
	fmt.Println()
	if targetDir == cwd {
		fmt.Println("✅ FormSpec project initialized successfully!")
	} else {
		fmt.Printf("✅ FormSpec project '%s' created successfully!\n", projectName)
	}
	fmt.Println()
	fmt.Println("Project structure:")
	printTree(targetDir, "")
	fmt.Println()
	fmt.Println("Next steps:")
	fmt.Printf("  1. Open this folder in VS Code\n")
	fmt.Printf("  2. Run: formspec dev\n")
	fmt.Println()
	fmt.Println("Then ask your AI coding agent (Agent mode) to build your app — it")
	fmt.Println("follows the 4-phase workflow (Discovery -> Proposal -> Draft ->")
	fmt.Println("Iterate) defined in AGENTS.md + .agents/skills/, with")
	fmt.Println("formspec validate --spec spec as the gate:")
	fmt.Println("  > buat formspec app untuk inventory management")
	fmt.Println()
	fmt.Println("YAML editor:")
	fmt.Println("  schemas/ + .vscode/settings.json (yaml.schemas) are ready —")
	fmt.Println("  spec/**/*.yaml gets autocomplete + validation in VS Code.")
}

func makeAgentsInstructions(projectName string) string {
	return fmt.Sprintf(`# %s — AGENTS.md

## Project Overview

This is a **FormSpec** application. FormSpec is a spec-first, declarative ecosystem
for business applications. YAML manifests (apiVersion/kind/metadata/spec) are
the single source of truth for API, UI, permissions, state machines, and events.

## How to Build a FormSpec App — 4 Phases

Follow the **formspec-app-workflow** skill (in {BT}.agents/skills/{BT}) when
creating or changing this app. It orchestrates the full lifecycle:

1. **Discovery** — ask business questions in plain language, output
   {BT}docs/overview.md{BT}, get user approval
2. **Proposal** — map to modules + entities (characteristics, state machines),
   output {BT}docs/architecture.md{BT} + {BT}docs/domain-model.md{BT}, get approval
3. **Draft** — write YAML manifests in {BT}spec/{BT}, validate after every write
4. **Iterate** — classify changes, update docs top-down, write a changelog entry

**Never jump straight to YAML.** Confirm with the user between phases. Which
phase you are in is decided by the user's request + their approvals — not by
which files happen to exist.

## Skills Loaded

This project includes AI skills in {BT}.agents/skills/{BT}. In Copilot Chat,
use {BT}/skills{BT} to see them:

- **formspec-app-workflow** — Full lifecycle orchestrator (Discovery → Proposal → Draft → Iterate)
- **formspec-kinds** — Complete catalog of all 33 FormSpec resource kinds
- **formspec-spec-structure** — Navigate the FormSpec spec docs
- **schema-validation** — Run {BT}formspec validate{BT}, classify errors, repair manifests

## Stack

| Layer | Technology |
|-------|-----------|
| Backend | Go, module github.com/primadi/formspec |
| Frontend | React + TypeScript + Vite + shadcn/ui |
| Database | PostgreSQL (production) / SQLite (dev) |
| Scripting | Starlark (sandboxed, editable via admin panel) |
| Manifest | YAML (apiVersion/kind/metadata/spec) |

## Key Commands

| Command | Purpose |
|---------|---------|
| {BT}formspec validate --spec spec{BT} | **Validation gate** — must report {BT}0 problem(s) found{BT} |
| {BT}formspec dev{BT} | Start development server (API + UI) |
| {BT}formspec generate{BT} | Generate typed TypeScript client from Entity manifests |

**Rule: validate after every significant write.** Run
{BT}formspec validate --spec spec{BT} whenever you add or change a manifest, and
fix errors (use the schema-validation skill) before moving on.

## Conventions

1. **Manifest first** — always write YAML spec before implementation
2. **Module granularity** — one Module = one business bounded context
3. **Entity characteristics** — master (stable data), transaction (append-heavy),
   reference (read-only seed), summary (system-managed projection)
4. **Permissions** — permission = resource + action, never hardcode role names
5. **Use ctx.* primitives** — ctx.db, ctx.cache, ctx.lock, ctx.queue,
   ctx.pubsub, ctx.storage — never raw SQL
6. **Derived by default** — Entity auto-generates CRUD API + Table + Forms + Page
   (95%% of the time, {BT}kind: Entity{BT} is enough — no UI overrides needed)

## Project Layout

{BT}{BT}{BT}
%s/
  AGENTS.md              # Agent instructions (tool-agnostic)
  formspec-app.yaml      # CLI config (NOT a kind: Config manifest)
  spec/                # All YAML manifests
    apps/              # kind: App manifests
    modules/           # kind: Module -> Entity, Page, Form, etc.
  app/                 # Optional sidecar (only with --with-sidecar)
  .agents/skills/      # AI skills for coding agents
{BT}{BT}{BT}
`, projectName, projectName)
}

// extractSkills reads embedded AI skills from the binary and writes them
// to .agents/skills/ in the target project.
func extractSkills(targetDir string) error {
	return fs.WalkDir(formspec.AISkillsFS, ".", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}

		// path is like "ai_skills/formspec-kinds/SKILL.md"
		// We want to write to .agents/skills/formspec-kinds/SKILL.md
		relPath := strings.TrimPrefix(path, "ai_skills/")
		if relPath == path {
			return nil // not an ai_skills file
		}

		destPath := filepath.Join(targetDir, ".agents", "skills", relPath)
		if err := os.MkdirAll(filepath.Dir(destPath), 0755); err != nil {
			return fmt.Errorf("mkdir %s: %w", filepath.Dir(destPath), err)
		}

		data, err := fs.ReadFile(formspec.AISkillsFS, path)
		if err != nil {
			return fmt.Errorf("read embedded %s: %w", path, err)
		}

		if err := os.WriteFile(destPath, data, 0644); err != nil {
			return fmt.Errorf("write %s: %w", destPath, err)
		}

		fmt.Fprintf(os.Stderr, "  ✓ .agents/skills/%s\n", relPath)
		return nil
	})
}

// fetchSchemas fetches the v1 schema set from the registry into the target
// project's schemas/ dir, so the YAML editor gets autocomplete + validation
// (see .vscode/settings.json -> yaml.schemas). The registry client first caches
// the schemas locally (os.UserCacheDir()/formspec/schemas) so later runs — and
// `formspec validate` — work without network.
func fetchSchemas(targetDir string) error {
	reg := schemaregistry.New(schemaRegistryBaseURL())
	if err := reg.EnsureFull("v1", false); err != nil {
		return err
	}
	srcDir, err := reg.VersionDir("v1")
	if err != nil {
		return err
	}
	return copySchemas(srcDir, filepath.Join(targetDir, "schemas"))
}

// copySchemas copies formspec.schema.json + kinds/*.schema.json from srcDir
// into destDir, preserving the layout expected by .vscode/settings.json.
func copySchemas(srcDir, destDir string) error {
	files := []string{"formspec.schema.json"}
	kindEntries, err := os.ReadDir(filepath.Join(srcDir, "kinds"))
	if err != nil {
		return fmt.Errorf("read cached kinds: %w", err)
	}
	for _, e := range kindEntries {
		if !e.IsDir() && strings.HasSuffix(e.Name(), ".schema.json") {
			files = append(files, "kinds/"+e.Name())
		}
	}
	for _, rel := range files {
		src := filepath.Join(srcDir, rel)
		dst := filepath.Join(destDir, rel)
		if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
			return fmt.Errorf("mkdir %s: %w", filepath.Dir(dst), err)
		}
		data, err := os.ReadFile(src)
		if err != nil {
			return fmt.Errorf("read %s: %w", rel, err)
		}
		if err := os.WriteFile(dst, data, 0o644); err != nil {
			return fmt.Errorf("write %s: %w", rel, err)
		}
		fmt.Fprintf(os.Stderr, "  ✓ schemas/%s\n", rel)
	}
	return nil
}

// printTree prints a directory tree for display after scaffolding.
func printTree(root string, indent string) {
	entries, err := os.ReadDir(root)
	if err != nil {
		return
	}
	var dirs, files []os.DirEntry
	for _, e := range entries {
		if e.Name() == ".git" {
			continue
		}
		if e.IsDir() {
			dirs = append(dirs, e)
		} else {
			files = append(files, e)
		}
	}

	for i, d := range dirs {
		prefix := "├── "
		childIndent := "│   "
		if i == len(dirs)-1 && len(files) == 0 {
			prefix = "└── "
			childIndent = "    "
		}
		fmt.Printf("%s%s%s/\n", indent, prefix, d.Name())
		printTree(filepath.Join(root, d.Name()), indent+childIndent)
	}

	for i, f := range files {
		prefix := "├── "
		if i == len(files)-1 {
			prefix = "└── "
		}
		fmt.Printf("%s%s%s\n", indent, prefix, f.Name())
	}
}

// toKebabCase converts a string to kebab-case.
func toKebabCase(s string) string {
	s = strings.TrimSpace(s)
	s = strings.ReplaceAll(s, " ", "-")
	s = strings.ReplaceAll(s, "_", "-")
	s = strings.ToLower(s)
	for strings.Contains(s, "--") {
		s = strings.ReplaceAll(s, "--", "-")
	}
	return s
}
