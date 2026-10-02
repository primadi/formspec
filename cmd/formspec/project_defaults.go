// ─── Command defaults that agree with `formspec dev` ───
//
// Every data-lifecycle command used to carry its own literals:
//
//	specPath := "spec"
//	dsn := "sqlite:.formspec/data.db"
//	workspace := "demo"          // or spec.DefaultWorkspaceSlug
//
// Those are the values `formspec dev` uses when formspec-app.yaml does NOT
// exist (dev.go's defaultSpecPath/defaultDSN). So in any project that HAS the
// config file — i.e. every project created by `formspec init` — running one of
// these commands from the project directory silently targeted a different
// database than the server the developer was looking at.
//
// Measured 2026-09-29 in examples/kafe, whose config declares
// `spec: spec` + `dsn: sqlite:.formspec/kafe.db`:
//
//	formspec dev   → .formspec/kafe.db, workspace "kafe"
//	formspec repl  → .formspec/data.db, workspace "demo"     ← different file AND tenant
//
// This was not academic. The documented repair loop is
// `formspec repl --no-sync -f repair.star` followed by `formspec migrate apply`;
// with each command opening its own default DSN, the repair landed in data.db,
// the migration reconciled data.db, and the dev server kept serving the
// unrepaired kafe.db. The failure looks like "the repair did nothing".
//
// The resolution order below is dev's, deliberately and not approximately:
//
//  1. the config file fills in values the flags did not set (config is the
//     default, a flag wins);
//  2. a relative SQLite DSN is anchored to the project root (resolveDSN);
//  3. the active workspace follows rule #48 (activeWorkspaceFor).
//
// One home for that rule means the next command added inherits it instead of
// re-deriving it — which is how the divergence appeared in the first place.
package main

import (
	"log"
	"os"

	"gopkg.in/yaml.v3"

	"github.com/primadi/formspec/pkg/spec"
)

// Fallbacks used when there is no formspec-app.yaml. They are identical to
// dev.go's defaultSpecPath/defaultDSN and to spec.DefaultWorkspaceSlug, and are
// named here so a reader can see all three in one place.
const (
	fallbackSpecPath = "spec"
	fallbackDSN      = "sqlite:.formspec/data.db"
)

// projectDefaults is the (spec, dsn, workspace) triple a command starts from
// before its flags are parsed.
type projectDefaults struct {
	SpecPath    string
	DSN         string
	WorkspaceID string
	// WorkspaceExplicit reports that the workspace came from the config file
	// (`workspace-id:`) rather than from a fallback, so rule #48 treats it as a
	// choice instead of adopting the only declared workspace.
	WorkspaceExplicit bool
}

// loadProjectDefaults reads formspec-app.yaml from the working directory and
// falls back to the historical literals when it is absent or unreadable.
//
// It logs which config file it used, exactly as `formspec dev` does: a command
// that resolves to a different database than the developer expects has to be
// visible in its own output, not discoverable by querying PRAGMA later.
func loadProjectDefaults() projectDefaults {
	d := projectDefaults{
		SpecPath:    fallbackSpecPath,
		DSN:         fallbackDSN,
		WorkspaceID: spec.DefaultWorkspaceSlug,
	}

	path := findConfigFile()
	if path == "" {
		return d
	}
	data, err := os.ReadFile(path)
	if err != nil {
		log.Printf("[formspec] warning: cannot read config %s: %v", path, err)
		return d
	}
	var cf configFile
	if err := yaml.Unmarshal(data, &cf); err != nil {
		log.Printf("[formspec] warning: cannot parse config %s: %v", path, err)
		return d
	}
	log.Printf("[formspec] using config: %s", path)

	if cf.Spec != nil {
		d.SpecPath = *cf.Spec
	}
	if cf.DSN != nil {
		d.DSN = *cf.DSN
	}
	if cf.WorkspaceID != nil {
		d.WorkspaceID = *cf.WorkspaceID
		d.WorkspaceExplicit = true
	}
	return d
}

// finishProjectDefaults applies the two steps that must happen AFTER flag
// parsing: anchoring a relative SQLite DSN to the project root, and resolving
// the active workspace against the declared ones (#48).
//
// `workspaceExplicit` is true when the caller saw `--workspace`/`--workspace-id`
// on the command line; without that flag the workspace may be adopted from the
// spec tree, which is the whole point of #48.
func finishProjectDefaults(specPath, dsn, workspaceID string, workspaceExplicit bool) (string, string, string) {
	dsn = resolveDSN(dsn, specPath)
	workspaceID = activeWorkspaceFor(specPath, workspaceID, workspaceExplicit)
	return specPath, dsn, workspaceID
}
