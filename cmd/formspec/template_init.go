// Project scaffold templates for `formspec init`.
//
// The committed template_init/ directory is the single source of truth for the
// files `formspec init` writes. Template files contain {{placeholder}} tokens
// that are substituted per project; the tree is embedded verbatim so the
// templates are ordinary files (editable, diffable) rather than Go string
// literals.
package main

import (
	"embed"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

// templateInitFS embeds the scaffold tree. The `all:` prefix is required so
// dot-prefixed entries (.gitignore, .github/, .vscode/) are included — plain
// patterns skip files whose names begin with "." or "_".
//
//go:embed all:template_init
var templateInitFS embed.FS

// templateRoot is the embed path prefix stripped when writing to disk.
const templateRoot = "template_init"

// templateData holds the values substituted into {{placeholder}} tokens.
type templateData struct {
	ProjectName string // project name as given on the CLI (may contain spaces)
	Module      string // kebab-case module name — metadata.name of App/Workspace
	SchemaURL   string // JSON Schema registry URL for .vscode/settings.json
}

// templateModuleNamed lists template file names renamed to <module>.yaml on
// extraction — the App and Workspace manifests are named after the module.
var templateModuleNamed = map[string]bool{
	"app.tmpl.yaml": true,
	"ws.tmpl.yaml":  true,
}

// templateSkipIfExists lists project-relative files that must never overwrite
// an existing file in the target project (respect the user's own settings).
var templateSkipIfExists = map[string]bool{
	".github/copilot-instructions.md": true,
	".vscode/settings.json":           true,
}

// extractTemplates walks the embedded template tree and writes every file into
// targetDir, applying rename rules (module-named manifests), placeholder
// substitution, and never-clobber protection.
func extractTemplates(targetDir string, data templateData) error {
	return fs.WalkDir(templateInitFS, templateRoot, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}

		rel := strings.TrimPrefix(p, templateRoot+"/")

		if templateModuleNamed[filepath.Base(rel)] {
			rel = filepath.Join(filepath.Dir(rel), data.Module+".yaml")
		}

		dest := filepath.Join(targetDir, filepath.FromSlash(rel))
		if templateSkipIfExists[rel] && fileExists(dest) {
			return nil // never clobber an existing project file
		}

		raw, err := fs.ReadFile(templateInitFS, p)
		if err != nil {
			return fmt.Errorf("read embedded %s: %w", p, err)
		}

		if err := os.MkdirAll(filepath.Dir(dest), 0755); err != nil {
			return fmt.Errorf("mkdir %s: %w", filepath.Dir(dest), err)
		}
		if err := os.WriteFile(dest, []byte(substituteTemplate(string(raw), data)), 0644); err != nil {
			return fmt.Errorf("write %s: %w", dest, err)
		}
		return nil
	})
}

// substituteTemplate replaces {{placeholder}} tokens with their values.
func substituteTemplate(content string, data templateData) string {
	return strings.NewReplacer(
		"{{projectName}}", data.ProjectName,
		"{{module}}", data.Module,
		"{{schemaURL}}", data.SchemaURL,
	).Replace(content)
}

// fileExists reports whether path exists (as a file or directory).
func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}
