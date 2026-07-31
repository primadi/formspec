// Command `forma init` scaffolds a new Forma project with the standard
// layout defined in docs/spec/platform/08-project-layout.md.
//
// It also extracts embedded AI skills (ai_skills/*) into .agents/skills/
// so that VS Code Copilot can assist with Forma app development.
//
// Usage:
//
//	forma init [project-name] [flags]
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

	forma "github.com/primadi/forma"
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
		fmt.Fprintf(os.Stderr, "Usage: forma init [project-name] [flags]\n\n")
		fmt.Fprintf(os.Stderr, "Scaffold a new Forma project with the standard layout.\n\n")
		fmt.Fprintf(os.Stderr, "Without arguments, initializes the current directory.\n")
		fmt.Fprintf(os.Stderr, "With a project name, creates a new subdirectory.\n\n")
		fmt.Fprintf(os.Stderr, "The project includes:\n")
		fmt.Fprintf(os.Stderr, "  - Standard directory structure (spec/)\n")
		fmt.Fprintf(os.Stderr, "  - forma-app.yaml configuration\n")
		fmt.Fprintf(os.Stderr, "  - .agents/skills/ with AI skills for VS Code Copilot\n")
		fmt.Fprintf(os.Stderr, "  - .github/copilot-instructions.md\n\n")
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
		// forma init my-project → create subdirectory
		projectName = fs.Arg(0)
		targetDir = filepath.Join(cwd, projectName)
	} else {
		// forma init → initialize current directory
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

	// forma-app.yaml
	writeFile("forma-app.yaml", fmt.Sprintf(`# ── Forma Dev Config ──
# This file configures {BT}forma dev{BT} and {BT}forma serve{BT}.
# It is NOT a kind: Config manifest — it is CLI tooling config only.
#
# See docs/spec/platform/08-project-layout.md for the full reference.

spec: spec
dsn: sqlite:.forma/%s.db
# runtime: node          # uncomment if you have an app/ sidecar
# app-dir: app
# app-entrypoint: src/app.ts
# dev: true
`, modName))

	// .gitignore
	writeFile(".gitignore", `# Forma runtime data
.forma/

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

	// .github/copilot-instructions.md
	writeFile(".github/copilot-instructions.md", makeCopilotInstructions(projectName))

	// Write embedded AI skills to .agents/skills/
	fmt.Fprintf(os.Stderr, "Extracting AI skills...\n")
	if err := extractSkills(targetDir); err != nil {
		fmt.Fprintf(os.Stderr, "Error: cannot extract skills: %v\n", err)
		os.Exit(1)
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
    "@forma/client": "*"
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

		writeFile("app/src/app.ts", `// Forma sidecar entrypoint.
// This file is loaded by the Forma runtime when forma-app.yaml has:
//   runtime: node
//   app-entrypoint: src/app.ts
//
// Handlers registered here can be referenced from YAML manifests as:
//   handler: { type: sidecar, ref: "handlerName" }

import { forma } from "@forma/client";

// Example: register a custom action handler
// forma.register("calculateTotal", async (ctx, input) => {
//   const { items } = input;
//   const total = items.reduce((sum, item) => sum + item.amount, 0);
//   return { total };
// });

console.log("Forma sidecar ready: " + forma.moduleName);
`)
	}

	// ── Success ─────────────────────────────────────────────────────
	fmt.Println()
	if targetDir == cwd {
		fmt.Println("✅ Forma project initialized successfully!")
	} else {
		fmt.Printf("✅ Forma project '%s' created successfully!\n", projectName)
	}
	fmt.Println()
	fmt.Println("Project structure:")
	printTree(targetDir, "")
	fmt.Println()
	fmt.Println("Next steps:")
	fmt.Printf("  1. Open this folder in VS Code\n")
	fmt.Printf("  2. Run: forma dev\n")
	fmt.Println()
	fmt.Println("Then ask Copilot (Agent mode) to create your app:")
	fmt.Println("  > buat forma app untuk inventory management")
}

func makeCopilotInstructions(projectName string) string {
	return fmt.Sprintf(`# %s — Copilot Instructions

## Project Overview

This is a **Forma** application. Forma is a spec-first, declarative ecosystem
for business applications. YAML manifests (apiVersion/kind/metadata/spec) are
the single source of truth for API, UI, permissions, state machines, and events.

## Skills Loaded

This project includes AI skills in {BT}.agents/skills/{BT}:

- **forma-spec-structure** — Navigate the Forma spec docs
- **forma-kinds** — Complete catalog of all Forma resource kinds

Use {BT}/skills{BT} in Copilot Chat to verify they are discovered. These skills give
the agent domain-specific knowledge about Forma kinds, manifest formats, and
spec documentation.

## Stack

| Layer | Technology |
|-------|-----------|
| Backend | Go, module github.com/primadi/forma |
| Frontend | React + TypeScript + Vite + shadcn/ui |
| Database | PostgreSQL (production) / SQLite (dev) |
| Scripting | Starlark (sandboxed, editable via admin panel) |
| Manifest | YAML (apiVersion/kind/metadata/spec) |

## Key Commands

| Command | Purpose |
|---------|---------|
| {BT}forma dev{BT} | Start development server (API + UI) |
| {BT}forma apply{BT} | Register YAML manifests |
| {BT}forma generate{BT} | Generate typed TypeScript client from Entity manifests |

## Conventions

1. **Manifest first** — always write YAML spec before implementation
2. **Module granularity** — one Module = one business bounded context
3. **Entity characteristics** — master (stable data), transaction (append-heavy),
   reference (read-only seed), summary (system-managed projection)
4. **Permissions** — permission = resource + action, never hardcode role names
5. **Use ctx.* primitives** — ctx.db, ctx.cache, ctx.lock, ctx.queue,
   ctx.pubsub, ctx.storage — never raw SQL
6. **Derived by default** — Entity auto-generates CRUD API + Table + Forms + Page

## Project Layout

{BT}{BT}{BT}
%s/
  forma-app.yaml       # CLI config (NOT a kind: Config manifest)
  spec/                # All YAML manifests
    apps/              # kind: App manifests
    modules/           # kind: Module -> Entity, Page, Form, etc.
  app/                 # Optional sidecar (only with --with-sidecar)
  .agents/skills/      # AI skills for Copilot
{BT}{BT}{BT}

## Creating a Forma App

When asked to create a Forma app:

1. **Identify the business domain** — what entities are needed?
2. **Choose Entity characteristics** — master, transaction, reference, or summary
3. **Write YAML manifests** in spec/modules/<module>/
4. **Organize by characteristic folders** — master/, transaction/, reference/, summary/
5. **Start with Entity** — 95%% of cases, the answer is kind: Entity
6. **Let derivation handle the rest** — Forms, Tables, Pages are auto-generated
`, projectName, projectName)
}

// extractSkills reads embedded AI skills from the binary and writes them
// to .agents/skills/ in the target project.
func extractSkills(targetDir string) error {
	return fs.WalkDir(forma.AISkillsFS, ".", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}

		// path is like "ai_skills/forma-kinds/SKILL.md"
		// We want to write to .agents/skills/forma-kinds/SKILL.md
		relPath := strings.TrimPrefix(path, "ai_skills/")
		if relPath == path {
			return nil // not an ai_skills file
		}

		destPath := filepath.Join(targetDir, ".agents", "skills", relPath)
		if err := os.MkdirAll(filepath.Dir(destPath), 0755); err != nil {
			return fmt.Errorf("mkdir %s: %w", filepath.Dir(destPath), err)
		}

		data, err := fs.ReadFile(forma.AISkillsFS, path)
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
