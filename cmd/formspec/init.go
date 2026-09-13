// Command `formspec init` scaffolds a new FormSpec project with the standard
// layout defined in docs/spec/platform/08-project-layout.md.
//
// Project files are rendered from the embedded template tree in
// cmd/formspec/template_init/ (see template_init.go) — {{placeholder}} tokens
// are substituted per project. It also extracts the embedded AI skills
// (ai_skills/*) into .agents/skills/ so that AI coding agents can assist with
// FormSpec app development, and writes .vscode/settings.json (yaml.schemas)
// pointing at the registry schema URL so the YAML editor gets autocomplete and
// validation without downloading a local schemas/ copy.
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
//	--module  Module name (default: project name, kebab-case)
//	--force   Overwrite existing directory without prompt
package main

import (
	"flag"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	formspec "github.com/primadi/formspec"
)

// schemaURL is the root FormSpec JSON Schema served by the schema registry.
// init wires it directly into .vscode/settings.json (yaml.schemas) — no
// local schemas/ download needed at scaffold time.
const schemaURL = "https://schemas.formspec.dev/v1/formspec.schema.json"

// docsURL is the published FormSpec documentation site; referenced from the
// scaffolded AGENTS.md and printed on success.
const docsURL = "https://docs.formspec.dev/"

func runInit(args []string) {
	fs := flag.NewFlagSet("init", flag.ExitOnError)
	moduleName := fs.String("module", "", "Module name (default: project name, kebab-case)")
	force := fs.Bool("force", false, "Overwrite existing directory without prompt")
	fs.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage: formspec init [project-name] [flags]\n\n")
		fmt.Fprintf(os.Stderr, "Scaffold a new FormSpec project with the standard layout.\n\n")
		fmt.Fprintf(os.Stderr, "Without arguments, initializes the current directory.\n")
		fmt.Fprintf(os.Stderr, "With a project name, creates a new subdirectory.\n\n")
		fmt.Fprintf(os.Stderr, "The project includes:\n")
		fmt.Fprintf(os.Stderr, "  - Standard directory structure (spec/)\n")
		fmt.Fprintf(os.Stderr, "  - formspec-app.yaml configuration\n")
		fmt.Fprintf(os.Stderr, "  - spec/apps/<module>.yaml — kind: App scaffold (with default confirm dialogs)\n")
		fmt.Fprintf(os.Stderr, "  - spec/workspaces/<module>.yaml — kind: Workspace seed\n")
		fmt.Fprintf(os.Stderr, "  - .vscode/settings.json registering yaml.schemas\n")
		fmt.Fprintf(os.Stderr, "    → yaml.schemas points to %s (no local schemas/ copy)\n", schemaURL)
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
		filepath.Join(targetDir, "spec", "workspaces"),
		filepath.Join(targetDir, ".agents", "skills"),
		filepath.Join(targetDir, ".vscode"),
		filepath.Join(targetDir, ".github"),
	}

	for _, d := range dirs {
		if err := os.MkdirAll(d, 0755); err != nil {
			fmt.Fprintf(os.Stderr, "Error: cannot create directory %s: %v\n", d, err)
			os.Exit(1)
		}
	}

	// ── Write files ─────────────────────────────────────────────────
	// All project files are rendered from the embedded template tree in
	// cmd/formspec/template_init/ (see template_init.go); {{placeholder}}
	// tokens are substituted per project. The AI skills are extracted
	// separately from ai_skills/ below.
	settingsPath := filepath.Join(targetDir, ".vscode", "settings.json")
	settingsExisted := fileExists(settingsPath)

	if err := extractTemplates(targetDir, templateData{
		ProjectName: projectName,
		Module:      modName,
		SchemaURL:   schemaURL,
	}); err != nil {
		fmt.Fprintf(os.Stderr, "Error: cannot write project files: %v\n", err)
		os.Exit(1)
	}

	// Write embedded AI skills to .agents/skills/
	fmt.Fprintf(os.Stderr, "Extracting AI skills...\n")
	if err := extractSkills(targetDir); err != nil {
		fmt.Fprintf(os.Stderr, "Error: cannot extract skills: %v\n", err)
		os.Exit(1)
	}

	// .vscode/settings.json is never clobbered (extractTemplates skips it when
	// present) — report the manual step in that case.
	if settingsExisted {
		fmt.Fprintf(os.Stderr, "  ⚠️  .vscode/settings.json already exists — add yaml.schemas manually:\n")
		fmt.Fprintf(os.Stderr, "     \"yaml.schemas\": {\"%s\": [\"spec/**/*.yaml\", \"spec/**/*.yml\"]}\n", schemaURL)
	} else {
		fmt.Fprintf(os.Stderr, "  ✓ .vscode/settings.json (yaml.schemas → %s)\n", schemaURL)
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
	fmt.Println("  .vscode/settings.json (yaml.schemas) is ready — spec/**/*.yaml gets")
	fmt.Println("  autocomplete + validation in VS Code via " + schemaURL + ".")
	fmt.Println()
	fmt.Println("Docs:")
	fmt.Println("  Kind reference + contracts: " + docsURL)
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
